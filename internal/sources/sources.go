// Package sources trae eventos de sitios externos.
package sources

import (
	"context"
	"fmt"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
)

type fuente struct {
	nombre string
	fetch  func(context.Context) ([]event.Event, error)
}

var fuentes = []fuente{
	{"dev.events", fetchDevEvents},
}

// FetchAll consulta todas las fuentes. Si alguna falla sigue con el resto
// y devuelve los errores aparte.
func FetchAll(ctx context.Context) ([]event.Event, []error) {
	var todos []event.Event
	var errores []error

	for _, f := range fuentes {
		eventos, err := f.fetch(ctx)
		if err != nil {
			errores = append(errores, fmt.Errorf("%s: %w", f.nombre, err))
			continue
		}
		todos = append(todos, eventos...)
	}

	return todos, errores
}
