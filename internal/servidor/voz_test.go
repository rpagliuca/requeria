package servidor

import (
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

	// Ponteiro: ler sem ouvir não anda. O sufixo de quando era o atalho da Siri também sai.
	linhas := `{"type":"user","uuid":"u1","timestamp":"2026-10-22T10:00:00Z","message":{"content":"[rex voz · pedido 1] como está o deploy?` + sufixosDaVoz[1] + `"}}
{"type":"assistant","uuid":"u2","timestamp":"2026-10-22T10:00:09Z","message":{"content":[{"type":"text","text":"Terminou."}]}}
`
	os.WriteFile(v.conversa, []byte(linhas), 0o644)
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if len(estado.Novidades) != 2 || estado.Novidades[0].Texto != "como está o deploy?" || *estado.Novidades[1].Pedido != 1 {
		t.Fatalf("novidades: %+v", estado.Novidades)
	}
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if len(estado.Novidades) != 2 {
		t.Fatalf("sem ouvir, continuam novas: %+v", estado.Novidades)
	}

	// ouvir (conector do app): viram ouvidas na hora.
	linhas += `{"type":"assistant","uuid":"u3","timestamp":"2026-10-22T10:01:00Z","message":{"content":[{"type":"text","text":"E os testes passaram."}]}}
`
	os.WriteFile(v.conversa, []byte(linhas), 0o644)
	a.chamar("GET", caminho+"?ouvir=true", "", nil, 200, &estado)
	if len(estado.Novidades) != 3 {
		t.Fatalf("com ouvir: %+v", estado.Novidades)
	}
	a.chamar("GET", caminho, "", nil, 200, &estado)
	if len(estado.Novidades) != 0 {
		t.Fatalf("ouvir devia marcar na hora: %+v", estado.Novidades)
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
		t.Fatalf("panorama: %+v", panorama)
	}
	p := panorama[0]
	if p.Sessao != s.Sessao.Id || p.Aguardando == nil || *p.Aguardando != "aprovar o PR" || p.Estado != api.VozOcioso ||
		p.NaoOuvidas != 1 || p.Ultima == nil || *p.Ultima != "PR aberto." || (*p.Titulos)[0] != "Deploy do cartório" {
		t.Fatalf("panorama: %+v", p)
	}
}
