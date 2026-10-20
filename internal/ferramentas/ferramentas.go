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
	mcp.AddTool(s, &mcp.Tool{Name: "criar_tarefa", Description: "Cria uma tarefa, já ligada aos requisitos e componentes informados."}, f.criarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "alterar_tarefa", Description: "Altera campos de uma tarefa."}, f.alterarTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "historico_da_tarefa", Description: "Lista as sessões que trabalharam na tarefa, da mais recente para a mais antiga."}, f.historicoDaTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "iniciar_sessao", Description: "Abre a Sessão de Trabalho deste chat, nas tarefas dadas ou sem nenhuma (vincule depois). Cria a pasta do journal. Recusa tarefa que já tem sessão ativa."}, f.iniciarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "sessao_atual", Description: "Mostra a Sessão de Trabalho aberta neste chat."}, f.sessaoAtual)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_tarefa", Description: "Liga mais uma tarefa à sessão deste chat."}, f.vincularTarefa)
	mcp.AddTool(s, &mcp.Tool{Name: "atualizar_journal", Description: "Atualiza os localizadores do journal e dos artefatos da sessão deste chat."}, f.atualizarJournal)
	mcp.AddTool(s, &mcp.Tool{Name: "encerrar_sessao", Description: "Encerra a sessão deste chat e grava o resumo em cada tarefa dela."}, f.encerrarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "repassar_sessao", Description: "Troca de agente: encerra a sessão deste chat e abre outra com o agente novo, nas mesmas tarefas."}, f.repassarSessao)
	mcp.AddTool(s, &mcp.Tool{Name: "buscar_requisitos", Description: "Busca requisitos por palavras (título pesa mais; sem acento; prefixo). Devolve só id, título, estado e componentes dos mais relevantes. Use antes de criar um requisito ou ligar uma tarefa."}, f.buscarRequisitos)
	mcp.AddTool(s, &mcp.Tool{Name: "arvore_de_requisitos", Description: "A cadeia de requisitos em texto (id e título, indentada), a partir do zero da organização; com componente, só os dele e os ancestrais. Use só para escolher o pai de um requisito novo: custa mais que buscar."}, f.arvoreDeRequisitos)
	mcp.AddTool(s, &mcp.Tool{Name: "obter_requisito", Description: "Obtém um requisito: trilhas até o zero (em títulos), origem, cadeia (deriva_de acima, derivados abaixo), componentes e tarefas que tentam atendê-lo."}, f.obterRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_requisito", Description: "Cria um requisito, com origem e os requisitos dos quais deriva (a organização vem do primeiro, se omitida)."}, f.criarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "alterar_requisito", Description: "Altera título, descrição, organização, origem ou prioridade (triagem: alta, media, baixa; vazio volta a não triado) de um requisito."}, f.alterarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "derivar_requisito", Description: "Registra que um requisito deriva de outro (um requisito pode derivar de vários). Recusa ciclos."}, f.derivarRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_requisito", Description: "Registra que uma tarefa tenta atender um requisito."}, f.vincularRequisito)
	mcp.AddTool(s, &mcp.Tool{Name: "registrar_execucao", Description: "Registra uma execução de conformidade: o requisito foi verificado e está atendido, com a evidência e a validade em dias (depois dela, a validação expira). Só registre o que você verificou de fato."}, f.registrarExecucao)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_componentes", Description: "Lista os componentes (repositórios, módulos, lugares, funcionalidades…) com os requisitos e tarefas de cada um, para reaproveitar em vez de duplicar."}, f.listarComponentes)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_componente", Description: "Cria um componente: qualquer coisa reaproveitável a que requisitos e tarefas se referem."}, f.criarComponente)
	mcp.AddTool(s, &mcp.Tool{Name: "vincular_componente", Description: "Liga um componente a um requisito ou a uma tarefa (informe um dos dois)."}, f.vincularComponente)
	mcp.AddTool(s, &mcp.Tool{Name: "opcoes_do_lancador", Description: "Modelos e esforços que o lançador de agentes aceita, e os padrões."}, f.opcoesDoLancador)
	mcp.AddTool(s, &mcp.Tool{Name: "lancar_agente", Description: "Sobe um agente novo (outro chat, em tmux) para trabalhar numa tarefa: põe a tarefa em andamento e abre a sessão de trabalho dele. Para delegar; nunca para a tarefa deste chat. Recusa tarefa com sessão ativa. Para parar o agente depois, mude o status da tarefa (alterar_tarefa)."}, f.lancarAgente)
	mcp.AddTool(s, &mcp.Tool{Name: "agentes_ativos", Description: "Sessões de trabalho ativas agora: id, agente, tarefas e início. Para saber quem está trabalhando em quê."}, f.agentesAtivos)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_templates", Description: "Tarefas pré-moldadas (templates): id, título, organização, periodicidade, se está devida e a instância aberta."}, f.listarTemplates)
	mcp.AddTool(s, &mcp.Tool{Name: "instanciar_template", Description: "Cria uma tarefa (a fazer) a partir de uma pré-moldada, com o contexto desta vez no topo da descrição e os requisitos dela. Não sobe agente: para isso, lancar_agente na tarefa criada."}, f.instanciarTemplate)
	mcp.AddTool(s, &mcp.Tool{Name: "listar_origens", Description: "Lista as origens de requisitos (pessoas e documentos), para reaproveitar em vez de duplicar."}, f.listarOrigens)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_origem", Description: "Cria uma origem de requisitos: uma pessoa (quem pede) ou um documento (lei, norma, contrato)."}, f.criarOrigem)
	mcp.AddTool(s, &mcp.Tool{Name: "criar_lembrete", Description: "Agenda um lembrete para o Rafael numa hora marcada (RFC 3339 com fuso), solto ou ligado a uma tarefa ou requisito. As integrações da instalação o avisam; o Requerial guarda só a hora."}, f.criarLembrete)
	mcp.AddTool(s, &mcp.Tool{Name: "proximos_lembretes", Description: "Lembretes que pedem atenção: os disparados e os que vencem nas próximas horas (padrão 24). Só id, texto, hora e status."}, f.proximosLembretes)
	mcp.AddTool(s, &mcp.Tool{Name: "adiar_lembrete", Description: "Reagenda um lembrete para daqui a N minutos ou para uma hora (RFC 3339)."}, f.adiarLembrete)
	mcp.AddTool(s, &mcp.Tool{Name: "confirmar_lembrete", Description: "Marca o lembrete como visto: ele para de pedir atenção. Só quando o Rafael disser que viu ou fez."}, f.confirmarLembrete)
	mcp.AddTool(s, &mcp.Tool{Name: "cancelar_lembrete", Description: "Cancela um lembrete que não vale mais."}, f.cancelarLembrete)
	mcp.AddTool(s, &mcp.Tool{Name: "aguardar_resposta", Description: "Avisa que a sessão deste chat entregou ou está bloqueada e espera o Rafael (aprovação, decisão, informação). Aparece nas pendências até retomar_trabalho ou o encerramento."}, f.aguardarResposta)
	mcp.AddTool(s, &mcp.Tool{Name: "retomar_trabalho", Description: "Tira a sessão deste chat da espera: o Rafael respondeu e o trabalho continuou."}, f.retomarTrabalho)
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

