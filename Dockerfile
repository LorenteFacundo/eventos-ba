FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.versionBuild=${VERSION}" -o /out/eventos ./cmd/eventos


FROM alpine:3.22

RUN apk add --no-cache ca-certificates su-exec \
 && addgroup -S eventos \
 && adduser -S -G eventos -H -s /sbin/nologin eventos

COPY --from=build /out/eventos /usr/local/bin/eventos
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

ENV EVENTOS_DB=/data/eventos.db \
    PORT=8080
EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["eventos", "serve"]
