package servidor

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/voz"
)

// Voz é o que o servidor precisa para o rex voz falar com o agente de uma sessão: a tela, o
// teclado e o transcript dele (o lançador, que conhece o tmux).
type Voz interface {
	Tela(x api.Sessao) (string, error)
	Escrever(x api.Sessao, texto string) error
	Teclar(x api.Sessao, tecla string) error
	Conversa(x api.Sessao) string
}

// ComVoz liga o rex voz aos agentes.
func (s *Servidor) ComVoz(v Voz) {
	s.voz = v
}

// sufixoDaVoz vai depois da frase do Rafael, numa linha só (uma quebra de linha mandaria a frase
// antes da hora): diz ao agente de onde ela veio.
const sufixoDaVoz = " — (ditado à Siri pelo rex voz: responda curto e em texto corrido; a resposta será resumida em voz)"

// intervaloDaEspera é de quanto em quanto tempo GET /voz?esperar olha de novo o agente.
var intervaloDaEspera = time.Second

// estadoDaVoz é a voz lida, com o ponteiro novo que não vai na resposta.
type estadoDaVoz struct {
	api.Voz
	sessao int64
	ate    string
}

func (s *Servidor) lerVoz() (estadoDaVoz, error) {
	v := estadoDaVoz{Voz: api.Voz{Estado: api.VozSemSessao, Novidades: []api.NovidadeDeVoz{}}}
	var sessao sql.NullInt64
	if err := s.db.QueryRow(`SELECT sessao FROM voz WHERE id = 1`).Scan(&sessao); err != nil || !sessao.Valid {
		return v, err
	}
	x, err := carregarSessao(s.db, sessao.Int64)
	if err != nil {
		return v, err
	}
	var lido sql.NullString
	if err := s.db.QueryRow(`SELECT voz_lido_ate FROM sessoes WHERE id = ?`, sessao.Int64).Scan(&lido); err != nil {
		return v, err
	}
	v.sessao = sessao.Int64
	v.Sessao, v.Nome, v.Tarefas = &x.Id, x.Nome, &x.Tarefas
	switch {
	case x.Status != api.Ativa:
		v.Estado = api.VozEncerrada
	case s.voz == nil:
		v.Estado = api.VozSemAgente
	default:
		if tela, err := s.voz.Tela(x); err != nil {
			v.Estado = api.VozSemAgente
		} else {
			estado, fim := voz.LerTela(tela)
			v.Estado = api.EstadoDaVoz(estado)
			if fim != "" {
				v.Tela = &fim
			}
		}
	}
	if s.voz == nil {
		return v, nil
	}
	if caminho := s.voz.Conversa(x); caminho != "" {
		l, err := voz.Ler(caminho, lido.String)
		if errors.Is(err, os.ErrNotExist) {
			return v, nil // agente que ainda não escreveu a conversa
		}
		if err != nil {
			return v, err
		}
		v.ate, v.Omitidas = l.Ate, l.Omitidas
		for _, n := range l.Novidades {
			nova := api.NovidadeDeVoz{De: api.DoAgente, Texto: strings.TrimSuffix(n.Texto, sufixoDaVoz), Em: n.Em}
			if n.DeRafael {
				nova.De = api.DoRafael
			}
			if n.Pedido > 0 {
				pedido := n.Pedido
				nova.Pedido = &pedido
			}
			v.Novidades = append(v.Novidades, nova)
		}
	}
	return v, nil
}

// agenteFalouDepois diz se há texto do agente mais novo que o momento dado.
func agenteFalouDepois(v estadoDaVoz, desde time.Time) bool {
	for _, n := range v.Novidades {
		if n.De == api.DoAgente && n.Em.After(desde) {
			return true
		}
	}
	return false
}

func (s *Servidor) ObterVoz(w http.ResponseWriter, r *http.Request, params api.ObterVozParams) {
	esperar := 0
	if params.Esperar != nil {
		esperar = *params.Esperar
	}
	if esperar < 0 || esperar > 20 {
		responderErro(w, falha(http.StatusBadRequest, "esperar vai de 0 a 20 segundos"))
		return
	}
	inicio := s.agora()
	limite := time.Now().Add(time.Duration(esperar) * time.Second)
	v, err := s.lerVoz()
	for err == nil && v.Estado == api.VozTrabalhando && !agenteFalouDepois(v, inicio) && time.Now().Before(limite) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(intervaloDaEspera):
		}
		v, err = s.lerVoz()
	}
	if err != nil {
		responderErro(w, err)
		return
	}
	if params.Entregar != nil && *params.Entregar && v.ate != "" {
		if _, err := s.db.Exec(`UPDATE voz SET proposto = ? WHERE id = 1 AND sessao = ?`, v.ate, v.sessao); err != nil {
			responderErro(w, err)
			return
		}
	}
	responder(w, http.StatusOK, v.Voz)
}

