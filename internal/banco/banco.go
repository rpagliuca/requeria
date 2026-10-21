// Package banco abre o SQLite do rex e aplica as migrações versionadas.
package banco

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migracoes/*.sql
var migracoes embed.FS

// Abrir abre (ou cria) o banco no caminho dado e aplica as migrações pendentes.
// Use ":memory:" para um banco efêmero.
func Abrir(caminho string) (*sql.DB, error) {
	dsn := "file:" + caminho + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if caminho != ":memory:" {
		dsn += "&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Um único escritor: o servidor serializa as escritas e evita SQLITE_BUSY.
	// Também mantém o mesmo banco quando ele é ":memory:".
	db.SetMaxOpenConns(1)
	if err := migrar(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrar(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS migracoes (nome TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	nomes, err := fs.Glob(migracoes, "migracoes/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(nomes)
	for _, nome := range nomes {
		var aplicada int
		if err := db.QueryRow(`SELECT count(*) FROM migracoes WHERE nome = ?`, nome).Scan(&aplicada); err != nil {
			return err
		}
		if aplicada > 0 {
			continue
		}
		conteudo, err := migracoes.ReadFile(nome)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(conteudo)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(`INSERT INTO migracoes (nome) VALUES (?)`, nome); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
