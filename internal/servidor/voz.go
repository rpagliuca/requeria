package servidor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/voz"
)

// Voz é o que o servidor precisa para o rex voz falar com o agente de uma sessão: a tela, o
// teclado e o transcript dele (o lançador, que conhece o tmux), e como lançar o intermediário.
type Voz interface {
	Tela(x api.Sessao) (string, error)
	Escrever(x api.Sessao, texto string) error
	Teclar(x api.Sessao, tecla string) error
	Conversa(x api.Sessao) string
	// Intermediario diz a tarefa fixa do intermediário e como lançá-lo; ok falso sem configuração.
	Intermediario() (tarefa string, pedido api.PedidoDeLancamento, ok bool)
}

// ComVoz liga o rex voz aos agentes.
func (s *Servidor) ComVoz(v Voz) {
	s.voz = v
}

// sufixoDaVoz vai depois da frase do Rafael, numa linha só (uma quebra de linha mandaria a frase
// antes da hora): diz ao agente de onde ela veio.
const sufixoDaVoz = " — (ditado à Siri pelo rex voz: responda curto e em texto corrido; a resposta será resumida em voz)"

// intervaloDaEspera é de quanto em quanto tempo as esperas olham de novo o agente.
var intervaloDaEspera = time.Second

// sessaoPorId acha a sessão pelo id dela (S-0001) ou pelo da tarefa com sessão ativa (T-0001).
func (s *Servidor) sessaoPorId(id string) (api.Sessao, int64, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if strings.HasPrefix(id, "T-") {
		nt, err := numero(id, "T")
		if err != nil {
			return api.Sessao{}, 0, err
		}
		t, err := carregarTarefa(s.db, nt)
		if err != nil {
			return api.Sessao{}, 0, err
		}
		if t.SessaoAtiva == nil {
			return api.Sessao{}, 0, falha(http.StatusConflict, "a tarefa %s não tem sessão ativa", id)
		}
		id = *t.SessaoAtiva
	}
	n, err := numero(id, "S")
	if err != nil {
		return api.Sessao{}, 0, err
	}
	x, err := carregarSessao(s.db, n)
	return x, n, err
}

// estadoDoAgente lê a tela do agente da sessão.
func (s *Servidor) estadoDoAgente(x api.Sessao) (api.EstadoDaVoz, *string) {
	switch {
	case x.Status != api.Ativa:
		return api.VozEncerrada, nil
	case s.voz == nil:
		return api.VozSemAgente, nil
	}
	tela, err := s.voz.Tela(x)
	if err != nil {
		return api.VozSemAgente, nil
	}
	estado, fim := voz.LerTela(tela)
	if fim == "" {
		return api.EstadoDaVoz(estado), nil
	}
	return api.EstadoDaVoz(estado), &fim
}

// lerConversa lê o transcript do agente depois do ponteiro do que o Rafael ouviu.
func (s *Servidor) lerConversa(x api.Sessao, n int64) (voz.Leitura, error) {
	if s.voz == nil {
		return voz.Leitura{}, nil
	}
	caminho := s.voz.Conversa(x)
	if caminho == "" {
		return voz.Leitura{}, nil
	}
	var lido sql.NullString
	if err := s.db.QueryRow(`SELECT voz_lido_ate FROM sessoes WHERE id = ?`, n).Scan(&lido); err != nil {
		return voz.Leitura{}, err
	}
	l, err := voz.Ler(caminho, lido.String)
	if errors.Is(err, os.ErrNotExist) {
		return voz.Leitura{}, nil // agente que ainda não escreveu a conversa
	}
	return l, err
}

// estadoDaVoz é a sessão lida, com o ponteiro novo que não vai na resposta.
type estadoDaVoz struct {
	api.Voz
	ate string
}

