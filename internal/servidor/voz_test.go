package servidor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/voz"
)

type vozFalsa struct {
	mu       sync.Mutex
	tela     string
	semTmux  bool
	escritos []string
	teclas   []string
	conversa string
	tarefa   string // do intermediário; vazio sem configuração
}

func (v *vozFalsa) Tela(api.Sessao) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.semTmux {
		return "", os.ErrNotExist
	}
	return v.tela, nil
}
func (v *vozFalsa) Escrever(_ api.Sessao, texto string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.escritos = append(v.escritos, texto)
	return nil
}
func (v *vozFalsa) Teclar(_ api.Sessao, tecla string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.teclas = append(v.teclas, tecla)
	return nil
}
func (v *vozFalsa) Conversa(api.Sessao) string { return v.conversa }
func (v *vozFalsa) Intermediario() (string, api.PedidoDeLancamento, bool) {
	instrucao := "Carregue a skill rex-voz."
	return v.tarefa, api.PedidoDeLancamento{Instrucao: &instrucao}, v.tarefa != ""
}
func (v *vozFalsa) mudar(f func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	f()
}

const (
	telaOciosa      = "✻ done\n❯ \n  ⏵⏵ bypass permissions on\n"
	telaTrabalhando = "✢ Thinking…\n❯ \n  ⏵⏵ bypass permissions on · esc to interrupt\n"
	telaPermissao   = " Do you want to proceed?\n ❯ 1. Yes\n   2. No\n Esc to cancel\n"
)

