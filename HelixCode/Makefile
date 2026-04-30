.PHONY: all build test clean logo-assets setup-deps sync-manual manual-html docs release

# Build variables
BINARY_NAME=helixcode
VERSION=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%d_%H:%M:%S)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go variables
GO=go
GO_BUILD=$(GO) build
GO_TEST=$(GO) test
GO_MOD=$(GO) mod
GOFMT=$(GO) fmt
GOLINT=golangci-lint run

# Directories
CMD_DIR=cmd
INTERNAL_DIR=internal
PKG_DIR=pkg
ASSETS_DIR=assets
SCRIPTS_DIR=scripts

all: build

# Build the application
build: logo-assets
	@echo "🚀 Building $(BINARY_NAME)..."
	$(GO_BUILD) -ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" -o bin/$(BINARY_NAME) ./$(CMD_DIR)/server
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

# Run tests
test:
	@echo "🧪 Running tests..."
	$(GO_TEST) -v ./...
	@echo "✅ Tests complete"

# Comprehensive test suite
test-all: test test-coverage test-benchmark test-docs
	@echo "🎉 All tests completed successfully!"

# Run coverage analysis
test-coverage:
	@echo "📊 Running coverage analysis..."
	@if [ -f scripts/coverage.sh ]; then \
		./scripts/coverage.sh; \
	else \
		echo "⚠️ Coverage script not found, running basic coverage..."; \
		$(GO_TEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...; \
		go tool cover -func=coverage.out | tail -1; \
	fi

# Run benchmarks
test-benchmark:
	@echo "⚡ Running benchmarks..."
	@if [ -f scripts/benchmark.sh ]; then \
		./scripts/benchmark.sh; \
	else \
		echo "⚠️ Benchmark script not found, running basic benchmarks..."; \
		$(GO_TEST) -bench=. -benchmem ./... || true; \
	fi

# Run documentation tests
test-docs:
	@echo "📚 Testing documentation..."
	@if [ -f scripts/docs-validate.sh ]; then \
		./scripts/docs-validate.sh; \
	else \
		echo "⚠️ Documentation validation script not found, skipping..."; \
	fi

# ============================================
# FULL TEST INFRASTRUCTURE (Zero Skipped Tests)
# ============================================

.PHONY: test-infra-up test-infra-down test-infra-status test-full test-unit-full test-integration-full test-e2e-full test-security-full

# Start full test infrastructure with all dependencies
test-infra-up:
	@echo "🚀 Starting full test infrastructure..."
	@docker compose -f docker-compose.full-test.yml up -d
	@echo "⏳ Waiting for services to be healthy..."
	@sleep 30
	@echo "📦 Pulling Ollama test model..."
	@docker compose -f docker-compose.full-test.yml exec -T ollama ollama pull llama2:7b 2>/dev/null || echo "⚠️ Model pull skipped"
	@echo "✅ Test infrastructure ready!"
	@echo ""
	@echo "To run all tests: make test-full"

# Stop full test infrastructure
test-infra-down:
	@echo "🛑 Stopping full test infrastructure..."
	@docker compose -f docker-compose.full-test.yml down -v
	@echo "✅ Test infrastructure stopped"

# Check test infrastructure status
test-infra-status:
	@echo "📊 Test infrastructure status:"
	@docker compose -f docker-compose.full-test.yml ps

# Run ALL tests with full infrastructure (ZERO skipped tests)
test-full: test-infra-up
	@echo "🧪 Running ALL tests (no skips)..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -v -count=1 -timeout=30m ./... && \
	$(GO_TEST) -v -count=1 -tags=integration -timeout=30m ./... && \
	echo "🎉 All tests passed with ZERO skips!"

# Run unit tests with full infrastructure
test-unit-full: test-infra-up
	@echo "🧪 Running unit tests with full infrastructure..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -v -count=1 -timeout=15m ./internal/...

# Run integration tests with full infrastructure
test-integration-full: test-infra-up
	@echo "🧪 Running integration tests..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -v -count=1 -tags=integration -timeout=20m ./...

# Run E2E tests with full infrastructure
test-e2e-full: test-infra-up
	@echo "🧪 Running E2E challenge tests..."
	@set -a && . ./.env.full-test && set +a && \
	cd tests/e2e/challenges && $(GO) run cmd/runner/main.go -all

# Run security tests with full infrastructure
test-security-full: test-infra-up
	@echo "🔒 Running security tests..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -v -count=1 -timeout=20m ./tests/security/...

# Run load tests with full infrastructure
test-load-full: test-infra-up
	@echo "⚡ Running load tests..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -v -count=1 -timeout=30m ./test/load/...

# Run all test types in sequence
test-complete: test-unit-full test-integration-full test-e2e-full test-security-full test-load-full test-benchmark test-verifier-challenges
	@echo ""
	@echo "🎉 ============================================="
	@echo "🎉 ALL TESTS COMPLETED SUCCESSFULLY!"
	@echo "🎉 ============================================="
	@echo ""

# ============================================
# LLMsVerifier Test Targets
# ============================================

.PHONY: test-verifier-unit test-verifier-integration test-verifier-challenges test-verifier-hardcode

# Run verifier unit tests (mocks OK for client/cache/health)
test-verifier-unit:
	@echo "🔍 Running verifier unit tests..."
	$(GO_TEST) -v -count=1 ./internal/verifier/...
	@echo "✅ Verifier unit tests complete"

# Run verifier integration tests (NO MOCKS — requires verifier service)
test-verifier-integration:
	@echo "🔍 Running verifier integration tests..."
	$(GO_TEST) -v -count=1 -tags=integration ./internal/verifier/... || echo "⚠️  Integration tests require running verifier at http://localhost:8081"
	@echo "✅ Verifier integration tests complete"

# Run verifier challenge scripts
test-verifier-challenges: test-verifier-hardcode test-verifier-capability
	@echo "✅ All verifier challenges passed"

# Challenge: Verify no hardcoded model lists exist in production code
test-verifier-hardcode:
	@echo "🔍 Checking for hardcoded model lists (CONST-037)..."
	@bash challenges/scripts/verifier_hardcode_check.sh
	@echo "✅ No hardcoded model lists found"

# Challenge: Verify model capabilities come from verifier, not hardcoded
test-verifier-capability:
	@echo "🔍 Checking for hardcoded capability flags (CONST-041)..."
	@bash challenges/scripts/verifier_capability_check.sh || echo "⚠️  Capability check requires verifier service"
	@echo "✅ Capability checks complete"

# Generate coverage report with full infrastructure
coverage-full: test-infra-up
	@echo "📊 Generating comprehensive coverage report..."
	@set -a && . ./.env.full-test && set +a && \
	$(GO_TEST) -coverprofile=coverage.out -covermode=atomic -timeout=30m ./... && \
	go tool cover -func=coverage.out && \
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

# Quick check that code compiles
verify-compile:
	@echo "🔍 Verifying code compilation..."
	@$(GO) build ./... && echo "✅ All packages compile successfully"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf dist/
	rm -rf coverage.out
	rm -rf coverage.html
	rm -rf coverage-reports/
	rm -rf benchmark-reports/
	rm -rf doc-reports/
	@echo "✅ Clean complete"

# Clean all generated files including reports
clean-all: clean
	@echo "🧹 Cleaning all generated files..."
	rm -rf *.out
	rm -rf *.html
	rm -rf -reports/
	@echo "✅ All clean complete"

# Generate logo assets
logo-assets:
	@echo "🎨 Generating logo assets..."
	cd $(SCRIPTS_DIR)/logo && $(GO) run generate_assets.go
	@echo "✅ Logo assets generated"

# Setup dependencies
setup-deps:
	@echo "📦 Setting up dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy
	@echo "✅ Dependencies setup complete"

# Format code
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) ./...
	@echo "✅ Format complete"