func (s *Servidor) lerVoz(x api.Sessao, n int64) (estadoDaVoz, error) {
	v := estadoDaVoz{Voz: api.Voz{Sessao: x.Id, Nome: x.Nome, Tarefas: x.Tarefas, Novidades: []api.NovidadeDeVoz{}}}
	v.Estado, v.Tela = s.estadoDoAgente(x)
	l, err := s.lerConversa(x, n)
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

func (s *Servidor) ObterVoz(w http.ResponseWriter, r *http.Request, id api.Id, params api.ObterVozParams) {
	esperar := 0
	if params.Esperar != nil {
		esperar = *params.Esperar
	}
	if esperar < 0 || esperar > 20 {
		responderErro(w, falha(http.StatusBadRequest, "esperar vai de 0 a 20 segundos"))
		return
	}
	x, n, err := s.sessaoPorId(id)
	if err != nil {
		responderErro(w, err)
		return
	}
	inicio := s.agora()
	limite := time.Now().Add(time.Duration(esperar) * time.Second)
	v, err := s.lerVoz(x, n)
	for err == nil && v.Estado == api.VozTrabalhando && !agenteFalouDepois(v, inicio) && time.Now().Before(limite) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(intervaloDaEspera):
		}
		v, err = s.lerVoz(x, n)
	}
	if err != nil {
		responderErro(w, err)
		return
	}
	if params.Entregar != nil && *params.Entregar && v.ate != "" {
		if _, err := s.db.Exec(`UPDATE sessoes SET voz_proposto = ? WHERE id = ?`, v.ate, n); err != nil {
			responderErro(w, err)
			return
		}
	}
	responder(w, http.StatusOK, v.Voz)
}

func (s *Servidor) PanoramaDaVoz(w http.ResponseWriter, r *http.Request) {
	sessoes, err := listarSessoes(s.db, `WHERE s.status = 'ativa' ORDER BY s.id`)
	if err != nil {
		responderErro(w, err)
		return
	}
	intermediario := ""
	if x, ok, err := s.intermediario(); err == nil && ok {
		intermediario = x.Id
	}
	titulos := map[string]string{}
	out := []api.PanoramaDeVoz{}
	for _, x := range sessoes {
		if x.Id == intermediario {
			continue
		}
		n, err := numero(x.Id, "S")
		if err != nil {
			responderErro(w, err)
			return
		}
		p := api.PanoramaDeVoz{Sessao: x.Id, Nome: x.Nome, Tarefas: x.Tarefas, Titulos: &[]string{}}
		p.Estado, p.Tela = s.estadoDoAgente(x)
		if x.Aguardando != nil {
			p.Aguardando = &x.Aguardando.Mensagem
		}
		for _, t := range x.Tarefas {
			if _, ok := titulos[t]; !ok {
				if nt, err := numero(t, "T"); err == nil {
					if tarefa, err := carregarTarefa(s.db, nt); err == nil {
						titulos[t] = tarefa.Titulo
					}
				}
			}
			*p.Titulos = append(*p.Titulos, titulos[t])
		}
		l, err := s.lerConversa(x, n)
		if err != nil {
			responderErro(w, err)
			return
		}
		p.NaoOuvidas = len(l.Novidades) + l.Omitidas
		if l.Ultima != nil {
			texto := strings.TrimSuffix(l.Ultima.Texto, sufixoDaVoz)
			p.Ultima, p.UltimaEm = &texto, &l.Ultima.Em
		}
		out = append(out, p)
	}
	responder(w, http.StatusOK, out)
}

// sessaoComAgente carrega a sessão, que precisa estar ativa e com agente.
func (s *Servidor) sessaoComAgente(id string) (api.Sessao, int64, error) {
	x, n, err := s.sessaoPorId(id)
	if err != nil {
		return x, 0, err
	}
	if x.Status != api.Ativa {
		return x, 0, falha(http.StatusConflict, "a sessão %s já foi encerrada", x.Id)
	}
	if s.voz == nil {
		return x, 0, falha(http.StatusConflict, "este rex não fala com agentes (sem lançador)")
	}
	return x, n, nil
}

func (s *Servidor) telaDaVoz(x api.Sessao) (voz.Estado, error) {
	tela, err := s.voz.Tela(x)
	if err != nil {
		return "", falha(http.StatusConflict, "o tmux do agente da sessão %s não foi achado", x.Id)
	}
	estado, _ := voz.LerTela(tela)
	return estado, nil
}

func (s *Servidor) EnviarVoz(w http.ResponseWriter, r *http.Request, id api.Id) {
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
	x, n, err := s.sessaoComAgente(id)
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

func (s *Servidor) TeclarVoz(w http.ResponseWriter, r *http.Request, id api.Id) {
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
	x, n, err := s.sessaoComAgente(id)
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
	v, err := s.lerVoz(x, n)
	if err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, v.Voz)
}

// confirmarOuvido: o atalho recebeu a última fala, então as novidades propostas de cada sessão
// viram ouvidas.
func (s *Servidor) confirmarOuvido() error {
	_, err := s.db.Exec(`UPDATE sessoes SET voz_lido_ate = voz_proposto, voz_proposto = NULL WHERE voz_proposto IS NOT NULL`)
	return err
}

