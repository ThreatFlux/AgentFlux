:
:
:
:
:∂ç
:∂ç
:∂ç
:∂ç
# AgentFlux

[![Go Report Card](https://goreportcard.com/badge/github.com/threatflux/agentflux)](https://goreportcard.com/report/github.com/threatflux/agentflux)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Warning - This is under development and not ready for use.

AgentFlux is a high-performance file system scanning tool developed by ThreatFlux that computes file hashes, extracts strings, and sends this information to an API backend. It's designed for efficient processing of large file systems with automatic deduplication, parallel processing, and secure communication.
  mm         
## Features

- **High Performance**: Parallel processing with configurable worker count
- **Multiple Hash Algorithms**: Support for MD5, SHA1, SHA256, and SHA512
- **Automatic File Deduplication**: Filter duplicates based on hash, path, or name
- **String Extraction**: Optional extraction and analysis of strings from binary files
- **Comprehensive Filtering**: Configurable exclusion patterns and file size limits
- **Secure API Communication**: Multiple authentication methods with automatic retries and batching
- **Detailed Logging**: Configurable logging system with different verbosity levels
- **Docker Support**: Production-ready Docker images with security best practices

## Installation

### From Binary

Download the latest release from our [releases page](https://github.com/threatflux/agentflux/releases).

### From Source

```bash
# Clone the repository
git clone https://github.com/threatflux/agentflux.git
cd agentflux

# Build the binary
make build
```

### Using Docker

```bash
# Pull the latest image
docker pull threatflux/agentflux:latest

# Or build locally
docker build -t threatflux/agentflux:latest .
```

## Usage

### Basic Usage

```bash
# Scan the current directory and send results to an API
./build/agentflux --paths="." --api="http://api.agent.threatflux.local:8800/results" --token="your-api-token"
```

### Scan Multiple Directories

```bash
# Scan multiple directories and exclude patterns
./build/agentflux --paths="/path1,/path2,/path3" --exclude="*.tmp,*.log,node_modules" --api="https://api.example.com/results" --token="your-api-token"
```

### Customize Processing

```bash
# Customize hash algorithm, worker count, and depth
./build/agentflux --algorithm=sha256 --workers=16 --depth=5 --max-size=50000000 --api="https://api.example.com/results" --token="your-api-token"
```

### Extract Strings

```bash
# Enable string extraction from binary files
./build/agentflux --strings --string-min=6 --api="https://api.example.com/results" --token="your-api-token"
```

### Advanced Logging

```bash
# Enable debug logging to a file
./build/agentflux --log-level=debug --log-file=./agentflux.log --api="https://api.example.com/results" --token="your-api-token"
```

### Using Docker

```bash
# Run with Docker
docker run -v /path/to/scan:/data threatflux/agentflux:latest --paths=/data --api=https://api.example.com/results --token=your-api-token
```

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `--paths` | Comma-separated list of paths to scan | `.` (current directory) |
| `--exclude` | Comma-separated list of glob patterns to exclude | (none) |
| `--algorithm` | Hash algorithm (md5, sha1, sha256, sha512) | `sha256` |
| `--workers` | Number of worker goroutines | Number of CPU cores |
| `--depth` | Maximum directory depth (-1 for unlimited) | `-1` (unlimited) |
| `--api` | API endpoint URL | (required) |
| `--token` | API authentication token | (required) |
| `--auth-method` | API auth method (bearer, basic, api-key) | `bearer` |
| `--batch` | API batch size | `100` |
| `--strings` | Extract strings from files | `false` |
| `--string-min` | Minimum string length to extract | `4` |
| `--max-size` | Maximum file size to process in bytes | `104857600` (100MB) |
| `--log-level` | Log level (debug, info, warn, error) | `info` |
| `--log-file` | Path to log file (empty for stderr) | (stderr) |
| `--version` | Show version information | `false` |

## API Integration

AgentFlux sends file processing results to an API endpoint in batches. Each result includes:

```json
[
  {
    "path": "/path/to/file.txt",
    "name": "file.txt",
    "size": 1234,
    "modTime": "2006-01-02T15:04:05Z07:00",
    "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "hashAlgorithm": "sha256",
    "mimeType": "text/plain",
    "strings": ["extracted", "strings", "from", "file"],
    "isExecutable": false,
    "processedAt": "2006-01-02T15:04:05Z07:00"
  },
  ...
]
```

### Authentication Methods

AgentFlux supports three authentication methods:

1. **Bearer Token** (default): Uses a bearer token in the Authorization header
   ```
   Authorization: Bearer your-token
   ```

2. **API Key**: Uses an API key in the X-API-Key header
   ```
   X-API-Key: your-api-key
   ```

3. **Basic Auth**: Uses HTTP Basic Authentication
   ```
   Authorization: Basic base64(username:password)
   ```

## Architecture

AgentFlux is organized into several packages:

- **api**: Handles sending results to the API endpoint
- **common**: Shared utilities for configuration and logging
- **dedup**: File deduplication functionality
- **processor**: File processing and hash computation
- **scanner**: File system scanning

The processing pipeline works as follows:

1. **Scanner**: Finds files matching criteria and sends paths to a channel
2. **Processor**: Computes hashes and extracts strings from files
3. **Deduplicator**: Filters out duplicate files based on a configurable strategy
4. **API Client**: Batches and sends results to the API endpoint

## Development

### Prerequisites

- Go 1.24.0+
- Docker (for containerized development)
- Make

### Development Commands

```bash
# Run tests
make test

# Generate code coverage
make coverage

# Run linters
make lint

# Run security checks
make security

# Format code
make fmt

# Run all checks
make all
```

### Docker Development Environment

```bash
# Build development container
make docker-dev-build

# Run specific tasks in the development container
make docker-fmt
make docker-lint
make docker-test
make docker-security

# Start a shell in the development container
make docker-shell
```

## Security Features

AgentFlux incorporates several security best practices:

- **Secure Docker Image**: Multi-stage build, non-root user, minimal dependencies
- **Supply Chain Security**: Software Bill of Materials (SBOM) generation
- **Defense in Depth**: Dropping unnecessary capabilities in Docker
- **Secure Defaults**: Conservative timeout and retry policies
- **Input Validation**: Thorough validation of command line arguments
- **Error Handling**: Proper error propagation and logging

## License

MIT
