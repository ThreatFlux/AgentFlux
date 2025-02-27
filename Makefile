# Required versions
REQUIRED_GO_VERSION = 1.24.0
REQUIRED_DOCKER_VERSION = 24.0.0

# Tool paths and versions
GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOSEC ?= gosec
GOVULNCHECK ?= govulncheck
DOCKER ?= docker
COSIGN ?= cosign
SYFT ?= syft
NANCY ?= nancy

# Version information
VERSION ?= $(shell git describe --tags --always || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Build flags
BUILD_FLAGS ?= -v
TEST_FLAGS ?= -v -race -cover
LINT_FLAGS ?= run --timeout=5m

# Coverage output paths
COVERAGE_PROFILE = coverage.out
COVERAGE_HTML = coverage.html

# Binary information
BINARY_NAME = agentflux
BINARY_PATH = build/$(BINARY_NAME)

# Docker information
DOCKER_REGISTRY ?= vtriple
DOCKER_IMAGE = $(DOCKER_REGISTRY)/$(BINARY_NAME)
DOCKER_TAG ?= $(VERSION)
DOCKER_LATEST = $(DOCKER_IMAGE):latest
DOCKER_DEV_IMAGE = $(DOCKER_REGISTRY)/go-dev
DOCKER_TEST_IMAGE = $(DOCKER_REGISTRY)/$(BINARY_NAME)-test

# LDFLAGS for binary
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)"

.PHONY: all build test test-integration test-benchmark lint clean docker-build check-versions install-tools security help version-info coverage docker-push docker-sign docker-verify install docker-run fmt docker-test docker-tests docker-dev-build docker-fmt docker-lint docker-security docker-coverage docker-all docker-shell run setup-git-hooks release

# Default target
.DEFAULT_GOAL := help

# Version check targets
check-versions: ## Check all required tool versions
	@echo "Checking required tool versions..."
	@echo "Checking Go version..."
	@$(GO) version || (echo "Error: Go not found" && exit 1)
	@echo "Checking Docker version..."
	@$(DOCKER) --version || (echo "Warning: Docker not found" && exit 1)
	@echo "All version checks completed"

# Install required tools
install-tools: ## Install required Go tools
	@echo "Installing security and linting tools..."
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/sonatype-nexus-community/nancy@latest
	@go install github.com/sigstore/cosign/cmd/cosign@latest
	@go install github.com/anchore/syft/cmd/syft@latest

build: check-versions ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p build
	$(GO) build $(BUILD_FLAGS) $(LDFLAGS) -o $(BINARY_PATH) ./cmd/agentflux

fmt: ## Format Go source files
	@echo "Formatting Go files..."
	@find . -name "*.go" -type f -not -path "./vendor/*" -exec $(GO) fmt {} \;

lint: ## Run golangci-lint for code analysis
	@echo "Running linters..."
	$(GOLANGCI_LINT) $(LINT_FLAGS) ./...

test: ## Run unit tests with coverage
	@echo "Running tests..."
	@$(GO) test $(TEST_FLAGS) ./pkg/...

test-integration: build ## Run integration tests
	@echo "Running integration tests..."
	@$(GO) test $(TEST_FLAGS) ./pkg/integration/...

