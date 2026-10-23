// Package captura interpreta o que foi ditado a um assistente de voz: separa, num pedido de
// lembrete, o que lembrar da data e hora, em português, sem chutar quando a hora é ambígua.
package captura

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Duvida é a recusa de interpretar, com a frase que pede ao usuário para dizer de novo.
type Duvida struct{ Frase string }

func (d *Duvida) Error() string { return d.Frase }

func duvida(formato string, args ...any) error { return &Duvida{Frase: fmt.Sprintf(formato, args...)} }

// Precisao diz se o pedido deu a hora (Hora), só um turno (Turno) ou só o dia (Dia).
type Precisao string

const (
	Hora  Precisao = "hora"
	Turno Precisao = "turno"
	Dia   Precisao = "dia"
)

// Lembrete é um pedido de lembrete interpretado. Em Turno e Dia, Quando é a meia-noite do dia e
// NomeDoTurno diz qual turno (manha, almoco, tarde ou noite); a hora de cada turno é da instalação.
type Lembrete struct {
	Texto       string
	Quando      time.Time
	Precisao    Precisao
	NomeDoTurno string
}

var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
	"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c",
	"Á", "a", "À", "a", "Â", "a", "Ã", "a", "É", "e", "Ê", "e", "Í", "i",
	"Ó", "o", "Ô", "o", "Õ", "o", "Ú", "u", "Ç", "c",
)

// normalizar põe em minúsculas e sem acento, rune a rune: o resultado tem o mesmo número de runes
// do original, para que um trecho encontrado nele possa ser recortado do original.
func normalizar(s string) []rune {
	var out []rune
	for _, r := range s {
		n := []rune(semAcento.Replace(strings.ToLower(string(r))))
		if len(n) != 1 {
			n = []rune{r}
		}
		out = append(out, n[0])
	}
	return out
}

var numerosPorExtenso = map[string]int{
	"um": 1, "uma": 1, "dois": 2, "duas": 2, "tres": 3, "quatro": 4, "cinco": 5, "seis": 6, "sete": 7,
	"oito": 8, "nove": 9, "dez": 10, "onze": 11, "doze": 12, "quinze": 15, "vinte": 20, "trinta": 30,
	"quarenta": 40, "quarenta e cinco": 45, "cinquenta": 50, "meia": 0,
}

var meses = map[string]time.Month{
	"janeiro": 1, "fevereiro": 2, "marco": 3, "abril": 4, "maio": 5, "junho": 6, "julho": 7,
	"agosto": 8, "setembro": 9, "outubro": 10, "novembro": 11, "dezembro": 12,
}

var diasDaSemana = map[string]time.Weekday{
	"domingo": time.Sunday, "segunda": time.Monday, "terca": time.Tuesday, "quarta": time.Wednesday,
	"quinta": time.Thursday, "sexta": time.Friday, "sabado": time.Saturday,
}

var nomesDosDias = []string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

const numero = `(\d{1,3}|um|uma|dois|duas|tres|quatro|cinco|seis|sete|oito|nove|dez|onze|doze|quinze|vinte|trinta|quarenta e cinco|quarenta|cinquenta|meia)`

