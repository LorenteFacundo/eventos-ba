package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiYAML []byte

// Swagger UI se carga desde un CDN con versión fija; la API solo sirve esta
// página, el script que la inicializa y el openapi.yaml.
const swaggerCDN = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14"

const docsHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>eventos-ba · documentación de la API</title>
  <link rel="stylesheet" href="` + swaggerCDN + `/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="` + swaggerCDN + `/swagger-ui-bundle.js"></script>
  <script src="/docs/init.js"></script>
</body>
</html>
`

const docsInitJS = `window.ui = SwaggerUIBundle({
  url: "/v1/openapi.yaml",
  dom_id: "#swagger-ui",
  deepLinking: true,
  tryItOutEnabled: true,
  defaultModelsExpandDepth: 1,
});
`

// La política global es default-src 'none'. La página de docs necesita cargar
// Swagger UI del CDN y pedirle el yaml a esta misma API.
const docsCSP = "default-src 'none'; script-src 'self' " + swaggerCDN + "; style-src 'unsafe-inline' " + swaggerCDN +
	"; img-src data: " + swaggerCDN + "; font-src " + swaggerCDN + "; connect-src 'self'; frame-ancestors 'none'"

func (srv *servidor) openapi(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/yaml; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=3600")
	h.Set("Access-Control-Allow-Origin", "*")
	w.Write(openapiYAML)
}

func (srv *servidor) docs(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", docsCSP)
	h.Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(docsHTML))
}

func (srv *servidor) docsInit(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/javascript; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(docsInitJS))
}

// redirigirV1 manda las rutas sin versión a /v1, para quien usaba la API
// antes de que la tuviera.
func redirigirV1(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/v1"+r.URL.RequestURI(), http.StatusPermanentRedirect)
}
