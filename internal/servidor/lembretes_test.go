package servidor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rpagliuca/rex/api"
)

// relogio fixa a hora do servidor; avancar a muda.
func (a *ambiente) relogio(t time.Time) *time.Time {
	agora := t
	a.servidor.agora = func() time.Time { return agora.UTC() }
	return &agora
}

func TestCicloDoLembrete(t *testing.T) {
	a := novoAmbiente(t)
	fuso := time.FixedZone("-03", -3*3600)
	agora := a.relogio(time.Date(2026, 10, 20, 14, 0, 0, 0, fuso))
	a.tarefa("Pagar contas")

	quando := time.Date(2026, 10, 20, 15, 0, 0, 0, fuso)
	a.chamar("POST", "/lembretes", "", api.NovoLembrete{Texto: "x", Quando: agora.Add(-time.Minute), Organizacao: "Exemplo"}, 400, nil)
	var l api.Lembrete
	a.chamar("POST", "/lembretes", "", api.NovoLembrete{Texto: "ligar para o contador", Quando: quando, Organizacao: "Exemplo", Tarefa: ptr("T-0001")}, 201, &l)
	if l.Id != "L-0001" || l.Status != api.Agendado || l.Autor != "local" || l.Quando.Format(time.RFC3339) != "2026-10-20T15:00:00-03:00" {
		t.Fatalf("lembrete inesperado: %+v", l)
	}

	// Antes da hora, nada dispara; depois, dispara uma vez.
	if n, _ := a.servidor.DispararVencidos(); n != 0 {
		t.Fatalf("disparou antes da hora: %d", n)
	}
	*agora = quando.Add(10 * time.Second)
	if n, _ := a.servidor.DispararVencidos(); n != 1 {
		t.Fatalf("esperava 1 disparo, veio %d", n)
	}
	if n, _ := a.servidor.DispararVencidos(); n != 0 {
		t.Fatalf("disparou de novo: %d", n)
	}
	a.chamar("GET", "/lembretes/L-0001", "", nil, 200, &l)
	if l.Status != api.Disparado || *l.Atrasado {
		t.Fatalf("esperava disparado no horário: %+v", l)
	}

	// Adiar reagenda; na nova hora, com o servidor parado por 10 min, dispara atrasado.
	minutos := 5
	a.chamar("POST", "/lembretes/L-0001/adiamento", "", api.Adiamento{Minutos: &minutos}, 200, &l)
	if l.Status != api.Adiado || l.Adiamentos != 1 || l.Quando.Format(time.RFC3339) != "2026-10-20T15:05:10-03:00" {
		t.Fatalf("adiamento inesperado: %+v", l)
	}
	*agora = quando.Add(15 * time.Minute)
	a.servidor.DispararVencidos()
	a.chamar("GET", "/lembretes/L-0001", "", nil, 200, &l)
	if l.Status != api.Disparado || !*l.Atrasado {
		t.Fatalf("esperava disparado atrasado: %+v", l)
	}

	// Confirmar encerra; repetir não muda nada; depois disso, adiar é conflito.
	a.chamar("POST", "/lembretes/L-0001/confirmacao", "", nil, 200, &l)
	a.chamar("POST", "/lembretes/L-0001/confirmacao", "", nil, 200, &l)
	if l.Status != api.Confirmado || l.ConfirmadoEm == nil {
		t.Fatalf("esperava confirmado: %+v", l)
	}
	a.chamar("POST", "/lembretes/L-0001/adiamento", "", api.Adiamento{Minutos: &minutos}, 409, nil)

	var eventos []api.Evento
	tipo := "lembrete_criado,lembrete_disparado,lembrete_adiado,lembrete_confirmado"
	a.chamar("GET", "/eventos?entidade=L-0001&tipo="+tipo, "", nil, 200, &eventos)
	var tipos []string
	for _, e := range eventos {
		tipos = append(tipos, e.Tipo)
	}
	if got := len(tipos); got != 5 {
		t.Fatalf("eventos do lembrete: %v", tipos)
	}

	// Listagem padrão: só os que pedem atenção.
	var lista []api.Lembrete
	a.chamar("GET", "/lembretes", "", nil, 200, &lista)
	if len(lista) != 0 {
		t.Fatalf("confirmado não pede atenção: %+v", lista)
	}
	a.chamar("GET", "/lembretes?status=confirmado,cancelado", "", nil, 200, &lista)
	if len(lista) != 1 {
		t.Fatalf("filtro por status: %+v", lista)
	}
}

