// Package event define el tipo Event, que comparten las fuentes, la base y la API.
package event

import "time"

// Argentina no tiene horario de verano, alcanza con un offset fijo.
var Argentina = time.FixedZone("ART", -3*60*60)

type Event struct {
	ID     int64     `json:"id"`
	Titulo string    `json:"titulo"`
	Inicio time.Time `json:"inicio"`
	Fin    time.Time `json:"fin,omitzero"`
	Lugar  string    `json:"lugar"`
	URL    string    `json:"url"`
	Fuente string    `json:"fuente"`
}
