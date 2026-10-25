FROM golang:1.25.5-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/callback-server .


FROM debian:12-slim AS runtime

COPY deploy/cryptopro/*.deb /tmp/cryptopro/

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        /tmp/cryptopro/*.deb \
    && rm -rf \
        /tmp/cryptopro \
        /var/lib/apt/lists/*

# Серверная лицензия CryptoPro CSP. Без неё работает пробный период 90 дней
# с момента сборки образа.
ARG CPRO_LICENSE=""
RUN if [ -n "$CPRO_LICENSE" ]; then \
        /opt/cprocsp/sbin/amd64/cpconfig -license -set "$CPRO_LICENSE"; \
    fi

WORKDIR /app

COPY --from=builder /out/callback-server /usr/local/bin/callback-server

ENTRYPOINT ["/usr/local/bin/callback-server"]
CMD ["-config", "/app/config.json"]