func TestVozPorSessao(t *testing.T) {
	a := novoAmbiente(t)
	esperaDaTecla, intervaloDaEspera = 0, 10*time.Millisecond
	v := &vozFalsa{tela: telaOciosa, conversa: filepath.Join(t.TempDir(), "c.jsonl")}
	a.servidor.ComVoz(v)

	tarefa := a.tarefa("Deploy")
	semSessao := a.tarefa("Sem agente")
	a.chamar("POST", "/voz/sessoes/"+semSessao.Id+"/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)
	a.chamar("GET", "/voz/sessoes/S-0099", "", nil, 404, nil)
	s := a.sessao("Claude Opus 5.5 Medium", tarefa.Id)

	var estado api.Voz
	a.chamar("GET", "/voz/sessoes/"+strings.ToLower(tarefa.Id), "", nil, 200, &estado)
	if estado.Sessao != s.Sessao.Id || estado.Estado != api.VozOcioso {
		t.Fatalf("pela tarefa: %+v", estado)
	}

	var pedido api.PedidoDeVoz
	caminho := "/voz/sessoes/" + s.Sessao.Id
	a.chamar("POST", caminho+"/mensagens", "", api.MensagemDeVoz{Texto: "como está\n o deploy?"}, 201, &pedido)
	a.chamar("POST", caminho+"/mensagens", "", api.MensagemDeVoz{Texto: "e o teste?"}, 201, &pedido)
	if pedido.Pedido != 2 || len(v.escritos) != 2 || !strings.HasPrefix(v.escritos[0], voz.Prefixo+"1] como está o deploy?") {
		t.Fatalf("pedidos: %+v %q", pedido, v.escritos)
	}

	// Tela de permissão: a frase cairia na escolha; a tecla responde.
	v.tela = telaPermissao
	a.chamar("POST", caminho+"/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if estado.Estado != api.VozPedindoPermissao || estado.Tela == nil || !strings.Contains(*estado.Tela, "1. Yes") {
		t.Fatalf("permissão: %+v", estado)
	}
	a.chamar("POST", caminho+"/teclas", "", api.TeclaDeVoz{Tecla: "1"}, 200, nil)
	v.tela = telaOciosa
	a.chamar("POST", caminho+"/teclas", "", api.TeclaDeVoz{Tecla: "1"}, 409, nil)
	a.chamar("POST", caminho+"/teclas", "", api.TeclaDeVoz{Tecla: "x"}, 400, nil)
	a.chamar("POST", caminho+"/teclas", "", api.TeclaDeVoz{Tecla: api.TeclaEscape}, 200, nil)
	if strings.Join(v.teclas, ",") != "1,Escape" {
		t.Fatalf("teclas: %v", v.teclas)
	}

	// Ponteiro: entregue não é ouvido até a confirmação.
	linhas := `{"type":"user","uuid":"u1","timestamp":"2026-10-22T10:00:00Z","message":{"content":"[rex voz · pedido 1] como está o deploy?` + sufixoDaVoz + `"}}
{"type":"assistant","uuid":"u2","timestamp":"2026-10-22T10:00:09Z","message":{"content":[{"type":"text","text":"Terminou."}]}}
`
	os.WriteFile(v.conversa, []byte(linhas), 0o644)
	a.chamar("GET", caminho+"?entregar=true", "", nil, 200, &estado)
	if len(estado.Novidades) != 2 || estado.Novidades[0].Texto != "como está o deploy?" || *estado.Novidades[1].Pedido != 1 {
		t.Fatalf("novidades: %+v", estado.Novidades)
	}
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if len(estado.Novidades) != 2 {
		t.Fatalf("sem confirmação, continuam novas: %+v", estado.Novidades)
	}
	if err := a.servidor.confirmarOuvido(); err != nil {
		t.Fatal(err)
	}
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if len(estado.Novidades) != 0 {
		t.Fatalf("depois da confirmação: %+v", estado.Novidades)
	}

	// esperar: com o agente trabalhando e calado, segura até o limite.
	v.tela = telaTrabalhando
	inicio := time.Now()
	a.chamar("GET", caminho+"?esperar=1", "", nil, 200, &estado)
	if time.Since(inicio) < time.Second || estado.Estado != api.VozTrabalhando {
		t.Fatalf("não esperou: %v %+v", time.Since(inicio), estado)
	}

	v.semTmux = true
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if estado.Estado != api.VozSemAgente {
		t.Fatalf("sem tmux: %+v", estado)
	}
	a.chamar("POST", caminho+"/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)
}

func TestVozPanorama(t *testing.T) {
	a := novoAmbiente(t)
	v := &vozFalsa{tela: telaOciosa, conversa: filepath.Join(t.TempDir(), "c.jsonl")}
	a.servidor.ComVoz(v)
	inter := a.tarefa("Intermediário")
	v.tarefa = inter.Id
	a.sessao("Claude Haiku 5.5 Low", inter.Id)
	deploy := a.tarefa("Deploy do cartório")
	s := a.sessao("Claude Opus 5.5 Medium", deploy.Id)
	a.chamar("POST", "/sessoes/"+s.Sessao.Id+"/espera", s.Token, api.Espera{Mensagem: "aprovar o PR"}, 200, nil)
	os.WriteFile(v.conversa, []byte(`{"type":"assistant","uuid":"u0","timestamp":"2026-10-22T09:00:00Z","message":{"content":[{"type":"text","text":"Começando."}]}}
{"type":"user","uuid":"u1","timestamp":"2026-10-22T10:00:00Z","message":{"content":"abre o PR"}}
{"type":"assistant","uuid":"u2","timestamp":"2026-10-22T10:00:09Z","message":{"content":[{"type":"text","text":"PR aberto."}]}}
`), 0o644)

	var panorama []api.PanoramaDeVoz
	a.chamar("GET", "/voz/sessoes", "", nil, 200, &panorama)
	if len(panorama) != 1 {
		t.Fatalf("o intermediário fica de fora: %+v", panorama)
	}
	p := panorama[0]
	if p.Sessao != s.Sessao.Id || p.Aguardando == nil || *p.Aguardando != "aprovar o PR" || p.Estado != api.VozOcioso ||
		p.NaoOuvidas != 1 || p.Ultima == nil || *p.Ultima != "PR aberto." || (*p.Titulos)[0] != "Deploy do cartório" {
		t.Fatalf("panorama: %+v", p)
	}
}

// responderComo manda a resposta do intermediário assim que houver frase esperando.
func responderComo(a *ambiente, r api.RespostaDeVoz) {
	go func() {
		corpo, _ := json.Marshal(r)
		for range 500 {
			resp, err := http.Post(a.srv.URL+"/voz/resposta", "application/json", bytes.NewReader(corpo))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusNoContent {
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

func TestVozIntermediario(t *testing.T) {
	a := novoAmbiente(t)
	intervaloDaEspera = 10 * time.Millisecond
	v := &vozFalsa{tela: telaOciosa, conversa: filepath.Join(t.TempDir(), "c.jsonl")}
	a.servidor.ComVoz(v)
	frase := api.FraseDeVoz{Conversa: "C1", Texto: "o que espera por mim?"}
	a.chamar("POST", "/voz/intermediario", "", frase, 404, nil) // sem configuração

	inter := a.tarefa("Intermediário")
	v.tarefa = inter.Id
	a.chamar("POST", "/voz/resposta", "", api.RespostaDeVoz{Fala: "oi"}, 409, nil)
	a.chamar("POST", "/voz/intermediario", "", frase, 409, nil) // sem lançador

	// Sem intermediário: o rex lança um, com a frase como primeira mensagem.
	l := &lancadorFalso{}
	a.servidor.ComLancador(l)
	a.servidor.AoMudarStatus(func(m MudancaDeStatus) {
		if m.Lancamento != nil {
			l.mudancas = append(l.mudancas, m)
			a.servidor.AbrirSessaoLancada(m.Tarefa.Id, "Claude Haiku 5.5 Low", "maquina-local", "chat", "rex-voz")
		}
	})
	responderComo(a, api.RespostaDeVoz{Fala: "Nada espera por você."})
	var resposta api.RespostaDeVoz
	a.chamar("POST", "/voz/intermediario", "", frase, 200, &resposta)
	if resposta.Fala != avisoDeIntermediarioNovo+"Nada espera por você." || resposta.Fim || len(l.mudancas) != 1 {
		t.Fatalf("lançado: %+v %d", resposta, len(l.mudancas))
	}
	instrucao := *l.mudancas[0].Lancamento.Instrucao
	if !strings.HasPrefix(instrucao, "Carregue a skill rex-voz.") || !strings.Contains(instrucao, " · conversa nova] o que espera por mim?") {
		t.Fatalf("instrução: %q", instrucao)
	}

	// Frase seguinte da mesma conversa: vai ao tmux do intermediário e confirma o que foi entregue.
	deploy := a.tarefa("Deploy")
	s := a.sessao("Claude Opus 5.5 Medium", deploy.Id)
	a.db.Exec(`UPDATE sessoes SET voz_proposto = 'u9' WHERE id = 2`)
	responderComo(a, api.RespostaDeVoz{Fala: "Tchau.", Fim: true})
	a.chamar("POST", "/voz/intermediario", "", api.FraseDeVoz{Conversa: "C1", Texto: "tchau"}, 200, &resposta)
	if resposta.Fala != "Tchau." || !resposta.Fim || len(v.escritos) != 1 ||
		!strings.HasPrefix(v.escritos[0], PrefixoDoIntermediario) || !strings.HasSuffix(v.escritos[0], "] tchau") ||
		strings.Contains(v.escritos[0], "conversa nova") {
		t.Fatalf("segunda frase: %+v %q", resposta, v.escritos)
	}
	var lido string
	a.db.QueryRow(`SELECT voz_lido_ate FROM sessoes WHERE id = 2`).Scan(&lido)
	if s.Sessao.Id != "S-0002" || lido != "u9" {
		t.Fatalf("confirmação: %s %q", s.Sessao.Id, lido)
	}

	// Depois do fim, a mesma conversa recomeça. Sem a ferramenta, vale o último texto do agente.
	os.WriteFile(v.conversa, []byte(`{"type":"assistant","uuid":"u1","timestamp":"2099-01-01T10:00:00Z","message":{"content":[{"type":"text","text":"Respondi no chat."}]}}
`), 0o644)
	a.chamar("POST", "/voz/intermediario", "", api.FraseDeVoz{Conversa: "C1", Texto: "oi"}, 200, &resposta)
	if resposta.Fala != "Respondi no chat." || !strings.Contains(v.escritos[1], "conversa nova") {
		t.Fatalf("sem a ferramenta: %+v %q", resposta, v.escritos)
	}
	os.Remove(v.conversa)

	// O tmux do intermediário sumiu: a sessão órfã é encerrada e outro sobe.
	v.mudar(func() { v.semTmux = true })
	responderComo(a, api.RespostaDeVoz{Fala: "Oi."})
	a.chamar("POST", "/voz/intermediario", "", api.FraseDeVoz{Conversa: "C2", Texto: "oi"}, 200, &resposta)
	if resposta.Fala != avisoDeIntermediarioNovo+"Oi." || len(l.mudancas) != 2 {
		t.Fatalf("relançado: %+v %d", resposta, len(l.mudancas))
	}
	var orfa api.Sessao
	a.chamar("GET", "/sessoes/S-0001", "", nil, 200, &orfa)
	if orfa.Status == api.Ativa {
		t.Fatalf("a órfã continua ativa: %+v", orfa)
	}
}
