# eventos-ba

API pública y gratuita de eventos IT en Buenos Aires: meetups, conferencias,
hackatones y talleres en CABA, GBA y online.

```bash
curl https://eventos-ba.fly.dev/eventos
```

> La URL de arriba es la del deploy en Fly.io. Si corrés tu propia instancia,
> reemplazala por la tuya.

## Endpoints

| Método | Ruta               | Descripción                                   |
|--------|--------------------|-----------------------------------------------|
| GET    | `/eventos`         | Próximos eventos, ordenados por fecha         |
| GET    | `/eventos/proximo` | El próximo evento                             |
| GET    | `/eventos/{id}`    | Un evento por su ID                           |
| GET    | `/`                | Información de la API                         |

No hace falta registrarse ni usar una API key para leer.

### Filtros de `/eventos`

Todos son opcionales y se pueden combinar.

| Parámetro | Valores                          | Ejemplo        |
|-----------|----------------------------------|----------------|
| `zona`    | `CABA`, `GBA`, `Online`          | `?zona=caba`   |
| `dias`    | eventos que empiezan en los próximos N días (1 a 3650) | `?dias=7` |
| `fuente`  | de dónde salió el evento: `manual`, `ia`, `dev.events` | `?fuente=manual` |
| `limite`  | cantidad máxima de resultados (1 a 500, por defecto 100) | `?limite=10` |

Los eventos que ya empezaron pero todavía no terminaron siguen apareciendo.

```bash
# Eventos presenciales en CABA de la próxima semana
curl "https://eventos-ba.fly.dev/eventos?zona=CABA&dias=7"
```

### Formato de un evento

```json
{
  "id": 16,
  "titulo": "#AdminBirras Noviembre (sysarmy)",
  "inicio": "2026-11-05T19:00:00-03:00",
  "fin": "2026-11-05T22:00:00-03:00",
  "lugar": "Basta Callejón, Palermo",
  "url": "https://www.meetup.com/sysarmy/events/316974697/",
  "fuente": "ia",
  "zona": "CABA"
}
```

| Campo    | Tipo   | Notas                                                    |
|----------|--------|----------------------------------------------------------|
| `id`     | número | Identificador del evento en esta API                     |
| `titulo` | texto  |                                                          |
| `inicio` | fecha  | RFC 3339 en hora argentina. `00:00` significa que solo se conoce el día |
| `fin`    | fecha  | Opcional: no aparece si no se conoce                     |
| `lugar`  | texto  | Dirección, barrio o `Online`. Puede estar vacío          |
| `url`    | texto  | Página oficial o de inscripción                          |
| `fuente` | texto  | Cómo llegó el evento a la base                           |
| `zona`   | texto  | `CABA`, `GBA` u `Online`                                 |

### Errores

Los errores devuelven un código HTTP y un JSON con el detalle:

```json
{ "error": "zona inválida: usá CABA, GBA u Online" }
```

| Código | Cuándo                                              |
|--------|-----------------------------------------------------|
| 400    | Parámetros o datos inválidos                        |
| 404    | El evento no existe, o no hay próximos eventos      |
| 429    | Demasiados pedidos (ver límites)                    |
| 500    | Error del servidor                                  |

### Límites

- Hasta 5 pedidos por segundo por IP, con ráfagas de hasta 60. Si se pasa, la
  API responde `429` con el header `Retry-After`.
- Las respuestas se pueden cachear 60 segundos (`Cache-Control: public, max-age=60`).

### Usarla desde una página web

CORS está habilitado para las lecturas, así que se puede consumir directo
desde el navegador:

```js
const res = await fetch("https://eventos-ba.fly.dev/eventos?dias=30");
const eventos = await res.json();
```

## Cargar eventos

La escritura está reservada a quien administra la instancia y requiere el
token configurado en `EVENTOS_TOKEN`.

| Método | Ruta            | Descripción                                   |
|--------|-----------------|-----------------------------------------------|
| POST   | `/eventos`      | Crea o actualiza un evento o una lista        |
| DELETE | `/eventos/{id}` | Borra un evento                               |

```bash
curl -X POST https://eventos-ba.fly.dev/eventos \
  -H "Authorization: Bearer $EVENTOS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"titulo": "Meetup de Go", "inicio": "2026-11-05T19:00:00-03:00",
       "url": "https://ejemplo.com/meetup", "zona": "CABA", "lugar": "Palermo"}'
```

- Si llega un evento con una `url` que ya existe, se actualiza en vez de duplicarse.
- Si un evento de la lista es inválido no se guarda ninguno, y la respuesta
  detalla el problema de cada uno.
- Máximo 500 eventos y 1 MB por pedido.

## Correrla localmente

Requiere Go 1.24 o más nuevo.

```bash
go run ./cmd/eventos import eventos.json   # carga eventos desde un archivo
go run ./cmd/eventos fetch                 # trae eventos de dev.events
go run ./cmd/eventos list                  # lista los próximos eventos
go run ./cmd/eventos serve                 # API en http://localhost:8080
```

| Variable            | Uso                                                           |
|---------------------|---------------------------------------------------------------|
| `EVENTOS_DB`        | Ruta de la base SQLite (por defecto `~/.local/share/eventos-ba/eventos.db`) |
| `EVENTOS_TOKEN`     | Habilita la escritura. Mínimo 32 caracteres: `openssl rand -hex 32` |
| `EVENTOS_HEADER_IP` | Header con la IP real del cliente detrás de un proxy (en Fly.io: `Fly-Client-IP`). Solo configurarlo si el proxy lo pisa siempre |
| `PORT`              | Puerto de la API                                              |

Tests:

```bash
go test ./...
```

## Seguridad

- Solo lectura pública; la escritura pide un token de 32 caracteres o más,
  comparado en tiempo constante y con su propio límite de intentos por IP.
- Límite de pedidos por IP, timeouts del servidor y tope de tamaño en headers,
  cuerpo y cantidad de resultados.
- Validación estricta de los datos (campos desconocidos, URLs, largos, fechas).
- Consultas SQL parametrizadas.
- Headers de seguridad (`HSTS`, `CSP`, `nosniff`, `X-Frame-Options`).
- Los errores internos se loguean pero no se muestran al cliente.
- En Docker corre con un usuario sin privilegios.

Si encontrás una vulnerabilidad, reportala de forma privada desde la pestaña
**Security → Report a vulnerability** del repositorio, no en un issue público.