func (s *Servidor) AssociarVoz(w http.ResponseWriter, r *http.Request) {
	var corpo api.AssociacaoDeVoz
	if err := ler(r, &corpo); err != nil {
		responderErro(w, err)
		return
	}
	id := strings.ToUpper(strings.TrimSpace(corpo.Id))
	err := s.emTransacao(func(tx *sql.Tx) error {
		var n int64
		if strings.HasPrefix(id, "T-") {
			nt, err := numero(id, "T")
			if err != nil {
				return err
			}
			t, err := carregarTarefa(tx, nt)
			if err != nil {
				return err
			}
			if t.SessaoAtiva == nil {
				return falha(http.StatusConflict, "a tarefa %s não tem sessão ativa: lance um agente nela antes", id)
			}
			if n, err = numero(*t.SessaoAtiva, "S"); err != nil {
				return err
			}
		} else {
			var err error
			if n, err = numero(id, "S"); err != nil {
				return err
			}
		}
		x, err := carregarSessao(tx, n)
		if err != nil {
			return err
		}
		if x.Status != api.Ativa {
			return falha(http.StatusConflict, "a sessão %s já foi encerrada", x.Id)
		}
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE voz SET sessao = ?, proposto = NULL WHERE id = 1`, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, x.Id, "voz_associada", nil)
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	v, err := s.lerVoz()
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, v.Voz)
}

// sessaoDaVoz carrega a sessão associada, que precisa estar ativa e com agente.
func (s *Servidor) sessaoDaVoz() (api.Sessao, int64, error) {
	var sessao sql.NullInt64
	if err := s.db.QueryRow(`SELECT sessao FROM voz WHERE id = 1`).Scan(&sessao); err != nil {
		return api.Sessao{}, 0, err
	}
	if !sessao.Valid {
		return api.Sessao{}, 0, falha(http.StatusConflict, "o rex voz não está associado a nenhuma sessão")
	}
	x, err := carregarSessao(s.db, sessao.Int64)
	if err != nil {
		return x, 0, err
	}
	if x.Status != api.Ativa {
		return x, 0, falha(http.StatusConflict, "a sessão associada, %s, já foi encerrada", x.Id)
	}
	if s.voz == nil {
		return x, 0, falha(http.StatusConflict, "este rex não fala com agentes (sem lançador)")
	}
	return x, sessao.Int64, nil
}

func (s *Servidor) telaDaVoz(x api.Sessao) (voz.Estado, error) {
	tela, err := s.voz.Tela(x)
	if err != nil {
		return "", falha(http.StatusConflict, "o tmux do agente da sessão %s não foi achado", x.Id)
	}
	estado, _ := voz.LerTela(tela)
	return estado, nil
}

func (s *Servidor) EnviarVoz(w http.ResponseWriter, r *http.Request) {
	var corpo api.MensagemDeVoz
	if err := ler(r, &corpo); err != nil {
		responderErro(w, err)
		return
	}
	texto := strings.Join(strings.Fields(corpo.Texto), " ")
	if texto == "" {
		responderErro(w, falha(http.StatusBadRequest, "o texto é obrigatório"))
		return
	}
	x, n, err := s.sessaoDaVoz()
	if err != nil {
		responderErro(w, err)
		return
	}
	estado, err := s.telaDaVoz(x)
	if err != nil {
		responderErro(w, err)
		return
	}
	if estado == voz.PedindoPermissao {
		responderErro(w, falha(http.StatusConflict, "o agente está esperando uma escolha na tela: responda antes com a tecla"))
		return
	}
	var pedido int
	if err := s.db.QueryRow(`SELECT voz_pedidos + 1 FROM sessoes WHERE id = ?`, n).Scan(&pedido); err != nil {
		responderErro(w, err)
		return
	}
	if err := s.voz.Escrever(x, voz.Prefixo+strconv.Itoa(pedido)+"] "+texto+sufixoDaVoz); err != nil {
		responderErro(w, err)
		return
	}
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessoes SET voz_pedidos = ? WHERE id = ?`, pedido, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, x.Id, "voz_pedido", map[string]any{"pedido": pedido, "texto": texto})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusCreated, api.PedidoDeVoz{Pedido: pedido, Sessao: x.Id, Estado: api.EstadoDaVoz(estado)})
}

// esperaDaTecla dá tempo à tela de mudar antes de lê-la de novo.
var esperaDaTecla = 800 * time.Millisecond

func (s *Servidor) TeclarVoz(w http.ResponseWriter, r *http.Request) {
	var corpo api.TeclaDeVoz
	if err := ler(r, &corpo); err != nil {
		responderErro(w, err)
		return
	}
	switch corpo.Tecla {
	case api.Tecla1, api.Tecla2, api.Tecla3, api.Tecla4, api.TeclaEnter, api.TeclaEscape:
	default:
		responderErro(w, falha(http.StatusBadRequest, "tecla desconhecida: %q", corpo.Tecla))
		return
	}
	x, _, err := s.sessaoDaVoz()
	if err != nil {
		responderErro(w, err)
		return
	}
	if corpo.Tecla != api.TeclaEscape {
		estado, err := s.telaDaVoz(x)
		if err != nil {
			responderErro(w, err)
			return
		}
		if estado != voz.PedindoPermissao {
			responderErro(w, falha(http.StatusConflict, "o agente não está esperando escolha: a tecla %s cairia na conversa", corpo.Tecla))
			return
		}
	}
	if err := s.voz.Teclar(x, string(corpo.Tecla)); err != nil {
		responderErro(w, err)
		return
	}
	err = s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, x.Id, "voz_tecla", map[string]any{"tecla": string(corpo.Tecla)})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	time.Sleep(esperaDaTecla)
	v, err := s.lerVoz()
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, v.Voz)
}

func (s *Servidor) ConfirmarVoz(w http.ResponseWriter, r *http.Request) {
	err := s.emTransacao(func(tx *sql.Tx) error {
		var sessao sql.NullInt64
		var proposto sql.NullString
		err := tx.QueryRow(`SELECT sessao, proposto FROM voz WHERE id = 1`).Scan(&sessao, &proposto)
		if errors.Is(err, sql.ErrNoRows) || err == nil && (!sessao.Valid || !proposto.Valid) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessoes SET voz_lido_ate = ? WHERE id = ?`, proposto.String, sessao.Int64); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE voz SET proposto = NULL WHERE id = 1`)
		return err
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
