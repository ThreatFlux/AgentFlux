#!/bin/sh
set -e
cd /Users/vtriple/agentflux

# Run all tests with coverage
echo "Running tests with coverage..."
echo "Running improved tests for processor package..."
go test -coverprofile=cover.out.processor -v ./pkg/processor/... -run TestHashProcessor_ -cover
echo "Running improved tests for scanner package..."
go test -coverprofile=cover.out.scanner1 -v ./pkg/scanner/... -run TestFileScanner_ScanExtended -cover
go test -coverprofile=cover.out.scanner2 -v ./pkg/scanner/... -run TestFileScanner_ProcessFile -cover
echo "Running improved tests for dedup package..."
go test -coverprofile=cover.out.dedup1 -v ./pkg/dedup/... -run TestDeduplicationEngine_ExtendedDedupTypes -cover
go test -coverprofile=cover.out.dedup2 -v ./pkg/dedup/... -run TestDeduplicationEngine_ErrorHandling -cover
echo "Running standard tests with coverage..."
go test -coverprofile=cover.out ./pkg/...

# Display coverage summary
echo ""
echo "Coverage summary:"
go tool cover -func=cover.out

# Generate HTML coverage report
go tool cover -html=cover.out -o coverage.html
echo "HTML coverage report generated: coverage.html"
