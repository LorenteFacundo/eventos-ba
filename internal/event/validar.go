package event

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const FuentePorDefecto = "manual"

const (
	maxTitulo = 200
	maxLugar  = 200
	maxFuente = 50
	maxURL    = 2048
)

var (
	fechaMinima = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	fechaMaxima = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
)

// ErrorValidacion lista los problemas de cada evento inválido.
type ErrorValidacion struct {
	Problemas []string
}

func (e *ErrorValidacion) Error() string {
	return strings.Join(e.Problemas, "\n")
}

// Preparar normaliza y valida eventos que llegan de afuera (archivo o API).
// El ID que traigan se ignora: lo asigna la base.
func Preparar(eventos []Event) error {
	var problemas []string
	for i := range eventos {
		e := &eventos[i]
		e.ID = 0
		e.normalizar()
		if err := e.Validar(); err != nil {
			problemas = append(problemas, fmt.Sprintf("evento #%d %q: %v", i+1, e.Titulo, err))
		}
	}

	if len(problemas) > 0 {
		return &ErrorValidacion{Problemas: problemas}
	}
	return nil
}

func (e *Event) normalizar() {
	e.Titulo = strings.TrimSpace(e.Titulo)
	e.Lugar = strings.TrimSpace(e.Lugar)
	e.URL = strings.TrimSpace(e.URL)
	e.Fuente = strings.TrimSpace(e.Fuente)
	if e.Fuente == "" {
		e.Fuente = FuentePorDefecto
	}
	// La zona se guarda con su forma canónica ("caba" -> "CABA"), así el
	// filtro zona = ? de la base siempre la encuentra.
	if z, ok := ParsearZona(string(e.Zona)); ok {
		e.Zona = z
	}
}

// Validos separa los eventos que vienen de una fuente externa: los inválidos
// se descartan con su motivo en vez de frenar la carga de los demás.
func Validos(eventos []Event) (validos []Event, descartados []error) {
	for i := range eventos {
		e := &eventos[i]
		e.normalizar()
		if err := e.Validar(); err != nil {
			descartados = append(descartados, fmt.Errorf("%q: %w", e.Titulo, err))
			continue
		}
		validos = append(validos, *e)
	}
	return validos, descartados
}

func (e Event) Validar() error {
	var problemas []string
	agregar := func(formato string, args ...any) {
		problemas = append(problemas, fmt.Sprintf(formato, args...))
	}

	switch {
	case strings.TrimSpace(e.Titulo) == "":
		agregar("falta el título")
	case utf8.RuneCountInString(e.Titulo) > maxTitulo:
		agregar("el título supera los %d caracteres", maxTitulo)
	}

	switch {
	case len(e.URL) > maxURL:
		agregar("la url supera los %d caracteres", maxURL)
	case !urlValida(e.URL):
		agregar("la url tiene que ser http:// o https:// y tener dominio")
	}

	if utf8.RuneCountInString(e.Lugar) > maxLugar {
		agregar("el lugar supera los %d caracteres", maxLugar)
	}
	if utf8.RuneCountInString(e.Fuente) > maxFuente {
		agregar("la fuente supera los %d caracteres", maxFuente)
	}

	switch {
	case e.Inicio.IsZero():
		agregar("falta la fecha de inicio")
	case e.Inicio.Before(fechaMinima) || e.Inicio.After(fechaMaxima):
		agregar("la fecha de inicio está fuera de rango")
	}
	if !e.Fin.IsZero() && e.Fin.Before(e.Inicio) {
		agregar("la fecha de fin es anterior al inicio")
	}

	if _, ok := ParsearZona(string(e.Zona)); !ok {
		agregar("zona %q inválida (usá %s, %s u %s)", e.Zona, ZonaCABA, ZonaGBA, ZonaOnline)
	}

	if len(problemas) > 0 {
		return errors.New(strings.Join(problemas, "; "))
	}
	return nil
}

func urlValida(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
