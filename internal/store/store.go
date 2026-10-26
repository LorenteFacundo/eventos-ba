// Package store guarda los eventos en SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/LorenteFacundo/eventos-ba/internal/event"
)

var ErrNoEncontrado = errors.New("evento no encontrado")

// La url es UNIQUE: si un evento vuelve a cargarse con la misma url se actualiza.
// Las fechas van como texto RFC3339 en UTC, así el orden alfabético coincide
// con el cronológico y se pueden comparar directo en SQL. termina sale de
// Event.Termina y es lo que se usa para saber si un evento ya pasó.
//
// Cada entrada es una migración; PRAGMA user_version guarda cuántas se
// aplicaron. Para cambiar el esquema se agrega una al final, nunca se edita
// una anterior.
var migraciones = []string{`
CREATE TABLE IF NOT EXISTS eventos (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	url         TEXT NOT NULL UNIQUE,
	titulo      TEXT NOT NULL,
	inicio      TEXT NOT NULL,
	fin         TEXT,
	lugar       TEXT NOT NULL DEFAULT '',
	modalidad   TEXT NOT NULL DEFAULT 'presencial',
	fuente      TEXT NOT NULL,
	termina     TEXT NOT NULL,
	actualizado TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS eventos_inicio ON eventos(inicio);
CREATE INDEX IF NOT EXISTS eventos_termina ON eventos(termina);
`}

// Tiene que coincidir con el orden de Scan en escanear.
const columnas = `id, url, titulo, inicio, fin, lugar, modalidad, fuente`

type Store struct {
	db *sql.DB
}

// Filtro para Buscar. Los campos en cero no filtran.
type Filtro struct {
	Desde     time.Time // eventos que terminan después de esta fecha
	Hasta     time.Time // eventos que empiezan antes de esta fecha
	Texto     string    // palabras que tienen que estar en el título o el lugar
	Modalidad event.Modalidad
	Fuente    string
	Limite    int
}

// RutaPorDefecto es $XDG_DATA_HOME/eventos-ba/eventos.db
// (~/.local/share/eventos-ba/eventos.db si la variable no está).
func RutaPorDefecto() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "eventos-ba", "eventos.db"), nil
}

func Open(ruta string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o750); err != nil {
		return nil, fmt.Errorf("creando carpeta de la base: %w", err)
	}

	// WAL deja leer mientras se escribe; busy_timeout evita "database is locked"
	// cuando dos escrituras coinciden.
	dsn := "file:" + ruta + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo la base: %w", err)
	}

	if err := migrar(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrando la base: %w", err)
	}

	return &Store{db: db}, nil
}

func migrar(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migraciones); i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migraciones[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migración %d: %w", i+1, err)
		}
		// PRAGMA no acepta parámetros, pero i+1 es un entero propio.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Ping confirma que la base responde; lo usa el healthcheck.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Guardar inserta o actualiza (por url) todos los eventos en una sola
// transacción y completa el ID de cada uno.
func (s *Store) Guardar(ctx context.Context, eventos []event.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO eventos (url, titulo, inicio, fin, lugar, modalidad, fuente, termina, actualizado)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			titulo      = excluded.titulo,
			inicio      = excluded.inicio,
			fin         = excluded.fin,
			lugar       = excluded.lugar,
			modalidad   = excluded.modalidad,
			fuente      = excluded.fuente,
			termina     = excluded.termina,
			actualizado = excluded.actualizado
		RETURNING id`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ahora := aTexto(time.Now())
	for i := range eventos {
		e := &eventos[i]
		fila := stmt.QueryRowContext(ctx, e.URL, e.Titulo, aTexto(e.Inicio), aTextoONulo(e.Fin), e.Lugar, string(e.Modalidad), e.Fuente, aTexto(e.Termina()), ahora)
		if err := fila.Scan(&e.ID); err != nil {
			return fmt.Errorf("guardando %q: %w", e.Titulo, err)
		}
	}

	return tx.Commit()
}

// Buscar devuelve los eventos ordenados por inicio. Nunca devuelve nil, así
// la API responde [] en vez de null.
func (s *Store) Buscar(ctx context.Context, f Filtro) ([]event.Event, error) {
	var condiciones []string
	var args []any

	if !f.Desde.IsZero() {
		condiciones = append(condiciones, "termina > ?")
		args = append(args, aTexto(f.Desde))
	}
	if !f.Hasta.IsZero() {
		condiciones = append(condiciones, "inicio < ?")
		args = append(args, aTexto(f.Hasta))
	}
	// Cada palabra tiene que coincidir con el comienzo de alguna palabra del
	// título o del lugar: "ia" encuentra "IA aplicada" pero no "Industria".
	for _, t := range terminos(f.Texto) {
		condiciones = append(condiciones, "' ' || simplificar(titulo || ' ' || lugar) LIKE ?")
		args = append(args, "% "+t+"%")
	}
	// Los híbridos sirven tanto a quien busca presencial como a quien busca online.
	switch f.Modalidad {
	case event.Presencial, event.Online:
		condiciones = append(condiciones, "modalidad IN (?, ?)")
		args = append(args, string(f.Modalidad), string(event.Hibrido))
	case event.Hibrido:
		condiciones = append(condiciones, "modalidad = ?")
		args = append(args, string(f.Modalidad))
	}
	if f.Fuente != "" {
		condiciones = append(condiciones, "fuente = ? COLLATE NOCASE")
		args = append(args, f.Fuente)
	}

	// Solo se concatenan fragmentos fijos; los valores van siempre como parámetros.
	consulta := "SELECT " + columnas + " FROM eventos"
	if len(condiciones) > 0 {
		consulta += " WHERE " + strings.Join(condiciones, " AND ")
	}
	consulta += " ORDER BY inicio, id"
	if f.Limite > 0 {
		consulta += " LIMIT ?"
		args = append(args, f.Limite)
	}

	filas, err := s.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	eventos := []event.Event{}
	for filas.Next() {
		e, err := escanear(filas)
		if err != nil {
			return nil, err
		}
		eventos = append(eventos, e)
	}

	return eventos, filas.Err()
}

func (s *Store) Obtener(ctx context.Context, id int64) (event.Event, error) {
	fila := s.db.QueryRowContext(ctx, "SELECT "+columnas+" FROM eventos WHERE id = ?", id)
	e, err := escanear(fila)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNoEncontrado
	}
	return e, err
}

func (s *Store) Borrar(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM eventos WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNoEncontrado
	}
	return nil
}

// escaneable cubre *sql.Row y *sql.Rows.
type escaneable interface {
	Scan(dest ...any) error
}

func escanear(f escaneable) (event.Event, error) {
	var e event.Event
	var inicio, modalidad string
	var fin sql.NullString

	if err := f.Scan(&e.ID, &e.URL, &e.Titulo, &inicio, &fin, &e.Lugar, &modalidad, &e.Fuente); err != nil {
		return e, err
	}
	e.Modalidad = event.Modalidad(modalidad)

	var err error
	if e.Inicio, err = desdeTexto(inicio); err != nil {
		return e, err
	}
	if fin.Valid {
		if e.Fin, err = desdeTexto(fin.String); err != nil {
			return e, err
		}
	}

	return e, nil
}

func aTexto(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func aTextoONulo(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return aTexto(t)
}

func desdeTexto(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return t, fmt.Errorf("fecha inválida en la base %q: %w", s, err)
	}
	return t.In(event.Argentina), nil
}
