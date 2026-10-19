// Package ferramentas expõe a API do requerial como servidor MCP para agentes de IA.
//
// É uma camada fina sobre a API HTTP: quem escreve no banco continua sendo só o servidor.
// Cada processo MCP corresponde a um chat do agente e guarda o token da sessão aberta nele;
// o token nunca aparece nas respostas, e toda escrita feita depois de iniciar a sessão é
// atribuída a ela.
package ferramentas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rpagliuca/requerial/api"
)

// Ferramentas guarda o cliente da API e a sessão aberta neste processo.
type Ferramentas struct {
	cliente *api.Client
	// ambiente lê as variáveis que o cliente do agente passa ao processo MCP, como
	// CLAUDE_EFFORT e CLAUDE_CODE_SESSION_ID. Substituível nos testes.
	ambiente func(string) string

	mu     sync.Mutex
	sessao string
	token  string
}

// Novo cria as ferramentas apontando para a API no endereço dado.
func Novo(endereco string) (*Ferramentas, error) {
	cliente, err := api.NewClient(endereco)
	if err != nil {
		return nil, err
	}
	return &Ferramentas{cliente: cliente, ambiente: os.Getenv}, nil
}

// Servidor monta o servidor MCP com todas as ferramentas registradas.
func (f *Ferramentas) Servidor() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "requerial", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Tarefas e Sessões de Trabalho do requerial. Ao começar a trabalhar numa tarefa, " +
			"chame iniciar_sessao; ao terminar, chame encerrar_sessao com um resumo para o próximo agente.",
	})

	mcp.AddTool(s, &mcp.Tool{Name: "obter_tarefa", Description: "Obtém uma tarefa, incluindo o resumo para retomar e a sessão ativa."}, f.obterTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_tarefa", Description: "Cria uma tarefa."}, f.criarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "alterar_tarefa", Description: "Altera campos de uma tarefa."}, f.alterarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "historico_da_tarefa", Description: "Lista as sessões que trabalharam na tarefa, da mais recente para a mais antiga."}, f.historicoDaTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "iniciar_sessao", Description: "Abre a Sessão de Trabalho deste chat nas tarefas dadas. Recusa tarefa que já tem sessão ativa."}, f.iniciarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "sessao_atual", Description: "Mostra a Sessão de Trabalho aberta neste chat."}, f.sessaoAtual)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_tarefa", Description: "Liga mais uma tarefa à sessão deste chat."}, f.vincularTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "atualizar_journal", Description: "Atualiza os localizadores do journal e dos artefatos da sessão deste chat."}, f.atualizarJournal)
	mcp.AddTool(s, &mcp.Tool{Name: "encerrar_sessao", Description: "Encerra a sessão deste chat e grava o resumo em cada tarefa dela."}, f.encerrarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "repassar_sessao", Description: "Troca de agente: encerra a sessão deste chat e abre outra com o agente novo, nas mesmas tarefas."}, f.repassarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_eventos", Description: "Lista o log de eventos, opcionalmente de uma entidade (T-0001, S-0001)."}, f.listarEventos)
	return s
}

// autorizar acrescenta o token da sessão aberta, se houver, para atribuir a escrita a ela.
func (f *Ferramentas) autorizar(ctx context.Context, req *http.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	return nil
}

// sessaoAberta devolve a sessão deste processo ou um erro que diz o que fazer.
func (f *Ferramentas) sessaoAberta() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessao == "" {
		return "", errors.New("não há sessão aberta neste chat; chame iniciar_sessao primeiro")
	}
	return f.sessao, nil
}

func (f *Ferramentas) guardar(criada api.SessaoCriada) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessao, f.token = criada.Sessao.Id, criada.Token
}

