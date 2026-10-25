package api

import (
	"encoding/json"
	"net/http"
)

func (srv *servidor) responderJSON(w http.ResponseWriter, estado int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(estado)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(datos); err != nil {
		srv.log.Error("escribiendo respuesta", "error", err)
	}
}

func (srv *servidor) responderError(w http.ResponseWriter, estado int, mensaje string) {
	srv.responderJSON(w, estado, map[string]string{"error": mensaje})
}

// errorInterno loguea el error real y al cliente le muestra uno genérico,
// para no filtrar detalles de la base ni del servidor.
func (srv *servidor) errorInterno(w http.ResponseWriter, r *http.Request, err error) {
	srv.log.Error("error interno", "metodo", r.Method, "ruta", r.URL.Path, "error", err)
	srv.responderError(w, http.StatusInternalServerError, "error interno del servidor")
}

// cacheable permite que navegadores y CDNs guarden la respuesta un minuto.
func cacheable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=60")
}
