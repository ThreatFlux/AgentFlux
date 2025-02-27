# AgentFlux Project Summary

## Project Overview

AgentFlux is a high-performance file system scanning tool that computes file hashes, extracts strings, and sends results to an API backend. It's designed for efficient processing of large file systems with automatic deduplication, parallel processing, and secure communication.

## Key Features

- **High-Performance Scanning**: Parallel processing with configurable worker count
- **Multiple Hash Algorithms**: Support for MD5, SHA1, SHA256, and SHA512
- **String Extraction**: Optional extraction of strings from binary files
- **Deduplication**: Automatic filtering of duplicate files
- **Flexible Filtering**: Configurable exclusion patterns and file size limits
- **API Integration**: Secure communication with customizable authentication
- **Container Support**: Docker integration with security best practices

## Project Structure

The project follows a clean, modular architecture:

```
agentflux/
├── cmd/                  # Application entrypoints
│   └── agentflux/        # Main application
├── pkg/                  # Core packages
│   ├── api/              # API client
│   ├── common/           # Shared utilities
│   │   ├── config/       # Configuration handling
│   │   ├── fileutils/    # File manipulation utilities
│   │   ├── logging/      # Logging system
│   │   └── pathutils/    # Path manipulation utilities
│   ├── dedup/            # Deduplication engine
│   ├── integration/      # Integration tests
│   ├── processor/        # File processing and hashing
│   └── scanner/          # File system scanning
├── scripts/              # Utility scripts
└── testdata/            # Test data files
```

## Development Workflow

The project uses a modern development workflow:

1. **Local Development**:
   - Use `make build` to build the application
   - Run tests with `make test`
   - Format code with `make fmt`
   - Run linters with `make lint`

2. **Containerized Development**:
   - Use Docker Compose for local development
   - Run `make docker-dev-build` to build the development container
   - Use `make docker-tests` to run tests in a container

3. **Integration Testing**:
   - Run `make test-integration` for integration tests
   - Use `scripts/integration-test.sh` for more comprehensive testing

4. **Deployment**:
   - Use `make docker-build` to build the production container
   - Run `make docker-push` to push to a registry

## Security Features

Security is a top priority for AgentFlux:

1. **Code Security**:
   - Input validation for all user-supplied data
   - Safe file handling with size limits
   - Proper error handling and logging

2. **Container Security**:
   - Multi-stage builds to minimize attack surface
   - Non-root user execution
   - Capability restrictions
   - Software Bill of Materials (SBOM) generation

3. **API Security**:
   - Multiple authentication methods
   - Secure credential handling
   - TLS configuration
   - Rate limiting and retries

## Testing Strategy

The project has a comprehensive testing strategy:

1. **Unit Tests**: Test individual components
2. **Integration Tests**: Test component interactions
3. **End-to-End Tests**: Test the full processing pipeline
4. **Benchmark Tests**: Measure performance

## CI/CD Pipeline

Continuous Integration and Deployment is handled through GitHub Actions:

1. **PR Validation**: Run tests, linters, and security scans on pull requests
2. **Build**: Build binaries and containers on merge to main
3. **Release**: Create releases and push containers on tag
4. **Security**: Regular scanning of dependencies and code

## Documentation

The project includes comprehensive documentation:

1. **User Documentation**: README with usage instructions
2. **Developer Documentation**: DEVELOPMENT.md with setup guide
3. **Contribution Guidelines**: CONTRIBUTING.md and CODE_OF_CONDUCT.md
4. **Code Documentation**: Package-level and function-level comments

## Getting Started

To get started with AgentFlux:

1. **Clone the repository**:
   ```bash
   git clone https://github.com/vtriple/agentflux.git
   cd agentflux
   ```

2. **Build the application**:
   ```bash
   make build
   ```

3. **Run AgentFlux**:
   ```bash
   ./build/agentflux --paths=/path/to/scan --api=https://api.example.com --token=your-token
   ```

4. **Using Docker**:
   ```bash
   docker run -v /path/to/scan:/data vtriple/agentflux --paths=/data --api=https://api.example.com --token=your-token
   ```

## License

AgentFlux is released under the MIT License.
