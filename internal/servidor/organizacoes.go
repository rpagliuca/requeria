package servidor

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/rpagliuca/requerial/api"
)

// ListarOrganizacoes: as que aparecem em tarefas, requisitos e templates, mais as que têm zero atribuído.
func (s *Servidor) ListarOrganizacoes(w http.ResponseWriter, r *http.Request) {
	linhas, err := s.db.Query(`SELECT nome, max(zero_id) FROM (
			SELECT organizacao AS nome, NULL AS zero_id FROM tarefas
			UNION SELECT organizacao, NULL FROM requisitos
			UNION SELECT organizacao, NULL FROM templates
			UNION SELECT nome, zero_id FROM organizacoes)
		GROUP BY nome ORDER BY nome`)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	orgs := []api.Organizacao{}
	for linhas.Next() {
		var o api.Organizacao
		var zero sql.NullInt64
		if err := linhas.Scan(&o.Nome, &zero); err != nil {
			responderErro(w, err)
			return
		}
		if zero.Valid {
			id := idRequisito(zero.Int64)
			o.Zero = &id
		}
		orgs = append(orgs, o)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, orgs)
}

func (s *Servidor) DefinirOrganizacao(w http.ResponseWriter, r *http.Request, nome string) {
	var def api.DefinicaoDeOrganizacao
	if err := ler(r, &def); err != nil {
		responderErro(w, err)
		return
	}
	err := s.emTransacao(func(tx *sql.Tx) error {
		quem, err := autor(tx, r)
		if err != nil {
			return err
		}
		n, err := numero(def.Zero, "R")
		if err != nil {
			return err
		}
		var org string
		err = tx.QueryRow(`SELECT organizacao FROM requisitos WHERE id = ?`, n).Scan(&org)
		if errors.Is(err, sql.ErrNoRows) {
			return falha(http.StatusNotFound, "requisito %s não encontrado", def.Zero)
		}
		if err != nil {
			return err
		}
		if org != nome {
			return falha(http.StatusBadRequest, "%s é da organização %s, não de %s", def.Zero, org, nome)
		}
		if _, err := tx.Exec(`INSERT INTO organizacoes (nome, zero_id) VALUES (?, ?)
			ON CONFLICT (nome) DO UPDATE SET zero_id = excluded.zero_id`, nome, n); err != nil {
			return err
		}
		return s.registrarEvento(tx, quem, "organizacao:"+nome, "zero_atribuido", map[string]any{"zero": def.Zero})
	})
	if err != nil {
		responderErro(w, err)
		return
	}
	id := def.Zero
	responder(w, http.StatusOK, api.Organizacao{Nome: nome, Zero: &id})
}
