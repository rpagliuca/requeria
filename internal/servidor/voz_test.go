package servidor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/voz"
)

type vozFalsa struct {
	tela     string
	semTmux  bool
	escritos []string
	teclas   []string
	conversa string
}

func (v *vozFalsa) Tela(api.Sessao) (string, error) {
	if v.semTmux {
		return "", os.ErrNotExist
	}
	return v.tela, nil
}
func (v *vozFalsa) Escrever(_ api.Sessao, texto string) error {
	v.escritos = append(v.escritos, texto)
	return nil
}
func (v *vozFalsa) Teclar(_ api.Sessao, tecla string) error {
	v.teclas = append(v.teclas, tecla)
	return nil
}
func (v *vozFalsa) Conversa(api.Sessao) string { return v.conversa }

const (
	telaOciosa      = "✻ done\n❯ \n  ⏵⏵ bypass permissions on\n"
	telaTrabalhando = "✢ Thinking…\n❯ \n  ⏵⏵ bypass permissions on · esc to interrupt\n"
	telaPermissao   = " Do you want to proceed?\n ❯ 1. Yes\n   2. No\n Esc to cancel\n"
)

func TestVozAssociaEnviaEConfirma(t *testing.T) {
	a := novoAmbiente(t)
	esperaDaTecla, intervaloDaEspera = 0, 10*time.Millisecond
	v := &vozFalsa{tela: telaOciosa, conversa: filepath.Join(t.TempDir(), "c.jsonl")}
	a.servidor.ComVoz(v)

	var estado api.Voz
	a.chamar("GET", "/voz", "", nil, 200, &estado)
	if estado.Estado != api.VozSemSessao || estado.Sessao != nil {
		t.Fatalf("sem associação: %+v", estado)
	}
	a.chamar("POST", "/voz/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)

	tarefa := a.tarefa("Deploy")
	semSessao := a.tarefa("Sem agente")
	a.chamar("PUT", "/voz/sessao", "", api.AssociacaoDeVoz{Id: semSessao.Id}, 409, nil)
	s := a.sessao("Claude Opus 5.5 Medium", tarefa.Id)
	a.chamar("PUT", "/voz/sessao", "", api.AssociacaoDeVoz{Id: strings.ToLower(tarefa.Id)}, 200, &estado)
	if estado.Sessao == nil || *estado.Sessao != s.Sessao.Id || estado.Estado != api.VozOcioso {
		t.Fatalf("associação pela tarefa: %+v", estado)
	}

	var pedido api.PedidoDeVoz
	a.chamar("POST", "/voz/mensagens", "", api.MensagemDeVoz{Texto: "como está\n o deploy?"}, 201, &pedido)
	a.chamar("POST", "/voz/mensagens", "", api.MensagemDeVoz{Texto: "e o teste?"}, 201, &pedido)
	if pedido.Pedido != 2 || len(v.escritos) != 2 || !strings.HasPrefix(v.escritos[0], voz.Prefixo+"1] como está o deploy?") {
		t.Fatalf("pedidos: %+v %q", pedido, v.escritos)
	}

	// Tela de permissão: a frase cairia na escolha; a tecla responde.
	v.tela = telaPermissao
	a.chamar("POST", "/voz/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)
	a.chamar("GET", "/voz", "", nil, 200, &estado)
	if estado.Estado != api.VozPedindoPermissao || estado.Tela == nil || !strings.Contains(*estado.Tela, "1. Yes") {
		t.Fatalf("permissão: %+v", estado)
	}
	a.chamar("POST", "/voz/teclas", "", api.TeclaDeVoz{Tecla: "1"}, 200, nil)
	v.tela = telaOciosa
	a.chamar("POST", "/voz/teclas", "", api.TeclaDeVoz{Tecla: "1"}, 409, nil)
	a.chamar("POST", "/voz/teclas", "", api.TeclaDeVoz{Tecla: "x"}, 400, nil)
	a.chamar("POST", "/voz/teclas", "", api.TeclaDeVoz{Tecla: api.TeclaEscape}, 200, nil)
	if strings.Join(v.teclas, ",") != "1,Escape" {
		t.Fatalf("teclas: %v", v.teclas)
	}

	// Ponteiro: entregue não é ouvido até a confirmação.
	linhas := `{"type":"user","uuid":"u1","timestamp":"2026-10-22T10:00:00Z","message":{"content":"[rex voz · pedido 1] como está o deploy?` + sufixoDaVoz + `"}}
{"type":"assistant","uuid":"u2","timestamp":"2026-10-22T10:00:09Z","message":{"content":[{"type":"text","text":"Terminou."}]}}
`
	os.WriteFile(v.conversa, []byte(linhas), 0o644)
	a.chamar("GET", "/voz?entregar=true", "", nil, 200, &estado)
	if len(estado.Novidades) != 2 || estado.Novidades[0].Texto != "como está o deploy?" || *estado.Novidades[1].Pedido != 1 {
		t.Fatalf("novidades: %+v", estado.Novidades)
	}
	a.chamar("GET", "/voz", "", nil, 200, &estado)
	if len(estado.Novidades) != 2 {
		t.Fatalf("sem confirmação, continuam novas: %+v", estado.Novidades)
	}
	a.chamar("POST", "/voz/confirmacao", "", nil, 204, nil)
	a.chamar("GET", "/voz", "", nil, 200, &estado)
	if len(estado.Novidades) != 0 {
		t.Fatalf("depois da confirmação: %+v", estado.Novidades)
	}
	a.chamar("POST", "/voz/confirmacao", "", nil, 204, nil) // sem proposta, nada

	// esperar: com o agente trabalhando e calado, segura até o limite.
	v.tela = telaTrabalhando
	inicio := time.Now()
	a.chamar("GET", "/voz?esperar=1", "", nil, 200, &estado)
	if time.Since(inicio) < time.Second || estado.Estado != api.VozTrabalhando {
		t.Fatalf("não esperou: %v %+v", time.Since(inicio), estado)
	}

	v.semTmux = true
	a.chamar("GET", "/voz", "", nil, 200, &estado)
	if estado.Estado != api.VozSemAgente {
		t.Fatalf("sem tmux: %+v", estado)
	}
	a.chamar("POST", "/voz/mensagens", "", api.MensagemDeVoz{Texto: "oi"}, 409, nil)
}
