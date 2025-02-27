#!/bin/bash
set -e

# Display help message
show_help() {
  echo "ThreatFlux AgentFlux Development Environment"
  echo ""
  echo "Usage: docker run -v $(pwd):/workspace vtriple/agentflux-dev [COMMAND]"
  echo ""
  echo "Commands:"
  echo "  lint       Run golangci-lint on the project"
  echo "  security   Run security checks (gosec, govulncheck, nancy)"
  echo "  fmt        Format Go files"
  echo "  test       Run tests with race detection and coverage"
  echo "  coverage   Generate coverage reports"
  echo "  all        Run all checks and tests"
  echo "  shell      Start a shell in the container"
  echo "  help       Show this help message"
  echo ""
  echo "Example:"
  echo "  docker run -v $(pwd):/workspace vtriple/agentflux-dev all"
}

# Execute the command passed as first argument
case "$1" in
  lint)
    echo "Running linters..."
    cd /workspace && golangci-lint run --timeout=5m ./...
    ;;
  security)
    echo "Running security scans..."
    cd /workspace && gosec ./...
    cd /workspace && govulncheck ./...
    cd /workspace && go list -json -deps ./... | nancy sleuth
    ;;
  fmt)
    echo "Formatting Go files..."
    cd /workspace && find . -name "*.go" -type f -exec go fmt {} \;
    ;;
  test)
    echo "Running tests..."
    cd /workspace && go test -v -race -cover ./pkg/... ./cmd/... ./internal/... 2>/dev/null || go test -v -race -cover ./...
    ;;
  coverage)
    echo "Generating coverage report..."
    cd /workspace && go test -coverprofile=coverage.out ./...
    cd /workspace && go tool cover -html=coverage.out -o coverage.html
    cd /workspace && go tool cover -func=coverage.out
    ;;
  all)
    echo "Running all checks and tests..."
    cd /workspace && find . -name "*.go" -type f -exec go fmt {} \;
    cd /workspace && golangci-lint run --timeout=5m ./...
    cd /workspace && gosec ./...
    cd /workspace && govulncheck ./...
    cd /workspace && go list -json -deps ./... | nancy sleuth
    cd /workspace && go test -v -race -cover ./...
    ;;
  shell)
    echo "Starting shell..."
    exec /bin/bash
    ;;
  help|"")
    show_help
    ;;
  *)
    echo "Unknown command: $1"
    show_help
    exit 1
    ;;
esac
