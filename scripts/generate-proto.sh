#!/bin/bash

echo "🔨 Generating Protocol Buffer files..."

# Install protoc-gen-go if not present
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate proto files
protoc --proto_path=shared/proto \
       --go_out=. \
       --go-grpc_out=. \
       shared/proto/common/v1/*.proto

echo "✅ Protocol Buffer files generated!"