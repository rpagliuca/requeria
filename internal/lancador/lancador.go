// Package lancador sobe e encerra agentes de IA em sessões tmux conforme o status das tarefas.
//
// Quando uma tarefa entra em em_andamento sem sessão ativa, por alguém que não é uma sessão (um
// humano no frontend, ou um agente que só coordena), o lançador abre uma sessão tmux com o agente
// configurado e um prompt que o manda trabalhar na tarefa. Quando a tarefa sai de em_andamento por
// mão de outro, o lançador pede ao agente vivo que escreva o resumo e encerre a sessão. Depois do
// encerramento, o próprio agente fecha o seu tmux (skill requerial); um tmux que sobrar é fechado
// quando a tarefa ganhar um agente novo.
//
// A sessão tmux carrega REQUERIAL_TAREFA no ambiente: é assim que o lançador a reencontra, sem
// guardar estado, e que o agente e o servidor MCP sabem em que tarefa estão.
package lancador

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/rpagliuca/requerial/api"
	"github.com/rpagliuca/requerial/internal/servidor"
)

// VariavelTarefa é a variável de ambiente da sessão tmux com o identificador da tarefa.
const VariavelTarefa = "REQUERIAL_TAREFA"

// Config vem de um arquivo JSON, particular de cada instalação.
type Config struct {
	// Diretorio onde o agente começa a trabalhar (normalmente o clone dos journals).
	Diretorio string `json:"diretorio"`
	// Tmux é o executável do tmux. Padrão: "tmux".
	Tmux string `json:"tmux"`
	// Comando do agente; o prompt entra como último argumento. "{nome}" vira o nome da sessão tmux.
	Comando []string `json:"comando"`
	// PromptIniciar e PromptEncerrar aceitam {tarefa}, {titulo} e {status}.
	PromptIniciar  string `json:"prompt_iniciar"`
	PromptEncerrar string `json:"prompt_encerrar"`
}

const promptIniciarPadrao = "Você vai trabalhar na tarefa {tarefa} do requerial ({titulo}). " +
	"Carregue a skill requerial e siga o ciclo dela: leia a tarefa, inicie a sessão de trabalho e trabalhe."

const promptEncerrarPadrao = "A tarefa {tarefa} foi movida para {status} por outra pessoa. " +
	"Pare o que está fazendo, atualize o journal e encerre a sessão de trabalho com um resumo para retomar " +
	"(skill requerial). Não altere o status da tarefa."

// Lancador reage às mudanças do servidor.
type Lancador struct {
	cfg Config
	// executar roda o tmux; substituível nos testes.
	executar func(args ...string) (string, error)
}

// Carregar lê a configuração do arquivo.
func Carregar(caminho string) (*Lancador, error) {
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(conteudo, &cfg); err != nil {
		return nil, fmt.Errorf("configuração do lançador: %w", err)
	}
	return Novo(cfg)
}

// Novo valida a configuração e preenche os padrões.
func Novo(cfg Config) (*Lancador, error) {
	if cfg.Diretorio == "" || len(cfg.Comando) == 0 {
		return nil, errors.New("configuração do lançador: diretorio e comando são obrigatórios")
	}
	if cfg.Tmux == "" {
		cfg.Tmux = "tmux"
	}
	if cfg.PromptIniciar == "" {
		cfg.PromptIniciar = promptIniciarPadrao
	}
	if cfg.PromptEncerrar == "" {
		cfg.PromptEncerrar = promptEncerrarPadrao
	}
	l := &Lancador{cfg: cfg}
	l.executar = func(args ...string) (string, error) {
		saida, err := exec.Command(l.cfg.Tmux, args...).CombinedOutput()
		return strings.TrimSpace(string(saida)), err
	}
	return l, nil
}

// Conectar registra o lançador nos avisos do servidor.
func (l *Lancador) Conectar(s *servidor.Servidor) {
	s.AoMudarStatus(l.MudouStatus)
}

// MudouStatus decide se sobe ou avisa um agente.
func (l *Lancador) MudouStatus(m servidor.MudancaDeStatus) {
	t := m.Tarefa
	autorESessao := strings.HasPrefix(m.Autor, "S-")
	switch {
	case t.Status == api.EmAndamento && m.Anterior != api.EmAndamento && t.SessaoAtiva == nil && !autorESessao:
		// Uma sessão que põe a tarefa em andamento vai trabalhar nela ela mesma.
		l.registrar(l.iniciar(t))
	case m.Anterior == api.EmAndamento && t.Status != api.EmAndamento && t.SessaoAtiva != nil && m.Autor != *t.SessaoAtiva:
		l.registrar(l.avisar(t))
	}
}

