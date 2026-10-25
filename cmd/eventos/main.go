// eventos carga, lista y sirve por HTTP eventos IT de Buenos Aires.
//
// Variables de entorno:
//
//	EVENTOS_DB         ruta de la base (por defecto ~/.local/share/eventos-ba/eventos.db)
//	EVENTOS_TOKEN      token para escribir por la API, mínimo 32 caracteres
//	EVENTOS_HEADER_IP  header con la IP real del cliente si hay un proxy adelante
//	PORT               puerto de la API
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
	"github.com/LorenteFacundo/eventos-ba/internal/sources"
	"github.com/LorenteFacundo/eventos-ba/internal/store"
)

const uso = `uso: eventos <comando>

comandos:
  fetch            trae eventos de las fuentes externas y los guarda
  list             muestra los próximos eventos
  import ARCHIVO   carga eventos desde un JSON (- lee de stdin)
  serve [DIR]      levanta la API (por defecto en localhost:8080)
`

type comando func(ctx context.Context, s *store.Store, args []string) error

var comandos = map[string]comando{
	"fetch":  cmdFetch,
	"list":   cmdList,
	"import": cmdImport,
	"serve":  cmdServe,
}

// errUso indica que el programa se usó mal; además del error se muestra la ayuda.
type errUso struct {
	mensaje string
}

func (e errUso) Error() string {
	return e.mensaje
}

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}

	fmt.Fprintln(os.Stderr, "error:", err)
	if errors.As(err, new(errUso)) {
		fmt.Fprint(os.Stderr, "\n"+uso)
		os.Exit(2)
	}
	os.Exit(1)
}

func run(args []string) error {
	if len(args) < 1 {
		return errUso{"falta el comando"}
	}
	cmd, ok := comandos[args[0]]
	if !ok {
		return errUso{fmt.Sprintf("comando desconocido %q", args[0])}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ruta, err := rutaBase()
	if err != nil {
		return err
	}
	s, err := store.Open(ruta)
	if err != nil {
		return err
	}
	defer s.Close()

	return cmd(ctx, s, args[1:])
}

func rutaBase() (string, error) {
	if ruta := os.Getenv("EVENTOS_DB"); ruta != "" {
		return ruta, nil
	}
	return store.RutaPorDefecto()
}

func cmdFetch(ctx context.Context, s *store.Store, _ []string) error {
	eventos, errores := sources.FetchAll(ctx)
	for _, err := range errores {
		fmt.Fprintln(os.Stderr, "aviso:", err)
	}

	// Las fuentes no pasan por la API, así que se validan acá.
	eventos, descartados := event.Validos(eventos)
	for _, err := range descartados {
		fmt.Fprintln(os.Stderr, "aviso: descarto", err)
	}

	if err := s.Guardar(ctx, eventos); err != nil {
		return err
	}
	fmt.Printf("Guardé %d eventos\n", len(eventos))
	return nil
}

func cmdList(ctx context.Context, s *store.Store, _ []string) error {
	eventos, err := s.Buscar(ctx, store.Filtro{Desde: time.Now()})
	if err != nil {
		return err
	}

	fmt.Printf("%d próximos eventos\n\n", len(eventos))
	for _, e := range eventos {
		fmt.Printf("%4d  %s  %-6s  %-10s  %s\n", e.ID, formatearFecha(e.Inicio), e.Zona, e.Fuente, e.Titulo)
	}
	return nil
}

// cmdImport se comporta igual que POST /eventos: si un evento es inválido
// no se importa ninguno.
func cmdImport(ctx context.Context, s *store.Store, args []string) error {
	if len(args) < 1 {
		return errUso{"import necesita un archivo"}
	}

	datos, err := leerArchivo(args[0])
	if err != nil {
		return err
	}

	eventos, err := event.DecodificarJSON(datos)
	if err != nil {
		return err
	}
	if err := event.Preparar(eventos); err != nil {
		return fmt.Errorf("no se importó nada:\n%w", err)
	}

	if err := s.Guardar(ctx, eventos); err != nil {
		return err
	}
	fmt.Printf("Importé %d eventos\n", len(eventos))
	return nil
}

func leerArchivo(archivo string) ([]byte, error) {
	if archivo == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(archivo)
}

// Los eventos sin hora (00:00) se muestran solo con el día.
func formatearFecha(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 {
		return t.Format("02/01") + "      "
	}
	return t.Format("02/01 15:04")
}
