package event

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodificarJSON acepta un evento suelto o una lista de eventos.
// Rechaza campos desconocidos para que un typo ("titlo") no pase en silencio.
func DecodificarJSON(datos []byte) ([]Event, error) {
	datos = bytes.TrimSpace(datos)

	dec := json.NewDecoder(bytes.NewReader(datos))
	dec.DisallowUnknownFields()

	if bytes.HasPrefix(datos, []byte("[")) {
		var eventos []Event
		return eventos, decodificar(dec, &eventos)
	}

	var e Event
	return []Event{e}, decodificar(dec, &e)
}

// decodificar lee un único valor y rechaza lo que venga después: Decode
// solo lee el primero y dejaría pasar `{...} basura`.
func decodificar(dec *json.Decoder, v any) error {
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("JSON inválido: sobran datos después del final")
	}
	return nil
}
