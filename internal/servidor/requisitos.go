package servidor

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rpagliuca/requerial/api"
)

func idOrigem(n int64) string    { return idCom("O", n) }
func idRequisito(n int64) string { return idCom("R", n) }

// ---- Origens ----

func lerOrigem(linha varredor) (api.Origem, error) {
	var o api.Origem
	var n int64
	var tipo, criada string
	if err := linha.Scan(&n, &o.Nome, &tipo, &o.Descricao, &criada); err != nil {
		return o, err
	}
	o.Id, o.Tipo, o.CriadaEm = idOrigem(n), api.TipoOrigem(tipo), lerMomento(criada)
	return o, nil
}

func carregarOrigem(c consultor, n int64) (api.Origem, error) {
	o, err := lerOrigem(c.QueryRow(`SELECT id, nome, tipo, descricao, criada_em FROM origens WHERE id = ?`, n))
	if errors.Is(err, sql.ErrNoRows) {
		return o, falha(http.StatusNotFound, "origem %s não encontrada", idOrigem(n))
	}
	return o, err
}

func tipoDeOrigemValido(t api.TipoOrigem) bool { return t == api.Pessoa || t == api.Documento }

// nomeEmUso traduz a violação de UNIQUE do nome numa mensagem clara.
func nomeEmUso(err error, nome string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return falha(http.StatusConflict, "já existe uma origem chamada %q", nome)
	}
	return err
}

func (s *Servidor) ListarOrigens(w http.ResponseWriter, r *http.Request) {
	linhas, err := s.db.Query(`SELECT id, nome, tipo, descricao, criada_em FROM origens ORDER BY nome`)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	origens := []api.Origem{}
	for linhas.Next() {
		o, err := lerOrigem(linhas)
		if err != nil {
			responderErro(w, err)
			return
		}
		origens = append(origens, o)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, origens)
}

func (s *Servidor) CriarOrigem(w http.ResponseWriter, r *http.Request) {
	var nova api.NovaOrigem
	if err := ler(r, &nova); err != nil {
		responderErro(w, err)
		return
	}
	var criada api.Origem
	err := s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if vazio(nova.Nome) {
			return falha(http.StatusBadRequest, "o nome é obrigatório")
		}
		if !tipoDeOrigemValido(nova.Tipo) {
			return falha(http.StatusBadRequest, "tipo de origem inválido: %q (pessoa ou documento)", nova.Tipo)
		}
		descricao := ""
		if nova.Descricao != nil {
			descricao = *nova.Descricao
		}
		res, err := tx.Exec(`INSERT INTO origens (nome, tipo, descricao, criada_em) VALUES (?, ?, ?, ?)`,
			strings.TrimSpace(nova.Nome), string(nova.Tipo), descricao, s.momento())
		if err != nil {
			return nomeEmUso(err, nova.Nome)
		}
		n, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if criada, err = carregarOrigem(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, criada.Id, "origem_criada", map[string]any{"nome": criada.Nome, "tipo": criada.Tipo})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criada)
}

func (s *Servidor) ObterOrigem(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "O")
	if err != nil {
		responderErro(w, err)
		return
	}
	o, err := carregarOrigem(s.db, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, o)
}

func (s *Servidor) AlterarOrigem(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "O")
	if err != nil {
		responderErro(w, err)
		return
	}
	var alteracao api.AlteracaoOrigem
	if err := ler(r, &alteracao); err != nil {
		responderErro(w, err)
		return
	}
	var alterada api.Origem
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if _, err := carregarOrigem(tx, n); err != nil {
			return err
		}
		campos := map[string]any{}
		if alteracao.Nome != nil {
			if vazio(*alteracao.Nome) {
				return falha(http.StatusBadRequest, "o nome não pode ficar vazio")
			}
			campos["nome"] = strings.TrimSpace(*alteracao.Nome)
		}
		if alteracao.Tipo != nil {
			if !tipoDeOrigemValido(*alteracao.Tipo) {
				return falha(http.StatusBadRequest, "tipo de origem inválido: %q", *alteracao.Tipo)
			}
			campos["tipo"] = string(*alteracao.Tipo)
		}
		if alteracao.Descricao != nil {
			campos["descricao"] = *alteracao.Descricao
		}
		if len(campos) == 0 {
			return falha(http.StatusBadRequest, "nenhum campo para alterar")
		}
		for coluna, valor := range campos {
			if _, err := tx.Exec(`UPDATE origens SET `+coluna+` = ? WHERE id = ?`, valor, n); err != nil {
				return nomeEmUso(err, fmt.Sprint(campos["nome"]))
			}
		}
		if alterada, err = carregarOrigem(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, alterada.Id, "origem_alterada", campos)
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, alterada)
}

