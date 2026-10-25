package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
	"github.com/LorenteFacundo/eventos-ba/internal/store"
)

const tokenDePrueba = "token-de-prueba-con-mas-de-32-caracteres"

func nuevaAPI(t *testing.T, cfg Config) http.Handler {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return Handler(s, cfg)
}

type pedido struct {
	metodo, ruta, cuerpo, token, ip string
}

func hacer(api http.Handler, p pedido) *httptest.ResponseRecorder {
	req := httptest.NewRequest(p.metodo, p.ruta, strings.NewReader(p.cuerpo))
	if p.cuerpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	if p.ip != "" {
		req.RemoteAddr = p.ip + ":1234"
	}
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func esperarEstado(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("estado = %d, want %d (cuerpo: %s)", rec.Code, want, rec.Body.String())
	}
}

const eventoJSON = `{"titulo": "Meetup de Go", "inicio": "2030-01-10T19:00:00-03:00",
	"url": "https://ejemplo.com/meetup", "zona": "CABA"}`

func TestCicloCompleto(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	rec := hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON, token: tokenDePrueba})
	esperarEstado(t, rec, http.StatusCreated)

	var creados []event.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &creados); err != nil || len(creados) != 1 {
		t.Fatalf("respuesta del POST: %s", rec.Body.String())
	}
	if creados[0].ID == 0 || creados[0].Fuente != event.FuentePorDefecto {
		t.Errorf("evento creado: %+v", creados[0])
	}
	ruta := "/eventos/" + strconv.FormatInt(creados[0].ID, 10)

	esperarEstado(t, hacer(api, pedido{metodo: "GET", ruta: ruta}), http.StatusOK)
	esperarEstado(t, hacer(api, pedido{metodo: "GET", ruta: "/eventos/proximo"}), http.StatusOK)

	rec = hacer(api, pedido{metodo: "GET", ruta: "/eventos?zona=caba&limite=10"})
	esperarEstado(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), "Meetup de Go") {
		t.Errorf("filtro por zona: %s", rec.Body.String())
	}

	esperarEstado(t, hacer(api, pedido{metodo: "DELETE", ruta: ruta, token: tokenDePrueba}), http.StatusNoContent)
	esperarEstado(t, hacer(api, pedido{metodo: "GET", ruta: ruta}), http.StatusNotFound)
	esperarEstado(t, hacer(api, pedido{metodo: "DELETE", ruta: ruta, token: tokenDePrueba}), http.StatusNotFound)
}

func TestEscrituraRequiereToken(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	esperarEstado(t, hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON}), http.StatusUnauthorized)
	esperarEstado(t, hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON, token: "otro"}), http.StatusUnauthorized)
	esperarEstado(t, hacer(api, pedido{metodo: "DELETE", ruta: "/eventos/1"}), http.StatusUnauthorized)
}

func TestSinTokenEsSoloLectura(t *testing.T) {
	api := nuevaAPI(t, Config{})
	esperarEstado(t, hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON, token: "x"}), http.StatusForbidden)
}

func TestFuerzaBrutaDelTokenSeCorta(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	var ultimo int
	for range 20 {
		ultimo = hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON, token: "adivinando"}).Code
	}
	if ultimo != http.StatusTooManyRequests {
		t.Errorf("después de 20 intentos: estado = %d, want 429", ultimo)
	}
}

func TestLimitePorIP(t *testing.T) {
	api := nuevaAPI(t, Config{})

	var ultimo int
	for range 100 {
		ultimo = hacer(api, pedido{metodo: "GET", ruta: "/eventos", ip: "203.0.113.1"}).Code
	}
	if ultimo != http.StatusTooManyRequests {
		t.Fatalf("después de 100 pedidos: estado = %d, want 429", ultimo)
	}

	// Otra IP no se ve afectada.
	esperarEstado(t, hacer(api, pedido{metodo: "GET", ruta: "/eventos", ip: "203.0.113.2"}), http.StatusOK)
}

func TestHeaderIPSoloSiEstaConfigurado(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("Fly-Client-IP", "203.0.113.9")

	sinHeader := &servidor{}
	if ip := sinHeader.ipCliente(req); ip != "10.0.0.1" {
		t.Errorf("sin HeaderIP: ip = %q, want 10.0.0.1", ip)
	}
	conHeader := &servidor{headerIP: "Fly-Client-IP"}
	if ip := conHeader.ipCliente(req); ip != "203.0.113.9" {
		t.Errorf("con HeaderIP: ip = %q, want 203.0.113.9", ip)
	}
}

func TestPedidosInvalidos(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	casos := []struct {
		nombre string
		p      pedido
		want   int
	}{
		{"evento inválido", pedido{metodo: "POST", ruta: "/eventos", cuerpo: `{"titulo": "x", "url": "ftp://a", "zona": "Marte"}`}, http.StatusBadRequest},
		{"JSON roto", pedido{metodo: "POST", ruta: "/eventos", cuerpo: `{"titulo": `}, http.StatusBadRequest},
		{"campo desconocido", pedido{metodo: "POST", ruta: "/eventos", cuerpo: `{"titlo": "x"}`}, http.StatusBadRequest},
		{"zona inválida", pedido{metodo: "GET", ruta: "/eventos?zona=Marte"}, http.StatusBadRequest},
		{"dias no numérico", pedido{metodo: "GET", ruta: "/eventos?dias=abc"}, http.StatusBadRequest},
		{"dias fuera de rango", pedido{metodo: "GET", ruta: "/eventos?dias=36500"}, http.StatusBadRequest},
		{"limite excesivo", pedido{metodo: "GET", ruta: "/eventos?limite=100000"}, http.StatusBadRequest},
		{"id no numérico", pedido{metodo: "GET", ruta: "/eventos/abc"}, http.StatusBadRequest},
		{"id negativo", pedido{metodo: "GET", ruta: "/eventos/-1"}, http.StatusBadRequest},
		{"método no permitido", pedido{metodo: "PUT", ruta: "/eventos"}, http.StatusMethodNotAllowed},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			c.p.token = tokenDePrueba
			esperarEstado(t, hacer(api, c.p), c.want)
		})
	}
}

func TestPostSinContentTypeJSON(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	req := httptest.NewRequest("POST", "/eventos", strings.NewReader(eventoJSON))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+tokenDePrueba)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	esperarEstado(t, rec, http.StatusUnsupportedMediaType)
}

func TestPostDemasiadoGrande(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})
	cuerpo := `{"titulo": "` + strings.Repeat("a", maxCuerpo) + `"}`
	esperarEstado(t, hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: cuerpo, token: tokenDePrueba}), http.StatusRequestEntityTooLarge)
}

func TestCabeceras(t *testing.T) {
	api := nuevaAPI(t, Config{Token: tokenDePrueba})

	get := hacer(api, pedido{metodo: "GET", ruta: "/eventos"})
	esperarEstado(t, get, http.StatusOK)
	if got := strings.TrimSpace(get.Body.String()); got != "[]" {
		t.Errorf("lista vacía: got %s, want []", got)
	}

	for header, want := range map[string]string{
		"Content-Type":                "application/json; charset=utf-8",
		"X-Content-Type-Options":      "nosniff",
		"X-Frame-Options":             "DENY",
		"Access-Control-Allow-Origin": "*",
		"Cache-Control":               "public, max-age=60",
	} {
		if got := get.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	post := hacer(api, pedido{metodo: "POST", ruta: "/eventos", cuerpo: eventoJSON, token: tokenDePrueba})
	if got := post.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("POST no debería habilitar CORS, got %q", got)
	}
	if got := post.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("POST Cache-Control = %q, want no-store", got)
	}
}
