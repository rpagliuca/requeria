package lancador

import (
	"strings"
	"testing"

	"github.com/rpagliuca/requerial/api"
	"github.com/rpagliuca/requerial/internal/servidor"
)

// tmuxFalso simula as sessões tmux: nome → tarefa do ambiente.
type tmuxFalso struct {
	sessoes  map[string]string
	chamadas []string
}

func (f *tmuxFalso) executar(args ...string) (string, error) {
	f.chamadas = append(f.chamadas, strings.Join(args, " "))
	alvo := func() string {
		for i, a := range args {
			if a == "-t" {
				return strings.TrimSuffix(strings.TrimPrefix(args[i+1], "="), ":")
			}
		}
		return ""
	}
	switch args[0] {
	case "list-sessions":
		var nomes []string
		for n := range f.sessoes {
			nomes = append(nomes, n)
		}
		return strings.Join(nomes, "\n"), nil
	case "show-environment":
		if t, ok := f.sessoes[alvo()]; ok && t != "" {
			return VariavelTarefa + "=" + t, nil
		}
		return "", errFalso
	case "has-session":
		if _, ok := f.sessoes[alvo()]; ok {
			return "", nil
		}
		return "", errFalso
	case "new-session":
		nome, tarefa := "", ""
		for i, a := range args {
			if a == "-s" {
				nome = args[i+1]
			}
			if a == "-e" {
				tarefa = strings.TrimPrefix(args[i+1], VariavelTarefa+"=")
			}
		}
		f.sessoes[nome] = tarefa
	case "kill-session":
		delete(f.sessoes, alvo())
	}
	return "", nil
}

type erroFalso struct{}

func (erroFalso) Error() string { return "falso" }

var errFalso = erroFalso{}

func novoLancador(t *testing.T) (*Lancador, *tmuxFalso) {
	t.Helper()
	l, err := Novo(Config{Diretorio: "/journals", Comando: []string{"agente", "--nome", "{nome}"}})
	if err != nil {
		t.Fatal(err)
	}
	f := &tmuxFalso{sessoes: map[string]string{"outra": ""}}
	l.executar = f.executar
	return l, f
}

func ptr(s string) *string { return &s }

func TestSobeAgenteAoEntrarEmAndamento(t *testing.T) {
	l, f := novoLancador(t)
	tarefa := api.Tarefa{Id: "T-0007", Titulo: "Revisão do backup de fotos", Status: api.EmAndamento}

	// Uma sessão que põe a própria tarefa em andamento não ganha outro agente.
	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.Pausada, Autor: "S-0001"})
	if len(f.sessoes) != 1 {
		t.Fatalf("subiu agente para mudança feita por sessão: %v", f.chamadas)
	}

	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.Pausada, Autor: "local"})
	if f.sessoes["revisao-backup-fotos"] != "T-0007" {
		t.Fatalf("agente não subiu como esperado: %v", f.sessoes)
	}
	ultima := f.chamadas[len(f.chamadas)-1]
	if !strings.Contains(ultima, "-c /journals") || !strings.Contains(ultima, "agente --nome revisao-backup-fotos") ||
		!strings.Contains(ultima, "tarefa T-0007") {
		t.Fatalf("comando inesperado: %s", ultima)
	}

	// Um tmux que sobrou da tarefa é fechado antes de subir o agente novo.
	tarefa.SessaoAtiva = nil
	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.Concluida, Autor: "local"})
	if len(f.sessoes) != 2 || f.sessoes["revisao-backup-fotos"] != "T-0007" {
		t.Fatalf("tmux antigo não foi substituído: %v", f.sessoes)
	}

	// Com sessão ativa, não sobe outro.
	tarefa.SessaoAtiva = ptr("S-0002")
	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.AFazer, Autor: "local"})
	if len(f.sessoes) != 2 {
		t.Fatalf("subiu agente para tarefa com sessão ativa: %v", f.sessoes)
	}
}

func TestAvisaAgenteAoSairDeAndamento(t *testing.T) {
	l, f := novoLancador(t)
	f.sessoes["backup"] = "T-0007"
	tarefa := api.Tarefa{Id: "T-0007", Titulo: "Backup", Status: api.Pausada, SessaoAtiva: ptr("S-0002")}

	// A própria sessão mudando o status: nada a avisar.
	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.EmAndamento, Autor: "S-0002"})
	for _, c := range f.chamadas {
		if strings.HasPrefix(c, "send-keys") {
			t.Fatalf("avisou o agente da própria mudança: %v", f.chamadas)
		}
	}

	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.EmAndamento, Autor: "local"})
	avisos := 0
	for _, c := range f.chamadas {
		if strings.HasPrefix(c, "send-keys -t =backup: -l") && strings.Contains(c, "pausada") {
			avisos++
		}
	}
	if avisos != 1 {
		t.Fatalf("aviso não enviado: %v", f.chamadas)
	}

}

func TestNomeCurto(t *testing.T) {
	casos := map[string]string{
		"Revisão do backup de fotos":    "revisao-backup-fotos",
		"Usina — causa raiz das perdas": "usina-causa-raiz",
		"AD":                            "tarefa",
	}
	for titulo, esperado := range casos {
		if obtido := nomeCurto(titulo); obtido != esperado {
			t.Errorf("nomeCurto(%q) = %q, esperado %q", titulo, obtido, esperado)
		}
	}
}

func TestLancamentoComModeloEsforcoEInstrucao(t *testing.T) {
	l, err := Novo(Config{
		Diretorio: "/journals", Comando: []string{"agente"},
		Modelos:           []Modelo{{Nome: "Modelo A", Argumentos: []string{"--model", "a"}}, {Nome: "Modelo B", Argumentos: []string{"--model", "b"}}},
		Esforcos:          []string{"low", "high"},
		ArgumentosEsforco: []string{"--effort", "{esforco}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := &tmuxFalso{sessoes: map[string]string{}}
	l.executar = f.executar

	ruim := api.PedidoDeLancamento{Modelo: ptr("Modelo Z")}
	if l.Validar(&ruim) == nil {
		t.Fatal("aceitou modelo desconhecido")
	}
	pedido := api.PedidoDeLancamento{Modelo: ptr("Modelo B"), Esforco: ptr("low"), Instrucao: ptr("Comece pelo teste que falhou.")}
	if err := l.Validar(&pedido); err != nil {
		t.Fatal(err)
	}
	// Relançar uma tarefa que já estava em andamento, sem agente, também vale.
	tarefa := api.Tarefa{Id: "T-0009", Titulo: "Corrigir importação", Status: api.EmAndamento}
	l.MudouStatus(servidor.MudancaDeStatus{Tarefa: tarefa, Anterior: api.EmAndamento, Autor: "local", Lancamento: &pedido})
	ultima := f.chamadas[len(f.chamadas)-1]
	if !strings.Contains(ultima, "agente --model b --effort low") || !strings.Contains(ultima, "Comece pelo teste que falhou.") {
		t.Fatalf("comando inesperado: %s", ultima)
	}

	padrao := api.PedidoDeLancamento{}
	if err := l.Validar(&padrao); err != nil || *padrao.Modelo != "Modelo A" || padrao.Esforco != nil {
		t.Fatalf("padrões inesperados: %+v %v", padrao, err)
	}
}
