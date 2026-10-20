// Package ferramentas expõe a API do Requerial como servidor MCP para agentes de IA.
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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rpagliuca/requerial/api"
)

// Ferramentas guarda o cliente da API e a sessão aberta neste processo.
type Ferramentas struct {
	cliente *api.Client
	// ambiente lê as variáveis que o cliente do agente passa ao processo MCP, como
	// CLAUDE_EFFORT e CLAUDE_CODE_SESSION_ID. Substituível nos testes.
	ambiente func(string) string
	// journals é a raiz onde cada sessão ganha a pasta do seu journal; vazio desliga.
	journals string

	mu     sync.Mutex
	sessao string
	token  string
	// adotada diz que a sessão veio pronta do lançador (REQUERIAL_SESSAO e REQUERIAL_TOKEN), e
	// conferida indica que o ambiente já foi lido.
	adotada   bool
	conferida bool
}

// Novo cria as ferramentas apontando para a API no endereço dado.
func Novo(endereco string) (*Ferramentas, error) {
	cliente, err := api.NewClient(endereco)
	if err != nil {
		return nil, err
	}
	return &Ferramentas{cliente: cliente, ambiente: os.Getenv}, nil
}

// ComJournals faz cada sessão iniciada sem journal ganhar a pasta <dir>/<S-0001>/journal.md.
func (f *Ferramentas) ComJournals(dir string) *Ferramentas {
	f.journals = dir
	return f
}

// Servidor monta o servidor MCP com todas as ferramentas registradas.
func (f *Ferramentas) Servidor() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "requerial", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Tarefas e Sessões de Trabalho do Requerial. Regra: toda sessão de trabalho é registrada " +
			"antes da primeira ação que muda estado, qualquer que seja a origem do chat (carregue a skill requerial). " +
			"Comece por sessao_atual (o lançador pode já ter aberto a sua); senão, iniciar_sessao na tarefa, ou sem " +
			"tarefa e depois criar_tarefa + vincular_tarefa. Ao terminar, encerrar_sessao com um resumo para o próximo agente.",
	})

	mcp.AddTool(s, &mcp.Tool{Name: "obter_tarefa", Description: "Obtém uma tarefa, incluindo o resumo para retomar e a sessão ativa."}, f.obterTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_tarefa", Description: "Cria uma tarefa."}, f.criarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "alterar_tarefa", Description: "Altera campos de uma tarefa."}, f.alterarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "historico_da_tarefa", Description: "Lista as sessões que trabalharam na tarefa, da mais recente para a mais antiga."}, f.historicoDaTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "iniciar_sessao", Description: "Abre a Sessão de Trabalho deste chat, nas tarefas dadas ou sem nenhuma (vincule depois). Cria a pasta do journal. Recusa tarefa que já tem sessão ativa."}, f.iniciarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "sessao_atual", Description: "Mostra a Sessão de Trabalho aberta neste chat."}, f.sessaoAtual)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_tarefa", Description: "Liga mais uma tarefa à sessão deste chat."}, f.vincularTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "atualizar_journal", Description: "Atualiza os localizadores do journal e dos artefatos da sessão deste chat."}, f.atualizarJournal)
	mcp.AddTool(s, &mcp.Tool{Name: "encerrar_sessao", Description: "Encerra a sessão deste chat e grava o resumo em cada tarefa dela."}, f.encerrarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "repassar_sessao", Description: "Troca de agente: encerra a sessão deste chat e abre outra com o agente novo, nas mesmas tarefas."}, f.repassarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "obter_requisito", Description: "Obtém um requisito: origem, cadeia (deriva_de acima, derivados abaixo) e tarefas que tentam atendê-lo."}, f.obterRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_requisito", Description: "Cria um requisito, com origem e os requisitos dos quais deriva (a organização vem do primeiro, se omitida)."}, f.criarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "alterar_requisito", Description: "Altera título, descrição, organização, origem ou prioridade (triagem: alta, media, baixa; vazio volta a não triado) de um requisito."}, f.alterarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "derivar_requisito", Description: "Registra que um requisito deriva de outro (um requisito pode derivar de vários). Recusa ciclos."}, f.derivarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_requisito", Description: "Registra que uma tarefa tenta atender um requisito."}, f.vincularRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "registrar_execucao", Description: "Registra uma execução de conformidade: o requisito foi verificado e está atendido, com a evidência e a validade em dias (depois dela, a validação expira). Só registre o que você verificou de fato."}, f.registrarExecucao)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_origens", Description: "Lista as origens de requisitos (pessoas e documentos), para reaproveitar em vez de duplicar."}, f.listarOrigens)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_origem", Description: "Cria uma origem de requisitos: uma pessoa (quem pede) ou um documento (lei, norma, contrato)."}, f.criarOrigem)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_eventos", Description: "Lista o log de eventos, opcionalmente de uma entidade (T-0001, S-0001)."}, f.listarEventos)
	return s
}

