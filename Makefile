VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X main.versionBuild=$(VERSION)

# Versiones fijas para que el CI no se rompa solo cuando sale una nueva.
STATICCHECK = honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOVULNCHECK = golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: build run test lint docker deploy

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o eventos ./cmd/eventos

run:
	go run ./cmd/eventos serve

test:
	go vet ./cmd/... ./internal/...
	go test -race ./cmd/... ./internal/...

lint:
	test -z "$$(gofmt -l cmd internal)"
	go run $(STATICCHECK) ./cmd/... ./internal/...
	go run $(GOVULNCHECK) ./cmd/... ./internal/...

docker:
	docker build --build-arg VERSION=$(VERSION) -t eventos-ba .

deploy:
	fly deploy --ha=false --build-arg VERSION=$(VERSION)
