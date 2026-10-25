package store

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
)

func abrirParaTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func nuevoEvento(titulo string, inicio time.Time, zona event.Zona, fuente string) event.Event {
	return event.Event{
		Titulo: titulo,
		Inicio: inicio,
		Fin:    inicio.Add(2 * time.Hour),
		URL:    "https://ejemplo.com/" + titulo,
		Fuente: fuente,
		Zona:   zona,
	}
}

func titulos(eventos []event.Event) []string {
	var ts []string
	for _, e := range eventos {
		ts = append(ts, e.Titulo)
	}
	return ts
}

func TestGuardarActualizaPorURL(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()
	inicio := time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina)

	primera := []event.Event{nuevoEvento("meetup", inicio, event.ZonaCABA, "ia")}
	if err := s.Guardar(ctx, primera); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	id := primera[0].ID
	if id == 0 {
		t.Fatal("no se asignó ID")
	}

	segunda := []event.Event{nuevoEvento("meetup", inicio, event.ZonaCABA, "ia")}
	segunda[0].Lugar = "Palermo"
	if err := s.Guardar(ctx, segunda); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	if segunda[0].ID != id {
		t.Errorf("el ID cambió al actualizar: %d -> %d", id, segunda[0].ID)
	}

	todos, err := s.Buscar(ctx, Filtro{})
	if err != nil {
		t.Fatalf("Buscar: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("got %d eventos, want 1", len(todos))
	}
	if todos[0].Lugar != "Palermo" {
		t.Errorf("Lugar = %q, want Palermo", todos[0].Lugar)
	}
	if !todos[0].Inicio.Equal(inicio) || todos[0].Inicio.Location() != event.Argentina {
		t.Errorf("Inicio = %v, want %v en hora argentina", todos[0].Inicio, inicio)
	}
}

func TestBuscar(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()
	ahora := time.Date(2030, 1, 1, 12, 0, 0, 0, event.Argentina)

	err := s.Guardar(ctx, []event.Event{
		nuevoEvento("pasado", ahora.Add(-48*time.Hour), event.ZonaCABA, "ia"),
		nuevoEvento("mañana", ahora.Add(24*time.Hour), event.ZonaCABA, "ia"),
		nuevoEvento("online", ahora.Add(72*time.Hour), event.ZonaOnline, "dev.events"),
		nuevoEvento("lejano", ahora.Add(30*24*time.Hour), event.ZonaGBA, "manual"),
	})
	if err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	casos := []struct {
		nombre string
		filtro Filtro
		want   []string
	}{
		{"sin filtro", Filtro{}, []string{"pasado", "mañana", "online", "lejano"}},
		{"desde", Filtro{Desde: ahora}, []string{"mañana", "online", "lejano"}},
		{"desde y hasta", Filtro{Desde: ahora, Hasta: ahora.AddDate(0, 0, 7)}, []string{"mañana", "online"}},
		{"zona", Filtro{Desde: ahora, Zona: event.ZonaCABA}, []string{"mañana"}},
		{"fuente sin distinguir mayúsculas", Filtro{Fuente: "DEV.EVENTS"}, []string{"online"}},
		{"límite", Filtro{Desde: ahora, Limite: 1}, []string{"mañana"}},
		{"sin resultados", Filtro{Zona: event.ZonaGBA, Fuente: "ia"}, nil},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			eventos, err := s.Buscar(ctx, c.filtro)
			if err != nil {
				t.Fatalf("Buscar: %v", err)
			}
			if got := titulos(eventos); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestObtenerYBorrar(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()

	eventos := []event.Event{nuevoEvento("meetup", time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina), event.ZonaCABA, "ia")}
	if err := s.Guardar(ctx, eventos); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	id := eventos[0].ID

	e, err := s.Obtener(ctx, id)
	if err != nil || e.Titulo != "meetup" {
		t.Fatalf("Obtener = %v, %v", e, err)
	}

	if err := s.Borrar(ctx, id); err != nil {
		t.Fatalf("Borrar: %v", err)
	}
	if _, err := s.Obtener(ctx, id); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("Obtener después de borrar: got %v, want ErrNoEncontrado", err)
	}
	if err := s.Borrar(ctx, id); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("Borrar dos veces: got %v, want ErrNoEncontrado", err)
	}
}