var (
	reRelativo    = regexp.MustCompile(`\b(?:daqui a|daqui|dentro de|em)\s+` + numero + `\s+(minutos?|min|horas?|h|dias?)(?:\s+e\s+meia)?\b`)
	reMeiaHora    = regexp.MustCompile(`\b(?:daqui a|dentro de|em)\s+meia\s+hora\b`)
	reDia         = regexp.MustCompile(`\b(?:depois de amanha|amanha|hoje)\b`)
	reSemana      = regexp.MustCompile(`\b(?:(?:na|no|nesta|neste|esta|este|proxima|proximo|na proxima|no proximo)\s+)?(segunda|terca|quarta|quinta|sexta|sabado|domingo)(?:-feira|\s+feira)?\b`)
	reData        = regexp.MustCompile(`\b(?:(?:no\s+)?dia\s+)?(\d{1,2})/(\d{1,2})(?:/(\d{2,4}))?\b`)
	reDiaMes      = regexp.MustCompile(`\b(?:(?:no\s+)?dia\s+)?(\d{1,2})\s+de\s+(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b`)
	reDiaSo       = regexp.MustCompile(`\b(?:no\s+)?dia\s+(\d{1,2})\b`)
	reMeioDia     = regexp.MustCompile(`\b(?:(?:ao|as|a)\s+)?(meio[- ]dia|meia[- ]noite)(?:\s+e\s+meia)?\b`)
	reMarcaDeHora = regexp.MustCompile(`\d\s*(h|:|horas?)`)
	// O almoço só com preposição: "almoço com a Ana" é o texto do lembrete.
	reTurno = regexp.MustCompile(`\b(?:(?:de|da|pela|na|nesta|esta|a)\s+)?(manha|tarde|noite)\b|\b(?:na hora do|no)\s+(almoco)\b`)
	reHora  = regexp.MustCompile(`\b(?:(?:as|a|ate as|pelas|por volta das)\s+)?(\d{1,2})(?:\s*(?:h|:|horas?)\s*(\d{2})?|\s+e\s+(meia|\d{1,2})(?:\s+minutos?)?)?(?:\s+(?:da|de)\s+(manha|tarde|noite|madrugada))?\b`)
)

// trecho é uma parte reconhecida do texto, em índices de rune.
type trecho struct{ inicio, fim int }

type leitura struct {
	norm     string
	trechos  []trecho
	original []rune
}

func (l *leitura) marcar(bytesInicio, bytesFim int) {
	l.trechos = append(l.trechos, trecho{
		utf8.RuneCountInString(l.norm[:bytesInicio]),
		utf8.RuneCountInString(l.norm[:bytesFim]),
	})
}

// apagar devolve o original sem os trechos reconhecidos, sem conectivos soltos nas pontas.
func (l *leitura) apagar() string {
	runes := append([]rune(nil), l.original...)
	for _, t := range l.trechos {
		for i := t.inicio; i < t.fim; i++ {
			runes[i] = ' '
		}
	}
	texto := strings.Join(strings.Fields(string(runes)), " ")
	texto = strings.Trim(texto, " ,.;:-")
	for {
		menor := strings.ToLower(texto)
		cortou := false
		for _, conectivo := range []string{"me lembre de ", "me lembra de ", "lembrar de ", "lembre de ", "lembrete de ", "lembrete ", "de ", "que ", "para ", "pra "} {
			if strings.HasPrefix(menor, conectivo) {
				texto, cortou = texto[len(conectivo):], true
				break
			}
		}
		for _, conectivo := range []string{" de", " para", " pra", " na", " no", " às", " as", " a", " e"} {
			if strings.HasSuffix(menor, conectivo) && !cortou {
				texto, cortou = texto[:len(texto)-len(conectivo)], true
				break
			}
		}
		texto = strings.Trim(texto, " ,.;:-")
		if !cortou {
			return texto
		}
	}
}

func valor(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return numerosPorExtenso[s]
}

