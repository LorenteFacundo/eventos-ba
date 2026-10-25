package event

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func eventoValido() Event {
	inicio := time.Date(2030, 1, 10, 19, 0, 0, 0, Argentina)
	return Event{
		Titulo: "Meetup de Go",
		Inicio: inicio,
		Fin:    inicio.Add(2 * time.Hour),
		URL:    "https://ejemplo.com/meetup",
		Zona:   ZonaCABA,
	}
}

func TestValidar(t *testing.T) {
	casos := []struct {
		nombre    string
		modificar func(e *Event)
		valido    bool
	}{
		{"completo", func(e *Event) {}, true},
		{"sin fin", func(e *Event) { e.Fin = time.Time{} }, true},
		{"sin título", func(e *Event) { e.Titulo = "  " }, false},
		{"título muy largo", func(e *Event) { e.Titulo = strings.Repeat("a", maxTitulo+1) }, false},
		{"url sin esquema", func(e *Event) { e.URL = "ejemplo.com" }, false},
		{"url sin dominio", func(e *Event) { e.URL = "https://" }, false},
		{"url javascript", func(e *Event) { e.URL = "javascript:alert(1)" }, false},
		{"lugar muy largo", func(e *Event) { e.Lugar = strings.Repeat("a", maxLugar+1) }, false},
		{"sin inicio", func(e *Event) { e.Inicio = time.Time{} }, false},
		{"inicio fuera de rango", func(e *Event) { e.Inicio = time.Date(1990, 1, 1, 0, 0, 0, 0, Argentina) }, false},
		{"fin antes del inicio", func(e *Event) { e.Fin = e.Inicio.Add(-time.Hour) }, false},
		{"zona inválida", func(e *Event) { e.Zona = "Córdoba" }, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			e := eventoValido()
			c.modificar(&e)

			err := e.Validar()
			if c.valido && err != nil {
				t.Errorf("esperaba válido, dio error: %v", err)
			}
			if !c.valido && err == nil {
				t.Error("esperaba error")
			}
		})
	}
}

func TestPrepararNormaliza(t *testing.T) {
	e := eventoValido()
	e.ID = 99
	e.Titulo = "  Meetup de Go  "
	e.Zona = "caba"
	eventos := []Event{e}

	if err := Preparar(eventos); err != nil {
		t.Fatalf("Preparar: %v", err)
	}

	got := eventos[0]
	if got.ID != 0 {
		t.Errorf("ID = %d, want 0", got.ID)
	}
	if got.Titulo != "Meetup de Go" {
		t.Errorf("Titulo = %q", got.Titulo)
	}
	if got.Fuente != FuentePorDefecto {
		t.Errorf("Fuente = %q, want %q", got.Fuente, FuentePorDefecto)
	}
	if got.Zona != ZonaCABA {
		t.Errorf("Zona = %q, want %q", got.Zona, ZonaCABA)
	}
}

func TestValidosDescartaSoloLosInvalidos(t *testing.T) {
	bueno := eventoValido()
	malo := eventoValido()
	malo.URL = "sin-esquema"

	validos, descartados := Validos([]Event{bueno, malo})

	if len(validos) != 1 || validos[0].Titulo != bueno.Titulo {
		t.Errorf("validos = %+v, want solo %q", validos, bueno.Titulo)
	}
	if len(descartados) != 1 {
		t.Fatalf("descartados = %v, want 1", descartados)
	}
	if !strings.Contains(descartados[0].Error(), "url") {
		t.Errorf("descartado = %q, want que mencione la url", descartados[0])
	}
}

func TestPrepararJuntaLosProblemas(t *testing.T) {
	malo1, malo2 := eventoValido(), eventoValido()
	malo1.URL = ""
	malo2.Zona = "Marte"

	err := Preparar([]Event{eventoValido(), malo1, malo2})

	var errValidacion *ErrorValidacion
	if !errors.As(err, &errValidacion) {
		t.Fatalf("esperaba *ErrorValidacion, got %v", err)
	}
	if len(errValidacion.Problemas) != 2 {
		t.Errorf("got %d problemas, want 2: %v", len(errValidacion.Problemas), errValidacion.Problemas)
	}
}

func TestParsearZona(t *testing.T) {
	casos := map[string]Zona{"CABA": ZonaCABA, "caba": ZonaCABA, "gba": ZonaGBA, "ONLINE": ZonaOnline}
	for texto, want := range casos {
		if got, ok := ParsearZona(texto); !ok || got != want {
			t.Errorf("ParsearZona(%q) = %q, %v; want %q", texto, got, ok, want)
		}
	}

	if _, ok := ParsearZona("Córdoba"); ok {
		t.Error("Córdoba no es una zona válida")
	}
}

func TestDecodificarJSON(t *testing.T) {
	t.Run("un evento", func(t *testing.T) {
		eventos, err := DecodificarJSON([]byte(`{"titulo": "A"}`))
		if err != nil || len(eventos) != 1 || eventos[0].Titulo != "A" {
			t.Errorf("got %v, %v", eventos, err)
		}
	})

	t.Run("lista", func(t *testing.T) {
		eventos, err := DecodificarJSON([]byte(`  [{"titulo": "A"}, {"titulo": "B"}]`))
		if err != nil || len(eventos) != 2 {
			t.Errorf("got %v, %v", eventos, err)
		}
	})

	t.Run("campo desconocido", func(t *testing.T) {
		if _, err := DecodificarJSON([]byte(`{"titlo": "A"}`)); err == nil {
			t.Error("esperaba error")
		}
	})

	t.Run("no es JSON", func(t *testing.T) {
		if _, err := DecodificarJSON([]byte(`hola`)); err == nil {
			t.Error("esperaba error")
		}
	})
}