// decodificar lê a resposta da API: o corpo em destino, ou a mensagem de erro dela.
func decodificar(resp *http.Response, err error, destino any) error {
	if err != nil {
		return fmt.Errorf("chamar a API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e api.Erro
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Mensagem != "" {
			return errors.New(e.Mensagem)
		}
		return fmt.Errorf("a API respondeu %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(destino)
}

type Id struct {
	Id string `json:"id" jsonschema:"identificador, como T-0001"`
}

type Sessoes struct {
	Sessoes []api.Sessao `json:"sessoes"`
}

type AlteracaoTarefa struct {
	Id string `json:"id" jsonschema:"identificador da tarefa, como T-0001"`
	api.AlteracaoTarefa
}

type NovaSessao struct {
	Modelo   string   `json:"modelo" jsonschema:"provider e modelo, sem o esforço, como 'Claude Opus 5.5'"`
	Esforco  *string  `json:"esforco,omitempty" jsonschema:"esforço de raciocínio (low, medium, high, xhigh, max); padrão: o informado pelo cliente do agente"`
	Chat     *string  `json:"chat,omitempty" jsonschema:"identificador do chat; padrão: o informado pelo cliente do agente"`
	Servidor *string  `json:"servidor,omitempty" jsonschema:"máquina onde a sessão roda; padrão: o nome desta máquina"`
	Journal  *string  `json:"journal,omitempty" jsonschema:"localizador do journal: <maquina>:<caminho>"`
	Tarefas  []string `json:"tarefas" jsonschema:"tarefas em que a sessão vai trabalhar"`
}

type Repasse struct {
	Modelo   string  `json:"modelo" jsonschema:"provider e modelo do agente novo, como 'Claude Fable 5.1'"`
	Esforco  string  `json:"esforco" jsonschema:"esforço do agente novo (low, medium, high, xhigh, max)"`
	Servidor *string `json:"servidor,omitempty" jsonschema:"padrão: o da sessão anterior"`
}

// nomesDeEsforco escreve os níveis de esforço como aparecem no nome do agente.
var nomesDeEsforco = map[string]string{
	"low": "Low", "medium": "Medium", "high": "High", "xhigh": "XHigh", "max": "Max",
}

// agente monta o nome imutável do agente: provider, modelo e esforço, como "Claude Opus 5.5 Medium".
// Sem esforço explícito, usa o que o cliente do agente informou ao processo MCP.
func (f *Ferramentas) agente(modelo string, esforco *string) (string, error) {
	modelo = strings.TrimSpace(modelo)
	if modelo == "" {
		return "", errors.New("informe o modelo, com o provider na frente, como 'Claude Opus 5.5'")
	}
	nivel := ""
	if esforco != nil {
		nivel = strings.TrimSpace(*esforco)
	}
	if nivel == "" {
		nivel = f.ambiente("CLAUDE_EFFORT")
	}
	if nivel == "" {
		return "", errors.New("o esforço não foi informado e o cliente do agente não o forneceu; pergunte ao usuário e passe em 'esforco'")
	}
	nome, ok := nomesDeEsforco[strings.ToLower(nivel)]
	if !ok {
		nome = strings.ToUpper(nivel[:1]) + nivel[1:]
	}
	return modelo + " " + nome, nil
}

func (f *Ferramentas) chatAtual(informado *string) *string {
	if informado != nil && *informado != "" {
		return informado
	}
	if id := f.ambiente("CLAUDE_CODE_SESSION_ID"); id != "" {
		return &id
	}
	return nil
}

type Vazio struct{}

type FiltroEventos struct {
	Entidade *string `json:"entidade,omitempty" jsonschema:"T-0001, S-0001 etc."`
}

type Eventos struct {
	Eventos []api.Evento `json:"eventos"`
}

func (f *Ferramentas) obterTarefa(ctx context.Context, _ *mcp.CallToolRequest, in Id) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.ObterTarefa(ctx, in.Id)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) criarTarefa(ctx context.Context, _ *mcp.CallToolRequest, in api.NovaTarefa) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.CriarTarefa(ctx, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) alterarTarefa(ctx context.Context, _ *mcp.CallToolRequest, in AlteracaoTarefa) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.AlterarTarefa(ctx, in.Id, in.AlteracaoTarefa, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) historicoDaTarefa(ctx context.Context, _ *mcp.CallToolRequest, in Id) (*mcp.CallToolResult, Sessoes, error) {
	var out Sessoes
	resp, err := f.cliente.ListarSessoesDaTarefa(ctx, in.Id)
	return nil, out, decodificar(resp, err, &out.Sessoes)
}

func (f *Ferramentas) iniciarSessao(ctx context.Context, _ *mcp.CallToolRequest, in NovaSessao) (*mcp.CallToolResult, api.Sessao, error) {
	if atual, err := f.sessaoAberta(); err == nil {
		return nil, api.Sessao{}, fmt.Errorf("este chat já tem a sessão %s aberta; use vincular_tarefa ou encerre-a antes", atual)
	}
	agente, err := f.agente(in.Modelo, in.Esforco)
	if err != nil {
		return nil, api.Sessao{}, err
	}
	servidor, _ := os.Hostname()
	if in.Servidor != nil && *in.Servidor != "" {
		servidor = *in.Servidor
	}
	var criada api.SessaoCriada
	resp, err := f.cliente.CriarSessao(ctx, api.NovaSessao{
		Agente: agente, TipoAgente: api.Ia, Chat: f.chatAtual(in.Chat), Servidor: servidor, Journal: in.Journal, Tarefas: in.Tarefas,
	})
	if err := decodificar(resp, err, &criada); err != nil {
		return nil, api.Sessao{}, err
	}
	f.guardar(criada)
	return nil, criada.Sessao, nil
}

func (f *Ferramentas) sessaoAtual(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.ObterSessao(ctx, id)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) vincularTarefa(ctx context.Context, _ *mcp.CallToolRequest, in api.Vinculo) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.VincularTarefa(ctx, id, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) atualizarJournal(ctx context.Context, _ *mcp.CallToolRequest, in api.AlteracaoSessao) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.AlterarSessao(ctx, id, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) encerrarSessao(ctx context.Context, _ *mcp.CallToolRequest, in api.Encerramento) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.EncerrarSessao(ctx, id, in, f.autorizar)
	if err := decodificar(resp, err, &out); err != nil {
		return nil, out, err
	}
	f.guardar(api.SessaoCriada{})
	return nil, out, nil
}

func (f *Ferramentas) repassarSessao(ctx context.Context, _ *mcp.CallToolRequest, in Repasse) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	// O esforço é obrigatório aqui: o do ambiente é o do início do chat e pode estar desatualizado.
	if strings.TrimSpace(in.Esforco) == "" {
		return nil, api.Sessao{}, errors.New("informe o esforço do agente novo em 'esforco'")
	}
	agente, err := f.agente(in.Modelo, &in.Esforco)
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var criada api.SessaoCriada
	resp, err := f.cliente.RepassarSessao(ctx, id, api.Repasse{Agente: agente, TipoAgente: api.Ia, Servidor: in.Servidor}, f.autorizar)
	if err := decodificar(resp, err, &criada); err != nil {
		return nil, api.Sessao{}, err
	}
	f.guardar(criada)
	return nil, criada.Sessao, nil
}

func (f *Ferramentas) listarEventos(ctx context.Context, _ *mcp.CallToolRequest, in FiltroEventos) (*mcp.CallToolResult, Eventos, error) {
	var out Eventos
	resp, err := f.cliente.ListarEventos(ctx, &api.ListarEventosParams{Entidade: in.Entidade})
	return nil, out, decodificar(resp, err, &out.Eventos)
}
