package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/LorenteFacundo/eventos-ba/internal/api"
	"github.com/LorenteFacundo/eventos-ba/internal/store"
)

const largoMinimoToken = 32

// cmdServe corre la API hasta recibir SIGINT/SIGTERM. Al apagarse espera a
// que terminen los pedidos en curso.
func cmdServe(ctx context.Context, s *store.Store, args []string) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	token := os.Getenv("EVENTOS_TOKEN")
	switch {
	case token == "":
		log.Warn("EVENTOS_TOKEN no está definido, la API queda en solo lectura")
	case len(token) < largoMinimoToken:
		return fmt.Errorf("EVENTOS_TOKEN tiene que tener al menos %d caracteres (generalo con: openssl rand -hex 32)", largoMinimoToken)
	}

	direccion := direccionServidor(args)
	servidor := &http.Server{
		Addr: direccion,
		Handler: api.Handler(s, api.Config{
			Token:    token,
			HeaderIP: os.Getenv("EVENTOS_HEADER_IP"),
			Log:      log,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errServidor := make(chan error, 1)
	go func() {
		log.Info("API escuchando", "direccion", direccion)
		errServidor <- servidor.ListenAndServe()
	}()

	select {
	case err := <-errServidor:
		return err
	case <-ctx.Done():
	}

	log.Info("apagando la API")
	ctxApagado, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()

	if err := servidor.Shutdown(ctxApagado); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// direccionServidor: primero el argumento, después $PORT, y si no localhost:8080.
func direccionServidor(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	if puerto := os.Getenv("PORT"); puerto != "" {
		return ":" + puerto
	}
	return "localhost:8080"
}
