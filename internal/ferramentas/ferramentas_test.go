package ferramentas

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	return novoChatCom(t, endereco, nil, "")
}

// novoChatCom permite acrescentar variáveis de ambiente do agente e a raiz dos journals.
func novoChatCom(t *testing.T, endereco string, extra map[string]string, journals string) *chat {
	t.Helper()
	f, err := Novo(endereco)
	if err != nil {
		t.Fatal(err)
	}
	ambiente := map[string]string{"CLAUDE_EFFORT": "medium", "CLAUDE_CODE_SESSION_ID": "chat-de-teste"}
	for k, v := range extra {
		ambiente[k] = v
	}
	f.ambiente = func(nome string) string { return ambiente[nome] }
	f.ComJournals(journals)
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

func TestSessaoLancadaUsaTarefaDoAmbienteECriaJournal(t *testing.T) {
	endereco := novaAPI(t)
	var tarefa api.Tarefa
	novoChat(t, endereco).chamar("criar_tarefa", map[string]any{"titulo": "Revisar backup", "organizacao": "Exemplo"}, &tarefa)

	raiz := t.TempDir()
	c := novoChatCom(t, endereco, map[string]string{"REQUERIAL_TAREFA": tarefa.Id}, raiz)
	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{"modelo": "Claude Opus 5.5"}, &sessao)
	if len(sessao.Tarefas) != 1 || sessao.Tarefas[0] != tarefa.Id {
		t.Fatalf("a sessão não pegou a tarefa do ambiente: %+v", sessao)
	}
	arquivo := filepath.Join(raiz, sessao.Id, "journal.md")
	conteudo, err := os.ReadFile(arquivo)
	if err != nil || !strings.Contains(string(conteudo), "# Sessão "+sessao.Id) {
		t.Fatalf("journal não criado: %v %q", err, conteudo)
	}
	if sessao.Journal == nil || !strings.HasSuffix(*sessao.Journal, ":"+arquivo) {
		t.Fatalf("localizador do journal não registrado: %v", sessao.Journal)
	}
}

func TestSessaoSemTarefaVinculaDepois(t *testing.T) {
	endereco := novaAPI(t)
	c := novoChat(t, endereco)
	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{"modelo": "Claude Opus 5.5"}, &sessao)
	if len(sessao.Tarefas) != 0 {
		t.Fatalf("sessão nasceu com tarefas: %+v", sessao)
	}
	var tarefa api.Tarefa
	c.chamar("criar_tarefa", map[string]any{"titulo": "Promovida", "organizacao": "Exemplo", "status": "em_andamento"}, &tarefa)
	c.chamar("vincular_tarefa", map[string]any{"tarefa": tarefa.Id}, &sessao)
	if len(sessao.Tarefas) != 1 {
		t.Fatalf("vínculo não registrado: %+v", sessao)
	}
}

