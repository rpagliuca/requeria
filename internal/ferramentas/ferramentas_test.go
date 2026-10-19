package ferramentas

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rpagliuca/requerial/api"
	"github.com/rpagliuca/requerial/internal/banco"
	"github.com/rpagliuca/requerial/internal/servidor"
)

type chat struct {
	t      *testing.T
	sessao *mcp.ClientSession
	brutas []string // respostas em texto, para conferir que o token nunca vaza
}

// novoChat sobe a API e conecta um cliente MCP, como faria um agente.
func novoChat(t *testing.T, endereco string) *chat {
	t.Helper()
	f, err := Novo(endereco)
	if err != nil {
		t.Fatal(err)
	}
	f.ambiente = func(nome string) string {
		return map[string]string{"CLAUDE_EFFORT": "medium", "CLAUDE_CODE_SESSION_ID": "chat-de-teste"}[nome]
	}
	ladoServidor, ladoCliente := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := f.Servidor().Connect(ctx, ladoServidor, nil); err != nil {
		t.Fatal(err)
	}
	cliente := mcp.NewClient(&mcp.Implementation{Name: "teste", Version: "0"}, nil)
	sessao, err := cliente.Connect(ctx, ladoCliente, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessao.Close() })
	return &chat{t: t, sessao: sessao}
}

func novaAPI(t *testing.T) string {
	t.Helper()
	db, err := banco.Abrir(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(servidor.Novo(db).Handler())
	t.Cleanup(func() { srv.Close(); db.Close() })
	return srv.URL
}

// chamar invoca a ferramenta; com destino, exige sucesso e decodifica a saída estruturada.
// Sem destino, exige erro e devolve a mensagem.
func (c *chat) chamar(nome string, argumentos any, destino any) string {
	c.t.Helper()
	res, err := c.sessao.CallTool(context.Background(), &mcp.CallToolParams{Name: nome, Arguments: argumentos})
	if err != nil {
		c.t.Fatalf("%s: %v", nome, err)
	}
	texto := ""
	for _, conteudo := range res.Content {
		if t, ok := conteudo.(*mcp.TextContent); ok {
			texto += t.Text
		}
	}
	c.brutas = append(c.brutas, texto)
	if destino == nil {
		if !res.IsError {
			c.t.Fatalf("%s: esperava erro, veio %s", nome, texto)
		}
		return texto
	}
	if res.IsError {
		c.t.Fatalf("%s: %s", nome, texto)
	}
	bruto, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(bruto, destino); err != nil {
		c.t.Fatal(err)
	}
	return texto
}

func TestCicloDeSessaoPeloMCP(t *testing.T) {
	endereco := novaAPI(t)
	c := novoChat(t, endereco)

	var tarefa api.Tarefa
	c.chamar("criar_tarefa", map[string]any{"titulo": "Configurar backup", "organizacao": "Exemplo"}, &tarefa)

	if msg := c.chamar("encerrar_sessao", map[string]any{"resumo": "x"}, nil); !strings.Contains(msg, "iniciar_sessao") {
		t.Fatalf("sem sessão, a mensagem deveria orientar o agente: %q", msg)
	}

	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{
		"modelo": "Claude Opus 5.5", "servidor": "maquina-local", "tarefas": []string{tarefa.Id},
	}, &sessao)
	if sessao.Id != "S-0001" || sessao.Status != api.Ativa || sessao.Agente != "Claude Opus 5.5 Medium" ||
		sessao.Chat == nil || *sessao.Chat != "chat-de-teste" {
		t.Fatalf("sessão inesperada: %+v", sessao)
	}

	// Um segundo iniciar no mesmo chat é recusado.
	c.chamar("iniciar_sessao", map[string]any{
		"modelo": "Claude Opus 5.5", "tarefas": []string{tarefa.Id},
	}, nil)

	// Outro chat não consegue abrir sessão na mesma tarefa.
	outro := novoChat(t, endereco)
	if msg := outro.chamar("iniciar_sessao", map[string]any{
		"modelo": "Claude Fable 5.1", "esforco": "high", "tarefas": []string{tarefa.Id},
	}, nil); !strings.Contains(msg, "S-0001") {
		t.Fatalf("a recusa deveria citar a sessão ativa: %q", msg)
	}

	// Escritas depois de iniciar são atribuídas à sessão.
	c.chamar("alterar_tarefa", map[string]any{"id": tarefa.Id, "status": "em_andamento"}, &tarefa)
	var eventos Eventos
	c.chamar("listar_eventos", map[string]any{"entidade": tarefa.Id}, &eventos)
	ultimo := eventos.Eventos[len(eventos.Eventos)-1]
	if ultimo.Tipo != "tarefa_alterada" || ultimo.Autor != "S-0001" {
		t.Fatalf("alteração não atribuída à sessão: %+v", ultimo)
	}

	c.chamar("encerrar_sessao", map[string]any{"resumo": "Destino configurado; falta testar a restauração."}, &sessao)
	var liberada api.Tarefa
	c.chamar("obter_tarefa", map[string]any{"id": tarefa.Id}, &liberada)
	if liberada.Resumo != "Destino configurado; falta testar a restauração." || liberada.SessaoAtiva != nil {
		t.Fatalf("resumo não sincronizado ou tarefa presa: %+v", liberada)
	}

	// Depois de encerrar, o chat pode abrir uma sessão nova.
	c.chamar("iniciar_sessao", map[string]any{
		"modelo": "Claude Opus 5.5", "tarefas": []string{tarefa.Id},
	}, &sessao)

	for _, texto := range c.brutas {
		if strings.Contains(texto, "token") {
			t.Fatalf("o token vazou numa resposta: %s", texto)
		}
	}
}

func TestRepassePeloMCP(t *testing.T) {
	c := novoChat(t, novaAPI(t))
	var tarefa api.Tarefa
	c.chamar("criar_tarefa", map[string]any{"titulo": "Configurar backup", "organizacao": "Exemplo"}, &tarefa)
	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{
		"modelo": "Claude Opus 5.5", "tarefas": []string{tarefa.Id},
	}, &sessao)

	var nova api.Sessao
	c.chamar("repassar_sessao", map[string]any{"modelo": "Claude Fable 5.1"}, nil)
	c.chamar("repassar_sessao", map[string]any{"modelo": "Claude Fable 5.1", "esforco": "xhigh"}, &nova)
	if nova.Anterior == nil || *nova.Anterior != sessao.Id {
		t.Fatalf("repasse sem anterior: %+v", nova)
	}

	// O chat passa a operar a sessão nova.
	var atual api.Sessao
	c.chamar("sessao_atual", map[string]any{}, &atual)
	if atual.Id != nova.Id || atual.Agente != "Claude Fable 5.1 XHigh" {
		t.Fatalf("o chat não passou para a sessão nova: %+v", atual)
	}
}
