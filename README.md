# eventos-ba

API pública y gratuita de eventos IT en la Ciudad de Buenos Aires: meetups,
conferencias, hackatones y talleres, presenciales y online.

```bash
curl https://eventos-ba.fly.dev/eventos
```

> La URL de arriba es la del deploy en Fly.io. Si corrés tu propia instancia,
> reemplazala por la tuya.

## Qué eventos incluye

Eventos de tecnología en la Ciudad de Buenos Aires, presenciales u online
organizados por comunidades de acá. Si aparece alguno en los alrededores se
suma igual; la ubicación exacta está en `lugar` y en la página oficial de cada
evento. Los online de otros países no se incluyen.

## Endpoints

| Método | Ruta               | Descripción                                   |
|--------|--------------------|-----------------------------------------------|
| GET    | `/eventos`         | Próximos eventos, ordenados por fecha         |
| GET    | `/eventos/proximo` | El próximo evento (acepta los mismos filtros) |
| GET    | `/eventos/{id}`    | Un evento por su ID                           |
| GET    | `/`                | Información de la API                         |

No hace falta registrarse ni usar una API key para leer.

### Filtros de `/eventos`

Todos son opcionales y se pueden combinar, salvo los de fecha (ver abajo).

| Parámetro | Busca                                                     | Ejemplo             |
|-----------|-----------------------------------------------------------|---------------------|
| `q`       | palabras en el título o el lugar (hasta 100 caracteres)   | `?q=python`         |
| `modalidad` | `presencial`, `online` (o `virtual`) o `hibrido`        | `?modalidad=online` |
| `fecha`   | eventos de un día                                         | `?fecha=2026-11-05` |
| `mes`     | eventos de un mes                                         | `?mes=2026-11`      |
| `desde`   | eventos desde un día                                      | `?desde=2026-11-01` |
| `hasta`   | eventos hasta un día, inclusive                           | `?hasta=2026-11-30` |
| `dias`    | eventos de los próximos N días (1 a 3650)                 | `?dias=7`           |
| `fuente`  | de dónde salió el evento: `manual`, `ia`, `dev.events`    | `?fuente=manual`    |
| `limite`  | cantidad máxima de resultados (1 a 500, por defecto 100)  | `?limite=10`        |

**Búsqueda por texto.** No distingue mayúsculas ni tildes, y cada palabra
tiene que coincidir con el comienzo de una palabra del título o el lugar:
`q=educacion` encuentra "Educación", `q=ia` encuentra "IA aplicada" pero no
"Industria". Con varias palabras tienen que estar todas: `q=ia+palermo`.

**Modalidad.** Los eventos híbridos aparecen tanto con `modalidad=presencial`
como con `modalidad=online`; `modalidad=hibrido` trae solo esos.

**Fechas.** Van en formato `AAAA-MM-DD` (o `AAAA-MM` para `mes`) y en hora
argentina. `fecha`, `mes`, `dias` y `desde`/`hasta` arman el período de
búsqueda, así que se usa uno solo de ellos. Sin ninguno, la API devuelve los
eventos de ahora en adelante; con `fecha`, `mes` o `desde` también se pueden
consultar eventos pasados. Un evento entra si está ocurriendo en algún momento
del período, aunque haya empezado antes. Si no se conoce la hora de fin, se
toma que dura 3 horas, o el día entero si solo se conoce la fecha.

```bash
# Eventos de IA en noviembre
curl "https://eventos-ba.fly.dev/eventos?q=ia&mes=2026-11"

# Eventos online del próximo mes
curl "https://eventos-ba.fly.dev/eventos?modalidad=online&dias=30"

# Lo que hay esta semana
curl "https://eventos-ba.fly.dev/eventos?dias=7"

# La próxima hackatón
curl "https://eventos-ba.fly.dev/eventos/proximo?q=hackaton"
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
  "modalidad": "presencial",
  "fuente": "ia"
}
```

| Campo    | Tipo   | Notas                                                    |
|----------|--------|----------------------------------------------------------|
| `id`     | número | Identificador del evento en esta API                     |
| `titulo` | texto  |                                                          |
| `inicio` | fecha  | RFC 3339 en hora argentina. `00:00` significa que solo se conoce el día |
| `fin`    | fecha  | Opcional: no aparece si no se conoce                     |
| `lugar`  | texto  | Dirección o barrio. Puede estar vacío                    |
| `url`    | texto  | Página oficial o de inscripción                          |
| `modalidad` | texto | `presencial`, `online` o `hibrido`                     |
| `fuente` | texto  | Cómo llegó el evento a la base                           |

### Errores

Los errores devuelven un código HTTP y un JSON con el detalle:

```json
{ "error": "mes tiene que tener el formato AAAA-MM, por ejemplo 2026-11" }
```

| Código | Cuándo                                              |
|--------|-----------------------------------------------------|
| 400    | Parámetros o datos inválidos                        |
| 404    | El evento no existe, o no hay ninguno que coincida  |
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
       "url": "https://ejemplo.com/meetup", "lugar": "Palermo"}'
```

- `modalidad` es opcional; si no viene, el evento se carga como `presencial`.
- Si llega un evento con una `url` que ya existe, se actualiza en vez de duplicarse.
- Si un evento de la lista es inválido no se guarda ninguno, y la respuesta
  detalla el problema de cada uno.
- Máximo 500 eventos y 1 MB por pedido.
- Errores propios de la escritura: `401` token inválido o ausente, `403` la
  instancia no tiene token configurado, `413` cuerpo o lista demasiado grandes,
  `415` el cuerpo no es `application/json`.

## Correrla localmente

Requiere Go 1.27.3 o más nuevo.

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
