package servidor

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rpagliuca/requerial/api"
	"github.com/rpagliuca/requerial/internal/banco"
)

type ambiente struct {
	t   *testing.T
	db  *sql.DB
	srv *httptest.Server
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	db, err := banco.Abrir(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Novo(db).Handler())
	t.Cleanup(func() { srv.Close(); db.Close() })
	return &ambiente{t: t, db: db, srv: srv}
}

// chamar faz a requisição, confere o status e decodifica a resposta em destino (se não for nil).
func (a *ambiente) chamar(metodo, caminho, token string, corpo any, esperado int, destino any) {
	a.t.Helper()
	var leitor *bytes.Reader
	if corpo != nil {
		texto, _ := json.Marshal(corpo)
		leitor = bytes.NewReader(texto)
	} else {
		leitor = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(metodo, a.srv.URL+caminho, leitor)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != esperado {
		var e api.Erro
		json.NewDecoder(resp.Body).Decode(&e)
		a.t.Fatalf("%s %s: status %d, esperado %d (%s)", metodo, caminho, resp.StatusCode, esperado, e.Mensagem)
	}
	if destino != nil {
		if err := json.NewDecoder(resp.Body).Decode(destino); err != nil {
			a.t.Fatal(err)
		}
	}
}

func (a *ambiente) tarefa(titulo string) api.Tarefa {
	a.t.Helper()
	var t api.Tarefa
	a.chamar("POST", "/tarefas", "", api.NovaTarefa{Titulo: titulo, Organizacao: "Exemplo"}, 201, &t)
	return t
}

func (a *ambiente) sessao(agente string, tarefas ...string) api.SessaoCriada {
	a.t.Helper()
	var s api.SessaoCriada
	a.chamar("POST", "/sessoes", "", api.NovaSessao{
		Agente: agente, TipoAgente: api.Ia, Servidor: "maquina-local", Tarefas: tarefas,
	}, 201, &s)
	return s
}

func TestTarefaExigeTituloEOrganizacao(t *testing.T) {
	a := novoAmbiente(t)
	a.chamar("POST", "/tarefas", "", api.NovaTarefa{Titulo: "x"}, 400, nil)
	a.chamar("POST", "/tarefas", "", api.NovaTarefa{Organizacao: "Exemplo"}, 400, nil)

	tarefa := a.tarefa("Configurar backup")
	if tarefa.Id != "T-0001" || tarefa.Status != api.AFazer {
		t.Fatalf("tarefa inesperada: %+v", tarefa)
	}
	estado := api.EmAndamento
	var alterada api.Tarefa
	a.chamar("PATCH", "/tarefas/T-0001", "", api.AlteracaoTarefa{Status: &estado}, 200, &alterada)
	if alterada.Status != api.EmAndamento {
		t.Fatalf("status não alterado: %s", alterada.Status)
	}
	a.chamar("GET", "/tarefas/T-0099", "", nil, 404, nil)
}

func TestUmaSessaoAtivaPorTarefa(t *testing.T) {
	a := novoAmbiente(t)
	a.tarefa("Configurar backup")
	a.tarefa("Revisar firewall")

	primeira := a.sessao("Claude Opus 5.5 Medium", "T-0001")
	var tarefa api.Tarefa
	a.chamar("GET", "/tarefas/T-0001", "", nil, 200, &tarefa)
	if tarefa.SessaoAtiva == nil || *tarefa.SessaoAtiva != primeira.Sessao.Id {
		t.Fatalf("tarefa sem a sessão ativa: %+v", tarefa)
	}

	// Outra sessão na mesma tarefa é recusada, inclusive por vínculo posterior.
	a.chamar("POST", "/sessoes", "", api.NovaSessao{
		Agente: "Claude Fable 5.1 High", TipoAgente: api.Ia, Servidor: "maquina-local", Tarefas: []string{"T-0001"},
	}, 409, nil)
	segunda := a.sessao("Claude Fable 5.1 High", "T-0002")
	a.chamar("POST", "/sessoes/"+segunda.Sessao.Id+"/tarefas", segunda.Token, api.Vinculo{Tarefa: "T-0001"}, 409, nil)

	// A recusa não deixou sessão órfã para trás.
	var sessoes []api.Sessao
	a.chamar("GET", "/sessoes", "", nil, 200, &sessoes)
	if len(sessoes) != 2 {
		t.Fatalf("esperava 2 sessões, há %d", len(sessoes))
	}
}

func TestEncerramentoExigeTokenESincronizaResumo(t *testing.T) {
	a := novoAmbiente(t)
	a.tarefa("Configurar backup")
	a.tarefa("Revisar firewall")
	s := a.sessao("Claude Opus 5.5 Medium", "T-0001", "T-0002")
	outra := a.sessao("Claude Opus 5.5 Medium", a.tarefa("Outra").Id)

	caminho := "/sessoes/" + s.Sessao.Id + "/encerramento"
	a.chamar("POST", caminho, "", api.Encerramento{Resumo: "feito"}, 401, nil)
	a.chamar("POST", caminho, outra.Token, api.Encerramento{Resumo: "feito"}, 401, nil)
	a.chamar("POST", caminho, s.Token, api.Encerramento{Resumo: " "}, 400, nil)

	journal := "github.com/exemplo/journals@abc123:2026/s-0001.md"
	var encerrada api.Sessao
	a.chamar("POST", caminho, s.Token, api.Encerramento{Resumo: "Segundo destino configurado; falta testar a restauração.", Journal: &journal}, 200, &encerrada)
	if encerrada.Status != api.Encerrada || encerrada.EncerradaEm == nil || *encerrada.Journal != journal ||
		encerrada.Resumo == nil || *encerrada.Resumo != "Segundo destino configurado; falta testar a restauração." {
		t.Fatalf("sessão não encerrada corretamente: %+v", encerrada)
	}

	for _, id := range []string{"T-0001", "T-0002"} {
		var tarefa api.Tarefa
		a.chamar("GET", "/tarefas/"+id, "", nil, 200, &tarefa)
		if tarefa.Resumo != "Segundo destino configurado; falta testar a restauração." || tarefa.SessaoAtiva != nil {
			t.Fatalf("%s sem resumo ou ainda presa: %+v", id, tarefa)
		}
	}

	// Liberada, a tarefa aceita nova sessão; o token antigo não serve para mais nada.
	a.sessao("Claude Fable 5.1 High", "T-0001")
	a.chamar("POST", caminho, s.Token, api.Encerramento{Resumo: "de novo"}, 409, nil)
	a.chamar("POST", "/tarefas", s.Token, api.NovaTarefa{Titulo: "x", Organizacao: "Exemplo"}, 401, nil)

	var historico []api.Sessao
	a.chamar("GET", "/tarefas/T-0001/sessoes", "", nil, 200, &historico)
	if len(historico) != 2 || historico[1].Id != s.Sessao.Id {
		t.Fatalf("histórico inesperado: %+v", historico)
	}
}

func TestRepasseTrocaAgenteEmUmaOperacao(t *testing.T) {
	a := novoAmbiente(t)
	a.tarefa("Configurar backup")
	s := a.sessao("Claude Opus 5.5 Medium", "T-0001")
	caminho := "/sessoes/" + s.Sessao.Id + "/repasse"

	a.chamar("POST", caminho, s.Token, api.Repasse{Agente: "Claude Opus 5.5 Medium", TipoAgente: api.Ia}, 400, nil)

	var nova api.SessaoCriada
	a.chamar("POST", caminho, s.Token, api.Repasse{Agente: "Claude Fable 5.1 High", TipoAgente: api.Ia}, 201, &nova)
	if nova.Sessao.Anterior == nil || *nova.Sessao.Anterior != s.Sessao.Id ||
		len(nova.Sessao.Tarefas) != 1 || nova.Sessao.Tarefas[0] != "T-0001" || nova.Sessao.Servidor != "maquina-local" {
		t.Fatalf("sessão nova inesperada: %+v", nova.Sessao)
	}

	var antiga api.Sessao
	a.chamar("GET", "/sessoes/"+s.Sessao.Id, "", nil, 200, &antiga)
	if antiga.Status != api.Substituida || antiga.MotivoEncerramento == nil || *antiga.MotivoEncerramento != api.TrocaDeAgente {
		t.Fatalf("sessão antiga não marcada como substituída: %+v", antiga)
	}

	var tarefa api.Tarefa
	a.chamar("GET", "/tarefas/T-0001", "", nil, 200, &tarefa)
	if *tarefa.SessaoAtiva != nova.Sessao.Id {
		t.Fatalf("a tarefa não passou para a sessão nova: %+v", tarefa)
	}

	// A sessão antiga não pode repassar de novo.
	a.chamar("POST", caminho, s.Token, api.Repasse{Agente: "Rafael", TipoAgente: api.Humano}, 409, nil)
}

func TestInvariantesNoBanco(t *testing.T) {
	a := novoAmbiente(t)
	a.tarefa("Configurar backup")
	a.sessao("Claude Opus 5.5 Medium", "T-0001")

	// Mesmo escrevendo direto no banco, as invariantes valem.
	if _, err := a.db.Exec(`UPDATE sessoes SET agente = 'outro' WHERE id = 1`); err == nil {
		t.Fatal("o banco aceitou trocar o agente da sessão")
	}
	if _, err := a.db.Exec(`INSERT INTO sessoes (agente, tipo_agente, servidor, token_hash, iniciada_em)
		VALUES ('x', 'ia', 'y', 'h', 'agora')`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO sessao_tarefa (sessao_id, tarefa_id) VALUES (2, 1)`); err == nil {
		t.Fatal("o banco aceitou duas sessões ativas na mesma tarefa")
	}
	if _, err := a.db.Exec(`UPDATE eventos SET autor = 'outro'`); err == nil {
		t.Fatal("o banco aceitou alterar o log de eventos")
	}
	if _, err := a.db.Exec(`DELETE FROM eventos`); err == nil {
		t.Fatal("o banco aceitou apagar o log de eventos")
	}
}

func TestEventosRegistramAutor(t *testing.T) {
	a := novoAmbiente(t)
	a.tarefa("Configurar backup")
	s := a.sessao("Claude Opus 5.5 Medium", "T-0001")
	resumo := "andamento"
	a.chamar("PATCH", "/tarefas/T-0001", s.Token, api.AlteracaoTarefa{Resumo: &resumo}, 200, nil)

	var eventos []api.Evento
	a.chamar("GET", "/eventos?entidade=T-0001", "", nil, 200, &eventos)
	if len(eventos) != 2 {
		t.Fatalf("esperava 2 eventos da tarefa, há %d: %+v", len(eventos), eventos)
	}
	if eventos[0].Tipo != "tarefa_criada" || eventos[0].Autor != "local" {
		t.Fatalf("evento de criação inesperado: %+v", eventos[0])
	}
	if eventos[1].Tipo != "tarefa_alterada" || eventos[1].Autor != s.Sessao.Id {
		t.Fatalf("alteração não atribuída à sessão: %+v", eventos[1])
	}
}

func TestImportacaoPreservaDatasEHistorico(t *testing.T) {
	a := novoAmbiente(t)
	inicio := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	journalA, journalB := "maquina-a:/tarefas/x/CLAUDE.md", "maquina-b:/tarefas/x/CLAUDE.md"
	var tarefa api.Tarefa
	a.chamar("POST", "/importacao", "", api.ImportacaoTarefa{
		Titulo: "Tarefa antiga", Organizacao: "Exemplo", Status: api.Pausada, CriadaEm: inicio, Origem: "pasta x",
		Sessoes: []api.SessaoImportada{
			{Agente: "Claude (modelo não registrado)", Servidor: "maquina-b", Journal: &journalB,
				IniciadaEm: inicio, EncerradaEm: inicio.Add(72 * time.Hour), Resumo: "mais recente"},
			{Agente: "Claude (modelo não registrado)", Servidor: "maquina-a", Journal: &journalA,
				IniciadaEm: inicio, EncerradaEm: inicio.Add(24 * time.Hour), Resumo: "mais antigo"},
		},
	}, 201, &tarefa)
	if !tarefa.CriadaEm.Equal(inicio) || tarefa.Resumo != "mais recente" || tarefa.SessaoAtiva != nil || tarefa.Status != api.Pausada {
		t.Fatalf("tarefa importada inesperada: %+v", tarefa)
	}

	var historico []api.Sessao
	a.chamar("GET", "/tarefas/"+tarefa.Id+"/sessoes", "", nil, 200, &historico)
	if len(historico) != 2 || historico[0].Status != api.Encerrada ||
		*historico[0].Resumo != "mais recente" || *historico[1].Resumo != "mais antigo" {
		t.Fatalf("histórico inesperado: %+v", historico)
	}

	// A tarefa importada fica livre para uma sessão nova.
	a.sessao("Claude Opus 5.5 Medium", tarefa.Id)
}
