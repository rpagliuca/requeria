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

// Precisao diz se o pedido deu a hora (Hora), só um turno (Turno), só o dia (Dia), só a semana
// (Semana) ou só o mês (Mes).
type Precisao string

const (
	Hora   Precisao = "hora"
	Turno  Precisao = "turno"
	Dia    Precisao = "dia"
	Semana Precisao = "semana"
	Mes    Precisao = "mes"
)

// Lembrete é um pedido de lembrete interpretado. Fora de Hora, Quando é a meia-noite do primeiro dia
// (a segunda da semana, o dia 1 do mês) e, em Turno, NomeDoTurno diz qual turno (manha, almoco,
// tarde ou noite); a hora de cada turno é da instalação.
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
	reRelativo    = regexp.MustCompile(`\b(?:daqui a|daqui|dentro de|em)\s+` + numero + `\s+(minutos?|min|horas?|h|dias?|semanas?|mes|meses)(?:\s+e\s+meia)?\b`)
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
	// Semana e mês sem dia: "semana que vem", "nesta semana", "na semana do dia 12", "mês que vem",
	// "em abril", "no mês de abril de 2027".
	reSemanaQueVem = regexp.MustCompile(`\b(?:(?:da|na|para a|pra)\s+)?(?:semana que vem|proxima semana)\b`)
	reEstaSemana   = regexp.MustCompile(`\b(?:(?:da|para|pra)\s+)?(?:nesta|nessa|esta|essa)\s+semana\b`)
	reSemanaDo     = regexp.MustCompile(`\b(?:(?:na|para a|pra)\s+)?semana\s+d[oe]\s+`)
	reMesQueVem    = regexp.MustCompile(`\b(?:(?:no|para o|pro)\s+)?(?:mes que vem|proximo mes)\b`)
	reEsteMes      = regexp.MustCompile(`\b(?:(?:para|pra)\s+)?(?:neste|nesse|este|esse)\s+mes\b`)
	reMesNome      = regexp.MustCompile(`\b(?:em|no mes de|para|pra|ate)\s+(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)(?:\s+de\s+(\d{4}))?\b`)
	reHora         = regexp.MustCompile(`\b(?:(?:as|a|ate as|pelas|por volta das)\s+)?(\d{1,2})(?:\s*(?:h|:|horas?)\s*(\d{2})?|\s+e\s+(meia|\d{1,2})(?:\s+minutos?)?)?(?:\s+(?:da|de)\s+(manha|tarde|noite|madrugada))?\b`)
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
		case strings.HasPrefix(unidade, "semana"):
			return l.periodoRelativo(ref.AddDate(0, 0, 7*n), Semana, n)
		case strings.HasPrefix(unidade, "mes"):
			return l.periodoRelativo(time.Date(ref.Year(), ref.Month()+time.Month(n), 1, 0, 0, 0, 0, ref.Location()), Mes, n)
		default:
			return l.diaRelativo(ref, n)
		}
		if d <= 0 {
			return Lembrete{}, duvida("Não entendi daqui a quanto tempo. Diga de novo, por exemplo: daqui a 20 minutos.")
		}
		return l.resultado(ref.Add(d).Truncate(time.Minute))
	}

	dia, diaExplicito, semanaOuMes, err := l.lerPeriodo(ref)
	if err != nil {
		return Lembrete{}, err
	}
	hora, minuto, periodo, ok := l.lerHora()
	turno, temTurno := l.lerTurno()
	if semanaOuMes != "" {
		if ok || temTurno {
			return Lembrete{}, duvida("Com a hora ou o turno, diga também o dia.")
		}
		return l.semHora(dia, semanaOuMes, "")
	}
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

// periodoRelativo é "daqui a 2 semanas" ou "daqui a 3 meses": a semana ou o mês inteiro, sem hora.
func (l *leitura) periodoRelativo(alvo time.Time, p Precisao, n int) (Lembrete, error) {
	if n <= 0 {
		return Lembrete{}, duvida("Não entendi daqui a quanto tempo. Diga de novo com a semana ou o mês.")
	}
	if _, _, _, ok := l.lerHora(); ok {
		return Lembrete{}, duvida("Com a hora, diga também o dia.")
	}
	if _, ok := l.lerTurno(); ok {
		return Lembrete{}, duvida("Com o turno, diga também o dia.")
	}
	return l.semHora(inicioDoPeriodo(alvo, p), p, "")
}

