#!/bin/sh
set -eu

# Los volúmenes se montan como root. Se le pasa la carpeta de la base al
# usuario eventos y se arranca la app sin privilegios.
if [ "$(id -u)" = "0" ]; then
	dir=$(dirname "$EVENTOS_DB")
	mkdir -p "$dir"
	chown -R eventos:eventos "$dir"
	exec su-exec eventos "$@"
fi

exec "$@"
