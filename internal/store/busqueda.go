package store

import (
	"database/sql/driver"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"modernc.org/sqlite"
)

// simplificar está disponible en SQL para comparar texto sin importar
// mayúsculas ni tildes.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("simplificar", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			s, _ := args[0].(string)
			return simplificar(s), nil
		})
}

// simplificar pasa a minúsculas, saca las tildes y deja solo letras y
// números separados por un espacio: "¡IA en Educación!" -> "ia en educacion".
func simplificar(s string) string {
	var b strings.Builder
	// NFD separa cada letra de su tilde ("ó" -> "o" + "´"), así la tilde se descarta sola.
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// terminos separa la búsqueda en palabras ya simplificadas. Como solo quedan
// letras y números, ninguna trae comodines de LIKE (% o _).
func terminos(busqueda string) []string {
	return strings.Fields(simplificar(busqueda))
}
