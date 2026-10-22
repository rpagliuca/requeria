package lancador

import (
	"errors"
	"time"

	"github.com/rpagliuca/rex/api"
	"github.com/rpagliuca/rex/internal/voz"
)

// O rex voz fala com o agente de uma sessão pelo tmux dele: lê a tela, escreve a frase do Rafael
// e aperta teclas, como o Rafael faria no laptop.

// ErrSemAgente é a sessão ativa cujo tmux não foi achado.
var ErrSemAgente = errors.New("o tmux do agente desta sessão não foi achado")

// painelDaSessao acha o tmux do agente: pelo chat (agente iniciado à mão também) e, sem ele,
// pela tarefa do ambiente (agente subido pelo lançador).
func (l *Lancador) painelDaSessao(x api.Sessao) string {
	if x.Chat != nil && *x.Chat != "" {
		if nome := l.painelDoChat(*x.Chat); nome != "" {
			return nome
		}
	}
	for _, t := range x.Tarefas {
		if nome, err := l.encontrar(t); err == nil && nome != "" {
			return nome
		}
	}
	return ""
}

// Tela devolve o que a tela do agente mostra agora.
func (l *Lancador) Tela(x api.Sessao) (string, error) {
	nome := l.painelDaSessao(x)
	if nome == "" {
		return "", ErrSemAgente
	}
	return l.executar("capture-pane", "-p", "-t", "="+nome+":")
}

// Escrever digita o texto no agente e manda; com um turno em andamento, o Claude Code o põe na fila.
func (l *Lancador) Escrever(x api.Sessao, texto string) error {
	nome := l.painelDaSessao(x)
	if nome == "" {
		return ErrSemAgente
	}
	if _, err := l.executar("send-keys", "-t", "="+nome+":", "-l", texto); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond) // o Enter separado do texto evita que vire quebra de linha
	_, err := l.executar("send-keys", "-t", "="+nome+":", "Enter")
	return err
}

// Teclar aperta uma tecla (nome do tmux: "1", "Enter", "Escape") no agente.
func (l *Lancador) Teclar(x api.Sessao, tecla string) error {
	nome := l.painelDaSessao(x)
	if nome == "" {
		return ErrSemAgente
	}
	_, err := l.executar("send-keys", "-t", "="+nome+":", tecla)
	return err
}

// Conversa devolve o transcript do agente da sessão, ou "".
func (l *Lancador) Conversa(x api.Sessao) string {
	if x.Chat == nil {
		return ""
	}
	return voz.Arquivo(l.cfg.Projetos, *x.Chat)
}

// Intermediario diz a tarefa do intermediário do rex voz e o pedido para lançá-lo.
func (l *Lancador) Intermediario() (string, api.PedidoDeLancamento, bool) {
	v := l.cfg.Voz
	if v == nil || v.Tarefa == "" {
		return "", api.PedidoDeLancamento{}, false
	}
	var p api.PedidoDeLancamento
	for campo, valor := range map[**string]string{&p.Modelo: v.Modelo, &p.Esforco: v.Esforco, &p.Nome: v.Nome, &p.Instrucao: v.Instrucao} {
		if valor != "" {
			*campo = &valor
		}
	}
	return v.Tarefa, p, true
}
