package servidor

import (
	"database/sql"
	"net/http"

	"github.com/rpagliuca/requerial/api"
)

func (s *Servidor) OpcoesDoLancador(w http.ResponseWriter, r *http.Request) {
	if s.lancador == nil {
		responderErro(w, falha(http.StatusNotFound, "o lançador de agentes não está configurado"))
		return
	}
	responder(w, http.StatusOK, s.lancador.Opcoes())
}

// LancarAgente põe a tarefa em andamento e pede um agente com as opções dadas. Serve também para
// relançar uma tarefa que ficou em andamento sem agente.
func (s *Servidor) LancarAgente(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "T")
	if err != nil {
		responderErro(w, err)
		return
	}
	var pedido api.PedidoDeLancamento
	if err := ler(r, &pedido); err != nil {
		responderErro(w, err)
		return
	}
	if s.lancador == nil {
		responderErro(w, falha(http.StatusNotFound, "o lançador de agentes não está configurado"))
		return
	}
	if err := s.lancador.Validar(&pedido); err != nil {
		responderErro(w, falha(http.StatusBadRequest, "%v", err))
		return
	}
	var tarefa api.Tarefa
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
		if atual.SessaoAtiva != nil {
			return falha(http.StatusConflict, "a tarefa %s já tem a sessão ativa %s", atual.Id, *atual.SessaoAtiva)
		}
		anterior = atual.Status
		if anterior != api.EmAndamento {
			if _, err := tx.Exec(`UPDATE tarefas SET status = ?, atualizada_em = ? WHERE id = ?`,
				string(api.EmAndamento), s.momento(), n); err != nil {
				return err
			}
			if err := s.registrarEvento(tx, quem, atual.Id, "tarefa_alterada", map[string]any{"status": api.EmAndamento}); err != nil {
				return err
			}
		}
		dados := map[string]any{"modelo": pedido.Modelo, "esforco": pedido.Esforco}
		if pedido.Instrucao != nil && !vazio(*pedido.Instrucao) {
			dados["instrucao"] = *pedido.Instrucao
		}
		if err := s.registrarEvento(tx, quem, atual.Id, "agente_pedido", dados); err != nil {
			return err
		}
		tarefa, err = carregarTarefa(tx, n)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	s.avisarMudanca(MudancaDeStatus{Tarefa: tarefa, Anterior: anterior, Autor: quem, Lancamento: &pedido})
	responder(w, http.StatusOK, tarefa)
}
