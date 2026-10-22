package servidor

import (
	"cmp"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/rpagliuca/rex/api"
)

// Busca e árvore: o que um agente precisa para achar o requisito certo gastando poucos tokens.

var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "í", "i", "ì", "i",
	"ó", "o", "ô", "o", "õ", "o", "ò", "o", "ú", "u", "ü", "u", "ù", "u", "ç", "c",
)

// palavras normaliza (minúsculas, sem acento) e separa em palavras.
func palavras(texto string) []string {
	return strings.FieldsFunc(semAcento.Replace(strings.ToLower(texto)), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

// casa: a palavra do texto é o termo, ou começa por ele (termos de 4+ letras: backup acha backups),
// ou o termo começa por ela (backups acha backup).
func casa(termo string, texto []string) bool {
	for _, p := range texto {
		if p == termo || len(termo) >= 4 && (strings.HasPrefix(p, termo) || len(p) >= 4 && strings.HasPrefix(termo, p)) {
			return true
		}
	}
	return false
}

// pontuar: cada termo vale 3 no título, 1 na descrição; ids (R-0001) casam direto.
func pontuar(termos []string, x api.Requisito) int {
	titulo, descricao := palavras(x.Id+" "+x.Titulo), palavras(x.Descricao)
	pontos := 0
	for _, t := range termos {
		switch {
		case casa(t, titulo):
			pontos += 3
		case casa(t, descricao):
			pontos++
		}
	}
	return pontos
}

var palavrasVazias = map[string]bool{"a": true, "o": true, "as": true, "os": true, "de": true, "da": true, "do": true,
	"das": true, "dos": true, "e": true, "em": true, "no": true, "na": true, "nos": true, "nas": true, "um": true,
	"uma": true, "para": true, "por": true, "com": true, "que": true, "se": true, "ao": true}

func (s *Servidor) todosOsRequisitos() ([]api.Requisito, ligacoes, error) {
	l, err := carregarLigacoes(s.db)
	if err != nil {
		return nil, l, err
	}
	linhas, err := s.db.Query(`SELECT ` + colunasRequisito + ` FROM requisitos ORDER BY id`)
	if err != nil {
		return nil, l, err
	}
	defer linhas.Close()
	var todos []api.Requisito
	for linhas.Next() {
		x, err := lerRequisito(linhas, l)
		if err != nil {
			return nil, l, err
		}
		todos = append(todos, x)
	}
	return todos, l, linhas.Err()
}

func sim(b *bool) bool { return b != nil && *b }

func temComponente(x api.Requisito, c *string) bool {
	if c == nil {
		return true
	}
	for _, k := range x.Componentes {
		if k == *c {
			return true
		}
	}
	return false
}

func (s *Servidor) BuscarRequisitos(w http.ResponseWriter, r *http.Request, params api.BuscarRequisitosParams) {
	var termos []string
	for _, t := range palavras(params.Q) {
		if !palavrasVazias[t] {
			termos = append(termos, t)
		}
	}
	if len(termos) == 0 {
		responderErro(w, falha(http.StatusBadRequest, "informe o que buscar em q"))
		return
	}
	limite := 10
	if params.Limite != nil && *params.Limite > 0 {
		limite = *params.Limite
	}
	todos, _, err := s.todosOsRequisitos()
	if err != nil {
		responderErro(w, err)
		return
	}
	type achado struct {
		x      api.Requisito
		pontos int
	}
	var achados []achado
	for _, x := range todos {
		if params.Organizacao != nil && x.Organizacao != *params.Organizacao || !temComponente(x, params.Componente) ||
			x.Obsoleto != nil && !sim(params.IncluirObsoletos) {
			continue
		}
		if p := pontuar(termos, x); p > 0 {
			achados = append(achados, achado{x, p})
		}
	}
	sort.SliceStable(achados, func(i, j int) bool { return achados[i].pontos > achados[j].pontos })
	resumos := []api.ResumoDeRequisito{}
	for i, a := range achados {
		if i == limite {
			break
		}
		resumo := api.ResumoDeRequisito{Id: a.x.Id, Titulo: a.x.Titulo, Organizacao: a.x.Organizacao,
			Estado: a.x.Estado, Componentes: a.x.Componentes}
		if a.x.Obsoleto != nil {
			obsoleto := true
			resumo.Obsoleto = &obsoleto
		}
		resumos = append(resumos, resumo)
	}
	responder(w, http.StatusOK, resumos)
}

func (s *Servidor) ArvoreDeRequisitos(w http.ResponseWriter, r *http.Request, params api.ArvoreDeRequisitosParams) {
	todos, _, err := s.todosOsRequisitos()
	if err != nil {
		responderErro(w, err)
		return
	}
	porId := map[string]api.Requisito{}
	for _, x := range todos {
		porId[x.Id] = x
	}
	// O zero: o da organização, atribuído, ou o geral.
	var zero string
	if params.Organizacao != nil {
		if err := s.db.QueryRow(`SELECT 'R-' || printf('%04d', zero_id) FROM organizacoes WHERE nome = ? AND zero_id IS NOT NULL`,
			*params.Organizacao).Scan(&zero); err != nil {
			responderErro(w, falha(http.StatusNotFound, "a organização %q não tem zero atribuído", *params.Organizacao))
			return
		}
	} else if n, err := raiz(s.db); err != nil || n == 0 {
		responder(w, http.StatusOK, api.ArvoreDeRequisitos{Texto: "", Requisitos: 0})
		return
	} else {
		zero = idRequisito(n)
	}
	// Com componente, só os dele e os ancestrais.
	var visiveis map[string]bool
	if params.Componente != nil {
		visiveis = map[string]bool{}
		var subir func(id string)
		subir = func(id string) {
			if visiveis[id] {
				return
			}
			visiveis[id] = true
			for _, p := range porId[id].DerivaDe {
				subir(p)
			}
		}
		for _, x := range todos {
			if temComponente(x, params.Componente) {
				subir(x.Id)
			}
		}
	}
	var b strings.Builder
	n := 0
	var descer func(id, pai string, nivel int, caminho map[string]bool)
	descer = func(id, pai string, nivel int, caminho map[string]bool) {
		x, ok := porId[id]
		if !ok || caminho[id] || visiveis != nil && !visiveis[id] {
			return
		}
		if params.Organizacao != nil && x.Organizacao != *params.Organizacao {
			return
		}
		marca := ""
		if x.Obsoleto != nil {
			if !sim(params.IncluirObsoletos) {
				return
			}
			marca = "[obsoleto] "
		}
		n++
		b.WriteString(strings.Repeat("  ", nivel) + id + " " + importanciaEmTexto(x, pai) + marca + x.Titulo + "\n")
		caminho[id] = true
		filhos := slices.Clone(x.Derivados)
		// Os mais importantes primeiro; a ordem por id desempata.
		slices.SortStableFunc(filhos, func(a, b string) int { return cmp.Compare(fatiaSob(porId[b], id), fatiaSob(porId[a], id)) })
		for _, f := range filhos {
			descer(f, id, nivel+1, caminho)
		}
		delete(caminho, id)
	}
	descer(zero, "", 0, map[string]bool{})
	responder(w, http.StatusOK, api.ArvoreDeRequisitos{Texto: b.String(), Requisitos: n})
}

func fatiaSob(x api.Requisito, pai string) float32 {
	for _, i := range x.Importancia {
		if i.Pai == pai {
			return i.Fatia
		}
	}
	return 0
}

// importanciaEmTexto: "[3 pts · 60% · 12% do total] " sob o pai; "?" nos pontos de ligação não triada.
func importanciaEmTexto(x api.Requisito, pai string) string {
	for _, i := range x.Importancia {
		if i.Pai == pai {
			pontos := "?"
			if i.Pontos != nil {
				pontos = strconv.Itoa(*i.Pontos)
			}
			return fmt.Sprintf("[%s pts · %s · %s do total] ", pontos, percentual(i.Fatia), percentual(x.Peso))
		}
	}
	return ""
}

func percentual(f float32) string {
	if f > 0 && f < 0.001 {
		return "<0,1%"
	}
	return strings.Replace(strconv.FormatFloat(float64(f)*100, 'f', 1, 32), ".", ",", 1) + "%"
}

// trilhas: cada caminho do zero até n, em "id título" separados por " › " (no máximo 5).
func trilhas(x api.Requisito, porId map[string]api.Requisito) []string {
	var todas []string
	var subir func(id string, caminho []string)
	subir = func(id string, caminho []string) {
		if len(todas) >= 5 {
			return
		}
		r := porId[id]
		caminho = append([]string{id + " " + r.Titulo}, caminho...)
		if len(r.DerivaDe) == 0 {
			todas = append(todas, strings.Join(caminho, " › "))
			return
		}
		for _, p := range r.DerivaDe {
			subir(p, caminho)
		}
	}
	for _, p := range x.DerivaDe {
		subir(p, nil)
	}
	return todas
}

const limiteDaConsulta, limiteMaximoDaConsulta = 20, 50

// ConsultarTarefas: a consulta de tarefas para agentes, filtrada e compacta (nunca a lista inteira).
func (s *Servidor) ConsultarTarefas(w http.ResponseWriter, r *http.Request, params api.ConsultarTarefasParams) {
	var filtros []string
	var args []any
	if params.Status != nil {
		var status []string
		for _, x := range strings.Split(*params.Status, ",") {
			x = strings.TrimSpace(x)
			if x == "" {
				continue
			}
			if !statusDeTarefaValido(api.StatusTarefa(x)) {
				responderErro(w, falha(http.StatusBadRequest, "status de tarefa inválido: %q", x))
				return
			}
			status = append(status, "?")
			args = append(args, x)
		}
		if len(status) > 0 {
			filtros = append(filtros, `t.status IN (`+strings.Join(status, ", ")+`)`)
		}
	}
	if params.Organizacao != nil && !vazio(*params.Organizacao) {
		filtros = append(filtros, `t.organizacao = ?`)
		args = append(args, strings.TrimSpace(*params.Organizacao))
	}
	ligado := func(id *string, prefixo, consulta string) error {
		if id == nil || vazio(*id) {
			return nil
		}
		n, err := numero(strings.TrimSpace(*id), prefixo)
		if err != nil {
			return err
		}
		filtros = append(filtros, consulta)
		args = append(args, n)
		return nil
	}
	if err := ligado(params.Componente, "C", `t.id IN (SELECT tarefa_id FROM tarefa_componente WHERE componente_id = ?)`); err != nil {
		responderErro(w, err)
		return
	}
	if err := ligado(params.Requisito, "R", `t.id IN (SELECT tarefa_id FROM tarefa_requisito WHERE requisito_id = ?)`); err != nil {
		responderErro(w, err)
		return
	}
	if len(filtros) == 0 {
		responderErro(w, falha(http.StatusBadRequest, "informe ao menos um filtro: status, organizacao, componente ou requisito"))
		return
	}
	limite := limiteDaConsulta
	if params.Limite != nil {
		limite = min(max(*params.Limite, 1), limiteMaximoDaConsulta)
	}
	linhas, err := s.db.Query(`SELECT t.id, t.titulo, t.status, t.organizacao, st.sessao_id, se.aguardando
		FROM tarefas t
		LEFT JOIN sessao_tarefa st ON st.tarefa_id = t.id AND st.ativa = 1
		LEFT JOIN sessoes se ON se.id = st.sessao_id
		WHERE `+strings.Join(filtros, " AND ")+`
		ORDER BY t.atualizada_em DESC, t.id DESC`, args...)
	if err != nil {
		responderErro(w, err)
		return
	}
	defer linhas.Close()
	out := api.ConsultaDeTarefas{Tarefas: []api.ResumoDeTarefa{}}
	for linhas.Next() {
		out.Total++
		if len(out.Tarefas) == limite {
			out.Fora++
			continue
		}
		var t api.ResumoDeTarefa
		var n int64
		var status string
		var sessao sql.NullInt64
		var aguardando sql.NullString
		if err := linhas.Scan(&n, &t.Titulo, &status, &t.Organizacao, &sessao, &aguardando); err != nil {
			responderErro(w, err)
			return
		}
		t.Id, t.Status = idTarefa(n), api.StatusTarefa(status)
		if sessao.Valid {
			id := idSessao(sessao.Int64)
			t.SessaoAtiva = &id
		}
		if aguardando.Valid {
			t.Aguardando = &aguardando.String
		}
		out.Tarefas = append(out.Tarefas, t)
	}
	if err := linhas.Err(); err != nil {
		responderErro(w, err)
		return
	}
	responder(w, http.StatusOK, out)
}
