package sources

import (
	"testing"
	"time"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
)

const htmlDePrueba = `<html><head>
<script type="application/ld+json">{"@type": "BreadcrumbList", "name": "Meetups"}</script>
<script type="application/ld+json">{"@type": "EducationEvent", "name": " Meetup Online ",
  "eventAttendanceMode": "https://schema.org/OnlineEventAttendanceMode",
  "startDate": "2026-12-03T00:00:00.000+00:00", "endDate": "2026-12-04T00:00:00.000+00:00",
  "url": "https://dev.events/a", "location": null}</script>
<script type="application/ld+json">{"@type": "EducationEvent", "name": "Conf presencial",
  "eventAttendanceMode": "https://schema.org/OfflineEventAttendanceMode",
  "startDate": "2026-11-14T00:00:00.000+00:00", "endDate": "2026-11-14T00:00:00.000+00:00",
  "url": "https://dev.events/b", "location": {"@type": "Place", "name": "Buenos Aires, Argentina"}}</script>
<script type="application/ld+json">esto no es JSON</script>
</head></html>`

func TestParsearDevEvents(t *testing.T) {
	eventos := parsearDevEvents([]byte(htmlDePrueba))

	if len(eventos) != 2 {
		t.Fatalf("got %d eventos, want 2", len(eventos))
	}

	online := eventos[0]
	if online.Titulo != "Meetup Online" {
		t.Errorf("Titulo = %q", online.Titulo)
	}
	if online.Zona != event.ZonaOnline || online.Lugar != "Online" {
		t.Errorf("Zona/Lugar = %q / %q", online.Zona, online.Lugar)
	}
	wantInicio := time.Date(2026, 12, 3, 0, 0, 0, 0, event.Argentina)
	if !online.Inicio.Equal(wantInicio) {
		t.Errorf("Inicio = %v, want %v", online.Inicio, wantInicio)
	}

	presencial := eventos[1]
	if presencial.Zona != event.ZonaCABA || presencial.Lugar != "Buenos Aires, Argentina" {
		t.Errorf("Zona/Lugar = %q / %q", presencial.Zona, presencial.Lugar)
	}
	if presencial.Fuente != "dev.events" {
		t.Errorf("Fuente = %q", presencial.Fuente)
	}

	for _, e := range eventos {
		if err := e.Validar(); err != nil {
			t.Errorf("%q no pasa la validación: %v", e.Titulo, err)
		}
	}
}