func (l *Lancador) iniciar(t api.Tarefa) error {
	// Um tmux antigo desta tarefa, sem sessão ativa, é de um agente que já terminou.
	if antigo, err := l.encontrar(t.Id); err != nil {
		return err
	} else if antigo != "" {
		if _, err := l.executar("kill-session", "-t", "="+antigo); err != nil {
			return fmt.Errorf("fechar o tmux antigo %s: %w", antigo, err)
		}
	}
	nome, err := l.nomeLivre(nomeCurto(t.Titulo))
	if err != nil {
		return err
	}
	args := []string{"new-session", "-d", "-s", nome, "-c", l.cfg.Diretorio, "-e", VariavelTarefa + "=" + t.Id}
	for _, parte := range l.cfg.Comando {
		args = append(args, strings.ReplaceAll(parte, "{nome}", nome))
	}
	args = append(args, preencher(l.cfg.PromptIniciar, t))
	if saida, err := l.executar(args...); err != nil {
		return fmt.Errorf("subir o agente da tarefa %s: %v: %s", t.Id, err, saida)
	}
	log.Printf("lançador: agente da tarefa %s subiu no tmux %s", t.Id, nome)
	return nil
}

func (l *Lancador) avisar(t api.Tarefa) error {
	nome, err := l.encontrar(t.Id)
	if err != nil {
		return err
	}
	if nome == "" {
		log.Printf("lançador: tarefa %s saiu de em_andamento, mas não há agente vivo da sessão %s; encerre-a pelo frontend", t.Id, *t.SessaoAtiva)
		return nil
	}
	if _, err := l.executar("send-keys", "-t", "="+nome+":", "-l", preencher(l.cfg.PromptEncerrar, t)); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond) // o Enter separado do texto evita que vire quebra de linha
	if _, err := l.executar("send-keys", "-t", "="+nome+":", "Enter"); err != nil {
		return err
	}
	log.Printf("lançador: agente do tmux %s avisado para encerrar (tarefa %s em %s)", nome, t.Id, t.Status)
	return nil
}

// encontrar devolve a sessão tmux com REQUERIAL_TAREFA igual ao id, ou "".
func (l *Lancador) encontrar(id string) (string, error) {
	saida, err := l.executar("list-sessions", "-F", "#{session_name}")
	if err != nil {
		return "", nil // sem servidor tmux, não há sessão
	}
	for _, nome := range strings.Fields(saida) {
		valor, err := l.executar("show-environment", "-t", "="+nome, VariavelTarefa)
		if err == nil && valor == VariavelTarefa+"="+id {
			return nome, nil
		}
	}
	return "", nil
}

func (l *Lancador) nomeLivre(base string) (string, error) {
	nome := base
	for i := 2; i < 100; i++ {
		if _, err := l.executar("has-session", "-t", "="+nome); err != nil {
			return nome, nil
		}
		nome = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("sem nome livre para o tmux a partir de %q", base)
}

func (l *Lancador) registrar(err error) {
	if err != nil {
		log.Printf("lançador: %v", err)
	}
}

func preencher(modelo string, t api.Tarefa) string {
	return strings.NewReplacer("{tarefa}", t.Id, "{titulo}", t.Titulo, "{status}", string(t.Status)).Replace(modelo)
}

var foraDoNome = regexp.MustCompile(`[^a-z0-9]+`)

var palavrasVazias = map[string]bool{
	"dos": true, "das": true, "com": true, "para": true, "por": true, "uma": true, "nos": true, "nas": true,
	"the": true, "and": true, "for": true,
}

// nomeCurto deriva do título um nome de sessão tmux: minúsculas, sem acento, até 20 caracteres.
func nomeCurto(titulo string) string {
	semAcento := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o",
		"ú", "u", "ç", "c",
	).Replace(strings.ToLower(titulo))
	palavras := strings.Fields(foraDoNome.ReplaceAllString(semAcento, " "))
	nome := ""
	for _, p := range palavras {
		if len(p) < 3 || palavrasVazias[p] {
			continue
		}
		proximo := p
		if nome != "" {
			proximo = nome + "-" + p
		}
		if len(proximo) > 20 {
			break
		}
		nome = proximo
	}
	if nome == "" {
		nome = "tarefa"
	}
	return nome
}