// ---- Requisitos ----

// ligacoes devolve, para cada requisito, os ids de pais, filhos e tarefas, numa leitura só de cada tabela.
type ligacoes struct {
	pais, filhos, tarefas map[int64][]string
}

func carregarLigacoes(c consultor) (ligacoes, error) {
	l := ligacoes{pais: map[int64][]string{}, filhos: map[int64][]string{}, tarefas: map[int64][]string{}}
	linhas, err := c.Query(`SELECT requisito_id, pai_id FROM derivacoes ORDER BY pai_id, requisito_id`)
	if err != nil {
		return l, err
	}
	for linhas.Next() {
		var filho, pai int64
		if err := linhas.Scan(&filho, &pai); err != nil {
			linhas.Close()
			return l, err
		}
		l.pais[filho] = append(l.pais[filho], idRequisito(pai))
		l.filhos[pai] = append(l.filhos[pai], idRequisito(filho))
	}
	if err := linhas.Close(); err != nil {
		return l, err
	}
	linhas, err = c.Query(`SELECT requisito_id, tarefa_id FROM tarefa_requisito ORDER BY tarefa_id`)
	if err != nil {
		return l, err
	}
	defer linhas.Close()
	for linhas.Next() {
		var requisito, tarefa int64
		if err := linhas.Scan(&requisito, &tarefa); err != nil {
			return l, err
		}
		l.tarefas[requisito] = append(l.tarefas[requisito], idTarefa(tarefa))
	}
	return l, linhas.Err()
}

func naoNulo(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

const colunasRequisito = `id, titulo, descricao, organizacao, origem_id, criado_em, atualizado_em`

func lerRequisito(linha varredor, l ligacoes) (api.Requisito, error) {
	var x api.Requisito
	var n int64
	var origem sql.NullInt64
	var criado, atualizado string
	if err := linha.Scan(&n, &x.Titulo, &x.Descricao, &x.Organizacao, &origem, &criado, &atualizado); err != nil {
		return x, err
	}
	x.Id, x.CriadoEm, x.AtualizadoEm = idRequisito(n), lerMomento(criado), lerMomento(atualizado)
	if origem.Valid {
		o := idOrigem(origem.Int64)
		x.Origem = &o
	}
	x.DerivaDe, x.Derivados, x.Tarefas = naoNulo(l.pais[n]), naoNulo(l.filhos[n]), naoNulo(l.tarefas[n])
	return x, nil
}

func carregarRequisito(c consultor, n int64) (api.Requisito, error) {
	l, err := carregarLigacoes(c)
	if err != nil {
		return api.Requisito{}, err
	}
	x, err := lerRequisito(c.QueryRow(`SELECT `+colunasRequisito+` FROM requisitos WHERE id = ?`, n), l)
	if errors.Is(err, sql.ErrNoRows) {
		return x, falha(http.StatusNotFound, "requisito %s não encontrado", idRequisito(n))
	}
	return x, err
}

func existeRequisito(c consultor, n int64) error {
	var um int
	err := c.QueryRow(`SELECT 1 FROM requisitos WHERE id = ?`, n).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		return falha(http.StatusNotFound, "requisito %s não encontrado", idRequisito(n))
	}
	return err
}

// derivar liga filho → pai, recusando o que fecharia um ciclo: o pai não pode já descender do filho.
func derivar(tx *sql.Tx, filho, pai int64) error {
	if filho == pai {
		return falha(http.StatusBadRequest, "um requisito não deriva de si mesmo")
	}
	if err := existeRequisito(tx, pai); err != nil {
		return err
	}
	var ciclo int
	err := tx.QueryRow(`WITH RECURSIVE abaixo(id) AS (
			SELECT ? UNION SELECT d.requisito_id FROM derivacoes d JOIN abaixo a ON d.pai_id = a.id)
		SELECT count(*) FROM abaixo WHERE id = ?`, filho, pai).Scan(&ciclo)
	if err != nil {
		return err
	}
	if ciclo > 0 {
		return falha(http.StatusConflict, "%s já deriva de %s; a derivação fecharia um ciclo", idRequisito(pai), idRequisito(filho))
	}
	_, err = tx.Exec(`INSERT OR IGNORE INTO derivacoes (requisito_id, pai_id) VALUES (?, ?)`, filho, pai)
	return err
}

