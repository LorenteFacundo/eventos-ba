// Package event define el tipo Event, que comparten las fuentes, la base y la API.
package event

import (
	"strings"
	"time"
)

// Argentina no tiene horario de verano, alcanza con un offset fijo.
var Argentina = time.FixedZone("ART", -3*60*60)

type Zona string

const (
	ZonaCABA   Zona = "CABA"
	ZonaGBA    Zona = "GBA"
	ZonaOnline Zona = "Online"
)

var zonas = []Zona{ZonaCABA, ZonaGBA, ZonaOnline}

// ParsearZona acepta la zona sin importar mayúsculas ("caba", "CABA", ...).
func ParsearZona(s string) (Zona, bool) {
	for _, z := range zonas {
		if strings.EqualFold(s, string(z)) {
			return z, true
		}
	}
	return "", false
}

type Event struct {
	ID     int64     `json:"id"`
	Titulo string    `json:"titulo"`
	Inicio time.Time `json:"inicio"`
	Fin    time.Time `json:"fin,omitzero"`
	Lugar  string    `json:"lugar"`
	URL    string    `json:"url"`
	Fuente string    `json:"fuente"`
	Zona   Zona      `json:"zona"`
}