// O intermediário: uma sessão permanente do Claude Code (a sessão ativa da tarefa configurada) a
// quem o rex leva as frases da Siri. Ele responde com POST /voz/resposta.

// falasDaVoz guarda a frase da Siri esperando resposta do intermediário. Uma de cada vez.
type falasDaVoz struct {
	vez       sync.Mutex // segura a vez da frase inteira, até a resposta
	mu        sync.Mutex
	esperando chan api.RespostaDeVoz
	conversa  string // a última conversa do atalho, enquanto não acabou
}

// limiteDaFala é quanto a frase espera a resposta do intermediário (a ponte espera um pouco mais).
var limiteDaFala = 4*time.Minute + 30*time.Second

// fusoDaVoz é o fuso da hora que vai com a frase.
var fusoDaVoz = func() *time.Location {
	if l, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return l
	}
	return time.FixedZone("-03", -3*60*60)
}()

var diasDaSemana = [...]string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

// PrefixoDoIntermediario marca no intermediário as frases que vieram da Siri.
const PrefixoDoIntermediario = "[rex voz · siri · "

func frasePara(agora time.Time, nova bool, texto string) string {
	l := agora.In(fusoDaVoz)
	cab := PrefixoDoIntermediario + diasDaSemana[l.Weekday()] + ", " + l.Format("02/01/2006 15:04")
	if nova {
		cab += " · conversa nova"
	}
	return cab + "] " + texto
}

// intermediario acha a sessão ativa da tarefa do intermediário.
func (s *Servidor) intermediario() (api.Sessao, bool, error) {
	if s.voz == nil {
		return api.Sessao{}, false, nil
	}
	tarefa, _, ok := s.voz.Intermediario()
	if !ok {
		return api.Sessao{}, false, nil
	}
	nt, err := numero(tarefa, "T")
	if err != nil {
		return api.Sessao{}, false, err
	}
	t, err := carregarTarefa(s.db, nt)
	if err != nil {
		return api.Sessao{}, false, err
	}
	if t.SessaoAtiva == nil {
		return api.Sessao{}, false, nil
	}
	n, err := numero(*t.SessaoAtiva, "S")
	if err != nil {
		return api.Sessao{}, false, err
	}
	x, err := carregarSessao(s.db, n)
	return x, err == nil, err
}

// lancarIntermediario encerra a sessão órfã do intermediário, se houver, e lança outro com a frase
// como primeira mensagem.
func (s *Servidor) lancarIntermediario(orfa *api.Sessao, frase string) error {
	tarefa, pedido, _ := s.voz.Intermediario()
	nt, err := numero(tarefa, "T")
	if err != nil {
		return err
	}
	if orfa != nil {
		n, err := numero(orfa.Id, "S")
		if err != nil {
			return err
		}
		var x api.Sessao
		err = s.emTransacao(func(tx *sql.Tx) error {
			resumo := "Intermediário do rex voz sem agente (o tmux dele sumiu): encerrado pelo rex, que lançou outro na frase seguinte da Siri."
			if err := s.encerrarComResumo(tx, n, api.Encerramento{Resumo: resumo}, autorLancador); err != nil {
				return err
			}
			x, err = carregarSessao(tx, n)
			return err
		})
		if err != nil {
			return err
		}
		s.avisarEncerramento(x)
	}
	if s.lancador == nil {
		return falha(http.StatusConflict, "o lançador de agentes não está configurado")
	}
	if err := s.lancador.Validar(&pedido); err != nil {
		return fmt.Errorf("pedido do intermediário: %w", err)
	}
	instrucao := frase
	if pedido.Instrucao != nil {
		instrucao = strings.TrimSpace(*pedido.Instrucao) + "\n\nPrimeira frase da Siri:\n" + frase
	}
	pedido.Instrucao = &instrucao
	_, err = s.pedirAgente(nt, pedido, func(*sql.Tx) (string, error) { return "voz", nil })
	return err
}

const avisoDeIntermediarioNovo = "Abri um intermediário novo. "