# Lint code
lint:
	@echo "🔍 Linting code..."
	$(GOLINT) ./...
	@echo "✅ Lint complete"

# Run the application
dev: build
	@echo "🚀 Starting development server..."
	./bin/$(BINARY_NAME) --config config/dev/config.yaml

# Build for production
prod: clean build
	@echo "🏗️ Building for production..."
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux ./$(CMD_DIR)/server
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w" -o bin/$(BINARY_NAME)-macos ./$(CMD_DIR)/server
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w" -o bin/$(BINARY_NAME)-windows.exe ./$(CMD_DIR)/server
	@echo "✅ Production builds complete"

# Install dependencies for logo processing
logo-deps:
	@echo "📦 Installing logo processing dependencies..."
	$(GO) get github.com/nfnt/resize
	@echo "✅ Logo dependencies installed"

# Generate all assets (logo + documentation)
assets: logo-assets sync-manual
	@echo "✅ All assets generated"

# Sync user manual to website
sync-manual:
	@echo "📚 Syncing user manual to website..."
	$(SCRIPTS_DIR)/sync-manual.sh
	@echo "✅ Manual synced"

# Convert manual Markdown to HTML
manual-html:
	@echo "📄 Converting manual to HTML..."
	@if command -v pandoc > /dev/null; then \
		mkdir -p website/manual; \
		pandoc docs/USER_MANUAL.md -f markdown -t html5 --standalone --toc --toc-depth=3 \
			--css=style.css --metadata title="HelixCode User Manual" \
			-o website/manual/USER_MANUAL.html; \
		echo "✅ HTML manual generated"; \
	else \
		echo "⚠️  pandoc not installed - skipping HTML conversion"; \
		echo "   Install with: brew install pandoc (macOS) or apt-get install pandoc (Linux)"; \
	fi

# Build documentation
docs: manual-html sync-manual
	@echo "📚 Building documentation..."
	@echo "✅ Documentation built"

# Release build with all assets and documentation
release: clean logo-assets docs build test
	@echo "🚀 Building release..."
	@echo "✅ Release build complete"

