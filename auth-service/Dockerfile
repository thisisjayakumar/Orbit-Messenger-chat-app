FROM golang:1.23 AS builder

COPY . /src
WORKDIR /src

# Build the specific service
RUN GOPROXY=https://goproxy.cn go build -ldflags "-X main.Version=docker" -o ./bin/auth-service ./auth-service/cmd/auth-service

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./auth-service", "-conf", "/data/conf"]
