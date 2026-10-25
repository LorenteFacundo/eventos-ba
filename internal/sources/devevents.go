package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
)

const devEventsURL = "https://dev.events/meetups/SA/AR/Buenos_Aires"

// dev.events no tiene API, pero la página incluye los eventos como JSON-LD (schema.org).
var jsonLDRegex = regexp.MustCompile(`(?s)<script[^>]*application/ld\+json[^>]*>(.*?)</script>`)

type devEvent struct {
	Type           string       `json:"@type"`
	Name           string       `json:"name"`
	AttendanceMode string       `json:"eventAttendanceMode"`
	StartDate      time.Time    `json:"startDate"`
	EndDate        time.Time    `json:"endDate"`
	URL            string       `json:"url"`
	Location       *devLocation `json:"location"`
}

type devLocation struct {
	Name string `json:"name"`
}

func fetchDevEvents(ctx context.Context) ([]event.Event, error) {
	html, err := descargar(ctx, devEventsURL)
	if err != nil {
		return nil, fmt.Errorf("descargando dev.events: %w", err)
	}
	return parsearDevEvents(html), nil
}

func parsearDevEvents(html []byte) []event.Event {
	var eventos []event.Event
	for _, match := range jsonLDRegex.FindAllSubmatch(html, -1) {
		var d devEvent
		if err := json.Unmarshal(match[1], &d); err != nil {
			continue
		}
		if d.Type != "EducationEvent" {
			continue // también hay bloques BreadcrumbList, etc.
		}
		// Los online de esa página son de cualquier parte del mundo, no de Buenos Aires.
		if strings.Contains(d.AttendanceMode, "OnlineEventAttendanceMode") {
			continue
		}
		eventos = append(eventos, convertirDevEvent(d))
	}
	return eventos
}

func convertirDevEvent(d devEvent) event.Event {
	e := event.Event{
		Titulo: strings.TrimSpace(d.Name),
		Inicio: soloFecha(d.StartDate),
		Fin:    soloFecha(d.EndDate),
		URL:    d.URL,
		Fuente: "dev.events",
	}
	e.Modalidad = event.Presencial
	if strings.Contains(d.AttendanceMode, "MixedEventAttendanceMode") {
		e.Modalidad = event.Hibrido
	}
	if d.Location != nil {
		e.Lugar = d.Location.Name
	}

	return e
}

// dev.events solo publica el día (00:00 UTC). Convertido a hora argentina
// caería el día anterior a las 21:00, así que se arma el mismo día en ART.
func soloFecha(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, event.Argentina)
}
