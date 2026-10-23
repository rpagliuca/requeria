package servidor

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rpagliuca/rex/api"
)

// Turnos são as horas locais em que começam a manhã, a tarde e a noite; a noite vai até a
// meia-noite e o dia começa com a manhã. Valores em minutos desde a meia-noite.
type Turnos struct {
	Manha, Tarde, Noite int
}

// TurnosPadrao: manhã às 8h, tarde às 13h, noite às 18h.
var TurnosPadrao = Turnos{Manha: 8 * 60, Tarde: 13 * 60, Noite: 18 * 60}

// Validar confere a ordem manhã < tarde < noite, dentro do dia.
func (t Turnos) Validar() error {
	if !(0 <= t.Manha && t.Manha < t.Tarde && t.Tarde < t.Noite && t.Noite < 24*60) {
		return fmt.Errorf("os turnos precisam começar em ordem dentro do dia (manhã < tarde < noite)")
	}
	return nil
}

func (s *Servidor) turnos() Turnos {
	if s.integracao.Turnos == (Turnos{}) {
		return TurnosPadrao
	}
	return s.integracao.Turnos
}

func precisaoValida(p api.PrecisaoLembrete) bool {
	switch p {
	case api.PrecisaoHora, api.PrecisaoTurno, api.PrecisaoDia:
		return true
	}
	return false
}

// janela devolve início e fim da janela de quando: o turno que contém a hora (antes da manhã conta
// como manhã) ou o dia inteiro, no fuso do próprio quando. Na precisão hora, a janela é o instante.
func (t Turnos) janela(p api.PrecisaoLembrete, quando time.Time) (inicio, fim time.Time) {
	dia := time.Date(quando.Year(), quando.Month(), quando.Day(), 0, 0, 0, 0, quando.Location())
	em := func(minutos int) time.Time { return dia.Add(time.Duration(minutos) * time.Minute) }
	amanha := dia.AddDate(0, 0, 1)
	switch p {
	case api.PrecisaoDia:
		return em(t.Manha), amanha
	case api.PrecisaoTurno:
		minutos := quando.Hour()*60 + quando.Minute()
		switch {
		case minutos < t.Tarde:
			return em(t.Manha), em(t.Tarde)
		case minutos < t.Noite:
			return em(t.Tarde), em(t.Noite)
		default:
			return em(t.Noite), amanha
		}
	}
	return quando, quando
}

// marcar põe quando no início da janela da precisão e recusa a janela que já acabou.
func (s *Servidor) marcar(p api.PrecisaoLembrete, quando time.Time) (time.Time, error) {
	if !precisaoValida(p) {
		return quando, falha(http.StatusBadRequest, "precisão de lembrete inválida: %q (hora, turno ou dia)", p)
	}
	inicio, fim := s.turnos().janela(p, quando)
	if p == api.PrecisaoHora {
		if quando.Before(s.agora()) {
			return quando, falha(http.StatusBadRequest, "a hora marcada já passou: %s", quando.Format(time.RFC3339))
		}
		return quando, nil
	}
	if !fim.After(s.agora()) {
		return quando, falha(http.StatusBadRequest, "o %s de %s já passou", p, quando.Format(time.RFC3339))
	}
	return inicio, nil
}