func (s *Servidor) ListarRequisitos(w http.ResponseWriter, r *http.Request, params api.ListarRequisitosParams) {
	consulta := `SELECT ` + colunasRequisito + ` FROM requisitos WHERE 1 = 1`
	var args []any
	if params.Organizacao != nil {
		consulta += ` AND organizacao = ?`
		args = append(args, *params.Organizacao)
	}
	if params.Origem != nil {
		n, err := numero(*params.Origem, "O")
		if err != nil {
			responderErro(w, err)
			return
		}
		consulta += ` AND origem_id = ?`
		args = append(args, n)
	}
	l, err := carregarLigacoes(s.db)
	if err != nil {
		responderErro(w, err)
		return
	}
	linhas, err := s.db.Query(consulta+` ORDER BY id`, args...)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	requisitos := []api.Requisito{}
	for linhas.Next() {
		x, err := lerRequisito(linhas, l)
		if err != nil {
			responderErro(w, err)
			return
		}
		requisitos = append(requisitos, x)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, requisitos)
}

// origemOpcional converte "O-0001" (ou vazio, que remove) no valor da coluna.
func origemOpcional(c consultor, id string) (any, error) {
	if vazio(id) {
		return nil, nil
	}
	n, err := numero(id, "O")
	if err != nil {
		return nil, err
	}
	if _, err := carregarOrigem(c, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *Servidor) CriarRequisito(w http.ResponseWriter, r *http.Request) {
	var novo api.NovoRequisito
	if err := ler(r, &novo); err != nil {
		responderErro(w, err)
		return
	}
	var criado api.Requisito
	err := s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if vazio(novo.Titulo) {
			return falha(http.StatusBadRequest, "o título é obrigatório")
		}
		var pais []int64
		if novo.DerivaDe != nil {
			for _, id := range *novo.DerivaDe {
				n, err := numero(id, "R")
				if err != nil {
					return err
				}
				if err := existeRequisito(tx, n); err != nil {
					return err
				}
				pais = append(pais, n)
			}
		}
		organizacao := ""
		if novo.Organizacao != nil {
			organizacao = strings.TrimSpace(*novo.Organizacao)
		}
		if organizacao == "" && len(pais) > 0 {
			// Padrão herdado do pai.
			if err := tx.QueryRow(`SELECT organizacao FROM requisitos WHERE id = ?`, pais[0]).Scan(&organizacao); err != nil {
				return err
			}
		}
		if organizacao == "" {
			return falha(http.StatusBadRequest, "a organização é obrigatória (ou derive de um requisito para herdá-la)")
		}
		var origem any
		if novo.Origem != nil {
			if origem, err = origemOpcional(tx, *novo.Origem); err != nil {
				return err
			}
		}
		descricao := ""
		if novo.Descricao != nil {
			descricao = *novo.Descricao
		}
		agora := s.momento()
		res, err := tx.Exec(`INSERT INTO requisitos (titulo, descricao, organizacao, origem_id, criado_em, atualizado_em)
			VALUES (?, ?, ?, ?, ?, ?)`, strings.TrimSpace(novo.Titulo), descricao, organizacao, origem, agora, agora)
		if err != nil {
			return err
		}
		n, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, pai := range pais {
			if err := derivar(tx, n, pai); err != nil {
				return err
			}
		}
		if criado, err = carregarRequisito(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, criado.Id, "requisito_criado", map[string]any{
			"titulo": criado.Titulo, "organizacao": criado.Organizacao, "origem": criado.Origem, "deriva_de": criado.DerivaDe,
		})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criado)
}

