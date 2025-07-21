.PHONY: all build install clean
.PHONY: test test-all test-integration test-coverage test-e2e
.PHONY: lint security check
.PHONY: dev test-nginx
.PHONY: build-all release
.PHONY: generate-config verify-config
.PHONY: setup-test-cluster deploy-e2e-resources cleanup-test-cluster

# ==========================================
# Build Variables
# ==========================================
BINARY_NAME=kubectl-bootscope
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_FLAGS=-ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -X main.builtBy=make"

# Default target
all: build

# Build the binary
build:
	go build ${BUILD_FLAGS} -o bin/${BINARY_NAME} ./cmd/bootscope

# Install as kubectl plugin
install: build
	mkdir -p ~/.kube/plugins/bootscope
	cp bin/${BINARY_NAME} /usr/local/bin/
	@echo "Installed ${BINARY_NAME} to /usr/local/bin/"
	@echo "You can now use: kubectl bootscope analyze pod <pod-name>"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf dist/
	rm -f ${BINARY_NAME}
	rm -f coverage.out coverage.html
	rm -f .tmp.bootscope.toml.example

# Run unit tests only
test:
	go test -v -short -count=1 -parallel=4 ./...

# Run unit and integration tests (excludes e2e tests which require a cluster)
test-all: test test-integration

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run end-to-end tests with an already running Kubernetes cluster
test-e2e: build deploy-e2e-resources
	@echo "Running end-to-end tests..."
	./test/run-e2e-tests.sh

# Run end-to-end tests with real Kubernetes cluster (requires kind)
test-e2e-full: build setup-test-cluster deploy-e2e-resources
	@echo "Running end-to-end tests..."
	./test/run-e2e-tests.sh

# Run linting
lint:
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m
	@echo "Linting passed"

# Run security scan
security:
	@echo "Running gosec security scan..."
	gosec -fmt text -severity medium ./...
	@echo "gosec scan passed"
	@echo "Running govulncheck..."
	@command -v govulncheck >/dev/null 2>&1 || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...
	@echo "Security scans passed"

# Run all checks (tests, lint, security)
check: test lint security
	@echo "All checks passed"

# Development workflow - analyze a specific pod
dev: build
	./bin/${BINARY_NAME} analyze pod $(POD) -n $(NAMESPACE)

# Quick test with a simple nginx pod
test-nginx: build
	kubectl run nginx-test --image=nginx:latest --restart=Never || true
	sleep 2
	./bin/${BINARY_NAME} analyze pod nginx-test
	kubectl delete pod nginx-test --ignore-not-found=true

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build ${BUILD_FLAGS} -o bin/${BINARY_NAME}-linux-amd64 ./cmd/bootscope
	GOOS=linux GOARCH=arm64 go build ${BUILD_FLAGS} -o bin/${BINARY_NAME}-linux-arm64 ./cmd/bootscope
	GOOS=darwin GOARCH=amd64 go build ${BUILD_FLAGS} -o bin/${BINARY_NAME}-darwin-amd64 ./cmd/bootscope
	GOOS=darwin GOARCH=arm64 go build ${BUILD_FLAGS} -o bin/${BINARY_NAME}-darwin-arm64 ./cmd/bootscope
	GOOS=windows GOARCH=amd64 go build ${BUILD_FLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe ./cmd/bootscope
	@echo "Built binaries for all platforms in bin/"

# Create release artifacts
release: build-all
	@echo "Creating release artifacts..."
	mkdir -p dist
	# Linux
	tar -czf dist/${BINARY_NAME}-linux-amd64.tar.gz -C bin ${BINARY_NAME}-linux-amd64
	tar -czf dist/${BINARY_NAME}-linux-arm64.tar.gz -C bin ${BINARY_NAME}-linux-arm64
	# macOS
	tar -czf dist/${BINARY_NAME}-darwin-amd64.tar.gz -C bin ${BINARY_NAME}-darwin-amd64
	tar -czf dist/${BINARY_NAME}-darwin-arm64.tar.gz -C bin ${BINARY_NAME}-darwin-arm64
	# Windows
	cd bin && zip ../dist/${BINARY_NAME}-windows-amd64.zip ${BINARY_NAME}-windows-amd64.exe
	@echo "Release artifacts created in dist/"

# Generate example configuration file
generate-config:
	@echo "Generating example configuration..."
	@go run ./cmd/bootscope config generate bootscope.toml.example
	@echo "Generated bootscope.toml.example"

# Verify the example config is up-to-date
verify-config: build
	@echo "Verifying example configuration is up-to-date..."
	@./bin/${BINARY_NAME} config generate .tmp.bootscope.toml.example
	@if ! diff -q bootscope.toml.example .tmp.bootscope.toml.example > /dev/null; then \
		echo "❌ ERROR: bootscope.toml.example is out of date!"; \
		echo "Please run 'make generate-config' to update it"; \
		rm -f .tmp.bootscope.toml.example; \
		exit 1; \
	else \
		echo "✅ Example configuration is up-to-date"; \
		rm -f .tmp.bootscope.toml.example; \
	fi

# Setup test cluster using kind
setup-test-cluster:
	@echo "Creating test cluster..."
	kind create cluster --name bootscope-test || true
	kubectl config use-context kind-bootscope-test

# Deploy e2e test resources
deploy-e2e-resources:
	@echo "Deploying e2e test resources..."
	kubectl apply -f test/e2e-resources/ --validate=false

# Delete test cluster
cleanup-test-cluster:
	kind delete cluster --name bootscope-test