func TestAgenteLancadoAdotaASessaoDoLancador(t *testing.T) {
	endereco := novaAPI(t)
	var tarefa api.Tarefa
	novoChat(t, endereco).chamar("criar_tarefa", map[string]any{"titulo": "Lançada", "organizacao": "Exemplo"}, &tarefa)
	cliente, err := api.NewClient(endereco)
	if err != nil {
		t.Fatal(err)
	}
	chat := "chat-de-teste"
	var criada api.SessaoCriada
	resp, err := cliente.CriarSessao(context.Background(), api.NovaSessao{
		Agente: "Claude Opus 5.5 Medium", TipoAgente: api.Ia, Servidor: "maquina", Chat: &chat, Tarefas: []string{tarefa.Id},
	})
	if err := decodificar(resp, err, &criada); err != nil {
		t.Fatal(err)
	}

	raiz := t.TempDir()
	c := novoChatCom(t, endereco, map[string]string{"REQUERIAL_SESSAO": criada.Sessao.Id, "REQUERIAL_TOKEN": criada.Token}, raiz)
	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{"modelo": "Claude Opus 5.5"}, &sessao)
	if sessao.Id != criada.Sessao.Id {
		t.Fatalf("o agente abriu outra sessão em vez de adotar: %s", sessao.Id)
	}
	if _, err := os.Stat(filepath.Join(raiz, sessao.Id, "journal.md")); err != nil {
		t.Fatalf("journal da sessão adotada não criado: %v", err)
	}
	c.chamar("alterar_tarefa", map[string]any{"id": tarefa.Id, "status": "em_validacao"}, &tarefa)
	var eventos []api.Evento
	resp, err = cliente.ListarEventos(context.Background(), &api.ListarEventosParams{Entidade: &tarefa.Id})
	if err := decodificar(resp, err, &eventos); err != nil {
		t.Fatal(err)
	}
	if ultimo := eventos[len(eventos)-1]; ultimo.Autor != criada.Sessao.Id {
		t.Fatalf("alteração não atribuída à sessão adotada: %+v", ultimo)
	}
	c.chamar("encerrar_sessao", map[string]any{"resumo": "feito"}, &sessao)
	if sessao.Status != api.Encerrada {
		t.Fatalf("sessão adotada não encerrou: %+v", sessao)
	}
}

func TestRequisitosPeloMCP(t *testing.T) {
	endereco := novaAPI(t)
	c := novoChat(t, endereco)
	var origem api.Origem
	c.chamar("criar_origem", map[string]any{"nome": "Lei de dados", "tipo": "documento"}, &origem)
	var origens Origens
	c.chamar("listar_origens", map[string]any{}, &origens)
	if len(origens.Origens) != 1 {
		t.Fatalf("origens: %+v", origens)
	}
	var pai, filho api.Requisito
	c.chamar("criar_requisito", map[string]any{"titulo": "Cumprir a lei", "organizacao": "Exemplo", "origem": origem.Id}, &pai)
	c.chamar("criar_requisito", map[string]any{"titulo": "Backup fora do ambiente", "deriva_de": []string{pai.Id}}, &filho)
	c.chamar("derivar_requisito", map[string]any{"id": pai.Id, "pai": filho.Id}, nil)
	var tarefa api.Tarefa
	c.chamar("criar_tarefa", map[string]any{"titulo": "Configurar backup", "organizacao": "Exemplo"}, &tarefa)
	c.chamar("vincular_requisito", map[string]any{"tarefa": tarefa.Id, "requisito": filho.Id}, &tarefa)
	c.chamar("obter_requisito", map[string]any{"id": filho.Id}, &filho)
	if len(filho.DerivaDe) != 1 || len(filho.Tarefas) != 1 || filho.Organizacao != "Exemplo" {
		t.Fatalf("requisito pelo MCP: %+v", filho)
	}
	var busca Requisitos
	c.chamar("buscar_requisitos", map[string]any{"texto": "backup"}, &busca)
	if len(busca.Requisitos) != 1 || busca.Requisitos[0].Id != filho.Id {
		t.Fatalf("busca pelo MCP: %+v", busca)
	}
	var arvore api.ArvoreDeRequisitos
	c.chamar("arvore_de_requisitos", map[string]any{}, &arvore)
	if arvore.Requisitos != 2 {
		t.Fatalf("árvore pelo MCP: %+v", arvore)
	}
	var tp api.Template
	corpo, _ := json.Marshal(api.NovoTemplate{Titulo: "Revisar", Organizacao: "Exemplo"})
	resp, err := http.Post(endereco+"/templates", "application/json", bytes.NewReader(corpo))
	if err != nil || resp.StatusCode != 201 {
		t.Fatalf("criar template: %v %v", err, resp)
	}
	json.NewDecoder(resp.Body).Decode(&tp)
	resp.Body.Close()
	var templates Templates
	c.chamar("listar_templates", map[string]any{}, &templates)
	if len(templates.Templates) != 1 || templates.Templates[0].Id != tp.Id {
		t.Fatalf("templates pelo MCP: %+v", templates)
	}
	var instancia api.Tarefa
	c.chamar("instanciar_template", map[string]any{"template": tp.Id, "contexto": "o PR 12"}, &instancia)
	if instancia.Template == nil || *instancia.Template != tp.Id {
		t.Fatalf("instância pelo MCP: %+v", instancia)
	}
	var ativos AgentesAtivos
	c.chamar("agentes_ativos", map[string]any{}, &ativos)
	if ativos.Agentes == nil {
		t.Fatalf("agentes ativos: %+v", ativos)
	}
	var execucao api.Execucao
	c.chamar("registrar_execucao", map[string]any{"requisito": pai.Id, "evidencia": "conferido", "validade_dias": 30}, &execucao)
	c.chamar("obter_requisito", map[string]any{"id": pai.Id}, &pai)
	if pai.EstadoProprio != api.Validado || pai.Estado != api.NaoAtendido || pai.UltimaExecucao == nil || pai.UltimaExecucao.Id != execucao.Id {
		t.Fatalf("execução pelo MCP: %+v", pai)
	}
}

