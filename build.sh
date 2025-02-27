#!/bin/bash
set -e

echo "Building AgentFlux..."
cd /Users/vtriple/agentflux
mkdir -p build
go build -o build/agentflux ./cmd/agentflux

echo "Testing compilation of standalone file..."
go run test_compile.go

echo "Running a simple test..."
go test -v ./pkg/common/fileutils

echo "Listing the build directory..."
ls -la build/
