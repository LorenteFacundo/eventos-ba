package api

import (
	"crypto/subtle"
	"net/http"
	"time"
)

func (srv *servidor) limitar(l *limitador, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.permitir(srv.ipCliente(r)) {
			w.Header().Set("Retry-After", "60")
			srv.responderError(w, http.StatusTooManyRequests, "demasiados pedidos, probá de nuevo en un rato")
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

// conToken protege las rutas de escritura. Tiene su propio límite de pedidos,
// mucho más bajo, para que no se pueda adivinar el token a fuerza bruta.
func (srv *servidor) conToken(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if srv.token == "" {
			srv.responderError(w, http.StatusForbidden, "la escritura está deshabilitada en este servidor")
			return
		}
		if !srv.limEscrituras.permitir(srv.ipCliente(r)) {
			w.Header().Set("Retry-After", "60")
			srv.responderError(w, http.StatusTooManyRequests, "demasiados pedidos de escritura, probá de nuevo en un rato")
			return
		}

		recibido := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(recibido), []byte("Bearer "+srv.token)) != 1 {
			srv.log.Warn("token inválido", "ip", srv.ipCliente(r), "ruta", r.URL.Path)
			srv.responderError(w, http.StatusUnauthorized, "token inválido o ausente")
			return
		}

		siguiente(w, r)
	}
}

func cabecerasSeguridad(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Cache-Control", "no-store")

		// Cualquier sitio puede leer la API desde el navegador, pero no escribir:
		// las escrituras se hacen con el token desde un servidor o la terminal.
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			h.Set("Access-Control-Allow-Origin", "*")
		}

		siguiente.ServeHTTP(w, r)
	})
}

func (srv *servidor) registrarPedidos(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rw := &respuestaConEstado{ResponseWriter: w, estado: http.StatusOK}

		siguiente.ServeHTTP(rw, r)

		srv.log.Info("pedido",
			"metodo", r.Method,
			"ruta", r.URL.RequestURI(),
			"estado", rw.estado,
			"ip", srv.ipCliente(r),
			"duracion", time.Since(inicio).Round(time.Microsecond),
		)
	})
}

// respuestaConEstado guarda el código de estado para poder loguearlo.
type respuestaConEstado struct {
	http.ResponseWriter
	estado int
}

func (r *respuestaConEstado) WriteHeader(estado int) {
	r.estado = estado
	r.ResponseWriter.WriteHeader(estado)
}
