GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

# Define all services
SERVICES := auth-service message-service chat-api presence-service media-service

# Proto file detection
ifeq ($(GOHOSTOS), windows)
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	SHARED_PROTO_FILES=$(shell $(Git_Bash) -c "find shared/proto -name *.proto")
else
	SHARED_PROTO_FILES=$(shell find shared/proto -name *.proto 2>/dev/null || echo "")
endif

.PHONY: init
# init env - install required tools
init:
	@echo "🔧 Installing required tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest
	@echo "✅ Tools installed successfully!"

.PHONY: generate-shared-proto
# generate shared proto files
generate-shared-proto:
	@echo "🔨 Generating shared proto files..."
	@if [ -n "$(SHARED_PROTO_FILES)" ]; then \
		protoc --proto_path=shared/proto \
		       --proto_path=third_party \
		       --go_out=paths=source_relative:shared/proto \
		       --go-grpc_out=paths=source_relative:shared/proto \
		       $(SHARED_PROTO_FILES); \
		echo "✅ Shared proto files generated!"; \
	else \
		echo "ℹ️  No shared proto files found"; \
	fi

.PHONY: generate-service-protos
# generate proto files for all services
generate-service-protos:
	@echo "🔨 Generating service proto files..."
	@for service in $(SERVICES); do \
		if [ -d $$service ]; then \
			echo "Generating protos for $$service..."; \
			$(MAKE) -C $$service api config || echo "Warning: Failed to generate protos for $$service"; \
		else \
			echo "Warning: $$service directory not found"; \
		fi \
	done
	@echo "✅ Service proto files generated!"

.PHONY: wire
# generate wire files for all services
wire:
	@echo "🔌 Generating wire dependency injection..."
	@for service in $(SERVICES); do \
		if [ -d $$service/cmd/$$service ]; then \
			echo "Generating wire for $$service..."; \
			cd $$service/cmd/$$service && wire || echo "Warning: Wire failed for $$service"; \
			cd - > /dev/null; \
		else \
			echo "Warning: $$service/cmd/$$service directory not found"; \
		fi \
	done
	@echo "✅ Wire files generated!"

.PHONY: generate
# generate all code (proto + wire)
generate: generate-shared-proto generate-service-protos wire
	@echo "🚀 Running go generate..."
	go generate ./...
	go mod tidy
	@echo "✅ All code generation completed!"

.PHONY: build
# build all services
build:
	@echo "🏗️  Building all services..."
	mkdir -p bin/
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/$$service ./$$service/cmd/$$service || exit 1; \
	done
	@echo "✅ All services built successfully!"

.PHONY: build-%
# build specific service (e.g., make build-auth-service)
build-%:
	@echo "🏗️  Building $*..."
	mkdir -p bin/
	go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/$* ./$*/cmd/$*

.PHONY: run-%
# run specific service (e.g., make run-auth-service)
run-%:
	@echo "🚀 Running $*..."
	go run ./$*/cmd/$* -conf ./$*/configs

.PHONY: test
# run tests for all services
test:
	@echo "🧪 Running tests..."
	go test -v ./...

.PHONY: test-%
# run tests for specific service (e.g., make test-auth-service)
test-%:
	@echo "🧪 Running tests for $*..."
	go test -v ./$*/...

.PHONY: clean
# clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	@for service in $(SERVICES); do \
		find $$service -name "*.pb.go" -delete 2>/dev/null || true; \
		find $$service -name "*_grpc.pb.go" -delete 2>/dev/null || true; \
		find $$service -name "*.pb.gw.go" -delete 2>/dev/null || true; \
		find $$service -name "wire_gen.go" -delete 2>/dev/null || true; \
	done
	@echo "✅ Cleaned successfully!"

.PHONY: dev-setup
# setup development environment
dev-setup:
	@echo "🚀 Setting up development environment..."
	@if [ -f scripts/dev-setup.sh ]; then \
		chmod +x scripts/dev-setup.sh && ./scripts/dev-setup.sh; \
	else \
		echo "❌ scripts/dev-setup.sh not found"; \
		exit 1; \
	fi

.PHONY: dev-down
# stop development environment
dev-down:
	@echo "🛑 Stopping development environment..."
	docker-compose -f docker-compose.dev.yml down

.PHONY: dev-logs
# show development environment logs
dev-logs:
	docker-compose -f docker-compose.dev.yml logs -f

.PHONY: lint
# run linter
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "❌ golangci-lint not installed. Install it with: brew install golangci-lint"; \
	fi

.PHONY: fmt
# format code
fmt:
	@echo "✨ Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "ℹ️  Install goimports for better formatting: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

.PHONY: deps
# download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	go mod download
	go mod tidy

.PHONY: all
# generate all code and build
all: generate build
	@echo "🎉 Everything is ready!"

.PHONY: help
# show help
help:
	@echo ''
	@echo 'Orbit Messenger Chat App - Available Commands:'
	@echo ''
	@echo 'Setup & Development:'
	@echo '  make init              Install required tools'
	@echo '  make dev-setup         Start development infrastructure'
	@echo '  make dev-down          Stop development infrastructure'
	@echo '  make dev-logs          Show development logs'
	@echo ''
	@echo 'Code Generation:'
	@echo '  make generate          Generate all proto files and wire code'
	@echo '  make generate-shared-proto    Generate shared proto files'
	@echo '  make generate-service-protos  Generate service proto files'
	@echo '  make wire              Generate wire dependency injection'
	@echo ''
	@echo 'Building:'
	@echo '  make build             Build all services'
	@echo '  make build-<service>   Build specific service'
	@echo '  make all               Generate and build everything'
	@echo ''
	@echo 'Running:'
	@echo '  make run-<service>     Run specific service'
	@echo ''
	@echo 'Testing:'
	@echo '  make test              Run all tests'
	@echo '  make test-<service>    Run tests for specific service'
	@echo ''
	@echo 'Code Quality:'
	@echo '  make fmt               Format code'
	@echo '  make lint              Run linter'
	@echo '  make deps              Download and tidy dependencies'
	@echo ''
	@echo 'Utilities:'
	@echo '  make clean             Clean build artifacts'
	@echo ''
	@echo 'Available services: $(SERVICES)'
	@echo ''

.DEFAULT_GOAL := help