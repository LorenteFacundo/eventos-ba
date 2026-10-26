package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxDescarga = 5 << 20 // 5 MB
	userAgent   = "eventos-ba/1.0 (+https://github.com/LorenteFacundo/eventos-ba)"
)

var cliente = &http.Client{Timeout: 15 * time.Second}

func descargar(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := cliente.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondió %s", url, resp.Status)
	}

	// Se lee un byte de más para distinguir "justo cabe" de "se pasó".
	datos, err := io.ReadAll(io.LimitReader(resp.Body, maxDescarga+1))
	if err != nil {
		return nil, err
	}
	if len(datos) > maxDescarga {
		return nil, fmt.Errorf("%s supera los %d MB", url, maxDescarga>>20)
	}
	return datos, nil
}