type Componentes struct {
	Componentes []api.Componente `json:"componentes"`
}

type VinculoComponente struct {
	Componente string `json:"componente" jsonschema:"componente, como C-0001"`
	Requisito  string `json:"requisito,omitempty" jsonschema:"requisito, como R-0001"`
	Tarefa     string `json:"tarefa,omitempty" jsonschema:"tarefa, como T-0001"`
}

type BuscaDeRequisitos struct {
	Texto       string `json:"texto" jsonschema:"palavras a buscar"`
	Organizacao string `json:"organizacao,omitempty" jsonschema:"só desta organização"`
	Componente  string `json:"componente,omitempty" jsonschema:"só deste componente, como C-0001"`
}

type Requisitos struct {
	Requisitos []api.ResumoDeRequisito `json:"requisitos"`
}

type FiltroDaArvore struct {
	Organizacao string `json:"organizacao,omitempty" jsonschema:"organização (o zero dela); vazio é a visão geral"`
	Componente  string `json:"componente,omitempty" jsonschema:"só os requisitos deste componente e os ancestrais"`
}

func opcional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type Lancamento struct {
	Tarefa    string `json:"tarefa" jsonschema:"tarefa, como T-0001"`
	Instrucao string `json:"instrucao,omitempty" jsonschema:"instrução adicional para o agente; o resto ele lê da tarefa e das sessões anteriores"`
	Modelo    string `json:"modelo,omitempty" jsonschema:"um dos modelos de opcoes_do_lancador; padrão: o configurado"`
	Esforco   string `json:"esforco,omitempty" jsonschema:"um dos esforços de opcoes_do_lancador; padrão: o configurado"`
}

