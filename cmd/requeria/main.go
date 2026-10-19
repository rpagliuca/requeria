// Comando requeria.
//
//	requeria servir   servidor da API (único processo que escreve no banco)
//	requeria mcp      servidor MCP por stdio, para um agente de IA, falando com a API
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rpagliuca/requeria/internal/banco"
	"github.com/rpagliuca/requeria/internal/ferramentas"
	"github.com/rpagliuca/requeria/internal/servidor"
)

const enderecoPadrao = "127.0.0.1:8787"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: requeria servir | mcp [opções]")
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
	caminho := opcoes.String("banco", "requeria.db", "arquivo do SQLite")
	endereco := opcoes.String("endereco", enderecoPadrao, "endereço HTTP (só local, até a integração com Tailscale)")
	opcoes.Parse(args)

	db, err := banco.Abrir(*caminho)
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer db.Close()

	log.Printf("requeria ouvindo em http://%s (banco %s)", *endereco, *caminho)
	log.Fatal(http.ListenAndServe(*endereco, servidor.Novo(db).Handler()))
}

func servirMCP(args []string) {
	opcoes := flag.NewFlagSet("mcp", flag.ExitOnError)
	endereco := opcoes.String("api", "http://"+enderecoPadrao, "endereço da API do requeria")
	opcoes.Parse(args)

	f, err := ferramentas.Novo(*endereco)
	if err != nil {
		log.Fatalf("cliente da API: %v", err)
	}
	// stdout é o canal do protocolo; logs vão para stderr.
	log.SetOutput(os.Stderr)
	if err := f.Servidor().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
