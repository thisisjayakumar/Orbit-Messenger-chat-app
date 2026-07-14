VERSION=$(shell git describe --tags --always)

.PHONY: build
# build the outbox-relay binary
build:
	@echo "🏗️  Building outbox-relay..."
	mkdir -p bin/
	go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/outbox-relay ./cmd/outbox-relay
	@echo "✅ outbox-relay built successfully!"

.PHONY: test
# run tests
test:
	@echo "🧪 Running tests..."
	go test -v -count=1 ./...

.PHONY: fmt
# format code
fmt:
	go fmt ./...

.PHONY: clean
# clean build artifacts
clean:
	rm -rf bin/

.PHONY: run
# run the service locally
run:
	go run ./cmd/outbox-relay

.DEFAULT_GOAL := help

help:
	@echo ''
	@echo 'Outbox Relay — Available Commands:'
	@echo ''
	@echo '  make build    Build the binary'
	@echo '  make test     Run tests'
	@echo '  make fmt      Format code'
	@echo '  make run      Run locally'
	@echo '  make clean    Clean build artifacts'
	@echo ''
