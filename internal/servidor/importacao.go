package servidor

import (
	"database/sql"
	"net/http"
	"sort"
	"time"

	"github.com/rpagliuca/rex/api"
)

// ImportarTarefa grava uma tarefa vinda de fora com as datas originais e as sessões dela, já
// encerradas e sem vínculo ativo. O resumo da tarefa é o da sessão encerrada por último.
func (s *Servidor) ImportarTarefa(w http.ResponseWriter, r *http.Request) {
	var imp api.ImportacaoTarefa
	if err := ler(r, &imp); err != nil {
		responderErro(w, err)
		return
	}
	var importada api.Tarefa
	err := s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if vazio(imp.Titulo) || vazio(imp.Organizacao) {
			return falha(http.StatusBadRequest, "título e organização são obrigatórios")
		}
		if !statusDeTarefaValido(imp.Status) {
			return falha(http.StatusBadRequest, "status de tarefa inválido: %q", imp.Status)
		}
		if len(imp.Sessoes) == 0 {
			return falha(http.StatusBadRequest, "a importação precisa de pelo menos uma sessão")
		}
		sessoes := append([]api.SessaoImportada(nil), imp.Sessoes...)
		sort.SliceStable(sessoes, func(i, j int) bool { return sessoes[i].EncerradaEm.Before(sessoes[j].EncerradaEm) })
		for _, x := range sessoes {
			if err := validarAgente(x.Agente, api.Ia); err != nil {
				return err
			}
			if vazio(x.Servidor) || vazio(x.Resumo) {
				return falha(http.StatusBadRequest, "toda sessão importada precisa de servidor e resumo")
			}
			if x.EncerradaEm.Before(x.IniciadaEm) {
				return falha(http.StatusBadRequest, "sessão importada termina antes de começar")
			}
		}
		ultima := sessoes[len(sessoes)-1]
		descricao := ""
		if imp.Descricao != nil {
			descricao = *imp.Descricao
		}
		data := func(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

		res, err := tx.Exec(`INSERT INTO tarefas (titulo, descricao, organizacao, status, resumo, criada_em, atualizada_em)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, imp.Titulo, descricao, imp.Organizacao, string(imp.Status), ultima.Resumo,
			data(imp.CriadaEm), data(ultima.EncerradaEm))
		if err != nil {
			return err
		}
		tarefa, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := s.registrarEvento(tx, quem, idTarefa(tarefa), "tarefa_importada", map[string]any{
			"origem": imp.Origem, "criada_em": imp.CriadaEm, "status": imp.Status,
		}); err != nil {
			return err
		}

		for _, x := range sessoes {
			_, hash, err := novoToken() // nunca entregue: a sessão já nasce encerrada
			if err != nil {
				return err
			}
			res, err := tx.Exec(`INSERT INTO sessoes (status, agente, tipo_agente, servidor, motivo_encerramento, journal,
				token_hash, iniciada_em, encerrada_em, resumo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				string(api.Encerrada), x.Agente, string(api.Ia), x.Servidor, string(api.FimDoTrabalho), x.Journal,
				hash, data(x.IniciadaEm), data(x.EncerradaEm), x.Resumo)
			if err != nil {
				return err
			}
			sessao, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO sessao_tarefa (sessao_id, tarefa_id, ativa) VALUES (?, ?, 0)`, sessao, tarefa); err != nil {
				return err
			}
			if err := s.registrarEvento(tx, quem, idSessao(sessao), "sessao_importada", map[string]any{
				"tarefa": idTarefa(tarefa), "origem": imp.Origem, "iniciada_em": x.IniciadaEm, "encerrada_em": x.EncerradaEm,
			}); err != nil {
				return err
			}
		}
		importada, err = carregarTarefa(tx, tarefa)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, importada)
}
