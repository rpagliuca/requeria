package servidor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rpagliuca/requerial/api"
)

const colunasSessao = `s.id, s.status, s.agente, s.tipo_agente, s.chat, s.servidor, s.anterior_id,
	s.motivo_encerramento, s.journal, s.artefatos, s.iniciada_em, s.encerrada_em, s.resumo`

// listarSessoes carrega as sessões que casam com o filtro, já com as tarefas de cada uma.
// As tarefas são lidas depois de fechar o cursor: o banco tem uma única conexão.
func listarSessoes(c consultor, filtro string, args ...any) ([]api.Sessao, error) {
	linhas, err := c.Query(`SELECT `+colunasSessao+` FROM sessoes s `+filtro, args...)
	if err != nil {
		return nil, err
	}
	sessoes := []api.Sessao{}
	var numeros []int64
	for linhas.Next() {
		var x api.Sessao
		var n int64
		var status, tipo, artefatos, iniciada string
		var chat, motivo, journal, encerrada, resumo sql.NullString
		var anterior sql.NullInt64
		if err := linhas.Scan(&n, &status, &x.Agente, &tipo, &chat, &x.Servidor, &anterior,
			&motivo, &journal, &artefatos, &iniciada, &encerrada, &resumo); err != nil {
			linhas.Close()
			return nil, err
		}
		x.Id = idSessao(n)
		x.Status = api.StatusSessao(status)
		x.TipoAgente = api.TipoAgente(tipo)
		x.IniciadaEm = lerMomento(iniciada)
		if chat.Valid {
			x.Chat = &chat.String
		}
		if anterior.Valid {
			id := idSessao(anterior.Int64)
			x.Anterior = &id
		}
		if motivo.Valid {
			m := api.MotivoEncerramento(motivo.String)
			x.MotivoEncerramento = &m
		}
		if journal.Valid {
			x.Journal = &journal.String
		}
		if encerrada.Valid {
			t := lerMomento(encerrada.String)
			x.EncerradaEm = &t
		}
		if resumo.Valid {
			x.Resumo = &resumo.String
		}
		if err := json.Unmarshal([]byte(artefatos), &x.Artefatos); err != nil {
			linhas.Close()
			return nil, err
		}
		sessoes = append(sessoes, x)
		numeros = append(numeros, n)
	}
	if err := linhas.Close(); err != nil {
		return nil, err
	}
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	for i, n := range numeros {
		tarefas, err := tarefasDaSessao(c, n, false)
		if err != nil {
			return nil, err
		}
		sessoes[i].Tarefas = make([]string, len(tarefas))
		for j, t := range tarefas {
			sessoes[i].Tarefas[j] = idTarefa(t)
		}
	}
	return sessoes, nil
}

func tarefasDaSessao(c consultor, n int64, soAtivas bool) ([]int64, error) {
	consulta := `SELECT tarefa_id FROM sessao_tarefa WHERE sessao_id = ?`
	if soAtivas {
		consulta += ` AND ativa = 1`
	}
	linhas, err := c.Query(consulta+` ORDER BY tarefa_id`, n)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var tarefas []int64
	for linhas.Next() {
		var t int64
		if err := linhas.Scan(&t); err != nil {
			return nil, err
		}
		tarefas = append(tarefas, t)
	}
	return tarefas, linhas.Err()
}

func carregarSessao(c consultor, n int64) (api.Sessao, error) {
	sessoes, err := listarSessoes(c, `WHERE s.id = ?`, n)
	if err != nil {
		return api.Sessao{}, err
	}
	if len(sessoes) == 0 {
		return api.Sessao{}, falha(http.StatusNotFound, "sessão %s não encontrada", idSessao(n))
	}
	return sessoes[0], nil
}

