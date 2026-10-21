package servidor

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/captura"
)

// autorDaCaptura é o autor, no log de eventos, de tudo que entra por POST /captura.
const autorDaCaptura = "captura"

// tamanhoDaFrase: a resposta é falada; frases longas cansam e o atalho pode cortar.
const tamanhoDaFrase = 200

func responderFrase(w http.ResponseWriter, status int, id, frase string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if id != "" {
		w.Header().Set("X-rex-Id", id)
	}
	w.WriteHeader(status)
	fmt.Fprint(w, captura.Encurtar(frase, tamanhoDaFrase))
}

// frase cabe o texto ditado numa frase de até tamanhoDaFrase, encurtando só o texto.
func frase(antes, texto, depois string) string {
	sobra := tamanhoDaFrase - len([]rune(antes)) - len([]rune(depois))
	if sobra < 10 {
		sobra = 10
	}
	return antes + captura.Encurtar(texto, sobra) + depois
}

func (s *Servidor) Capturar(w http.ResponseWriter, r *http.Request) {
	if s.integracao.TokenDeCaptura == "" {
		responderFrase(w, http.StatusNotFound, "", "A captura não está ligada neste rex.")
		return
	}
	token := tokenDaRequisicao(r)
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.integracao.TokenDeCaptura)) != 1 {
		responderFrase(w, http.StatusUnauthorized, "", "O rex recusou o token de captura.")
		return
	}
	var c api.Captura
	if err := ler(r, &c); err != nil {
		responderFrase(w, http.StatusBadRequest, "", "O pedido chegou malformado ao rex.")
		return
	}
	if vazio(c.Texto) {
		responderFrase(w, http.StatusBadRequest, "", "Não ouvi nada para registrar. Diga de novo.")
		return
	}
	organizacao := s.integracao.OrganizacaoDaCaptura
	if c.Organizacao != nil && !vazio(*c.Organizacao) {
		organizacao = strings.TrimSpace(*c.Organizacao)
	}
	if organizacao == "" {
		responderFrase(w, http.StatusBadRequest, "", "Falta a organização: configure uma padrão para a captura.")
		return
	}
	texto := strings.TrimSpace(c.Texto)
	ditaEm := s.agora()
	if c.DitaEm != nil {
		ditaEm = *c.DitaEm
	}
	simular := c.Simular != nil && *c.Simular

	var lembrete captura.Lembrete
	if c.Tipo == api.TipoDeCapturaLembrete {
		var err error
		lembrete, err = captura.InterpretarLembrete(texto, ditaEm, s.fuso())
		var d *captura.Duvida
		if errors.As(err, &d) {
			responderFrase(w, http.StatusUnprocessableEntity, "", d.Frase)
			return
		}
		if err != nil {
			responderFrase(w, http.StatusInternalServerError, "", "O rex falhou ao entender o pedido.")
			return
		}
	}

	if simular {
		switch c.Tipo {
		case api.TipoDeCapturaLembrete:
			responderFrase(w, http.StatusOK, "", frase("Entendi o lembrete: ", lembrete.Texto, ", "+captura.Descrever(lembrete.Quando)+"."))
		case api.TipoDeCapturaTarefa:
			responderFrase(w, http.StatusOK, "", frase("Entendi a tarefa: ", texto, "."))
		case api.TipoDeCapturaRequisito:
			responderFrase(w, http.StatusOK, "", frase("Entendi o requisito: ", texto, "."))
		default:
			responderFrase(w, http.StatusBadRequest, "", "Tipo de captura desconhecido: diga lembrete, tarefa ou requisito.")
		}
		return
	}

	chave := ""
	if c.Chave != nil {
		chave = strings.TrimSpace(*c.Chave)
	}
	var status int
	var id, resposta string
	var mudanca *MudancaDeStatus
	err := s.emTransacao(func(tx *sql.Tx) error {
		if chave != "" {
			var criado sql.NullString
			err := tx.QueryRow(`SELECT resposta, criado FROM capturas WHERE chave = ?`, chave).Scan(&resposta, &criado)
			if err == nil {
				status, id = http.StatusOK, criado.String
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		status = http.StatusCreated
		switch c.Tipo {
		case api.TipoDeCapturaLembrete:
			x, err := s.inserirLembrete(tx, autorDaCaptura, api.NovoLembrete{
				Texto: lembrete.Texto, Quando: lembrete.Quando, Organizacao: organizacao,
			})
			if err != nil {
				return err
			}
			id = x.Id
			resposta = frase("Lembrete "+x.Id+": ", x.Texto, ", "+captura.Descrever(lembrete.Quando)+".")
			if !lembrete.Quando.After(s.agora()) {
				resposta = frase("Lembrete "+x.Id+": ", x.Texto, ", "+captura.Descrever(lembrete.Quando)+". Chegou depois da hora e já está tocando.")
			}
		case api.TipoDeCapturaTarefa:
			x, err := s.inserirTarefa(tx, autorDaCaptura, texto, "Capturada por voz em "+ditaEm.In(s.fuso()).Format(time.RFC3339)+".",
				organizacao, api.AFazer, nil)
			if err != nil {
				return err
			}
			id, mudanca = x.Id, &MudancaDeStatus{Tarefa: x, Autor: autorDaCaptura}
			resposta = frase("Tarefa "+x.Id+" criada: ", x.Titulo, ".")
		case api.TipoDeCapturaRequisito:
			descricao := "Ditado por voz em " + ditaEm.In(s.fuso()).Format(time.RFC3339) + " (verbatim, para triagem)."
			novo := api.NovoRequisito{
				Titulo:      texto,
				Organizacao: &organizacao,
				Descricao:   &descricao,
			}
			var zero sql.NullInt64
			if err := tx.QueryRow(`SELECT zero_id FROM organizacoes WHERE nome = ?`, organizacao).Scan(&zero); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if zero.Valid {
				novo.DerivaDe = &[]string{idRequisito(zero.Int64)}
			}
			x, err := s.inserirRequisito(tx, autorDaCaptura, novo)
			if err != nil {
				return err
			}
			id = x.Id
			resposta = frase("Requisito "+x.Id+" registrado para triagem: ", x.Titulo, ".")
		default:
			return falha(http.StatusBadRequest, "Tipo de captura desconhecido: diga lembrete, tarefa ou requisito.")
		}
		if chave == "" {
			return nil
		}
		_, err := tx.Exec(`INSERT INTO capturas (chave, status, resposta, criado, criada_em) VALUES (?, ?, ?, ?, ?)`,
			chave, http.StatusCreated, resposta, id, s.momento())
		return err
	})
	var h *erroHTTP
	switch {
	case errors.As(err, &h) && h.status < 500:
		responderFrase(w, h.status, "", h.mensagem)
		return
	case err != nil:
		responderFrase(w, http.StatusInternalServerError, "", "O rex falhou ao registrar. Tente de novo.")
		return
	}
	if mudanca != nil {
		s.avisarMudanca(*mudanca)
	}
	if status == http.StatusCreated && c.Tipo == api.TipoDeCapturaLembrete && !lembrete.Quando.After(s.agora()) {
		// Chegou atrasado: dispara já, sem esperar o agendador.
		s.DispararVencidos()
	}
	responderFrase(w, status, id, resposta)
}
