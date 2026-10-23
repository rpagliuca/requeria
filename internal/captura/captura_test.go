package captura

import (
	"errors"
	"testing"
	"time"
)

func TestInterpretarLembrete(t *testing.T) {
	fuso, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	// Terça-feira, 20/10/2026, 14:10 em São Paulo.
	ref := time.Date(2026, 10, 20, 14, 10, 0, 0, fuso)
	casos := []struct {
		dito, texto, quando string
	}{
		{"ligar para o contador amanhã às 9", "ligar para o contador", "2026-10-21T09:00:00-03:00"},
		{"Me lembra de tirar o lixo daqui a 20 minutos", "tirar o lixo", "2026-10-20T14:30:00-03:00"},
		{"tomar remédio daqui a duas horas e meia", "tomar remédio", "2026-10-20T16:40:00-03:00"},
		{"reunião com o João às 15h30", "reunião com o João", "2026-10-20T15:30:00-03:00"},
		{"buscar o carro às 5 da tarde", "buscar o carro", "2026-10-20T17:00:00-03:00"},
		{"pagar o boleto dia 25 às 8", "pagar o boleto", "2026-10-25T08:00:00-03:00"},
		{"pagar o boleto 03/11 às 10:15", "pagar o boleto", "2026-11-03T10:15:00-03:00"},
		{"dentista sexta-feira às 9 e meia", "dentista", "2026-10-23T09:30:00-03:00"},
		{"almoço com a Ana ao meio-dia de amanhã", "almoço com a Ana", "2026-10-21T12:00:00-03:00"},
		{"ligar para 3 clientes às 16h", "ligar para 3 clientes", "2026-10-20T16:00:00-03:00"},
		{"enviar relatório 5 de novembro às 14h", "enviar relatório", "2026-11-05T14:00:00-03:00"},
		{"regar as plantas daqui a meia hora", "regar as plantas", "2026-10-20T14:40:00-03:00"},
		{"terça às 10 revisar contrato", "revisar contrato", "2026-10-20T00:00:00-03:00"},
	}
	// "terça" dita numa terça: hoje às 10 já passou → semana que vem.
	casos[len(casos)-1].quando = "2026-10-27T10:00:00-03:00"
	for _, c := range casos {
		l, err := InterpretarLembrete(c.dito, ref, fuso)
		if err != nil {
			t.Errorf("%q: %v", c.dito, err)
			continue
		}
		if l.Texto != c.texto || l.Quando.Format(time.RFC3339) != c.quando {
			t.Errorf("%q: %q %s, esperado %q %s", c.dito, l.Texto, l.Quando.Format(time.RFC3339), c.texto, c.quando)
		}
	}
}

// Sem hora, o turno ou o dia: nada de hora inventada (R-0514).
func TestInterpretarLembreteSemHora(t *testing.T) {
	fuso, _ := time.LoadLocation("America/Sao_Paulo")
	// Terça-feira, 20/10/2026, 14:10 em São Paulo.
	ref := time.Date(2026, 10, 20, 14, 10, 0, 0, fuso)
	casos := []struct {
		dito, texto, dia string
		precisao         Precisao
		turno, fala      string
	}{
		{"ligar para o contador amanhã", "ligar para o contador", "2026-10-21", Dia, "", "quarta 21/10, o dia todo"},
		{"pagar o boleto sexta", "pagar o boleto", "2026-10-23", Dia, "", "sexta 23/10, o dia todo"},
		{"renovar o seguro dia 30", "renovar o seguro", "2026-10-30", Dia, "", "sexta 30/10, o dia todo"},
		{"comprar pão amanhã de manhã", "comprar pão", "2026-10-21", Turno, "manha", "quarta 21/10 de manhã"},
		{"ligar para a escola hoje à noite", "ligar para a escola", "2026-10-20", Turno, "noite", "terça 20/10 à noite"},
		{"regar as plantas à tarde", "regar as plantas", "2026-10-20", Turno, "tarde", "terça 20/10 à tarde"},
		{"pagar a conta amanhã no almoço", "pagar a conta", "2026-10-21", Turno, "almoco", "quarta 21/10 no almoço"},
		{"almoço com a Ana amanhã", "almoço com a Ana", "2026-10-21", Dia, "", "quarta 21/10, o dia todo"},
		{"ligar para o banco na hora do almoço", "ligar para o banco", "2026-10-20", Turno, "almoco", "terça 20/10 no almoço"},
		{"revisar o contrato daqui a 3 dias", "revisar o contrato", "2026-10-23", Dia, "", "sexta 23/10, o dia todo"},
		{"lavar o carro daqui a 2 dias de manhã", "lavar o carro", "2026-10-22", Turno, "manha", "quinta 22/10 de manhã"},
	}
	for _, c := range casos {
		l, err := InterpretarLembrete(c.dito, ref, fuso)
		if err != nil {
			t.Errorf("%q: %v", c.dito, err)
			continue
		}
		if l.Texto != c.texto || l.Quando.Format("2006-01-02") != c.dia || l.Precisao != c.precisao || l.NomeDoTurno != c.turno || DescreverLembrete(l) != c.fala {
			t.Errorf("%q: %+v %q, esperado %q %s %s %q %q", c.dito, l, DescreverLembrete(l), c.texto, c.dia, c.precisao, c.turno, c.fala)
		}
	}

	// Com hora, o turno solto vira o período da hora, e a precisão é hora.
	l, err := InterpretarLembrete("buscar as crianças amanhã à tarde às 5", ref, fuso)
	if err != nil || l.Precisao != Hora || l.Texto != "buscar as crianças" || l.Quando.Format(time.RFC3339) != "2026-10-21T17:00:00-03:00" {
		t.Fatalf("turno com hora: %+v %v", l, err)
	}
}

func TestInterpretarLembreteNaoChuta(t *testing.T) {
	fuso, _ := time.LoadLocation("America/Sao_Paulo")
	ref := time.Date(2026, 10, 20, 14, 10, 0, 0, fuso)
	for _, dito := range []string{
		"ligar para o contador",      // sem hora
		"ligar para o contador às 3", // 03:00 de hoje já passou
		"pagar o boleto 10/10 às 9",  // data passada
		"pagar o boleto 31/02 às 9",  // data que não existe
		"amanhã às 9",                // sem o que lembrar
	} {
		_, err := InterpretarLembrete(dito, ref, fuso)
		var d *Duvida
		if !errors.As(err, &d) || d.Frase == "" {
			t.Errorf("%q: esperava dúvida, veio %v", dito, err)
		}
	}
}

func TestDescrever(t *testing.T) {
	fuso, _ := time.LoadLocation("America/Sao_Paulo")
	if got := Descrever(time.Date(2026, 10, 21, 9, 0, 0, 0, fuso)); got != "quarta 21/10 às 09:00" {
		t.Fatal(got)
	}
}
