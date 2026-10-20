package servidor

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/rpagliuca/requerial/api"
)

type consultor interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

const colunasTarefa = `t.id, t.titulo, t.descricao, t.organizacao, t.status, t.resumo, t.criada_em, t.atualizada_em, t.template_id,
	(SELECT st.sessao_id FROM sessao_tarefa st WHERE st.tarefa_id = t.id AND st.ativa = 1)`

type varredor interface{ Scan(dest ...any) error }

func lerTarefa(linha varredor) (api.Tarefa, error) {
	var t api.Tarefa
	var n int64
	var status, criada, atualizada string
	var sessao, template sql.NullInt64
	if err := linha.Scan(&n, &t.Titulo, &t.Descricao, &t.Organizacao, &status, &t.Resumo, &criada, &atualizada, &template, &sessao); err != nil {
		return t, err
	}
	t.Id = idTarefa(n)
	t.Status = api.StatusTarefa(status)
	t.CriadaEm = lerMomento(criada)
	t.AtualizadaEm = lerMomento(atualizada)
	if sessao.Valid {
		id := idSessao(sessao.Int64)
		t.SessaoAtiva = &id
	}
	if template.Valid {
		id := idTemplate(template.Int64)
		t.Template = &id
	}
	return t, nil
}

func carregarTarefa(c consultor, n int64) (api.Tarefa, error) {
	t, err := lerTarefa(c.QueryRow(`SELECT `+colunasTarefa+` FROM tarefas t WHERE t.id = ?`, n))
	if errors.Is(err, sql.ErrNoRows) {
		return t, falha(http.StatusNotFound, "tarefa %s não encontrada", idTarefa(n))
	}
	if err != nil {
		return t, err
	}
	requisitos, err := requisitosDaTarefa(c, n)
	if err != nil {
		return t, err
	}
	t.Requisitos = &requisitos
	componentes, err := idsLigados(c, idComponente, `SELECT componente_id FROM tarefa_componente WHERE tarefa_id = ? ORDER BY componente_id`, n)
	t.Componentes = &componentes
	return t, err
}

func statusDeTarefaValido(s api.StatusTarefa) bool {
	switch s {
	case api.AFazer, api.EmAndamento, api.Pausada, api.Bloqueada, api.EmValidacao, api.Concluida:
		return true
	}
	return false
}

func vazio(s string) bool { return strings.TrimSpace(s) == "" }

func (s *Servidor) ListarTarefas(w http.ResponseWriter, r *http.Request, params api.ListarTarefasParams) {
	consulta := `SELECT ` + colunasTarefa + ` FROM tarefas t WHERE 1 = 1`
	var args []any
	if params.Status != nil {
		consulta += ` AND t.status = ?`
		args = append(args, string(*params.Status))
	}
	if params.Organizacao != nil {
		consulta += ` AND t.organizacao = ?`
		args = append(args, *params.Organizacao)
	}
	linhas, err := s.db.Query(consulta+` ORDER BY t.id`, args...)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	tarefas := []api.Tarefa{}
	for linhas.Next() {
		t, err := lerTarefa(linhas)
		if err != nil {
			responderErro(w, err)
			return
		}
		tarefas = append(tarefas, t)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, tarefas)
}

