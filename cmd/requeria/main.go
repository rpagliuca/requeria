// Comando requeria: servidor da API de Tarefas e Sessões de Trabalho.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/rpagliuca/requeria/internal/banco"
	"github.com/rpagliuca/requeria/internal/servidor"
)

func main() {
	caminho := flag.String("banco", "requeria.db", "arquivo do SQLite")
	endereco := flag.String("endereco", "127.0.0.1:8787", "endereço HTTP (só local, até a integração com Tailscale)")
	flag.Parse()

	db, err := banco.Abrir(*caminho)
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer db.Close()

	log.Printf("requeria ouvindo em http://%s (banco %s)", *endereco, *caminho)
	log.Fatal(http.ListenAndServe(*endereco, servidor.Novo(db).Handler()))
}