type AgenteAtivo struct {
	Sessao     string   `json:"sessao"`
	Agente     string   `json:"agente"`
	Tarefas    []string `json:"tarefas"`
	IniciadaEm string   `json:"iniciada_em"`
}

type AgentesAtivos struct {
	Agentes []AgenteAtivo `json:"agentes"`
}

type ResumoDeTemplate struct {
	Id                string  `json:"id"`
	Titulo            string  `json:"titulo"`
	Organizacao       string  `json:"organizacao"`
	PeriodicidadeDias *int    `json:"periodicidade_dias,omitempty"`
	Devido            bool    `json:"devido"`
	InstanciaAberta   *string `json:"instancia_aberta,omitempty"`
}

type Templates struct {
	Templates []ResumoDeTemplate `json:"templates"`
}

type Instancia struct {
	Template string `json:"template" jsonschema:"pré-moldada, como TP-0001"`
	Contexto string `json:"contexto,omitempty" jsonschema:"informações desta vez (link, quem pediu, o que mudou)"`
}

type Origens struct {
	Origens []api.Origem `json:"origens"`
}

func (f *Ferramentas) buscarRequisitos(ctx context.Context, _ *mcp.CallToolRequest, in BuscaDeRequisitos) (*mcp.CallToolResult, Requisitos, error) {
	var out Requisitos
	resp, err := f.cliente.BuscarRequisitos(ctx, &api.BuscarRequisitosParams{Q: in.Texto, Organizacao: opcional(in.Organizacao), Componente: opcional(in.Componente)})
	return nil, out, decodificar(resp, err, &out.Requisitos)
}