func (s *Servidor) CriarTarefa(w http.ResponseWriter, r *http.Request) {
	var nova api.NovaTarefa
	if err := ler(r, &nova); err != nil {
		responderErro(w, err)
		return
	}
	var criada api.Tarefa
	var quem string
	err := s.emTransacao(func(tx *sql.Tx) (err error) {
		quem, err = autor(tx, r)
		if err != nil {
			return err
		}
		if vazio(nova.Titulo) {
			return falha(http.StatusBadRequest, "o título é obrigatório")
		}
		if vazio(nova.Organizacao) {
			return falha(http.StatusBadRequest, "a organização é obrigatória")
		}
		status := api.AFazer
		if nova.Status != nil {
			status = *nova.Status
		}
		if !statusDeTarefaValido(status) {
			return falha(http.StatusBadRequest, "status de tarefa inválido: %q", status)
		}
		descricao := ""
		if nova.Descricao != nil {
			descricao = *nova.Descricao
		}
		if criada, err = s.inserirTarefa(tx, quem, nova.Titulo, descricao, nova.Organizacao, status, nil); err != nil {
			return err
		}
		t := mustNumero(criada.Id, "T")
		if nova.Requisitos != nil {
			for _, id := range *nova.Requisitos {
				n, err := numero(id, "R")
				if err != nil {
					return err
				}
				if err := existeRequisito(tx, n); err != nil {
					return err
				}
				if _, err := tx.Exec(`INSERT OR IGNORE INTO tarefa_requisito (tarefa_id, requisito_id) VALUES (?, ?)`, t, n); err != nil {
					return err
				}
			}
		}
		if nova.Componentes != nil {
			for _, id := range *nova.Componentes {
				if err := s.mudarComponente(tx, r, "tarefa_componente", "tarefa_id", criada.Id, t, id, true); err != nil {
					return err
				}
			}
		}
		criada, err = carregarTarefa(tx, t)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	s.avisarMudanca(MudancaDeStatus{Tarefa: criada, Autor: quem})
	responder(w, http.StatusCreated, criada)
}

// inserirTarefa grava a tarefa e o evento de criação; template é o número do template ou nil.
func (s *Servidor) inserirTarefa(tx *sql.Tx, quem, titulo, descricao, organizacao string, status api.StatusTarefa, template any) (api.Tarefa, error) {
	agora := s.momento()
	res, err := tx.Exec(`INSERT INTO tarefas (titulo, descricao, organizacao, status, criada_em, atualizada_em, template_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, titulo, descricao, organizacao, string(status), agora, agora, template)
	if err != nil {
		return api.Tarefa{}, err
	}
	n, err := res.LastInsertId()
	if err != nil {
		return api.Tarefa{}, err
	}
	criada, err := carregarTarefa(tx, n)
	if err != nil {
		return criada, err
	}
	dados := map[string]any{"titulo": criada.Titulo, "organizacao": criada.Organizacao, "status": criada.Status}
	if criada.Template != nil {
		dados["template"] = *criada.Template
	}
	return criada, s.registrarEvento(tx, quem, criada.Id, "tarefa_criada", dados)
}

func (s *Servidor) ObterTarefa(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	t, err := carregarTarefa(s.db, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, t)
}

func (s *Servidor) AlterarTarefa(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	var alteracao api.AlteracaoTarefa
	if err := ler(r, &alteracao); err != nil {
		responderErro(w, err)
		return
	}
	var alterada api.Tarefa
	var anterior api.StatusTarefa
	var quem string
	err = s.emTransacao(func(tx *sql.Tx) (err error) {
		if quem, err = autor(tx, r); err != nil {
			return err
		}
		atual, err := carregarTarefa(tx, n)
		if err != nil {
			return err
		}
		anterior = atual.Status
		campos := map[string]any{}
		var sets []string
		var args []any
		definir := func(coluna string, valor any) {
			sets = append(sets, coluna+" = ?")
			args = append(args, valor)
			campos[coluna] = valor
		}
		if alteracao.Titulo != nil {
			if vazio(*alteracao.Titulo) {
				return falha(http.StatusBadRequest, "o título não pode ficar vazio")
			}
			definir("titulo", *alteracao.Titulo)
		}
		if alteracao.Descricao != nil {
			definir("descricao", *alteracao.Descricao)
		}
		if alteracao.Organizacao != nil {
			if vazio(*alteracao.Organizacao) {
				return falha(http.StatusBadRequest, "a organização não pode ficar vazia")
			}
			definir("organizacao", *alteracao.Organizacao)
		}
		if alteracao.Status != nil {
			if !statusDeTarefaValido(*alteracao.Status) {
				return falha(http.StatusBadRequest, "status de tarefa inválido: %q", *alteracao.Status)
			}
			definir("status", string(*alteracao.Status))
		}
		if alteracao.Resumo != nil {
			definir("resumo", *alteracao.Resumo)
		}
		if len(sets) == 0 {
			return falha(http.StatusBadRequest, "nenhum campo para alterar")
		}
		sets = append(sets, "atualizada_em = ?")
		args = append(args, s.momento(), n)
		if _, err := tx.Exec(`UPDATE tarefas SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return err
		}
		if alterada, err = carregarTarefa(tx, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, alterada.Id, "tarefa_alterada", campos)
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	s.avisarMudanca(MudancaDeStatus{Tarefa: alterada, Anterior: anterior, Autor: quem})
	responder(w, http.StatusOK, alterada)
}

func (s *Servidor) ListarSessoesDaTarefa(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	if _, err := carregarTarefa(s.db, n); err != nil {
		responderErro(w, err)
		return
	}
	sessoes, err := listarSessoes(s.db, `WHERE s.id IN (SELECT sessao_id FROM sessao_tarefa WHERE tarefa_id = ?) ORDER BY s.id DESC`, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, sessoes)
}
