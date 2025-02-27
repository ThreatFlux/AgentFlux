# AgentFlux Testing Checklist

This document provides a comprehensive checklist for testing AgentFlux to ensure everything is working correctly.

## Initial Setup Testing

- [ ] Clone the repository
- [ ] Verify that all required files are present
- [ ] Make script files executable: `chmod +x scripts/*.sh entrypoint.sh dev-entrypoint.sh`
- [ ] Run `make install-tools` to install required tools

## Build Testing

- [ ] Run `make build` to build the application
- [ ] Verify that the binary is created at `build/agentflux`
- [ ] Run `make version-info` to check build information

## Unit Testing

- [ ] Run `make test` to execute unit tests
- [ ] Verify that all tests pass
- [ ] Run `make coverage` to generate and view test coverage
- [ ] Verify coverage meets acceptable thresholds (aim for >80%)

## Integration Testing

- [ ] Run `make test-integration` to execute integration tests
- [ ] Run `./scripts/integration-test.sh` for more comprehensive testing
- [ ] Verify that all integration tests pass

## Docker Testing

- [ ] Run `make docker-build` to build the Docker image
- [ ] Run `make docker-test` to verify the Docker image
- [ ] Test with Docker Compose: `docker-compose up app`
- [ ] Test the development container: `make docker-dev-build && make docker-shell`

## Functional Testing

- [ ] Scan a small directory:
  ```bash
  ./build/agentflux --paths=./testdata --api=https://httpbin.org/post --token=test-token
  ```
- [ ] Scan with string extraction:
  ```bash
  ./build/agentflux --strings --string-min=4 --paths=./testdata --api=https://httpbin.org/post --token=test-token
  ```
- [ ] Test with exclusion patterns:
  ```bash
  ./build/agentflux --exclude="*.bin" --paths=./testdata --api=https://httpbin.org/post --token=test-token
  ```
- [ ] Test with depth limitation:
  ```bash
  ./build/agentflux --depth=1 --paths=./testdata --api=https://httpbin.org/post --token=test-token
  ```
- [ ] Test different hash algorithms:
  ```bash
  ./build/agentflux --algorithm=md5 --paths=./testdata --api=https://httpbin.org/post --token=test-token
  ```

## Performance Testing

- [ ] Run `./scripts/benchmark.sh` to perform benchmark tests
- [ ] Test with a larger dataset:
  ```bash
  ./build/agentflux --paths=/path/to/large/dir --api=https://httpbin.org/post --token=test-token
  ```
- [ ] Test with different worker counts to find optimal performance:
  ```bash
  ./build/agentflux --workers=4 --paths=/path/to/large/dir --api=https://httpbin.org/post --token=test-token
  ```

## Security Testing

- [ ] Verify that the application handles invalid inputs gracefully
- [ ] Test with files of various sizes to ensure proper handling
- [ ] Run security scanning tools:
  ```bash
  make security
  ```
- [ ] Test Docker image security:
  ```bash
  docker run --cap-drop=ALL -v $(pwd)/testdata:/data vtriple/agentflux:latest --paths=/data --api=https://httpbin.org/post --token=test-token
  ```

## Environment Testing

- [ ] Test on Linux systems
- [ ] Test on macOS systems
- [ ] Test on Windows systems (if applicable)
- [ ] Test with different Go versions (minimum 1.24)

## Release Testing

- [ ] Test the release script with dry run:
  ```bash
  ./scripts/release.sh --version 1.0.0 --dry-run
  ```
- [ ] Verify GitHub Actions workflow with a test PR

## Troubleshooting Common Issues

If you encounter issues during testing, check the following:

1. **Build failures**:
   - Ensure Go 1.24+ is installed
   - Check for syntax errors in the code
   - Verify that all dependencies are available

2. **Test failures**:
   - Check test logs for specific error messages
   - Verify that test dependencies are installed
   - Ensure test data is correctly set up

3. **Docker issues**:
   - Verify Docker is installed and running
   - Check Docker version compatibility
   - Ensure proper file permissions for mounted volumes

4. **API connectivity**:
   - Verify network connectivity to the API endpoint
   - Check authentication credentials
   - Look for timeout or connection refused errors

## Reporting Test Results

When reporting test results, include:

1. Operating system version
2. Go version
3. Command executed
4. Error messages or logs
5. Steps to reproduce

This helps in diagnosing and fixing issues more effectively.