// autorizar acrescenta o token da sessão aberta, se houver, para atribuir a escrita a ela.
func (f *Ferramentas) autorizar(ctx context.Context, req *http.Request) error {
	f.adotar(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	return nil
}

// adotar assume a sessão que o lançador abriu para este agente, se houver, e cria a pasta do journal
// dela. Roda uma vez, na primeira ferramenta que precisa da sessão.
func (f *Ferramentas) adotar(ctx context.Context) {
	f.mu.Lock()
	if f.conferida || f.sessao != "" {
		f.mu.Unlock()
		return
	}
	f.conferida = true
	id, token := f.ambiente("REQUERIAL_SESSAO"), f.ambiente("REQUERIAL_TOKEN")
	if id == "" || token == "" {
		f.mu.Unlock()
		return
	}
	f.sessao, f.token, f.adotada = id, token, true
	f.mu.Unlock()
	if f.journals == "" {
		return
	}
	var x api.Sessao
	resp, err := f.cliente.ObterSessao(ctx, id)
	if err := decodificar(resp, err, &x); err != nil || x.Journal != nil {
		return
	}
	if _, err := f.criarJournal(ctx, x, x.Servidor); err != nil {
		log.Printf("criar o journal da sessão %s: %v", id, err)
	}
}

// sessaoAberta devolve a sessão deste processo ou um erro que diz o que fazer.
func (f *Ferramentas) sessaoAberta() (string, error) {
	f.adotar(context.Background())
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
	Tarefas  []string `json:"tarefas,omitempty" jsonschema:"tarefas em que a sessão vai trabalhar; pode ficar vazio e ser vinculado depois. Padrão: a tarefa em REQUERIAL_TAREFA, se o lançador definiu"`
}

type Repasse struct {
	Modelo   string  `json:"modelo" jsonschema:"provider e modelo do agente novo, como 'Claude Fable 5.1'"`
	Esforco  string  `json:"esforco" jsonschema:"esforço do agente novo (low, medium, high, xhigh, max)"`
	Servidor *string `json:"servidor,omitempty" jsonschema:"padrão: o da sessão anterior"`
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
	return api.NomeDoAgente(modelo, nivel), nil
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
	f.adotar(ctx)
	f.mu.Lock()
	adotada, id := f.adotada, f.sessao
	f.mu.Unlock()
	if adotada {
		// O lançador já abriu a sessão deste agente: é ela.
		var x api.Sessao
		resp, err := f.cliente.ObterSessao(ctx, id)
		return nil, x, decodificar(resp, err, &x)
	}
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
	tarefas := in.Tarefas
	if len(tarefas) == 0 {
		if t := f.ambiente("REQUERIAL_TAREFA"); t != "" {
			tarefas = []string{t}
		}
	}
	var criada api.SessaoCriada
	resp, err := f.cliente.CriarSessao(ctx, api.NovaSessao{
		Agente: agente, TipoAgente: api.Ia, Chat: f.chatAtual(in.Chat), Servidor: servidor, Journal: in.Journal, Tarefas: tarefas,
	})
	if err := decodificar(resp, err, &criada); err != nil {
		return nil, api.Sessao{}, err
	}
	f.guardar(criada)
	if in.Journal == nil && f.journals != "" {
		sessao, err := f.criarJournal(ctx, criada.Sessao, servidor)
		if err != nil {
			return nil, criada.Sessao, fmt.Errorf("sessão %s aberta, mas a pasta do journal falhou: %w", criada.Sessao.Id, err)
		}
		return nil, sessao, nil
	}
	return nil, criada.Sessao, nil
}

// criarJournal cria <journals>/<S-0001>/journal.md e registra o localizador na sessão.
func (f *Ferramentas) criarJournal(ctx context.Context, x api.Sessao, servidor string) (api.Sessao, error) {
	pasta := filepath.Join(f.journals, x.Id)
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return x, err
	}
	arquivo := filepath.Join(pasta, "journal.md")
	if _, err := os.Stat(arquivo); errors.Is(err, os.ErrNotExist) {
		cabecalho := fmt.Sprintf("# Sessão %s\n\n- Tarefas: %s\n- Agente: %s\n- Máquina: %s\n- Início: %s\n\n## Journal\n\n",
			x.Id, strings.Join(x.Tarefas, ", "), x.Agente, servidor, x.IniciadaEm.Format(time.RFC3339))
		if err := os.WriteFile(arquivo, []byte(cabecalho), 0o644); err != nil {
			return x, err
		}
	}
	journal := servidor + ":" + arquivo
	var alterada api.Sessao
	resp, err := f.cliente.AlterarSessao(ctx, x.Id, api.AlteracaoSessao{Journal: &journal}, f.autorizar)
	return alterada, decodificar(resp, err, &alterada)
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

type AlteracaoRequisito struct {
	Id string `json:"id" jsonschema:"identificador do requisito, como R-0001"`
	api.AlteracaoRequisito
}

type Derivacao struct {
	Id  string `json:"id" jsonschema:"requisito que deriva, como R-0002"`
	Pai string `json:"pai" jsonschema:"requisito do qual ele deriva, como R-0001"`
}

type VinculoRequisito struct {
	Tarefa    string `json:"tarefa" jsonschema:"tarefa, como T-0001"`
	Requisito string `json:"requisito" jsonschema:"requisito que a tarefa tenta atender, como R-0001"`
}

type NovaExecucao struct {
	Requisito string `json:"requisito" jsonschema:"requisito verificado, como R-0001"`
	api.NovaExecucao
}

type Origens struct {
	Origens []api.Origem `json:"origens"`
}

func (f *Ferramentas) obterRequisito(ctx context.Context, _ *mcp.CallToolRequest, in Id) (*mcp.CallToolResult, api.Requisito, error) {
	var out api.Requisito
	resp, err := f.cliente.ObterRequisito(ctx, in.Id)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) criarRequisito(ctx context.Context, _ *mcp.CallToolRequest, in api.NovoRequisito) (*mcp.CallToolResult, api.Requisito, error) {
	var out api.Requisito
	resp, err := f.cliente.CriarRequisito(ctx, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) alterarRequisito(ctx context.Context, _ *mcp.CallToolRequest, in AlteracaoRequisito) (*mcp.CallToolResult, api.Requisito, error) {
	var out api.Requisito
	resp, err := f.cliente.AlterarRequisito(ctx, in.Id, in.AlteracaoRequisito, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) derivarRequisito(ctx context.Context, _ *mcp.CallToolRequest, in Derivacao) (*mcp.CallToolResult, api.Requisito, error) {
	var out api.Requisito
	resp, err := f.cliente.DerivarRequisito(ctx, in.Id, api.Derivacao{Pai: in.Pai}, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) vincularRequisito(ctx context.Context, _ *mcp.CallToolRequest, in VinculoRequisito) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.VincularRequisito(ctx, in.Tarefa, api.VinculoRequisito{Requisito: in.Requisito}, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) registrarExecucao(ctx context.Context, _ *mcp.CallToolRequest, in NovaExecucao) (*mcp.CallToolResult, api.Execucao, error) {
	var out api.Execucao
	resp, err := f.cliente.RegistrarExecucao(ctx, in.Requisito, in.NovaExecucao, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) listarOrigens(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, Origens, error) {
	var out Origens
	resp, err := f.cliente.ListarOrigens(ctx)
	return nil, out, decodificar(resp, err, &out.Origens)
}

func (f *Ferramentas) criarOrigem(ctx context.Context, _ *mcp.CallToolRequest, in api.NovaOrigem) (*mcp.CallToolResult, api.Origem, error) {
	var out api.Origem
	resp, err := f.cliente.CriarOrigem(ctx, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}
