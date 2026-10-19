// Comando requerial.
//
//	requerial servir   servidor da API (único processo que escreve no banco)
//	requerial mcp      servidor MCP por stdio, para um agente de IA, falando com a API
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rpagliuca/requerial/internal/banco"
	"github.com/rpagliuca/requerial/internal/ferramentas"
	"github.com/rpagliuca/requerial/internal/lancador"
	"github.com/rpagliuca/requerial/internal/servidor"
	"github.com/rpagliuca/requerial/web"
)

const enderecoPadrao = "127.0.0.1:8787"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: requerial servir | mcp [opções]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "servir":
		servir(os.Args[2:])
	case "mcp":
		servirMCP(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido: %q (use servir ou mcp)\n", os.Args[1])
		os.Exit(2)
	}
}

func servir(args []string) {
	opcoes := flag.NewFlagSet("servir", flag.ExitOnError)
	caminho := opcoes.String("banco", "requerial.db", "arquivo do SQLite")
	endereco := opcoes.String("endereco", enderecoPadrao, "endereço HTTP (só local, até a integração com Tailscale)")
	configLancador := opcoes.String("lancador", "", "arquivo JSON do lançador de agentes (opcional)")
	opcoes.Parse(args)

	db, err := banco.Abrir(*caminho)
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer db.Close()

	s := servidor.Novo(db)
	if *configLancador != "" {
		l, err := lancador.Carregar(*configLancador, s.Tarefa)
		if err != nil {
			log.Fatalf("lançador: %v", err)
		}
		l.Conectar(s)
		log.Printf("lançador de agentes ativo (%s)", *configLancador)
	}

	log.Printf("requerial ouvindo em http://%s (banco %s)", *endereco, *caminho)
	log.Fatal(http.ListenAndServe(*endereco, rotas(s.Handler())))
}

// rotas junta a API e o frontend no mesmo endereço: os recursos da API têm caminho próprio e o
// resto é o frontend, que navega pelo hash da URL.
func rotas(apiHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	for _, recurso := range []string{"/tarefas", "/sessoes", "/eventos", "/importacao"} {
		mux.Handle(recurso, apiHandler)
		mux.Handle(recurso+"/", apiHandler)
	}
	mux.Handle("/", web.Handler())
	return mux
}

func servirMCP(args []string) {
	opcoes := flag.NewFlagSet("mcp", flag.ExitOnError)
	endereco := opcoes.String("api", "http://"+enderecoPadrao, "endereço da API do requerial")
	journals := opcoes.String("journals", "", "raiz dos journals: cada sessão ganha <raiz>/<S-0001>/journal.md (opcional)")
	opcoes.Parse(args)

	f, err := ferramentas.Novo(*endereco)
	if err != nil {
		log.Fatalf("cliente da API: %v", err)
	}
	f.ComJournals(*journals)
	// stdout é o canal do protocolo; logs vão para stderr.
	log.SetOutput(os.Stderr)
	if err := f.Servidor().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
