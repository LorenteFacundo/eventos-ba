package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// Pasado este tiempo sin pedidos, el contador de una IP se descarta.
	inactividadMaxima = 3 * time.Minute
	// Tope de IPs en memoria, para que un ataque con muchas IPs no la agote.
	maxClientes = 50_000
)

// limitador aplica un token bucket por IP.
type limitador struct {
	mu             sync.Mutex
	clientes       map[string]*cliente
	tasa           rate.Limit
	rafaga         int
	ultimaLimpieza time.Time
}

type cliente struct {
	lim   *rate.Limiter
	visto time.Time
}

func nuevoLimitador(tasa rate.Limit, rafaga int) *limitador {
	return &limitador{
		clientes:       make(map[string]*cliente),
		tasa:           tasa,
		rafaga:         rafaga,
		ultimaLimpieza: time.Now(),
	}
}

func (l *limitador) permitir(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	ahora := time.Now()
	if ahora.Sub(l.ultimaLimpieza) > time.Minute {
		for ip, c := range l.clientes {
			if ahora.Sub(c.visto) > inactividadMaxima {
				delete(l.clientes, ip)
			}
		}
		l.ultimaLimpieza = ahora
	}

	c, ok := l.clientes[ip]
	if !ok {
		if len(l.clientes) >= maxClientes {
			return false
		}
		c = &cliente{lim: rate.NewLimiter(l.tasa, l.rafaga)}
		l.clientes[ip] = c
	}
	c.visto = ahora
	return c.lim.Allow()
}

// ipCliente usa el header configurado solo si la API corre detrás de un proxy
// que lo pisa (como Fly-Client-IP en Fly.io). Si no, cualquiera podría mandarlo
// con una IP inventada para esquivar el límite.
func (srv *servidor) ipCliente(r *http.Request) string {
	if srv.headerIP != "" {
		if ip := strings.TrimSpace(r.Header.Get(srv.headerIP)); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