test-benchmark: build ## Run benchmark tests
	@echo "Running benchmark tests..."
	@./scripts/benchmark.sh

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@$(GO) test -coverprofile=$(COVERAGE_PROFILE) ./pkg/...
	@$(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@$(GO) tool cover -func=$(COVERAGE_PROFILE)

security: ## Run security scans
	@echo "Running security scans..."
	@$(GOSEC) ./...
	@$(GOVULNCHECK) ./...
	@go list -json -deps ./... | $(NANCY) sleuth

docker-build: check-versions ## Build Docker image
	@echo "Building Docker image..."
	@$(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_LATEST) \
		.

docker-test-build: ## Build Docker test image
	@echo "Building Docker test image..."
	@$(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_TEST_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_TEST_IMAGE):latest \
		-f Dockerfile.test \
		.

docker-sign: ## Sign Docker image with cosign
	@echo "Signing Docker image..."
	@$(COSIGN) sign --key cosign.key $(DOCKER_IMAGE):$(DOCKER_TAG)
	@$(COSIGN) sign --key cosign.key $(DOCKER_LATEST)

docker-test: ## Test Docker image capabilities
	@echo "Testing Docker image..."
	@$(DOCKER) run \
		--cap-drop=ALL \
		$(DOCKER_IMAGE):$(DOCKER_TAG) --version

docker-verify: ## Verify Docker image signature
	@echo "Verifying Docker image signature..."
	@$(COSIGN) verify --key cosign.pub $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run: ## Run Docker container with security options
	@echo "Running Docker container with security options..."
	@$(DOCKER) run \
		--cap-drop=ALL \
		-v $(PWD)/data:/data \
		$(DOCKER_IMAGE):$(DOCKER_TAG) --paths=/data --api=https://example.com/api --token=demo-token

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing Docker image..."
	@$(DOCKER) push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@$(DOCKER) push $(DOCKER_LATEST)

install: build ## Install the binary
	@echo "Installing $(BINARY_NAME)..."
	@install -m 755 $(BINARY_PATH) /usr/local/bin/$(BINARY_NAME)

clean: ## Remove build artifacts and generated files
	@echo "Cleaning all artifacts and generated files..."
	@rm -f $(BINARY_PATH)
	@rm -f $(COVERAGE_PROFILE)
	@rm -f $(COVERAGE_HTML)
	@rm -rf vendor/
	@rm -rf build/
	@rm -f *.log
	@rm -f *.out
	@rm -f *.test
	@rm -f *.prof
	@rm -rf dist/
	@go clean -cache -testcache -modcache -fuzzcache

run: build ## Run the application with default arguments
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_PATH) --api=https://example.com/api --token=demo-token

setup-git-hooks: ## Set up Git hooks for development
	@echo "Setting up Git hooks..."
	@chmod +x .githooks/* scripts/*.sh
	@./scripts/setup-git-hooks.sh

release: ## Create a new release
	@echo "Creating a new release..."
	@scripts/release.sh --version $(VERSION)

all: fmt test security lint build docker-build ## Run all checks and build

help: ## Display available commands
	@echo "AgentFlux - A high-performance file system scanning tool"
	@echo ""
	@echo "Available commands:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

version-info: ## Display version information
	@echo "Build Information:"
	@echo "  Version:    $(VERSION)"
	@echo "  Commit:     $(COMMIT)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo "\nRequired Versions:"
	@echo "  Go:     $(REQUIRED_GO_VERSION)+"
	@echo "  Docker: $(REQUIRED_DOCKER_VERSION)+"
	@echo "\nInstalled Versions:"
	@$(GO) version
	@$(DOCKER) --version

# Docker development environment targets
docker-dev-build: ## Build the development Docker image
	@echo "Building development Docker image..."
	@$(DOCKER) build -t $(DOCKER_DEV_IMAGE) -f Dockerfile.dev .

docker-fmt: docker-dev-build ## Format Go source files using Docker
	@echo "Formatting Go files using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) fmt

docker-lint: docker-dev-build ## Run golangci-lint for code analysis using Docker
	@echo "Running linters using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) lint

docker-security: docker-dev-build ## Run security scans using Docker
	@echo "Running security scans using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) security

docker-tests: docker-dev-build ## Run unit tests with coverage using Docker
	@echo "Running tests using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) test

docker-coverage: docker-dev-build ## Generate test coverage report using Docker
	@echo "Generating coverage report using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) coverage

docker-all: docker-dev-build ## Run all checks and tests using Docker
	@echo "Running all checks and tests using Docker..."
	@$(DOCKER) run -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) all

docker-shell: docker-dev-build ## Start a shell in the development container
	@echo "Starting shell in development container..."
	@$(DOCKER) run -it -v $(CURDIR):/workspace $(DOCKER_DEV_IMAGE) shell

docker-compose-up: ## Start all services using docker-compose
	@echo "Starting services with docker-compose..."
	@docker-compose up

docker-compose-test: ## Run integration tests using docker-compose
	@echo "Running integration tests with docker-compose..."
	@docker-compose run --rm integration
