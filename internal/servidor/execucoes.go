package servidor

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/rpagliuca/rex/api"
)

func idExecucao(n int64) string { return idCom("E", n) }

const colunasExecucao = `id, requisito_id, evidencia, realizada_em, valida_ate, autor, registrada_em`

// lerExecucao devolve também o número do requisito, para quem agrupa por ele.
func lerExecucao(linha varredor) (api.Execucao, int64, error) {
	var e api.Execucao
	var n, requisito int64
	var realizada, registrada string
	var validaAte sql.NullString
	if err := linha.Scan(&n, &requisito, &e.Evidencia, &realizada, &validaAte, &e.Autor, &registrada); err != nil {
		return e, 0, err
	}
	e.Id, e.Requisito, e.RealizadaEm, e.RegistradaEm = idExecucao(n), idRequisito(requisito), lerMomento(realizada), lerMomento(registrada)
	if validaAte.Valid {
		v := lerMomento(validaAte.String)
		e.ValidaAte = &v
	}
	return e, requisito, nil
}

func (s *Servidor) ListarExecucoes(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "R")
	if err == nil {
		err = existeRequisito(s.db, n)
	}
	if err != nil {
		responderErro(w, err)
		return
	}
	linhas, err := s.db.Query(`SELECT `+colunasExecucao+` FROM execucoes WHERE requisito_id = ? ORDER BY realizada_em DESC, id DESC`, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	execucoes := []api.Execucao{}
	for linhas.Next() {
		e, _, err := lerExecucao(linhas)
		if err != nil {
			responderErro(w, err)
			return
		}
		execucoes = append(execucoes, e)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, execucoes)
}

func (s *Servidor) RegistrarExecucao(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, err := numero(id, "R")
	if err != nil {
		responderErro(w, err)
		return
	}
	var nova api.NovaExecucao
	if err := ler(r, &nova); err != nil {
		responderErro(w, err)
		return
	}
	var criada api.Execucao
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if err := existeRequisito(tx, n); err != nil {
			return err
		}
		if vazio(nova.Evidencia) {
			return falha(http.StatusBadRequest, "a evidência é obrigatória: o que foi verificado e como")
		}
		realizada := s.agora()
		if nova.RealizadaEm != nil {
			realizada = nova.RealizadaEm.UTC()
		}
		if realizada.After(s.agora().Add(time.Minute)) {
			return falha(http.StatusBadRequest, "a execução não pode ser no futuro")
		}
		var validaAte any
		if nova.ValidadeDias != nil {
			if *nova.ValidadeDias < 1 {
				return falha(http.StatusBadRequest, "a validade é de pelo menos 1 dia")
			}
			validaAte = realizada.AddDate(0, 0, *nova.ValidadeDias).Format(time.RFC3339Nano)
		}
		res, err := tx.Exec(`INSERT INTO execucoes (requisito_id, evidencia, realizada_em, valida_ate, autor, registrada_em)
			VALUES (?, ?, ?, ?, ?, ?)`, n, strings.TrimSpace(nova.Evidencia), realizada.Format(time.RFC3339Nano), validaAte, quem, s.momento())
		if err != nil {
			return err
		}
		e, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if criada, _, err = lerExecucao(tx.QueryRow(`SELECT `+colunasExecucao+` FROM execucoes WHERE id = ?`, e)); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, idRequisito(n), "execucao_registrada", map[string]any{"execucao": criada.Id, "valida_ate": criada.ValidaAte})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, criada)
}