func TestPendencias(t *testing.T) {
	a := novoAmbiente(t)
	agora := a.relogio(time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC))
	for i, minutos := range []int{30, 90, 300} {
		a.chamar("POST", "/lembretes", "", api.NovoLembrete{Texto: "lembrete " + string(rune('a'+i)), Quando: agora.Add(time.Duration(minutos) * time.Minute), Organizacao: "Exemplo"}, 201, nil)
	}
	*agora = agora.Add(31 * time.Minute)
	a.servidor.DispararVencidos()

	a.tarefa("Revisar")
	status := api.EmValidacao
	a.chamar("PATCH", "/tarefas/T-0001", "", api.AlteracaoTarefa{Status: &status}, 200, nil)
	s := a.sessao("Claude Opus 5.5 Medium")
	a.chamar("POST", "/sessoes/"+s.Sessao.Id+"/espera", s.Token, api.Espera{Mensagem: "aprovar o PR"}, 200, nil)
	a.chamar("POST", "/sessoes/"+s.Sessao.Id+"/espera", "", api.Espera{Mensagem: "x"}, 401, nil)

	var p api.Pendencias
	a.chamar("GET", "/integracao/pendencias", "", nil, 200, &p)
	if len(p.Lembretes) != 2 || p.Lembretes[0].Status != api.Disparado || p.Lembretes[1].Id != "L-0002" {
		t.Fatalf("lembretes pendentes: %+v", p.Lembretes)
	}
	if len(p.TarefasEmValidacao) != 1 || len(p.SessoesAguardando) != 1 || p.SessoesAguardando[0].Aguardando.Mensagem != "aprovar o PR" {
		t.Fatalf("pendências: %+v", p)
	}
	a.chamar("GET", "/integracao/pendencias?horizonte=600", "", nil, 200, &p)
	if len(p.Lembretes) != 3 {
		t.Fatalf("horizonte maior: %+v", p.Lembretes)
	}

	// O cursor continua o log a partir do retrato.
	cursor := p.UltimoEvento
	a.chamar("DELETE", "/sessoes/"+s.Sessao.Id+"/espera", s.Token, nil, 200, nil)
	var eventos []api.Evento
	a.chamar("GET", "/eventos?depois="+itoa(cursor), "", nil, 200, &eventos)
	if len(eventos) != 1 || eventos[0].Tipo != "sessao_retomada" {
		t.Fatalf("eventos depois do cursor: %+v", eventos)
	}
	a.chamar("GET", "/integracao/pendencias", "", nil, 200, &p)
	if len(p.SessoesAguardando) != 0 {
		t.Fatalf("sessão retomada ainda aguarda: %+v", p.SessoesAguardando)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// capturar faz POST /captura e devolve o status, a frase e o id criado.
func (a *ambiente) capturar(token string, c api.Captura) (int, string, string) {
	a.t.Helper()
	corpo, _ := json.Marshal(c)
	req, _ := http.NewRequest("POST", a.srv.URL+"/captura", bytes.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	frase, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(frase), resp.Header.Get("X-rex-Id")
}

func TestCaptura(t *testing.T) {
	a := novoAmbiente(t)
	fuso, _ := time.LoadLocation("America/Sao_Paulo")
	agora := a.relogio(time.Date(2026, 10, 20, 14, 10, 0, 0, fuso))
	lembrete := api.Captura{Tipo: api.TipoDeCapturaLembrete, Texto: "ligar para o contador amanhã às 9"}

	if st, _, _ := a.capturar("segredo", lembrete); st != 404 {
		t.Fatalf("captura desligada deveria ser 404: %d", st)
	}
	a.servidor.ComIntegracao(Integracao{Fuso: fuso, TokenDeCaptura: "segredo", OrganizacaoDaCaptura: "Pessoal"})
	if st, _, _ := a.capturar("errado", lembrete); st != 401 {
		t.Fatalf("token errado deveria ser 401: %d", st)
	}

	chave := "msg-1"
	lembrete.Chave = &chave
	st, frase, id := a.capturar("segredo", lembrete)
	if st != 201 || id != "L-0001" || frase != "Lembrete L-0001: ligar para o contador, quarta 21/10 às 09:00." {
		t.Fatalf("captura de lembrete: %d %q %q", st, frase, id)
	}
	// A fila reentrega: mesma chave, mesmo resultado, nada novo.
	if st2, frase2, id2 := a.capturar("segredo", lembrete); st2 != 200 || frase2 != frase || id2 != id {
		t.Fatalf("repetição: %d %q %q", st2, frase2, id2)
	}
	var l api.Lembrete
	a.chamar("GET", "/lembretes/L-0001", "", nil, 200, &l)
	if l.Autor != "captura" || l.Organizacao != "Pessoal" {
		t.Fatalf("lembrete capturado: %+v", l)
	}
	a.chamar("GET", "/lembretes/L-0002", "", nil, 404, nil)

	// Hora ambígua: pede de novo.
	if st, frase, _ := a.capturar("segredo", api.Captura{Tipo: api.TipoDeCapturaLembrete, Texto: "ligar às 3"}); st != 422 || frase == "" {
		t.Fatalf("ambígua: %d %q", st, frase)
	}

	// Ditado às 14:10, chegou às 14:40 pela fila: "daqui a 20 minutos" já passou; cria e dispara.
	dita := *agora
	*agora = agora.Add(30 * time.Minute)
	st, frase, id = a.capturar("segredo", api.Captura{Tipo: api.TipoDeCapturaLembrete, Texto: "tirar o bolo do forno daqui a 20 minutos", DitaEm: &dita})
	a.chamar("GET", "/lembretes/"+id, "", nil, 200, &l)
	if st != 201 || l.Status != api.Disparado || !*l.Atrasado || frase != "Lembrete L-0002: tirar o bolo do forno, terça 20/10 às 14:30. Chegou depois da hora e já está tocando." {
		t.Fatalf("atrasado: %d %q %+v", st, frase, l)
	}

	// Tarefa: sempre a fazer, sem lançar agente.
	st, frase, id = a.capturar("segredo", api.Captura{Tipo: api.TipoDeCapturaTarefa, Texto: "Trocar o filtro da caixa d'água"})
	var tarefa api.Tarefa
	a.chamar("GET", "/tarefas/"+id, "", nil, 200, &tarefa)
	if st != 201 || tarefa.Status != api.AFazer || tarefa.Organizacao != "Pessoal" || frase != "Tarefa T-0001 criada: Trocar o filtro da caixa d'água." {
		t.Fatalf("tarefa: %d %q %+v", st, frase, tarefa)
	}

	// Requisito: verbatim, sob o zero da organização, sem prioridade.
	var zero api.Requisito
	a.chamar("POST", "/requisitos", "", api.NovoRequisito{Titulo: "Tudo", Organizacao: ptr("Geral")}, 201, nil)
	a.chamar("POST", "/requisitos", "", api.NovoRequisito{Titulo: "Vida pessoal em ordem", Organizacao: ptr("Pessoal")}, 201, &zero)
	a.chamar("PUT", "/organizacoes/Pessoal", "", api.DefinicaoDeOrganizacao{Zero: zero.Id}, 200, nil)
	st, _, id = a.capturar("segredo", api.Captura{Tipo: api.TipoDeCapturaRequisito, Texto: "o portão abre pelo celular"})
	var req api.Requisito
	a.chamar("GET", "/requisitos/"+id, "", nil, 200, &req)
	if st != 201 || req.Titulo != "o portão abre pelo celular" || len(req.DerivaDe) != 1 || req.DerivaDe[0] != zero.Id || req.Prioridade != nil {
		t.Fatalf("requisito: %d %+v", st, req)
	}

	// Simular não cria nada.
	simular := true
	if st, frase, id := a.capturar("segredo", api.Captura{Tipo: api.TipoDeCapturaTarefa, Texto: "x", Simular: &simular}); st != 200 || id != "" || frase != "Entendi a tarefa: x." {
		t.Fatalf("simulação: %d %q %q", st, frase, id)
	}
	if st, _, _ := a.capturar("segredo", api.Captura{Tipo: "outro", Texto: "x"}); st != 400 {
		t.Fatalf("tipo desconhecido: %d", st)
	}
}
