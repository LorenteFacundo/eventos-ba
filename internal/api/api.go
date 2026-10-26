// Package api expone los eventos como una API HTTP con JSON.
// Las rutas están documentadas en el README.
package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
	"github.com/LorenteFacundo/eventos-ba/internal/store"
)

const (
	maxCuerpo           = 1 << 20 // 1 MB
	maxEventosPorPedido = 500
	limitePorDefecto    = 100
	limiteMaximo        = 500
	maxBusqueda         = 100
)

type Config struct {
	// Token para POST y DELETE. Vacío deja la API en solo lectura.
	Token string
	// HeaderIP es el header con la IP real del cliente cuando hay un proxy
	// adelante (Fly-Client-IP en Fly.io). Vacío usa la IP de la conexión.
	HeaderIP string
	Log      *slog.Logger
}

type servidor struct {
	store         *store.Store
	token         string
	headerIP      string
	log           *slog.Logger
	limLecturas   *limitador
	limEscrituras *limitador
}

func Handler(s *store.Store, cfg Config) http.Handler {
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	srv := &servidor{
		store:         s,
		token:         cfg.Token,
		headerIP:      cfg.HeaderIP,
		log:           log,
		limLecturas:   nuevoLimitador(5, 60),                         // 5 por segundo, ráfagas de 60
		limEscrituras: nuevoLimitador(rate.Every(6*time.Second), 10), // 10 por minuto
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.info)
	mux.HandleFunc("GET /eventos", srv.listarEventos)
	mux.HandleFunc("GET /eventos/proximo", srv.proximoEvento)
	mux.HandleFunc("GET /eventos/{id}", srv.obtenerEvento)
	mux.HandleFunc("POST /eventos", srv.conToken(srv.crearEventos))
	mux.HandleFunc("DELETE /eventos/{id}", srv.conToken(srv.borrarEvento))

	var h http.Handler = mux
	h = srv.limitar(srv.limLecturas, h)
	h = cabecerasSeguridad(h)
	h = srv.registrarPedidos(h)
	return h
}

func (srv *servidor) info(w http.ResponseWriter, r *http.Request) {
	cacheable(w)
	srv.responderJSON(w, http.StatusOK, map[string]any{
		"nombre":      "eventos-ba",
		"descripcion": "Eventos IT en la Ciudad de Buenos Aires, presenciales y online",
		"docs":        "https://github.com/LorenteFacundo/eventos-ba",
		"rutas": []string{
			"GET /eventos",
			"GET /eventos/proximo",
			"GET /eventos/{id}",
		},
		"filtros": map[string]string{
			"q":         "palabras del título o el lugar",
			"modalidad": "presencial, online o hibrido (los híbridos entran en los dos)",
			"fecha":     "un día: AAAA-MM-DD",
			"mes":       "un mes: AAAA-MM",
			"desde":     "desde un día: AAAA-MM-DD",
			"hasta":     "hasta un día inclusive: AAAA-MM-DD",
			"dias":      "los próximos N días",
			"fuente":    "manual, ia, dev.events",
			"limite":    fmt.Sprintf("máximo de resultados (1 a %d)", limiteMaximo),
		},
	})
}