func TestLembretesEEsperaPeloMCP(t *testing.T) {
	c := novoChat(t, novaAPI(t))
	var tarefa api.Tarefa
	c.chamar("criar_tarefa", map[string]any{"titulo": "Pagar contas", "organizacao": "Exemplo"}, &tarefa)
	var sessao api.Sessao
	c.chamar("iniciar_sessao", map[string]any{"modelo": "Claude Opus 5.5", "tarefas": []string{tarefa.Id}}, &sessao)

	quando := time.Now().Add(2 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	if msg := c.chamar("criar_lembrete", map[string]any{"texto": "x", "quando": "amanhã", "organizacao": "Exemplo"}, nil); !strings.Contains(msg, "RFC 3339") {
		t.Fatalf("hora inválida deveria orientar: %q", msg)
	}
	var l ResumoDeLembrete
	c.chamar("criar_lembrete", map[string]any{"texto": "ligar para o contador", "quando": quando, "organizacao": "Exemplo", "tarefa": tarefa.Id}, &l)
	if l.Id != "L-0001" || l.Status != "agendado" || l.Quando != quando {
		t.Fatalf("lembrete: %+v", l)
	}
	var proximos Lembretes
	c.chamar("proximos_lembretes", map[string]any{"horas": 1}, &proximos)
	if len(proximos.Lembretes) != 0 {
		t.Fatalf("fora da janela: %+v", proximos)
	}
	c.chamar("proximos_lembretes", map[string]any{}, &proximos)
	if len(proximos.Lembretes) != 1 {
		t.Fatalf("dentro da janela padrão: %+v", proximos)
	}
	c.chamar("adiar_lembrete", map[string]any{"id": "L-0001", "minutos": 30}, &l)
	if l.Status != "adiado" {
		t.Fatalf("adiado: %+v", l)
	}
	c.chamar("confirmar_lembrete", map[string]any{"id": "L-0001"}, &l)
	if l.Status != "confirmado" {
		t.Fatalf("confirmado: %+v", l)
	}

	var eventos Eventos
	c.chamar("listar_eventos", map[string]any{"entidade": "L-0001"}, &eventos)
	for _, e := range eventos.Eventos {
		if e.Autor != sessao.Id {
			t.Fatalf("evento do lembrete sem a sessão como autora: %+v", e)
		}
	}

	c.chamar("aguardar_resposta", map[string]any{"mensagem": "aprovar o PR"}, &sessao)
	if sessao.Aguardando == nil || sessao.Aguardando.Mensagem != "aprovar o PR" {
		t.Fatalf("aguardando: %+v", sessao)
	}
	var retomada api.Sessao
	c.chamar("retomar_trabalho", map[string]any{}, &retomada)
	if retomada.Aguardando != nil {
		t.Fatalf("retomada: %+v", retomada)
	}
}
