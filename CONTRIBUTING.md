# Contributing to AgentFlux

Thank you for your interest in contributing to AgentFlux! We welcome contributions from everyone, whether it's in the form of bug reports, feature requests, documentation improvements, or code changes.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally
   ```bash
   git clone https://github.com/yourusername/agentflux.git
   cd agentflux
   ```
3. **Set up the development environment**
   ```bash
   make install-tools  # Install required tools
   ```
4. **Create a branch** for your changes
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Workflow

1. **Make your changes**: Implement your feature or bug fix
2. **Add tests**: Add tests for your changes to ensure they work correctly and won't break in the future
3. **Run tests**: Ensure all tests pass
   ```bash
   make test
   ```
4. **Run linters**: Make sure your code follows our coding standards
   ```bash
   make lint
   ```
5. **Run security checks**: Verify there are no security issues
   ```bash
   make security
   ```
6. **Format your code**: Ensure your code is properly formatted
   ```bash
   make fmt
   ```
7. **Build the project**: Make sure everything builds correctly
   ```bash
   make build
   ```

## Pull Request Process

1. **Update the documentation**: If your changes require documentation updates, make sure to include them
2. **Update the tests**: Add or modify tests as necessary
3. **Commit your changes**: Use clear commit messages that explain the changes you've made
4. **Push to your fork**: Push your changes to your GitHub fork
   ```bash
   git push origin feature/your-feature-name
   ```
5. **Create a Pull Request**: Open a PR against the `main` branch of the original repository
6. **Code Review**: Address any feedback from the code review process
7. **Merge**: Once your PR is approved, it will be merged into the main branch

## Coding Standards

We follow standard Go coding practices:

1. **Go Format**: All code must be formatted with `gofmt`
2. **Go Lint**: All code must pass linting checks
3. **Error Handling**: Properly handle and return errors
4. **Documentation**: All exported functions, types, and variables must be documented
5. **Testing**: All new code should have accompanying tests

## Testing

When adding new features or fixing bugs, please include tests that cover your changes. Follow these guidelines for testing:

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test interactions between components
- **Test Coverage**: Aim for high test coverage for all code paths
- **Test Edge Cases**: Ensure your tests handle edge cases and error conditions

Run tests using:
```bash
make test
```

Generate a coverage report using:
```bash
make coverage
```

## Documentation

Documentation is crucial for the project. When making changes, please:

1. **Update README.md**: If your changes affect usage or installation
2. **Update Code Comments**: Ensure all exported functions, types, and variables are documented
3. **Examples**: Add examples for new features
4. **Update DEVELOPMENT.md**: If your changes affect the development process

Thank you for contributing to AgentFlux!
