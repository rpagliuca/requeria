// Package web embute o frontend compilado (npm run build em web/) no binário do rex.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Handler serve o frontend. Sem o build, explica como gerá-lo.
func Handler() http.Handler {
	arquivos, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	if _, err := fs.Stat(arquivos, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend não compilado: rode npm ci && npm run build em web/ e recompile o binário", http.StatusNotFound)
		})
	}
	return http.FileServerFS(arquivos)
}
