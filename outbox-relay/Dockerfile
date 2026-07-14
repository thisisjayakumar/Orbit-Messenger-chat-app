FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags "-X main.Version=docker" -o ./bin/outbox-relay ./outbox-relay/cmd/outbox-relay

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        netbase \
        && rm -rf /var/lib/apt/lists/* \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app
WORKDIR /app

EXPOSE 8005
VOLUME /data/conf

CMD ["./outbox-relay"]
