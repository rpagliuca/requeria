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

func TestInterpretarLembreteNaoChuta(t *testing.T) {
	fuso, _ := time.LoadLocation("America/Sao_Paulo")
	ref := time.Date(2026, 10, 20, 14, 10, 0, 0, fuso)
	for _, dito := range []string{
		"ligar para o contador",        // sem hora
		"ligar para o contador amanhã", // dia sem hora
		"ligar para o contador às 3",   // 03:00 de hoje já passou
		"pagar o boleto 10/10 às 9",    // data passada
		"pagar o boleto 31/02 às 9",    // data que não existe
		"amanhã às 9",                  // sem o que lembrar
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