// InterpretarLembrete separa o que lembrar e quando. ref é o instante em que foi dito; horas sem
// fuso valem no fuso dado.
func InterpretarLembrete(texto string, ref time.Time, fuso *time.Location) (Lembrete, error) {
	ref = ref.In(fuso)
	l := &leitura{original: []rune(texto)}
	l.norm = string(normalizar(texto))

	if m := reMeiaHora.FindStringIndex(l.norm); m != nil {
		l.marcar(m[0], m[1])
		return l.resultado(ref.Add(30 * time.Minute).Truncate(time.Minute))
	}
	if m := reRelativo.FindStringSubmatchIndex(l.norm); m != nil {
		n := valor(l.norm[m[2]:m[3]])
		unidade := l.norm[m[4]:m[5]]
		meia := strings.HasSuffix(l.norm[m[0]:m[1]], "e meia")
		l.marcar(m[0], m[1])
		var d time.Duration
		switch {
		case strings.HasPrefix(unidade, "min"):
			d = time.Duration(n) * time.Minute
		case strings.HasPrefix(unidade, "h"):
			d = time.Duration(n) * time.Hour
			if meia {
				d += 30 * time.Minute
			}
		default:
			return l.diaRelativo(ref, n)
		}
		if d <= 0 {
			return Lembrete{}, duvida("Não entendi daqui a quanto tempo. Diga de novo, por exemplo: daqui a 20 minutos.")
		}
		return l.resultado(ref.Add(d).Truncate(time.Minute))
	}

	dia, diaExplicito, err := l.lerDia(ref)
	if err != nil {
		return Lembrete{}, err
	}
	hora, minuto, periodo, ok := l.lerHora()
	turno, temTurno := l.lerTurno()
	switch {
	case !ok && temTurno:
		return l.semHora(dia, Turno, turno)
	case !ok && diaExplicito:
		return l.semHora(dia, Dia, "")
	case !ok:
		return Lembrete{}, duvida("Não entendi quando. Diga de novo com o dia, o turno ou a hora.")
	case periodo == "" && temTurno:
		periodo = turno // "à tarde, às 3"
	}
	switch periodo {
	case "almoco", "tarde", "noite":
		if hora < 12 {
			hora += 12
		}
	case "manha", "madrugada":
		if hora == 12 {
			hora = 0
		}
	}
	if hora > 23 || minuto > 59 {
		return Lembrete{}, duvida("Não entendi a hora do lembrete. Diga de novo com o dia e a hora.")
	}
	quando := time.Date(dia.Year(), dia.Month(), dia.Day(), hora, minuto, 0, 0, fuso)
	if quando.Before(ref) {
		if diaExplicito {
			return Lembrete{}, duvida("%s já passou. Diga de novo com o dia e a hora.", Descrever(quando))
		}
		return Lembrete{}, duvida("Hoje às %02d:%02d já passou. Diga de novo com o dia, ou com da manhã, da tarde ou da noite.", hora, minuto)
	}
	return l.resultado(quando)
}

func (l *leitura) diaRelativo(ref time.Time, dias int) (Lembrete, error) {
	if dias <= 0 {
		return Lembrete{}, duvida("Não entendi daqui a quantos dias. Diga de novo com o dia e a hora.")
	}
	hora, minuto, periodo, ok := l.lerHora()
	turno, temTurno := l.lerTurno()
	d := ref.AddDate(0, 0, dias)
	if !ok {
		if temTurno {
			return l.semHora(d, Turno, turno)
		}
		return l.semHora(d, Dia, "")
	}
	if periodo == "" {
		periodo = turno
	}
	if (periodo == "almoco" || periodo == "tarde" || periodo == "noite") && hora < 12 {
		hora += 12
	}
	return l.resultado(time.Date(d.Year(), d.Month(), d.Day(), hora, minuto, 0, 0, ref.Location()))
}

func (l *leitura) resultado(quando time.Time) (Lembrete, error) {
	texto := l.apagar()
	if texto == "" {
		return Lembrete{}, duvida("Entendi a hora, mas não o que lembrar. Diga de novo o lembrete inteiro.")
	}
	return Lembrete{Texto: texto, Quando: quando, Precisao: Hora}, nil
}

// semHora é o lembrete de um turno ou de um dia inteiro: sem hora inventada.
func (l *leitura) semHora(dia time.Time, p Precisao, turno string) (Lembrete, error) {
	x, err := l.resultado(time.Date(dia.Year(), dia.Month(), dia.Day(), 0, 0, 0, 0, dia.Location()))
	x.Precisao, x.NomeDoTurno = p, turno
	return x, err
}