// vincular liga a tarefa à sessão, recusando se outra sessão já está ativa nela.
func vincular(tx *sql.Tx, sessao, tarefa int64) error {
	if _, err := carregarTarefa(tx, tarefa); err != nil {
		return err
	}
	var outra int64
	err := tx.QueryRow(`SELECT sessao_id FROM sessao_tarefa WHERE tarefa_id = ? AND ativa = 1`, tarefa).Scan(&outra)
	switch {
	case err == nil && outra == sessao:
		return nil
	case err == nil:
		return falha(http.StatusConflict, "a tarefa %s já tem a sessão ativa %s", idTarefa(tarefa), idSessao(outra))
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	_, err = tx.Exec(`INSERT INTO sessao_tarefa (sessao_id, tarefa_id) VALUES (?, ?)`, sessao, tarefa)
	return err
}

func validarAgente(agente string, tipo api.TipoAgente) error {
	if vazio(agente) {
		return falha(http.StatusBadRequest, "o agente é obrigatório")
	}
	if tipo != api.Ia && tipo != api.Humano {
		return falha(http.StatusBadRequest, "tipo de agente inválido: %q", tipo)
	}
	return nil
}

// iniciar cria uma sessão ativa ligada às tarefas e devolve o número e o token.
func (s *Servidor) iniciar(tx *sql.Tx, agente string, tipo api.TipoAgente, chat *string, servidor string,
	anterior *int64, journal *string, tarefas []int64) (int64, string, error) {
	token, hash, err := novoToken()
	if err != nil {
		return 0, "", err
	}
	res, err := tx.Exec(`INSERT INTO sessoes (agente, tipo_agente, chat, servidor, anterior_id, journal, token_hash, iniciada_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, agente, string(tipo), chat, servidor, anterior, journal, hash, s.momento())
	if err != nil {
		return 0, "", err
	}
	n, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	for _, t := range tarefas {
		if err := vincular(tx, n, t); err != nil {
			return 0, "", err
		}
	}
	return n, token, nil
}

// encerrar fecha a sessão e libera as tarefas dela.
func (s *Servidor) encerrar(tx *sql.Tx, n int64, status api.StatusSessao, motivo api.MotivoEncerramento) error {
	if _, err := tx.Exec(`UPDATE sessoes SET status = ?, motivo_encerramento = ?, encerrada_em = ? WHERE id = ?`,
		string(status), string(motivo), s.momento(), n); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE sessao_tarefa SET ativa = 0 WHERE sessao_id = ?`, n)
	return err
}

func (s *Servidor) ListarSessoes(w http.ResponseWriter, r *http.Request, params api.ListarSessoesParams) {
	filtro, args := `ORDER BY s.id`, []any{}
	if params.Status != nil {
		filtro, args = `WHERE s.status = ? ORDER BY s.id`, []any{string(*params.Status)}
	}
	sessoes, err := listarSessoes(s.db, filtro, args...)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, sessoes)
}

