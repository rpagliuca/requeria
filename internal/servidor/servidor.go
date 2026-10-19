// Package servidor implementa a API do requerial sobre o SQLite.
//
// Toda escrita passa por aqui, numa transação, e gera um evento no log. As invariantes são
// conferidas antes da escrita, para dar uma mensagem clara, e o banco as impõe de novo.
package servidor

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rpagliuca/requerial/api"
)

// Servidor implementa api.ServerInterface.
type Servidor struct {
	db    *sql.DB
	agora func() time.Time
}

var _ api.ServerInterface = (*Servidor)(nil)

// Novo cria o servidor sobre um banco já migrado.
func Novo(db *sql.DB) *Servidor {
	return &Servidor{db: db, agora: func() time.Time { return time.Now().UTC() }}
}

// Handler devolve o http.Handler com todas as rotas da API.
func (s *Servidor) Handler() http.Handler {
	return api.Handler(s)
}

// erroHTTP é um erro que já sabe qual status devolver.
type erroHTTP struct {
	status   int
	mensagem string
}

func (e *erroHTTP) Error() string { return e.mensagem }

func falha(status int, formato string, args ...any) error {
	return &erroHTTP{status: status, mensagem: fmt.Sprintf(formato, args...)}
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(corpo)
}

func responderErro(w http.ResponseWriter, err error) {
	var e *erroHTTP
	if errors.As(err, &e) {
		responder(w, e.status, api.Erro{Mensagem: e.mensagem})
		return
	}
	responder(w, http.StatusInternalServerError, api.Erro{Mensagem: err.Error()})
}

func ler(r *http.Request, destino any) error {
	if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
		return falha(http.StatusBadRequest, "corpo inválido: %v", err)
	}
	return nil
}

// emTransacao executa f numa transação, desfeita se f falhar.
func (s *Servidor) emTransacao(f func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := f(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Servidor) momento() string {
	return s.agora().Format(time.RFC3339Nano)
}

func lerMomento(texto string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, texto)
	return t
}

// Identificadores públicos: T-0001 para tarefas, S-0001 para sessões.

func idTarefa(n int64) string { return fmt.Sprintf("T-%04d", n) }
func idSessao(n int64) string { return fmt.Sprintf("S-%04d", n) }

func numero(id, prefixo string) (int64, error) {
	resto, ok := strings.CutPrefix(id, prefixo+"-")
	n, err := strconv.ParseInt(resto, 10, 64)
	if !ok || err != nil || n <= 0 {
		return 0, falha(http.StatusNotFound, "identificador inválido: %q", id)
	}
	return n, nil
}

func (s *Servidor) registrarEvento(tx *sql.Tx, autor, entidade, tipo string, dados map[string]any) error {
	if dados == nil {
		dados = map[string]any{}
	}
	texto, err := json.Marshal(dados)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO eventos (momento, autor, entidade, tipo, dados) VALUES (?, ?, ?, ?, ?)`,
		s.momento(), autor, entidade, tipo, string(texto))
	return err
}

// Tokens de sessão: o servidor guarda só o hash.

func novoToken() (token, hash string, err error) {
	bruto := make([]byte, 32)
	if _, err := rand.Read(bruto); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(bruto)
	return token, hashDoToken(token), nil
}

func hashDoToken(token string) string {
	soma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(soma[:])
}

func tokenDaRequisicao(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return strings.TrimSpace(token)
}

// autor identifica quem faz a requisição: a sessão dona do token, ou "local" sem token.
// Um token inválido ou de sessão encerrada é recusado, para nenhuma escrita ser atribuída errado.
func autor(tx *sql.Tx, r *http.Request) (string, error) {
	token := tokenDaRequisicao(r)
	if token == "" {
		return "local", nil
	}
	var n int64
	var status string
	err := tx.QueryRow(`SELECT id, status FROM sessoes WHERE token_hash = ?`, hashDoToken(token)).Scan(&n, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", falha(http.StatusUnauthorized, "token de sessão inválido")
	}
	if err != nil {
		return "", err
	}
	if status != string(api.Ativa) {
		return "", falha(http.StatusUnauthorized, "o token pertence à sessão %s, que já foi encerrada", idSessao(n))
	}
	return idSessao(n), nil
}

// autenticarSessao exige o token da própria sessão e que ela esteja ativa.
func autenticarSessao(tx *sql.Tx, r *http.Request, n int64) error {
	token := tokenDaRequisicao(r)
	if token == "" {
		return falha(http.StatusUnauthorized, "esta operação exige o token da sessão %s", idSessao(n))
	}
	var hash, status string
	err := tx.QueryRow(`SELECT token_hash, status FROM sessoes WHERE id = ?`, n).Scan(&hash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return falha(http.StatusNotFound, "sessão %s não encontrada", idSessao(n))
	}
	if err != nil {
		return err
	}
	if hash != hashDoToken(token) {
		return falha(http.StatusUnauthorized, "o token não pertence à sessão %s", idSessao(n))
	}
	if status != string(api.Ativa) {
		return falha(http.StatusConflict, "a sessão %s já foi encerrada", idSessao(n))
	}
	return nil
}
