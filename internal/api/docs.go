package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiYAML []byte

// Swagger UI se carga desde un CDN con versión fija y hash de integridad; la
// API solo sirve esta página, el script que la inicializa y el openapi.yaml.
const (
	swaggerCDN     = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14"
	swaggerCSSHash = "sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn"
	swaggerJSHash  = "sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep"
)

const docsHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>eventos-ba · documentación de la API</title>
  <link rel="stylesheet" href="` + swaggerCDN + `/swagger-ui.css" integrity="` + swaggerCSSHash + `" crossorigin="anonymous">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="` + swaggerCDN + `/swagger-ui-bundle.js" integrity="` + swaggerJSHash + `" crossorigin="anonymous"></script>
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
// Swagger UI del CDN y pedirle el yaml a esta misma API. En CSP una fuente con
// ruta tiene que terminar en "/" para cubrir los archivos de adentro.
const docsCSP = "default-src 'none'; script-src 'self' " + swaggerCDN + "/; style-src 'unsafe-inline' " + swaggerCDN +
	"/; img-src data: " + swaggerCDN + "/; font-src " + swaggerCDN + "/; connect-src 'self'; frame-ancestors 'none'"

func openapi(w http.ResponseWriter, r *http.Request) {
	servirEstatico(w, "application/yaml; charset=utf-8", openapiYAML)
}

func docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", docsCSP)
	servirEstatico(w, "text/html; charset=utf-8", []byte(docsHTML))
}

func docsInit(w http.ResponseWriter, r *http.Request) {
	servirEstatico(w, "text/javascript; charset=utf-8", []byte(docsInitJS))
}

func servirEstatico(w http.ResponseWriter, tipo string, contenido []byte) {
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(contenido)
}

// redirigirV1 manda las rutas sin versión a /v1, para quien usaba la API
// antes de que la tuviera.
func redirigirV1(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/v1"+r.URL.RequestURI(), http.StatusPermanentRedirect)
}
