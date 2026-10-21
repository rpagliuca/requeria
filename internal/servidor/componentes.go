package servidor

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rpagliuca/rex/api"
)

func idComponente(n int64) string { return idCom("C", n) }

// idsLigados lê uma coluna de ids numa consulta e os formata.
func idsLigados(c consultor, formato func(int64) string, consulta string, args ...any) ([]string, error) {
	linhas, err := c.Query(consulta, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	ids := []string{}
	for linhas.Next() {
		var n int64
		if err := linhas.Scan(&n); err != nil {
			return nil, err
		}
		ids = append(ids, formato(n))
	}
	return ids, linhas.Err()
}

func idsDeTexto(c consultor, consulta string, args ...any) ([]string, error) {
	linhas, err := c.Query(consulta, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	valores := []string{}
	for linhas.Next() {
		var v string
		if err := linhas.Scan(&v); err != nil {
			return nil, err
		}
		valores = append(valores, v)
	}
	return valores, linhas.Err()
}

// gravarOrganizacoes substitui as organizações do componente; pelo menos uma.
func gravarOrganizacoes(tx *sql.Tx, n int64, orgs []string) error {
	var limpas []string
	for _, o := range orgs {
		if !vazio(o) {
			limpas = append(limpas, strings.TrimSpace(o))
		}
	}
	if len(limpas) == 0 {
		return falha(http.StatusBadRequest, "o componente precisa de pelo menos uma organização")
	}
	if _, err := tx.Exec(`DELETE FROM componente_organizacao WHERE componente_id = ?`, n); err != nil {
		return err
	}
	for _, o := range limpas {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO componente_organizacao (componente_id, organizacao) VALUES (?, ?)`, n, o); err != nil {
			return err
		}
	}
	return nil
}

func carregarComponente(c consultor, n int64) (api.Componente, error) {
	var x api.Componente
	var criado string
	err := c.QueryRow(`SELECT nome, descricao, criado_em FROM componentes WHERE id = ?`, n).Scan(&x.Nome, &x.Descricao, &criado)
	if errors.Is(err, sql.ErrNoRows) {
		return x, falha(http.StatusNotFound, "componente %s não encontrado", idComponente(n))
	}
	if err != nil {
		return x, err
	}
	x.Id, x.CriadoEm = idComponente(n), lerMomento(criado)
	if x.Organizacoes, err = idsDeTexto(c, `SELECT organizacao FROM componente_organizacao WHERE componente_id = ? ORDER BY organizacao`, n); err != nil {
		return x, err
	}
	if x.Requisitos, err = idsLigados(c, idRequisito, `SELECT requisito_id FROM requisito_componente WHERE componente_id = ? ORDER BY requisito_id`, n); err != nil {
		return x, err
	}
	x.Tarefas, err = idsLigados(c, idTarefa, `SELECT tarefa_id FROM tarefa_componente WHERE componente_id = ? ORDER BY tarefa_id`, n)
	return x, err
}

func nomeDeComponenteEmUso(err error, nome string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return falha(http.StatusConflict, "já existe um componente chamado %q", nome)
	}
	return err
}

func (s *Servidor) ListarComponentes(w http.ResponseWriter, r *http.Request) {
	ids, err := idsLigados(s.db, func(n int64) string { return idComponente(n) }, `SELECT id FROM componentes ORDER BY nome`)
	if err != nil {
		responderErro(w, err)
		return
	}
	componentes := []api.Componente{}
	for _, id := range ids {
		x, err := carregarComponente(s.db, mustNumero(id, "C"))
		if err != nil {
			responderErro(w, err)
			return
		}
		componentes = append(componentes, x)
	}
	responder(w, http.StatusOK, componentes)
}

func (s *Servidor) CriarComponente(w http.ResponseWriter, r *http.Request) {
	var novo api.NovoComponente
	if err := ler(r, &novo); err != nil {
		responderErro(w, err)
		return
	}
	var criado api.Componente
	err := s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if vazio(novo.Nome) {
			return falha(http.StatusBadRequest, "o nome é obrigatório")
		}
		descricao := ""
		if novo.Descricao != nil {
			descricao = *novo.Descricao
		}
		res, err := tx.Exec(`INSERT INTO componentes (nome, descricao, criado_em) VALUES (?, ?, ?)`,
			strings.TrimSpace(novo.Nome), descricao, s.momento())
		if err != nil {
			return nomeDeComponenteEmUso(err, novo.Nome)
		}
		n, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := gravarOrganizacoes(tx, n, novo.Organizacoes); err != nil {
			return err
		}
		if criado, err = carregarComponente(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, criado.Id, "componente_criado", map[string]any{"nome": criado.Nome})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criado)
}

func (s *Servidor) ObterComponente(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "C")
	if err != nil {
		responderErro(w, err)
		return
	}
	x, err := carregarComponente(s.db, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

func (s *Servidor) AlterarComponente(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "C")
	if err != nil {
		responderErro(w, err)
		return
	}
	var alteracao api.AlteracaoComponente
	if err := ler(r, &alteracao); err != nil {
		responderErro(w, err)
		return
	}
	var alterado api.Componente
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if _, err := carregarComponente(tx, n); err != nil {
			return err
		}
		campos := map[string]any{}
		if alteracao.Nome != nil {
			if vazio(*alteracao.Nome) {
				return falha(http.StatusBadRequest, "o nome não pode ficar vazio")
			}
			campos["nome"] = strings.TrimSpace(*alteracao.Nome)
		}
		if alteracao.Descricao != nil {
			campos["descricao"] = *alteracao.Descricao
		}
		if len(campos) == 0 && alteracao.Organizacoes == nil {
			return falha(http.StatusBadRequest, "nenhum campo para alterar")
		}
		for coluna, valor := range campos {
			if _, err := tx.Exec(`UPDATE componentes SET `+coluna+` = ? WHERE id = ?`, valor, n); err != nil {
				return nomeDeComponenteEmUso(err, fmt.Sprint(campos["nome"]))
			}
		}
		if alteracao.Organizacoes != nil {
			if err := gravarOrganizacoes(tx, n, *alteracao.Organizacoes); err != nil {
				return err
			}
			campos["organizacoes"] = *alteracao.Organizacoes
		}
		if alterado, err = carregarComponente(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, alterado.Id, "componente_alterado", campos)
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, alterado)
}

// mudarComponente liga ou desliga um componente de um requisito (tabela requisito_componente) ou
// de uma tarefa (tarefa_componente) e registra o evento na entidade.
func (s *Servidor) mudarComponente(tx *sql.Tx, r *http.Request, tabela, coluna, entidade string, n int64, componente string, vincular bool) error {
	quem, err := autor(tx, r)
	if err != nil {
		return err
	}
	c, err := numero(componente, "C")
	if err != nil {
		return err
	}
	if _, err := carregarComponente(tx, c); err != nil {
		return err
	}
	tipo := "componente_vinculado"
	if vincular {
		_, err = tx.Exec(`INSERT OR IGNORE INTO `+tabela+` (`+coluna+`, componente_id) VALUES (?, ?)`, n, c)
	} else {
		tipo = "componente_desvinculado"
		_, err = tx.Exec(`DELETE FROM `+tabela+` WHERE `+coluna+` = ? AND componente_id = ?`, n, c)
	}
	if err != nil {
		return err
	}
	return s.registrarEvento(tx, quem, entidade, tipo, map[string]any{"componente": idComponente(c)})
}

func (s *Servidor) componenteDoRequisito(w http.ResponseWriter, r *http.Request, id, componente string, vincular bool) {
	n, err := numero(id, "R")
	if err != nil {
		responderErro(w, err)
		return
	}
	var x api.Requisito
	err = s.emTransacao(func(tx *sql.Tx) error {
		if err := existeRequisito(tx, n); err != nil {
			return err
		}
		if err := s.mudarComponente(tx, r, "requisito_componente", "requisito_id", idRequisito(n), n, componente, vincular); err != nil {
			return err
		}
		x, err = carregarRequisito(tx, n)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

func (s *Servidor) componenteDaTarefa(w http.ResponseWriter, r *http.Request, id, componente string, vincular bool) {
	n, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	var x api.Tarefa
	err = s.emTransacao(func(tx *sql.Tx) error {
		if _, err := carregarTarefa(tx, n); err != nil {
			return err
		}
		if err := s.mudarComponente(tx, r, "tarefa_componente", "tarefa_id", idTarefa(n), n, componente, vincular); err != nil {
			return err
		}
		x, err = carregarTarefa(tx, n)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

func (s *Servidor) VincularComponenteARequisito(w http.ResponseWriter, r *http.Request, id api.Id) {
	var v api.VinculoComponente
	if err := ler(r, &v); err != nil {
		responderErro(w, err)
		return
	}
	s.componenteDoRequisito(w, r, id, v.Componente, true)
}

func (s *Servidor) DesvincularComponenteDeRequisito(w http.ResponseWriter, r *http.Request, id api.Id, componente string) {
	s.componenteDoRequisito(w, r, id, componente, false)
}

func (s *Servidor) VincularComponenteATarefa(w http.ResponseWriter, r *http.Request, id api.Id) {
	var v api.VinculoComponente
	if err := ler(r, &v); err != nil {
		responderErro(w, err)
		return
	}
	s.componenteDaTarefa(w, r, id, v.Componente, true)
}

func (s *Servidor) DesvincularComponenteDeTarefa(w http.ResponseWriter, r *http.Request, id api.Id, componente string) {
	s.componenteDaTarefa(w, r, id, componente, false)
}
