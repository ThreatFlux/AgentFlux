# AgentFlux Development Guide

This document contains information for developers working on the AgentFlux project.

## Project Structure

The project follows standard Go project layout:

- `cmd/agentflux/`: Main application entrypoint
- `pkg/`: Core packages
  - `api/`: API client for sending results
  - `common/`: Shared utilities
    - `config/`: Configuration handling
    - `logging/`: Logging system
  - `dedup/`: Deduplication engine
  - `processor/`: File processing and hashing
  - `scanner/`: File system scanning
- `scripts/`: Development and testing scripts
- `testdata/`: Sample data for testing

## Development Setup

### Prerequisites

- Go 1.24.0+
- Docker (for containerized development)
- Make

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/vtriple/agentflux.git
   cd agentflux
   ```

2. Run tests:
   ```bash
   make test
   ```

3. Build the application:
   ```bash
   make build
   ```

4. Run the application:
   ```bash
   ./build/agentflux --paths=./testdata --api=https://api.example.com --token=test-token
   ```

### Docker Development

For development in a containerized environment:

1. Build the development container:
   ```bash
   make docker-dev-build
   ```

2. Run tests in the container:
   ```bash
   make docker-tests
   ```

3. Format code in the container:
   ```bash
   make docker-fmt
   ```

4. Run all checks in the container:
   ```bash
   make docker-all
   ```

## Testing

### Unit Tests

Run unit tests with:
```bash
make test
```

Generate a coverage report with:
```bash
make coverage
```

### Integration Testing

For quick integration testing, use the provided script:
```bash
./scripts/test.sh
```

This script creates test files and runs the application against them with a mock API server.

### Benchmarking

To benchmark the application with different configurations:
```bash
./scripts/benchmark.sh
```

## Building and Releasing

### Local Build

```bash
make build
```

### Docker Build

```bash
make docker-build
```

### Release Process

1. Update version in the code
2. Create and push a tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. The CI workflow will automatically:
   - Build the binary
   - Create a Docker image
   - Create a GitHub release
   - Push the Docker image to the registry

## Code Standards

- Follow Go best practices and style guides
- Ensure all code passes `golangci-lint`
- Write tests for all new functionality
- Document all exported functions, types, and constants
- Keep dependencies to a minimum
