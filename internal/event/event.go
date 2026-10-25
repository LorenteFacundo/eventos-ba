// Package event define el tipo Event, que comparten las fuentes, la base y la API.
package event

import (
	"strings"
	"time"
)

// Argentina no tiene horario de verano, alcanza con un offset fijo.
var Argentina = time.FixedZone("ART", -3*60*60)

// Duración que se asume para un evento con horario pero sin fin conocido.
const duracionEstimada = 3 * time.Hour

type Modalidad string

const (
	Presencial Modalidad = "presencial"
	Online     Modalidad = "online"
	Hibrido    Modalidad = "hibrido"
)

var modalidades = map[string]Modalidad{
	"presencial": Presencial,
	"online":     Online,
	"virtual":    Online,
	"hibrido":    Hibrido,
	"híbrido":    Hibrido,
}

// ParsearModalidad acepta mayúsculas, tildes y "virtual" como sinónimo de online.
func ParsearModalidad(s string) (Modalidad, bool) {
	m, ok := modalidades[strings.ToLower(strings.TrimSpace(s))]
	return m, ok
}

type Event struct {
	ID        int64     `json:"id"`
	Titulo    string    `json:"titulo"`
	Inicio    time.Time `json:"inicio"`
	Fin       time.Time `json:"fin,omitzero"`
	Lugar     string    `json:"lugar"`
	URL       string    `json:"url"`
	Modalidad Modalidad `json:"modalidad"`
	Fuente    string    `json:"fuente"`
}

// SinHorario indica que solo se conoce el día: el inicio es a las 00:00.
func (e Event) SinHorario() bool {
	return esMedianoche(e.Inicio)
}

// Termina es el momento en que el evento deja de estar por venir. Sin fin
// conocido se estima: el día entero si no tiene horario, o tres horas.
func (e Event) Termina() time.Time {
	if e.SinHorario() {
		fin := e.Fin
		if fin.IsZero() {
			fin = e.Inicio
		}
		// Un fin a las 00:00 también es solo un día: dura hasta que termina ese día.
		if esMedianoche(fin) {
			return fin.AddDate(0, 0, 1)
		}
		return fin
	}
	if !e.Fin.IsZero() {
		return e.Fin
	}
	return e.Inicio.Add(duracionEstimada)
}

func esMedianoche(t time.Time) bool {
	t = t.In(Argentina)
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}