func (srv *servidor) listarEventos(w http.ResponseWriter, r *http.Request) {
	filtro, err := filtroDesdeQuery(r)
	if err != nil {
		srv.responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	eventos, err := srv.store.Buscar(r.Context(), filtro)
	if err != nil {
		srv.errorInterno(w, r, err)
		return
	}
	cacheable(w)
	srv.responderJSON(w, http.StatusOK, eventos)
}

func filtroDesdeQuery(r *http.Request) (store.Filtro, error) {
	q := r.URL.Query()
	filtro := store.Filtro{Fuente: q.Get("fuente"), Limite: limitePorDefecto}

	if q.Has("modalidad") {
		m, ok := event.ParsearModalidad(q.Get("modalidad"))
		if !ok {
			return filtro, errors.New("modalidad tiene que ser presencial, online o hibrido")
		}
		filtro.Modalidad = m
	}

	texto := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(texto) > maxBusqueda {
		return filtro, fmt.Errorf("q no puede superar los %d caracteres", maxBusqueda)
	}
	filtro.Texto = texto

	var err error
	filtro.Desde, filtro.Hasta, err = periodo(q, time.Now())
	if err != nil {
		return filtro, err
	}

	if l := q.Get("limite"); l != "" {
		limite, err := strconv.Atoi(l)
		if err != nil || limite < 1 || limite > limiteMaximo {
			return filtro, fmt.Errorf("limite tiene que ser un número entre 1 y %d", limiteMaximo)
		}
		filtro.Limite = limite
	}

	return filtro, nil
}

// periodo arma el rango de fechas con fecha, mes, dias o desde/hasta, que no
// se pueden combinar entre sí. Sin ninguno van los eventos de ahora en adelante.
// Hasta queda abierto: es el primer instante que ya no entra.
func periodo(q url.Values, ahora time.Time) (desde, hasta time.Time, err error) {
	formas := 0
	for _, p := range []string{"fecha", "mes", "dias"} {
		if q.Has(p) {
			formas++
		}
	}
	if q.Has("desde") || q.Has("hasta") {
		formas++
	}
	if formas > 1 {
		return desde, hasta, errors.New("usá uno solo de estos filtros: fecha, mes, dias o desde/hasta")
	}

	switch {
	case q.Has("fecha"):
		dia, err := parsearDia("fecha", q.Get("fecha"))
		return dia, dia.AddDate(0, 0, 1), err

	case q.Has("mes"):
		mes, err := time.ParseInLocation("2006-01", q.Get("mes"), event.Argentina)
		if err != nil {
			return desde, hasta, errors.New("mes tiene que tener el formato AAAA-MM, por ejemplo 2026-11")
		}
		return mes, mes.AddDate(0, 1, 0), nil

	case q.Has("dias"):
		dias, err := strconv.Atoi(q.Get("dias"))
		if err != nil || dias < 1 || dias > 3650 {
			return desde, hasta, errors.New("dias tiene que ser un número entre 1 y 3650")
		}
		return ahora, ahora.AddDate(0, 0, dias), nil
	}

	desde = ahora
	if q.Has("desde") {
		if desde, err = parsearDia("desde", q.Get("desde")); err != nil {
			return desde, hasta, err
		}
	}
	if q.Has("hasta") {
		dia, err := parsearDia("hasta", q.Get("hasta"))
		if err != nil {
			return desde, hasta, err
		}
		hasta = dia.AddDate(0, 0, 1) // el día de hasta entra completo
		if !hasta.After(desde) {
			return desde, hasta, errors.New("hasta no puede ser anterior a desde (sin desde, se toma hoy)")
		}
	}
	return desde, hasta, nil
}

func parsearDia(nombre, valor string) (time.Time, error) {
	dia, err := time.ParseInLocation(time.DateOnly, valor, event.Argentina)
	if err != nil {
		return dia, fmt.Errorf("%s tiene que tener el formato AAAA-MM-DD, por ejemplo 2026-11-05", nombre)
	}
	return dia, nil
}

// proximoEvento acepta los mismos filtros que /eventos.
func (srv *servidor) proximoEvento(w http.ResponseWriter, r *http.Request) {
	filtro, err := filtroDesdeQuery(r)
	if err != nil {
		srv.responderError(w, http.StatusBadRequest, err.Error())
		return
	}
	filtro.Limite = 1

	eventos, err := srv.store.Buscar(r.Context(), filtro)
	if err != nil {
		srv.errorInterno(w, r, err)
		return
	}
	if len(eventos) == 0 {
		srv.responderError(w, http.StatusNotFound, "no hay eventos que coincidan")
		return
	}
	cacheable(w)
	srv.responderJSON(w, http.StatusOK, eventos[0])
}

func (srv *servidor) obtenerEvento(w http.ResponseWriter, r *http.Request) {
	id, ok := srv.leerID(w, r)
	if !ok {
		return
	}

	e, err := srv.store.Obtener(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoEncontrado):
		srv.responderError(w, http.StatusNotFound, err.Error())
	case err != nil:
		srv.errorInterno(w, r, err)
	default:
		cacheable(w)
		srv.responderJSON(w, http.StatusOK, e)
	}
}

// crearEventos recibe un evento o una lista. Si alguno es inválido no se
// guarda ninguno y se devuelve el detalle de cada problema.
func (srv *servidor) crearEventos(w http.ResponseWriter, r *http.Request) {
	tipo, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || tipo != "application/json" {
		srv.responderError(w, http.StatusUnsupportedMediaType, "el cuerpo tiene que ser application/json")
		return
	}

	cuerpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCuerpo))
	if err != nil {
		var errTamaño *http.MaxBytesError
		if errors.As(err, &errTamaño) {
			srv.responderError(w, http.StatusRequestEntityTooLarge, "el cuerpo supera 1 MB")
			return
		}
		srv.responderError(w, http.StatusBadRequest, "no se pudo leer el cuerpo")
		return
	}

	eventos, err := event.DecodificarJSON(cuerpo)
	if err != nil {
		srv.responderError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(eventos) > maxEventosPorPedido {
		srv.responderError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("máximo %d eventos por pedido", maxEventosPorPedido))
		return
	}

	if err := event.Preparar(eventos); err != nil {
		var errValidacion *event.ErrorValidacion
		if errors.As(err, &errValidacion) {
			srv.responderJSON(w, http.StatusBadRequest, map[string]any{
				"error":    "eventos inválidos",
				"detalles": errValidacion.Problemas,
			})
			return
		}
		srv.errorInterno(w, r, err)
		return
	}

	if err := srv.store.Guardar(r.Context(), eventos); err != nil {
		srv.errorInterno(w, r, err)
		return
	}
	srv.responderJSON(w, http.StatusCreated, eventos)
}

func (srv *servidor) borrarEvento(w http.ResponseWriter, r *http.Request) {
	id, ok := srv.leerID(w, r)
	if !ok {
		return
	}

	err := srv.store.Borrar(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoEncontrado):
		srv.responderError(w, http.StatusNotFound, err.Error())
	case err != nil:
		srv.errorInterno(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (srv *servidor) leerID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		srv.responderError(w, http.StatusBadRequest, "el id tiene que ser un número positivo")
		return 0, false
	}
	return id, true
}