func (s *Servidor) ObterRequisito(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "R")
	if err != nil {
		responderErro(w, err)
		return
	}
	x, err := carregarRequisito(s.db, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

func (s *Servidor) AlterarRequisito(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "R")
	if err != nil {
		responderErro(w, err)
		return
	}
	var alteracao api.AlteracaoRequisito
	if err := ler(r, &alteracao); err != nil {
		responderErro(w, err)
		return
	}
	var alterado api.Requisito
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if err := existeRequisito(tx, n); err != nil {
			return err
		}
		campos := map[string]any{}
		var sets []string
		var args []any
		definir := func(coluna string, valor any, registro any) {
			sets = append(sets, coluna+" = ?")
			args = append(args, valor)
			campos[coluna] = registro
		}
		if alteracao.Titulo != nil {
			if vazio(*alteracao.Titulo) {
				return falha(http.StatusBadRequest, "o título não pode ficar vazio")
			}
			definir("titulo", strings.TrimSpace(*alteracao.Titulo), *alteracao.Titulo)
		}
		if alteracao.Descricao != nil {
			definir("descricao", *alteracao.Descricao, *alteracao.Descricao)
		}
		if alteracao.Organizacao != nil {
			if vazio(*alteracao.Organizacao) {
				return falha(http.StatusBadRequest, "a organização não pode ficar vazia")
			}
			definir("organizacao", strings.TrimSpace(*alteracao.Organizacao), *alteracao.Organizacao)
		}
		if alteracao.Origem != nil {
			origem, err := origemOpcional(tx, *alteracao.Origem)
			if err != nil {
				return err
			}
			definir("origem_id", origem, *alteracao.Origem)
		}
		if len(sets) == 0 {
			return falha(http.StatusBadRequest, "nenhum campo para alterar")
		}
		sets = append(sets, "atualizado_em = ?")
		args = append(args, s.momento(), n)
		if _, err := tx.Exec(`UPDATE requisitos SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return err
		}
		if alterado, err = carregarRequisito(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, alterado.Id, "requisito_alterado", campos)
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, alterado)
}

// mudarCadeia roda uma alteração de derivação no requisito e devolve o requisito atualizado.
func (s *Servidor) mudarCadeia(w http.ResponseWriter, r *http.Request, id string, corpo any,
	f func(tx *sql.Tx, n int64, quem string) error) {
	n, err := numero(id, "R")
	if err != nil {
		responderErro(w, err)
		return
	}
	if corpo != nil {
		if err := ler(r, corpo); err != nil {
			responderErro(w, err)
			return
		}
	}
	var x api.Requisito
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if err := existeRequisito(tx, n); err != nil {
			return err
		}
		if err := f(tx, n, quem); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requisitos SET atualizado_em = ? WHERE id = ?`, s.momento(), n); err != nil {
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

func (s *Servidor) DerivarRequisito(w http.ResponseWriter, r *http.Request, id api.Id) {
	var d api.Derivacao
	s.mudarCadeia(w, r, id, &d, func(tx *sql.Tx, n int64, quem string) error {
		pai, err := numero(d.Pai, "R")
		if err != nil {
			return err
		}
		if err := derivar(tx, n, pai); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, idRequisito(n), "derivacao_registrada", map[string]any{"pai": idRequisito(pai)})
	})
}

func (s *Servidor) DesfazerDerivacao(w http.ResponseWriter, r *http.Request, id api.Id, paiID string) {
	s.mudarCadeia(w, r, id, nil, func(tx *sql.Tx, n int64, quem string) error {
		pai, err := numero(paiID, "R")
		if err != nil {
			return err
		}
		res, err := tx.Exec(`DELETE FROM derivacoes WHERE requisito_id = ? AND pai_id = ?`, n, pai)
		if err != nil {
			return err
		}
		if afetadas, _ := res.RowsAffected(); afetadas == 0 {
			return falha(http.StatusNotFound, "%s não deriva de %s", idRequisito(n), idRequisito(pai))
		}
		return s.registrarEvento(tx, quem, idRequisito(n), "derivacao_desfeita", map[string]any{"pai": idRequisito(pai)})
	})
}

// ---- Tarefa ↔ Requisito ----

func requisitosDaTarefa(c consultor, tarefa int64) ([]string, error) {
	linhas, err := c.Query(`SELECT requisito_id FROM tarefa_requisito WHERE tarefa_id = ? ORDER BY requisito_id`, tarefa)
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
		ids = append(ids, idRequisito(n))
	}
	return ids, linhas.Err()
}

func (s *Servidor) mudarVinculoDeRequisito(w http.ResponseWriter, r *http.Request, id, requisito string, vincular bool) {
	t, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	var tarefa api.Tarefa
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if _, err := carregarTarefa(tx, t); err != nil {
			return err
		}
		n, err := numero(requisito, "R")
		if err != nil {
			return err
		}
		if err := existeRequisito(tx, n); err != nil {
			return err
		}
		tipo := "requisito_vinculado"
		if vincular {
			_, err = tx.Exec(`INSERT OR IGNORE INTO tarefa_requisito (tarefa_id, requisito_id) VALUES (?, ?)`, t, n)
		} else {
			tipo = "requisito_desvinculado"
			_, err = tx.Exec(`DELETE FROM tarefa_requisito WHERE tarefa_id = ? AND requisito_id = ?`, t, n)
		}
		if err != nil {
			return err
		}
		if err := s.registrarEvento(tx, quem, idTarefa(t), tipo, map[string]any{"requisito": idRequisito(n)}); err != nil {
			return err
		}
		tarefa, err = carregarTarefa(tx, t)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, tarefa)
}

func (s *Servidor) VincularRequisito(w http.ResponseWriter, r *http.Request, id api.Id) {
	var v api.VinculoRequisito
	if err := ler(r, &v); err != nil {
		responderErro(w, err)
		return
	}
	s.mudarVinculoDeRequisito(w, r, id, v.Requisito, true)
}

func (s *Servidor) DesvincularRequisito(w http.ResponseWriter, r *http.Request, id api.Id, requisito string) {
	s.mudarVinculoDeRequisito(w, r, id, requisito, false)
}