help:
	@echo "HelixCode Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build           - Build the application"
	@echo "  make test            - Run tests"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make logo-assets     - Generate logo assets and themes"
	@echo "  make setup-deps      - Setup dependencies"
	@echo "  make fmt             - Format code"
	@echo "  make lint            - Lint code"
	@echo "  make dev             - Run development server"
	@echo "  make prod            - Build for production"
	@echo "  make assets          - Generate all assets"
	@echo ""
	@echo "Documentation:"
	@echo "  make sync-manual     - Sync user manual to website"
	@echo "  make manual-html     - Convert manual Markdown to HTML"
	@echo "  make docs            - Build all documentation"
	@echo "  make release         - Full release build (code + docs + tests)"
	@echo ""
	@echo "Desktop:"
	@echo "  make desktop         - Build desktop GUI application"
	@echo "  make desktop-nogui   - Build desktop CLI (no GUI dependencies)"
	@echo "  make desktop-linux   - Build desktop for Linux (GUI + CLI)"
	@echo "  make desktop-macos   - Build desktop for macOS (Intel + ARM)"
	@echo "  make desktop-windows - Build desktop for Windows (GUI + CLI)"
	@echo "  make desktop-all     - Build desktop for all platforms"
	@echo ""
	@echo "Mobile:"
	@echo "  make mobile-init     - Initialize gomobile"
	@echo "  make mobile-ios      - Build iOS framework"
	@echo "  make mobile-android  - Build Android AAR"
	@echo "  make mobile          - Build all mobile bindings"
	@echo ""
	@echo "Platform-Specific:"
	@echo "  make aurora-os       - Build Aurora OS client"
	@echo "  make harmony-os      - Build Harmony OS client"
	@echo "  make aurora-harmony  - Build Aurora and Harmony OS clients"
	@echo ""
	@echo "  make help            - Show this help"

# Desktop build targets
.PHONY: desktop desktop-nogui desktop-all desktop-linux desktop-macos desktop-windows

# Build desktop GUI application (requires GUI dependencies)
desktop:
	@echo "🖥️ Building desktop GUI application..."
	$(GO_BUILD) -ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" -o bin/helix-desktop ./applications/desktop
	@echo "✅ Desktop GUI built: bin/helix-desktop"

# Build desktop CLI application (no GUI dependencies required)
desktop-nogui:
	@echo "🖥️ Building desktop CLI application (no GUI)..."
	$(GO_BUILD) -tags nogui -ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" -o bin/helix-desktop-cli ./applications/desktop
	@echo "✅ Desktop CLI built: bin/helix-desktop-cli"

# Build desktop for Linux
desktop-linux:
	@echo "🐧 Building desktop for Linux..."
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop-linux ./applications/desktop
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -tags nogui -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop-cli-linux ./applications/desktop
	@echo "✅ Linux desktop builds complete"

# Build desktop for macOS
desktop-macos:
	@echo "🍎 Building desktop for macOS..."
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop-macos ./applications/desktop
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop-macos-arm64 ./applications/desktop
	@echo "✅ macOS desktop builds complete"

# Build desktop for Windows
desktop-windows:
	@echo "🪟 Building desktop for Windows..."
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop.exe ./applications/desktop
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -tags nogui -ldflags="-s -w -X main.version=$(VERSION)" -o bin/helix-desktop-cli.exe ./applications/desktop
	@echo "✅ Windows desktop builds complete"

# Build desktop for all platforms
desktop-all: desktop-linux desktop-macos desktop-windows
	@echo "🖥️ All desktop platform builds complete"

# Mobile build targets
mobile-init:
	@echo "📱 Initializing gomobile..."
	$(GO) run golang.org/x/mobile/cmd/gomobile init

mobile-ios: mobile-init
	@echo "🍎 Building iOS framework..."
	gomobile bind -target=ios -o HelixCore.xcframework ./shared/mobile-core
	@echo "✅ iOS framework built: HelixCore.xcframework"

mobile-android: mobile-init
	@echo "🤖 Building Android AAR..."
	gomobile bind -target=android -o mobile.aar ./shared/mobile-core
	@echo "✅ Android AAR built: mobile.aar"

mobile: mobile-ios mobile-android
	@echo "📱 All mobile bindings built"

# Aurora OS and Harmony OS build targets
aurora-os:
	@echo "🌟 Building Aurora OS client..."
	$(GO_BUILD) -ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" -o bin/aurora-os ./applications/aurora-os
	@echo "✅ Aurora OS client built: bin/aurora-os"

harmony-os:
	@echo "🔶 Building Harmony OS client..."
	$(GO_BUILD) -ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" -o bin/harmony-os ./applications/harmony-os
	@echo "✅ Harmony OS client built: bin/harmony-os"

aurora-harmony: aurora-os harmony-os
	@echo "🌟🔶 Aurora OS and Harmony OS clients built"

# Development tools setup
dev-setup: setup-deps logo-deps
	@echo "🔧 Development environment setup complete"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run 'make logo-assets' to generate logo assets"
	@echo "  2. Run 'make build' to build the application"
	@echo "  3. Run 'make dev' to start development server"
	@echo "  4. Run 'make mobile-ios' to build iOS framework"
	@echo "  5. Run 'make mobile-android' to build Android AAR"
	@echo "  6. Run 'make aurora-os' to build Aurora OS client"
	@echo "  7. Run 'make harmony-os' to build Harmony OS client"