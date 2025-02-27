#!/bin/bash
set -e

cd /Users/vtriple/agentflux

echo "Checking go.mod..."
cat go.mod

echo -e "\nChecking module imports..."
go list ./...

echo -e "\nChecking compiler..."
go build -v ./cmd/agentflux

echo -e "\nRunning basic test validation..."
go test -v ./pkg/common/fileutils