func (s *Servidor) CriarSessao(w http.ResponseWriter, r *http.Request) {
	var nova api.NovaSessao
	if err := ler(r, &nova); err != nil {
		responderErro(w, err)
		return
	}
	var criada api.SessaoCriada
	err := s.emTransacao(func(tx *sql.Tx) error {
		if err := validarAgente(nova.Agente, nova.TipoAgente); err != nil {
			return err
		}
		if vazio(nova.Servidor) {
			return falha(http.StatusBadRequest, "o servidor é obrigatório")
		}
		if len(nova.Tarefas) == 0 {
			return falha(http.StatusBadRequest, "a sessão precisa de pelo menos uma tarefa")
		}
		tarefas := make([]int64, len(nova.Tarefas))
		for i, id := range nova.Tarefas {
			t, err := numero(id, "T")
			if err != nil {
				return err
			}
			tarefas[i] = t
		}
		n, token, err := s.iniciar(tx, nova.Agente, nova.TipoAgente, nova.Chat, nova.Servidor, nil, nova.Journal, tarefas)
		if err != nil {
			return err
		}
		if criada.Sessao, err = carregarSessao(tx, n); err != nil {
			return err
		}
		criada.Token = token
		return s.registrarEvento(tx, criada.Sessao.Id, criada.Sessao.Id, "sessao_iniciada", map[string]any{
			"agente": nova.Agente, "tipo_agente": nova.TipoAgente, "servidor": nova.Servidor, "tarefas": criada.Sessao.Tarefas,
		})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criada)
}

func (s *Servidor) ObterSessao(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "S")
	if err != nil {
		responderErro(w, err)
		return
	}
	x, err := carregarSessao(s.db, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

// operarSessao autentica a sessão, executa f e devolve a sessão atualizada.
func (s *Servidor) operarSessao(w http.ResponseWriter, r *http.Request, id api.Id, corpo any,
	f func(tx *sql.Tx, n int64) error) {
	n, err := numero(id, "S")
	if err != nil {
		responderErro(w, err)
		return
	}
	if err := ler(r, corpo); err != nil {
		responderErro(w, err)
		return
	}
	var x api.Sessao
	err = s.emTransacao(func(tx *sql.Tx) error {
		if err := autenticarSessao(tx, r, n); err != nil {
			return err
		}
		if err := f(tx, n); err != nil {
			return err
		}
		x, err = carregarSessao(tx, n)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, x)
}

func (s *Servidor) AlterarSessao(w http.ResponseWriter, r *http.Request, id api.Id) {
	var alteracao api.AlteracaoSessao
	s.operarSessao(w, r, id, &alteracao, func(tx *sql.Tx, n int64) error {
		campos := map[string]any{}
		if alteracao.Journal != nil {
			if _, err := tx.Exec(`UPDATE sessoes SET journal = ? WHERE id = ?`, *alteracao.Journal, n); err != nil {
				return err
			}
			campos["journal"] = *alteracao.Journal
		}
		if alteracao.Artefatos != nil {
			texto, err := json.Marshal(*alteracao.Artefatos)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE sessoes SET artefatos = ? WHERE id = ?`, string(texto), n); err != nil {
				return err
			}
			campos["artefatos"] = *alteracao.Artefatos
		}
		if len(campos) == 0 {
			return falha(http.StatusBadRequest, "nenhum campo para alterar")
		}
		return s.registrarEvento(tx, idSessao(n), idSessao(n), "sessao_alterada", campos)
	})
}

func (s *Servidor) VincularTarefa(w http.ResponseWriter, r *http.Request, id api.Id) {
	var vinculo api.Vinculo
	s.operarSessao(w, r, id, &vinculo, func(tx *sql.Tx, n int64) error {
		t, err := numero(vinculo.Tarefa, "T")
		if err != nil {
			return err
		}
		if err := vincular(tx, n, t); err != nil {
			return err
		}
		return s.registrarEvento(tx, idSessao(n), idSessao(n), "tarefa_vinculada", map[string]any{"tarefa": idTarefa(t)})
	})
}

func (s *Servidor) EncerrarSessao(w http.ResponseWriter, r *http.Request, id api.Id) {
	var enc api.Encerramento
	s.operarSessao(w, r, id, &enc, func(tx *sql.Tx, n int64) error {
		if vazio(enc.Resumo) {
			return falha(http.StatusBadRequest, "o resumo é obrigatório: é ele que o próximo agente recebe")
		}
		if enc.Journal != nil {
			if _, err := tx.Exec(`UPDATE sessoes SET journal = ? WHERE id = ?`, *enc.Journal, n); err != nil {
				return err
			}
		}
		if enc.Artefatos != nil {
			texto, err := json.Marshal(*enc.Artefatos)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE sessoes SET artefatos = ? WHERE id = ?`, string(texto), n); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE sessoes SET resumo = ? WHERE id = ?`, enc.Resumo, n); err != nil {
			return err
		}
		tarefas, err := tarefasDaSessao(tx, n, true)
		if err != nil {
			return err
		}
		for _, t := range tarefas {
			if _, err := tx.Exec(`UPDATE tarefas SET resumo = ?, atualizada_em = ? WHERE id = ?`, enc.Resumo, s.momento(), t); err != nil {
				return err
			}
			if err := s.registrarEvento(tx, idSessao(n), idTarefa(t), "resumo_sincronizado", map[string]any{"sessao": idSessao(n)}); err != nil {
				return err
			}
		}
		if err := s.encerrar(tx, n, api.Encerrada, api.FimDoTrabalho); err != nil {
			return err
		}
		return s.registrarEvento(tx, idSessao(n), idSessao(n), "sessao_encerrada", map[string]any{"motivo": api.FimDoTrabalho})
	})
}

func (s *Servidor) RepassarSessao(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "S")
	if err != nil {
		responderErro(w, err)
		return
	}
	var repasse api.Repasse
	if err := ler(r, &repasse); err != nil {
		responderErro(w, err)
		return
	}
	var criada api.SessaoCriada
	err = s.emTransacao(func(tx *sql.Tx) error {
		if err := autenticarSessao(tx, r, n); err != nil {
			return err
		}
		if err := validarAgente(repasse.Agente, repasse.TipoAgente); err != nil {
			return err
		}
		atual, err := carregarSessao(tx, n)
		if err != nil {
			return err
		}
		if atual.Agente == repasse.Agente && atual.TipoAgente == repasse.TipoAgente {
			return falha(http.StatusBadRequest, "o agente novo é igual ao da sessão %s; não há o que repassar", atual.Id)
		}
		tarefas, err := tarefasDaSessao(tx, n, true)
		if err != nil {
			return err
		}
		if err := s.encerrar(tx, n, api.Substituida, api.TrocaDeAgente); err != nil {
			return err
		}
		servidor := atual.Servidor
		if repasse.Servidor != nil && !vazio(*repasse.Servidor) {
			servidor = *repasse.Servidor
		}
		chat := atual.Chat
		if repasse.Chat != nil {
			chat = repasse.Chat
		}
		nova, token, err := s.iniciar(tx, repasse.Agente, repasse.TipoAgente, chat, servidor, &n, atual.Journal, tarefas)
		if err != nil {
			return err
		}
		if criada.Sessao, err = carregarSessao(tx, nova); err != nil {
			return err
		}
		criada.Token = token
		if err := s.registrarEvento(tx, atual.Id, atual.Id, "sessao_substituida", map[string]any{
			"motivo": api.TrocaDeAgente, "nova": criada.Sessao.Id,
		}); err != nil {
			return err
		}
		return s.registrarEvento(tx, criada.Sessao.Id, criada.Sessao.Id, "sessao_iniciada", map[string]any{
			"agente": repasse.Agente, "tipo_agente": repasse.TipoAgente, "servidor": servidor,
			"tarefas": criada.Sessao.Tarefas, "anterior": atual.Id,
		})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criada)
}

func (s *Servidor) ListarEventos(w http.ResponseWriter, r *http.Request, params api.ListarEventosParams) {
	consulta, args := `SELECT id, momento, autor, entidade, tipo, dados FROM eventos`, []any{}
	if params.Entidade != nil {
		consulta, args = consulta+` WHERE entidade = ?`, []any{*params.Entidade}
	}
	linhas, err := s.db.Query(consulta+` ORDER BY id`, args...)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	eventos := []api.Evento{}
	for linhas.Next() {
		var e api.Evento
		var momento, dados string
		if err := linhas.Scan(&e.Id, &momento, &e.Autor, &e.Entidade, &e.Tipo, &dados); err != nil {
			responderErro(w, err)
			return
		}
		e.Momento = lerMomento(momento)
		if err := json.Unmarshal([]byte(dados), &e.Dados); err != nil {
			responderErro(w, err)
			return
		}
		eventos = append(eventos, e)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, eventos)
}
