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
<script type="application/ld+json">{"@type": "EducationEvent", "name": " Conf presencial ",
  "eventAttendanceMode": "https://schema.org/OfflineEventAttendanceMode",
  "startDate": "2026-11-14T00:00:00.000+00:00", "endDate": "2026-11-14T00:00:00.000+00:00",
  "url": "https://dev.events/b", "location": {"@type": "Place", "name": "Buenos Aires, Argentina"}}</script>
<script type="application/ld+json">esto no es JSON</script>
</head></html>`

func TestParsearDevEvents(t *testing.T) {
	eventos := parsearDevEvents([]byte(htmlDePrueba))

	if len(eventos) != 1 {
		t.Fatalf("got %d eventos, want 1 (el online se descarta)", len(eventos))
	}

	e := eventos[0]
	if e.Titulo != "Conf presencial" || e.Lugar != "Buenos Aires, Argentina" || e.Fuente != "dev.events" {
		t.Errorf("evento = %+v", e)
	}
	wantInicio := time.Date(2026, 11, 14, 0, 0, 0, 0, event.Argentina)
	if !e.Inicio.Equal(wantInicio) {
		t.Errorf("Inicio = %v, want %v", e.Inicio, wantInicio)
	}
	if err := e.Validar(); err != nil {
		t.Errorf("no pasa la validación: %v", err)
	}
}