func (f *Ferramentas) arvoreDeRequisitos(ctx context.Context, _ *mcp.CallToolRequest, in FiltroDaArvore) (*mcp.CallToolResult, api.ArvoreDeRequisitos, error) {
	var out api.ArvoreDeRequisitos
	resp, err := f.cliente.ArvoreDeRequisitos(ctx, &api.ArvoreDeRequisitosParams{Organizacao: opcional(in.Organizacao), Componente: opcional(in.Componente)})
	return nil, out, decodificar(resp, err, &out)
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

func (f *Ferramentas) listarComponentes(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, Componentes, error) {
	var out Componentes
	resp, err := f.cliente.ListarComponentes(ctx)
	return nil, out, decodificar(resp, err, &out.Componentes)
}

func (f *Ferramentas) criarComponente(ctx context.Context, _ *mcp.CallToolRequest, in api.NovoComponente) (*mcp.CallToolResult, api.Componente, error) {
	var out api.Componente
	resp, err := f.cliente.CriarComponente(ctx, in, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) vincularComponente(ctx context.Context, _ *mcp.CallToolRequest, in VinculoComponente) (*mcp.CallToolResult, Vazio, error) {
	corpo := api.VinculoComponente{Componente: in.Componente}
	switch {
	case in.Requisito != "" && in.Tarefa == "":
		resp, err := f.cliente.VincularComponenteARequisito(ctx, in.Requisito, corpo, f.autorizar)
		return nil, Vazio{}, decodificar(resp, err, &api.Requisito{})
	case in.Tarefa != "" && in.Requisito == "":
		resp, err := f.cliente.VincularComponenteATarefa(ctx, in.Tarefa, corpo, f.autorizar)
		return nil, Vazio{}, decodificar(resp, err, &api.Tarefa{})
	}
	return nil, Vazio{}, fmt.Errorf("informe o requisito ou a tarefa (só um dos dois)")
}

func (f *Ferramentas) opcoesDoLancador(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, api.OpcoesDeLancamento, error) {
	var out api.OpcoesDeLancamento
	resp, err := f.cliente.OpcoesDoLancador(ctx)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) lancarAgente(ctx context.Context, _ *mcp.CallToolRequest, in Lancamento) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.LancarAgente(ctx, in.Tarefa, api.PedidoDeLancamento{Instrucao: opcional(in.Instrucao),
		Modelo: opcional(in.Modelo), Esforco: opcional(in.Esforco)}, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) agentesAtivos(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, AgentesAtivos, error) {
	ativa := api.Ativa
	var sessoes []api.Sessao
	resp, err := f.cliente.ListarSessoes(ctx, &api.ListarSessoesParams{Status: &ativa})
	if err := decodificar(resp, err, &sessoes); err != nil {
		return nil, AgentesAtivos{}, err
	}
	out := AgentesAtivos{Agentes: []AgenteAtivo{}}
	for _, x := range sessoes {
		out.Agentes = append(out.Agentes, AgenteAtivo{x.Id, x.Agente, x.Tarefas, x.IniciadaEm.Format("2006-01-02 15:04")})
	}
	return nil, out, nil
}

func (f *Ferramentas) listarTemplates(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, Templates, error) {
	var todos []api.Template
	resp, err := f.cliente.ListarTemplates(ctx)
	if err := decodificar(resp, err, &todos); err != nil {
		return nil, Templates{}, err
	}
	out := Templates{Templates: []ResumoDeTemplate{}}
	for _, t := range todos {
		out.Templates = append(out.Templates, ResumoDeTemplate{t.Id, t.Titulo, t.Organizacao, t.PeriodicidadeDias, t.Devido, t.InstanciaAberta})
	}
	return nil, out, nil
}

func (f *Ferramentas) instanciarTemplate(ctx context.Context, _ *mcp.CallToolRequest, in Instancia) (*mcp.CallToolResult, api.Tarefa, error) {
	var out api.Tarefa
	resp, err := f.cliente.InstanciarTemplate(ctx, in.Template, api.PedidoDeInstancia{Contexto: opcional(in.Contexto)}, f.autorizar)
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

type NovoLembrete struct {
	Texto       string `json:"texto" jsonschema:"o que lembrar, como o Rafael vai ler"`
	Quando      string `json:"quando" jsonschema:"hora marcada em RFC 3339 com fuso, como 2026-10-21T09:00:00-03:00"`
	Organizacao string `json:"organizacao" jsonschema:"Cartório, Yes ou Pessoal (ou a da instalação)"`
	Tarefa      string `json:"tarefa,omitempty" jsonschema:"tarefa ligada, como T-0001"`
	Requisito   string `json:"requisito,omitempty" jsonschema:"requisito ligado, como R-0001"`
}

type ResumoDeLembrete struct {
	Id     string `json:"id"`
	Texto  string `json:"texto"`
	Quando string `json:"quando"`
	Status string `json:"status"`
}

type Lembretes struct {
	Lembretes []ResumoDeLembrete `json:"lembretes"`
}

type FiltroDeLembretes struct {
	Horas int `json:"horas,omitempty" jsonschema:"janela à frente, em horas; padrão 24"`
}

type AdiamentoDeLembrete struct {
	Id      string `json:"id" jsonschema:"lembrete, como L-0001"`
	Minutos int    `json:"minutos,omitempty" jsonschema:"daqui a quantos minutos"`
	Ate     string `json:"ate,omitempty" jsonschema:"ou a nova hora, em RFC 3339 com fuso"`
}

type Espera struct {
	Mensagem string `json:"mensagem" jsonschema:"o que você precisa do Rafael, em uma linha"`
}

func resumirLembrete(x api.Lembrete) ResumoDeLembrete {
	return ResumoDeLembrete{x.Id, x.Texto, x.Quando.Format(time.RFC3339), string(x.Status)}
}

func lerHora(campo, valor string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, valor)
	if err != nil {
		return t, fmt.Errorf("%s: use RFC 3339 com fuso, como 2026-10-21T09:00:00-03:00 (%v)", campo, err)
	}
	return t, nil
}

func (f *Ferramentas) criarLembrete(ctx context.Context, _ *mcp.CallToolRequest, in NovoLembrete) (*mcp.CallToolResult, ResumoDeLembrete, error) {
	quando, err := lerHora("quando", in.Quando)
	if err != nil {
		return nil, ResumoDeLembrete{}, err
	}
	var x api.Lembrete
	resp, err := f.cliente.CriarLembrete(ctx, api.NovoLembrete{
		Texto: in.Texto, Quando: quando, Organizacao: in.Organizacao, Tarefa: opcional(in.Tarefa), Requisito: opcional(in.Requisito),
	}, f.autorizar)
	if err := decodificar(resp, err, &x); err != nil {
		return nil, ResumoDeLembrete{}, err
	}
	return nil, resumirLembrete(x), nil
}

func (f *Ferramentas) proximosLembretes(ctx context.Context, _ *mcp.CallToolRequest, in FiltroDeLembretes) (*mcp.CallToolResult, Lembretes, error) {
	horas := in.Horas
	if horas <= 0 {
		horas = 24
	}
	horizonte := horas * 60
	var p api.Pendencias
	resp, err := f.cliente.ListarPendencias(ctx, &api.ListarPendenciasParams{Horizonte: &horizonte})
	if err := decodificar(resp, err, &p); err != nil {
		return nil, Lembretes{}, err
	}
	out := Lembretes{Lembretes: []ResumoDeLembrete{}}
	for _, x := range p.Lembretes {
		out.Lembretes = append(out.Lembretes, resumirLembrete(x))
	}
	return nil, out, nil
}

func (f *Ferramentas) adiarLembrete(ctx context.Context, _ *mcp.CallToolRequest, in AdiamentoDeLembrete) (*mcp.CallToolResult, ResumoDeLembrete, error) {
	var a api.Adiamento
	if in.Minutos > 0 {
		a.Minutos = &in.Minutos
	}
	if in.Ate != "" {
		ate, err := lerHora("ate", in.Ate)
		if err != nil {
			return nil, ResumoDeLembrete{}, err
		}
		a.Ate = &ate
	}
	var x api.Lembrete
	resp, err := f.cliente.AdiarLembrete(ctx, in.Id, a, f.autorizar)
	if err := decodificar(resp, err, &x); err != nil {
		return nil, ResumoDeLembrete{}, err
	}
	return nil, resumirLembrete(x), nil
}

func (f *Ferramentas) confirmarLembrete(ctx context.Context, _ *mcp.CallToolRequest, in Id) (*mcp.CallToolResult, ResumoDeLembrete, error) {
	var x api.Lembrete
	resp, err := f.cliente.ConfirmarLembrete(ctx, in.Id, f.autorizar)
	if err := decodificar(resp, err, &x); err != nil {
		return nil, ResumoDeLembrete{}, err
	}
	return nil, resumirLembrete(x), nil
}

func (f *Ferramentas) cancelarLembrete(ctx context.Context, _ *mcp.CallToolRequest, in Id) (*mcp.CallToolResult, ResumoDeLembrete, error) {
	var x api.Lembrete
	resp, err := f.cliente.CancelarLembrete(ctx, in.Id, f.autorizar)
	if err := decodificar(resp, err, &x); err != nil {
		return nil, ResumoDeLembrete{}, err
	}
	return nil, resumirLembrete(x), nil
}

func (f *Ferramentas) aguardarResposta(ctx context.Context, _ *mcp.CallToolRequest, in Espera) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.AguardarResposta(ctx, id, api.Espera{Mensagem: in.Mensagem}, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}

func (f *Ferramentas) retomarTrabalho(ctx context.Context, _ *mcp.CallToolRequest, _ Vazio) (*mcp.CallToolResult, api.Sessao, error) {
	id, err := f.sessaoAberta()
	if err != nil {
		return nil, api.Sessao{}, err
	}
	var out api.Sessao
	resp, err := f.cliente.RetomarSessao(ctx, id, f.autorizar)
	return nil, out, decodificar(resp, err, &out)
}
