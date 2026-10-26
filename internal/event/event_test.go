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
		Titulo:    "Meetup de Go",
		Inicio:    inicio,
		Fin:       inicio.Add(2 * time.Hour),
		URL:       "https://ejemplo.com/meetup",
		Modalidad: Presencial,
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
		{"online", func(e *Event) { e.Modalidad = Online }, true},
		{"modalidad inválida", func(e *Event) { e.Modalidad = "telepatía" }, false},
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
	e.Modalidad = ""
	e.ID = 99
	e.Titulo = "  Meetup de Go  "
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
	if got.Modalidad != Presencial {
		t.Errorf("Modalidad = %q, want %q", got.Modalidad, Presencial)
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
	malo2.Inicio = time.Time{}

	err := Preparar([]Event{eventoValido(), malo1, malo2})

	var errValidacion *ErrorValidacion
	if !errors.As(err, &errValidacion) {
		t.Fatalf("esperaba *ErrorValidacion, got %v", err)
	}
	if len(errValidacion.Problemas) != 2 {
		t.Errorf("got %d problemas, want 2: %v", len(errValidacion.Problemas), errValidacion.Problemas)
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

	t.Run("datos después del final", func(t *testing.T) {
		if _, err := DecodificarJSON([]byte(`{"titulo": "A"} basura`)); err == nil {
			t.Error("esperaba error")
		}
	})
}

func TestParsearModalidad(t *testing.T) {
	casos := map[string]Modalidad{
		"presencial": Presencial,
		"Online":     Online,
		"VIRTUAL":    Online,
		"híbrido":    Hibrido,
		" hibrido ":  Hibrido,
	}
	for texto, want := range casos {
		if got, ok := ParsearModalidad(texto); !ok || got != want {
			t.Errorf("ParsearModalidad(%q) = %q, %v; want %q", texto, got, ok, want)
		}
	}
	if _, ok := ParsearModalidad("remoto"); ok {
		t.Error("remoto no debería ser una modalidad válida")
	}
}

func TestTermina(t *testing.T) {
	dia := func(d, h, m int) time.Time { return time.Date(2030, 1, d, h, m, 0, 0, Argentina) }

	casos := []struct {
		nombre      string
		inicio, fin time.Time
		want        time.Time
	}{
		{"con fin", dia(10, 19, 0), dia(10, 22, 0), dia(10, 22, 0)},
		{"sin fin", dia(10, 19, 0), time.Time{}, dia(10, 22, 0)},
		{"termina a medianoche", dia(10, 20, 0), dia(11, 0, 0), dia(11, 0, 0)},
		{"solo el día", dia(10, 0, 0), time.Time{}, dia(11, 0, 0)},
		{"varios días sin horario", dia(10, 0, 0), dia(12, 0, 0), dia(13, 0, 0)},
		{"sin horario de inicio pero con fin", dia(10, 0, 0), dia(10, 18, 0), dia(10, 18, 0)},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			e := Event{Inicio: c.inicio, Fin: c.fin}
			if got := e.Termina(); !got.Equal(c.want) {
				t.Errorf("Termina() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPrepararAceptaVirtual(t *testing.T) {
	e := eventoValido()
	e.Modalidad = "Virtual"
	eventos := []Event{e}
	if err := Preparar(eventos); err != nil {
		t.Fatalf("Preparar: %v", err)
	}
	if eventos[0].Modalidad != Online {
		t.Errorf("Modalidad = %q, want %q", eventos[0].Modalidad, Online)
	}
}