// inicioDoPeriodo é a meia-noite da segunda da semana de t, ou do dia 1 do mês de t.
func inicioDoPeriodo(t time.Time, p Precisao) time.Time {
	dia := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	if p == Mes {
		return dia.AddDate(0, 0, 1-dia.Day())
	}
	return dia.AddDate(0, 0, -(int(dia.Weekday())+6)%7)
}

// lerPeriodo acha a semana ou o mês e, dentro deles, o dia, se dito: "quarta da semana que vem" e
// "em abril, dia 20" são dias. Sem dia, devolve o início da semana ou do mês com semanaOuMes
// preenchido. Um dia que não cabe na semana ou no mês ("amanhã, semana que vem") vence.
func (l *leitura) lerPeriodo(ref time.Time) (dia time.Time, diaExplicito bool, semanaOuMes Precisao, err error) {
	hoje := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, ref.Location())
	segunda := inicioDoPeriodo(hoje, Semana)
	if m := reSemanaDo.FindStringIndex(l.norm); m != nil {
		// "na semana do dia 12": a semana do dia dito.
		l.marcar(m[0], m[1])
		dia, explicito, err := l.lerDia(ref)
		if err != nil || !explicito {
			return dia, explicito, "", duvida("Não entendi a semana. Diga de novo, por exemplo: na semana do dia 12.")
		}
		return inicioDoPeriodo(dia, Semana), true, Semana, nil
	}
	for _, sem := range []struct {
		re      *regexp.Regexp
		semanas int
	}{{reSemanaQueVem, 1}, {reEstaSemana, 0}} {
		m := sem.re.FindStringIndex(l.norm)
		if m == nil {
			continue
		}
		l.marcar(m[0], m[1])
		inicio := segunda.AddDate(0, 0, 7*sem.semanas)
		if w := reSemana.FindStringSubmatchIndex(l.norm); w != nil && !l.dentroDeTrecho(w[0]) {
			alvo := diasDaSemana[l.norm[w[2]:w[3]]]
			l.marcar(w[0], w[1])
			return inicio.AddDate(0, 0, (int(alvo)+6)%7), true, "", nil
		}
		if dia, explicito, err := l.lerDia(ref); err != nil || explicito {
			return dia, explicito, "", err
		}
		return inicio, true, Semana, nil
	}
	var mes time.Time
	if m := reMesQueVem.FindStringIndex(l.norm); m != nil {
		l.marcar(m[0], m[1])
		mes = time.Date(ref.Year(), ref.Month()+1, 1, 0, 0, 0, 0, ref.Location())
	} else if m := reEsteMes.FindStringIndex(l.norm); m != nil {
		l.marcar(m[0], m[1])
		mes = time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
	} else if m := reMesNome.FindStringSubmatchIndex(l.norm); m != nil {
		l.marcar(m[0], m[1])
		// Sem ano, o próximo mês com esse nome; o mês corrente vale ainda este ano.
		nome, ano := meses[l.norm[m[2]:m[3]]], ref.Year()
		if m[4] >= 0 {
			ano = valor(l.norm[m[4]:m[5]])
		} else if nome < ref.Month() {
			ano++
		}
		mes = time.Date(ano, nome, 1, 0, 0, 0, 0, ref.Location())
	} else {
		dia, explicito, err := l.lerDia(ref)
		return dia, explicito, "", err
	}
	if m := reDiaSo.FindStringSubmatchIndex(l.norm); m != nil && !l.dentroDeTrecho(m[0]) {
		d := valor(l.norm[m[2]:m[3]])
		l.marcar(m[0], m[1])
		dia, _, err := dataValida(mes.Year(), mes.Month(), d, ref)
		return dia, true, "", err
	}
	if dia, explicito, err := l.lerDia(ref); err != nil || explicito {
		return dia, explicito, "", err
	}
	return mes, true, Mes, nil
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

var nomesDosMeses = []string{"", "janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto",
	"setembro", "outubro", "novembro", "dezembro"}

var falaDoTurno = map[string]string{"manha": "de manhã", "almoco": "no almoço", "tarde": "à tarde", "noite": "à noite"}

// DescreverLembrete fala o quando do lembrete: "terça 21/10 às 09:00", "terça 21/10 à tarde",
// "terça 21/10, o dia todo", "na semana de 26/10" ou "em abril de 2027".
func DescreverLembrete(x Lembrete) string {
	switch x.Precisao {
	case Semana:
		return fmt.Sprintf("na semana de %02d/%02d", x.Quando.Day(), int(x.Quando.Month()))
	case Mes:
		return fmt.Sprintf("em %s de %d", nomesDosMeses[x.Quando.Month()], x.Quando.Year())
	}
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