func (s *Servidor) FalarAoIntermediario(w http.ResponseWriter, r *http.Request) {
	var corpo api.FraseDeVoz
	if err := ler(r, &corpo); err != nil {
		responderErro(w, err)
		return
	}
	texto := strings.Join(strings.Fields(corpo.Texto), " ")
	if texto == "" {
		responderErro(w, falha(http.StatusBadRequest, "o texto é obrigatório"))
		return
	}
	if s.voz == nil {
		responderErro(w, falha(http.StatusNotFound, "este rex não fala com agentes (sem lançador)"))
		return
	}
	if _, _, ok := s.voz.Intermediario(); !ok {
		responderErro(w, falha(http.StatusNotFound, "o intermediário do rex voz não está configurado (voz no lançador)"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), limiteDaFala)
	defer cancel()
	f := &s.falas
	f.vez.Lock()
	defer f.vez.Unlock()

	// A frase seguinte da mesma conversa prova que a fala anterior chegou.
	nova := corpo.Conversa == "" || corpo.Conversa != f.conversa
	if !nova {
		if err := s.confirmarOuvido(); err != nil {
			log.Printf("rex voz: confirmar o ouvido: %v", err)
		}
	}
	f.conversa = "" // volta a valer só com a resposta entregue
	frase := frasePara(s.agora(), nova, texto)

	ch := make(chan api.RespostaDeVoz, 1)
	f.mu.Lock()
	f.esperando = ch
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.esperando = nil
		f.mu.Unlock()
	}()

	x, ok, err := s.intermediario()
	if err != nil {
		responderErro(w, err)
		return
	}
	lancado := false
	if ok {
		estado, _ := s.estadoDoAgente(x)
		switch estado {
		case api.VozSemAgente:
			if err := s.lancarIntermediario(&x, frase); err != nil {
				responderErro(w, err)
				return
			}
			lancado = true
		case api.VozPedindoPermissao:
			s.responderFala(w, api.RespostaDeVoz{Fala: "O intermediário está parado numa tela de permissão no laptop. Resolva lá e tente de novo.", Fim: true})
			return
		default:
			if err := s.voz.Escrever(x, frase); err != nil {
				responderErro(w, err)
				return
			}
		}
	} else {
		if err := s.lancarIntermediario(nil, frase); err != nil {
			responderErro(w, err)
			return
		}
		lancado = true
	}
	enviada := s.agora()

	resposta, err := s.esperarResposta(ctx, ch, enviada)
	if err != nil {
		responderErro(w, err)
		return
	}
	if lancado {
		resposta.Fala = avisoDeIntermediarioNovo + resposta.Fala
	}
	f.conversa = corpo.Conversa
	s.responderFala(w, resposta)
}

func (s *Servidor) responderFala(w http.ResponseWriter, r api.RespostaDeVoz) {
	if r.Fim {
		s.falas.conversa = ""
	}
	responder(w, http.StatusOK, r)
}

// esperarResposta espera o POST /voz/resposta do intermediário. Se ele terminar o turno sem chamar
// a ferramenta, vale o último texto dele depois da frase.
func (s *Servidor) esperarResposta(ctx context.Context, ch chan api.RespostaDeVoz, enviada time.Time) (api.RespostaDeVoz, error) {
	for {
		select {
		case r := <-ch:
			return r, nil
		case <-ctx.Done():
			return api.RespostaDeVoz{}, falha(http.StatusGatewayTimeout, "o intermediário não respondeu a tempo")
		case <-time.After(intervaloDaEspera):
		}
		x, ok, err := s.intermediario()
		if err != nil || !ok {
			continue // o lançador ainda está subindo o agente
		}
		if estado, _ := s.estadoDoAgente(x); estado != api.VozOcioso {
			continue
		}
		n, err := numero(x.Id, "S")
		if err != nil {
			return api.RespostaDeVoz{}, err
		}
		l, err := s.lerConversa(x, n)
		if err != nil {
			return api.RespostaDeVoz{}, err
		}
		if l.Ultima != nil && l.Ultima.Em.After(enviada) {
			select {
			case r := <-ch: // a ferramenta chegou junto
				return r, nil
			default:
			}
			return api.RespostaDeVoz{Fala: l.Ultima.Texto}, nil
		}
	}
}

func (s *Servidor) ResponderVoz(w http.ResponseWriter, r *http.Request) {
	var corpo api.RespostaDeVoz
	if err := ler(r, &corpo); err != nil {
		responderErro(w, err)
		return
	}
	corpo.Fala = strings.TrimSpace(corpo.Fala)
	if corpo.Fala == "" {
		responderErro(w, falha(http.StatusBadRequest, "a fala é obrigatória"))
		return
	}
	f := &s.falas
	f.mu.Lock()
	ch := f.esperando
	f.esperando = nil
	f.mu.Unlock()
	if ch == nil {
		responderErro(w, falha(http.StatusConflict, "nenhuma frase da Siri está esperando resposta: responda no chat"))
		return
	}
	ch <- corpo
	w.WriteHeader(http.StatusNoContent)
}