// lerTurno acha "de manhã", "no almoço", "à tarde", "à noite" fora da hora já lida (e marca o trecho).
func (l *leitura) lerTurno() (string, bool) {
	for _, m := range reTurno.FindAllStringSubmatchIndex(l.norm, -1) {
		if l.dentroDeTrecho(m[0]) {
			continue
		}
		l.marcar(m[0], m[1])
		if m[2] < 0 {
			return l.norm[m[4]:m[5]], true // almoco
		}
		return l.norm[m[2]:m[3]], true
	}
	return "", false
}

// lerDia acha o dia; sem dia, hoje (diaExplicito falso).
func (l *leitura) lerDia(ref time.Time) (time.Time, bool, error) {
	hoje := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, ref.Location())
	if m := reDia.FindStringIndex(l.norm); m != nil {
		palavra := l.norm[m[0]:m[1]]
		l.marcar(m[0], m[1])
		switch palavra {
		case "amanha":
			return hoje.AddDate(0, 0, 1), true, nil
		case "depois de amanha":
			return hoje.AddDate(0, 0, 2), true, nil
		}
		return hoje, true, nil
	}
	if m := reSemana.FindStringSubmatchIndex(l.norm); m != nil {
		alvo := diasDaSemana[l.norm[m[2]:m[3]]]
		l.marcar(m[0], m[1])
		// O próximo dia com esse nome; o mesmo dia de hoje vale hoje se a hora ainda não passou
		// (conferido depois), e por isso a comparação da hora usa diaExplicito.
		dias := (int(alvo) - int(hoje.Weekday()) + 7) % 7
		dia := hoje.AddDate(0, 0, dias)
		if dias == 0 {
			if _, _, _, ok := l.espiarHora(); ok {
				h, mi := l.horaDeHoje()
				if time.Date(dia.Year(), dia.Month(), dia.Day(), h, mi, 0, 0, ref.Location()).Before(ref) {
					dia = dia.AddDate(0, 0, 7)
				}
			}
		}
		return dia, true, nil
	}
	if m := reData.FindStringSubmatchIndex(l.norm); m != nil {
		d, me := valor(l.norm[m[2]:m[3]]), valor(l.norm[m[4]:m[5]])
		ano := ref.Year()
		if m[6] >= 0 {
			ano = valor(l.norm[m[6]:m[7]])
			if ano < 100 {
				ano += 2000
			}
		}
		l.marcar(m[0], m[1])
		return dataValida(ano, time.Month(me), d, ref)
	}
	if m := reDiaMes.FindStringSubmatchIndex(l.norm); m != nil {
		d, me := valor(l.norm[m[2]:m[3]]), meses[l.norm[m[4]:m[5]]]
		l.marcar(m[0], m[1])
		return dataValida(ref.Year(), me, d, ref)
	}
	if m := reDiaSo.FindStringSubmatchIndex(l.norm); m != nil {
		d := valor(l.norm[m[2]:m[3]])
		l.marcar(m[0], m[1])
		mes := ref.Month()
		ano := ref.Year()
		if d < ref.Day() {
			mes++
			if mes > 12 {
				mes, ano = 1, ano+1
			}
		}
		return dataValida(ano, mes, d, ref)
	}
	return hoje, false, nil
}

func dataValida(ano int, mes time.Month, dia int, ref time.Time) (time.Time, bool, error) {
	t := time.Date(ano, mes, dia, 0, 0, 0, 0, ref.Location())
	if mes < 1 || mes > 12 || t.Day() != dia || t.Month() != mes {
		return t, true, duvida("Não entendi a data. Diga de novo com o dia e a hora.")
	}
	return t, true, nil
}

// lerHora acha a hora (e marca o trecho); periodo é manha, tarde, noite, madrugada ou vazio.
func (l *leitura) lerHora() (hora, minuto int, periodo string, ok bool) {
	hora, minuto, periodo, m := l.achaHora()
	if m == nil {
		return 0, 0, "", false
	}
	l.marcar(m[0], m[1])
	return hora, minuto, periodo, true
}

