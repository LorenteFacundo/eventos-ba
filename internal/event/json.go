package event

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DecodificarJSON acepta un evento suelto o una lista de eventos.
// Rechaza campos desconocidos para que un typo ("titlo") no pase en silencio.
func DecodificarJSON(datos []byte) ([]Event, error) {
	datos = bytes.TrimSpace(datos)

	dec := json.NewDecoder(bytes.NewReader(datos))
	dec.DisallowUnknownFields()

	if bytes.HasPrefix(datos, []byte("[")) {
		var eventos []Event
		if err := dec.Decode(&eventos); err != nil {
			return nil, fmt.Errorf("JSON inválido: %w", err)
		}
		return eventos, nil
	}

	var e Event
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("JSON inválido: %w", err)
	}
	return []Event{e}, nil
}
