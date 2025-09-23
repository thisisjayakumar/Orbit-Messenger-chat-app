FROM golang:1.23 AS builder

COPY . /src
WORKDIR /src

# Build the specific service
RUN GOPROXY=https://goproxy.cn go build -ldflags "-X main.Version=docker" -o ./bin/chat-api ./chat-api/cmd/chat-api

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app

WORKDIR /app

EXPOSE 8003
VOLUME /data/conf

CMD ["./chat-api", "-conf", "/data/conf"]