func (l *leitura) espiarHora() (int, int, string, bool) {
	h, mi, p, m := l.achaHora()
	return h, mi, p, m != nil
}

// horaDeHoje é a hora como ficaria, com o período aplicado (para comparar com agora).
func (l *leitura) horaDeHoje() (int, int) {
	h, mi, p, _ := l.achaHora()
	if (p == "tarde" || p == "noite") && h < 12 {
		h += 12
	}
	return h, mi
}

func (l *leitura) achaHora() (hora, minuto int, periodo string, indices []int) {
	if m := reMeioDia.FindStringSubmatchIndex(l.norm); m != nil {
		hora = 12
		if strings.HasPrefix(l.norm[m[2]:m[3]], "meia") {
			hora = 0
		}
		if strings.HasSuffix(l.norm[m[0]:m[1]], "e meia") {
			minuto = 30
		}
		return hora, minuto, "", m[:2]
	}
	// A hora precisa de marca ("às 9", "9h", "9:30", "9 e meia", "9 da manhã"): um número solto no
	// texto ("ligar para 3 clientes") não é hora.
	for _, m := range reHora.FindAllStringSubmatchIndex(l.norm, -1) {
		todo := l.norm[m[0]:m[1]]
		temMarca := m[4] >= 0 || m[6] >= 0 || m[8] >= 0 ||
			strings.HasPrefix(todo, "as ") || strings.HasPrefix(todo, "a ") || strings.HasPrefix(todo, "ate as ") ||
			strings.HasPrefix(todo, "pelas ") || strings.HasPrefix(todo, "por volta das ") ||
			reMarcaDeHora.MatchString(todo)
		if !temMarca || l.dentroDeTrecho(m[0]) {
			continue
		}
		hora = valor(l.norm[m[2]:m[3]])
		if m[4] >= 0 {
			minuto = valor(l.norm[m[4]:m[5]])
		}
		if m[6] >= 0 {
			minuto = valor(l.norm[m[6]:m[7]])
			if l.norm[m[6]:m[7]] == "meia" {
				minuto = 30
			}
		}
		if m[8] >= 0 {
			periodo = l.norm[m[8]:m[9]]
		}
		return hora, minuto, periodo, m[:2]
	}
	return 0, 0, "", nil
}

// dentroDeTrecho: a posição (em bytes do normalizado) já foi reconhecida como dia ou data.
func (l *leitura) dentroDeTrecho(bytes int) bool {
	r := utf8.RuneCountInString(l.norm[:bytes])
	for _, t := range l.trechos {
		if r >= t.inicio && r < t.fim {
			return true
		}
	}
	return false
}

// Descrever é como a resposta fala uma hora marcada: "terça 21/10 às 09:00".
func Descrever(t time.Time) string {
	return fmt.Sprintf("%s %02d/%02d às %02d:%02d", nomesDosDias[t.Weekday()], t.Day(), int(t.Month()), t.Hour(), t.Minute())
}

var falaDoTurno = map[string]string{"manha": "de manhã", "almoco": "no almoço", "tarde": "à tarde", "noite": "à noite"}

// DescreverLembrete fala o quando do lembrete: "terça 21/10 às 09:00", "terça 21/10 à tarde" ou
// "terça 21/10, o dia todo".
func DescreverLembrete(x Lembrete) string {
	dia := fmt.Sprintf("%s %02d/%02d", nomesDosDias[x.Quando.Weekday()], x.Quando.Day(), int(x.Quando.Month()))
	switch x.Precisao {
	case Turno:
		return dia + " " + falaDoTurno[x.NomeDoTurno]
	case Dia:
		return dia + ", o dia todo"
	}
	return Descrever(x.Quando)
}

// Encurtar corta s em até n runes, com reticências.
func Encurtar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
