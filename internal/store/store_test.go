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

func nuevoEvento(titulo string, inicio time.Time, fuente string) event.Event {
	return event.Event{
		Titulo:    titulo,
		Inicio:    inicio,
		Fin:       inicio.Add(2 * time.Hour),
		URL:       "https://ejemplo.com/" + titulo,
		Fuente:    fuente,
		Modalidad: event.Presencial,
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

	primera := []event.Event{nuevoEvento("meetup", inicio, "ia")}
	if err := s.Guardar(ctx, primera); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	id := primera[0].ID
	if id == 0 {
		t.Fatal("no se asignó ID")
	}

	segunda := []event.Event{nuevoEvento("meetup", inicio, "ia")}
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
		nuevoEvento("pasado", ahora.Add(-48*time.Hour), "ia"),
		nuevoEvento("mañana", ahora.Add(24*time.Hour), "ia"),
		nuevoEvento("en tres días", ahora.Add(72*time.Hour), "dev.events"),
		nuevoEvento("lejano", ahora.Add(30*24*time.Hour), "manual"),
	})
	if err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	casos := []struct {
		nombre string
		filtro Filtro
		want   []string
	}{
		{"sin filtro", Filtro{}, []string{"pasado", "mañana", "en tres días", "lejano"}},
		{"desde", Filtro{Desde: ahora}, []string{"mañana", "en tres días", "lejano"}},
		{"desde y hasta", Filtro{Desde: ahora, Hasta: ahora.AddDate(0, 0, 7)}, []string{"mañana", "en tres días"}},
		{"hasta no incluye el límite", Filtro{Hasta: ahora.Add(24 * time.Hour)}, []string{"pasado"}},
		{"fuente sin distinguir mayúsculas", Filtro{Fuente: "DEV.EVENTS"}, []string{"en tres días"}},
		{"límite", Filtro{Desde: ahora, Limite: 1}, []string{"mañana"}},
		{"sin resultados", Filtro{Desde: ahora, Fuente: "otra"}, nil},
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

func TestBuscarPorTexto(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()
	inicio := time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina)

	ia := nuevoEvento("TECWEEK ED: IA y Educación", inicio, "ia")
	ia.Lugar = "Usina del Arte, La Boca"
	industria := nuevoEvento("Industria 4.0", inicio.Add(time.Hour), "ia")
	industria.Lugar = "Palermo"
	if err := s.Guardar(ctx, []event.Event{ia, industria}); err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	casos := []struct {
		texto string
		want  []string
	}{
		{"educacion", []string{ia.Titulo}},
		{"EDUCACIÓN", []string{ia.Titulo}},
		{"educ", []string{ia.Titulo}},
		{"ia", []string{ia.Titulo}},
		{"palermo", []string{industria.Titulo}},
		{"ia boca", []string{ia.Titulo}},
		{"ia palermo", nil},
		{"4.0", []string{industria.Titulo}},
		{"100%", nil},
		{"   ", []string{ia.Titulo, industria.Titulo}},
	}

	for _, c := range casos {
		t.Run(c.texto, func(t *testing.T) {
			eventos, err := s.Buscar(ctx, Filtro{Texto: c.texto})
			if err != nil {
				t.Fatalf("Buscar: %v", err)
			}
			if got := titulos(eventos); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuscarPorModalidad(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()
	inicio := time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina)

	presencial := nuevoEvento("presencial", inicio, "ia")
	online := nuevoEvento("online", inicio.Add(time.Hour), "ia")
	online.Modalidad = event.Online
	hibrido := nuevoEvento("hibrido", inicio.Add(2*time.Hour), "ia")
	hibrido.Modalidad = event.Hibrido
	if err := s.Guardar(ctx, []event.Event{presencial, online, hibrido}); err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	casos := []struct {
		modalidad event.Modalidad
		want      []string
	}{
		{"", []string{"presencial", "online", "hibrido"}},
		{event.Presencial, []string{"presencial", "hibrido"}},
		{event.Online, []string{"online", "hibrido"}},
		{event.Hibrido, []string{"hibrido"}},
	}

	for _, c := range casos {
		t.Run(string(c.modalidad), func(t *testing.T) {
			eventos, err := s.Buscar(ctx, Filtro{Modalidad: c.modalidad})
			if err != nil {
				t.Fatalf("Buscar: %v", err)
			}
			if got := titulos(eventos); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
			if len(eventos) > 0 && eventos[0].Modalidad == "" {
				t.Error("la modalidad no se leyó de la base")
			}
		})
	}
}

// Un evento sin fin o sin horario no tiene que desaparecer apenas empieza.
func TestBuscarEventosSinFin(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()

	conHora := nuevoEvento("con hora", time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina), "ia")
	conHora.Fin = time.Time{}
	soloDia := nuevoEvento("solo el día", time.Date(2030, 1, 10, 0, 0, 0, 0, event.Argentina), "dev.events")
	soloDia.Fin = soloDia.Inicio
	if err := s.Guardar(ctx, []event.Event{conHora, soloDia}); err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	casos := []struct {
		ahora time.Time
		want  []string
	}{
		{time.Date(2030, 1, 10, 12, 0, 0, 0, event.Argentina), []string{"solo el día", "con hora"}},
		{time.Date(2030, 1, 10, 20, 0, 0, 0, event.Argentina), []string{"solo el día", "con hora"}},
		{time.Date(2030, 1, 10, 23, 0, 0, 0, event.Argentina), []string{"solo el día"}},
		{time.Date(2030, 1, 11, 0, 0, 0, 0, event.Argentina), nil},
	}

	for _, c := range casos {
		t.Run(c.ahora.Format("15:04"), func(t *testing.T) {
			eventos, err := s.Buscar(ctx, Filtro{Desde: c.ahora})
			if err != nil {
				t.Fatalf("Buscar: %v", err)
			}
			if got := titulos(eventos); !slices.Equal(got, c.want) {
				t.Errorf("a las %s: got %v, want %v", c.ahora.Format("02/01 15:04"), got, c.want)
			}
		})
	}
}

func TestSimplificar(t *testing.T) {
	casos := map[string]string{
		"¡IA en Educación!":     "ia en educacion",
		"  Año   2030 ":         "ano 2030",
		"#AdminBirras":          "adminbirras",
		"Node.js / C#":          "node js c",
		"São Paulo, Düsseldorf": "sao paulo dusseldorf",
	}
	for entrada, want := range casos {
		if got := simplificar(entrada); got != want {
			t.Errorf("simplificar(%q) = %q, want %q", entrada, got, want)
		}
	}
}

func TestMigrarEsIdempotente(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "test.db")
	for range 2 {
		s, err := Open(ruta)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		var version int
		if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != len(migraciones) {
			t.Errorf("user_version = %d, %v; want %d", version, err, len(migraciones))
		}
		s.Close()
	}
}

func TestObtenerYBorrar(t *testing.T) {
	s := abrirParaTest(t)
	ctx := context.Background()

	eventos := []event.Event{nuevoEvento("meetup", time.Date(2030, 1, 10, 19, 0, 0, 0, event.Argentina), "ia")}
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
