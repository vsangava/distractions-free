.PHONY: help build build-all test clean release check-release dev-install dev-uninstall

VERSION ?= dev
BINARY_NAME := sentinel
LDFLAGS := -X github.com/vsangava/sentinel/internal/version.Version=$(VERSION)

help:
	@echo "📦 Sentinel Build Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build for current OS (macOS/Linux/Windows)"
	@echo "  build-all          Build for macOS (ARM64 + x86_64) and Windows"
	@echo "  test               Run all tests"
	@echo "  release            Pre-release check: build all + test + verify binaries"
	@echo "  clean              Remove built binaries"
	@echo "  check-release      Same as 'release' (alias)"
	@echo ""
	@echo "Developer install targets:"
	@echo "  dev-install        Build and install the service locally (requires sudo)"
	@echo "  dev-uninstall      Uninstall and clean up the service (requires sudo)"
	@echo ""
	@echo "Example release workflow:"
	@echo "  make release"
	@echo "  git tag v1.0.0"
	@echo "  git push origin v1.0.0"

# Build for current OS
build:
	@echo "🔨 Building $(BINARY_NAME) for current OS (version=$(VERSION))..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/app
	@echo "✅ Built: $(BINARY_NAME)"

# Build for all platforms
build-all: clean
	@echo "🔨 Building for all platforms (version=$(VERSION))..."
	@echo ""

	@echo "  → macOS ARM64 (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-macos-arm64 ./cmd/app
	@echo "     ✅ $(BINARY_NAME)-macos-arm64"

	@echo "  → macOS x86_64 (Intel)..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-macos-amd64 ./cmd/app
	@echo "     ✅ $(BINARY_NAME)-macos-amd64"

	@echo "  → Windows x86_64..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe ./cmd/app
	@echo "     ✅ $(BINARY_NAME)-windows-amd64.exe"
	@echo ""

# Run tests
test:
	@echo "🧪 Running tests..."
	go test ./...
	@echo "✅ All tests passed"

# Clean binaries
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)-macos-arm64 $(BINARY_NAME)-macos-amd64 $(BINARY_NAME)-windows-amd64.exe $(BINARY_NAME)
	@echo "✅ Cleaned"

# Pre-release check
release: test build-all verify-binaries
	@echo ""
	@echo "✨ Pre-release check complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. git tag v<version>  (e.g., git tag v1.0.0)"
	@echo "  2. git push origin v<version>"
	@echo "  3. GitHub Actions will build and release automatically"
	@echo ""

# Alias for release
check-release: release

# Build and install service locally (developer use only)
dev-install: build
	sudo ./sentinel setup

# Uninstall and clean up the service (developer use only)
dev-uninstall:
	sudo /usr/local/bin/sentinel clean --yes

# Verify binaries exist and have content
verify-binaries:
	@echo ""
	@echo "📋 Verifying binaries..."
	@if [ ! -f "$(BINARY_NAME)-macos-arm64" ]; then echo "❌ Missing $(BINARY_NAME)-macos-arm64"; exit 1; fi
	@if [ ! -f "$(BINARY_NAME)-macos-amd64" ]; then echo "❌ Missing $(BINARY_NAME)-macos-amd64"; exit 1; fi
	@if [ ! -f "$(BINARY_NAME)-windows-amd64.exe" ]; then echo "❌ Missing $(BINARY_NAME)-windows-amd64.exe"; exit 1; fi
	@echo "  ✅ $(BINARY_NAME)-macos-arm64 ($(shell ls -lh $(BINARY_NAME)-macos-arm64 | awk '{print $$5}'))"
	@echo "  ✅ $(BINARY_NAME)-macos-amd64 ($(shell ls -lh $(BINARY_NAME)-macos-amd64 | awk '{print $$5}'))"
	@echo "  ✅ $(BINARY_NAME)-windows-amd64.exe ($(shell ls -lh $(BINARY_NAME)-windows-amd64.exe | awk '{print $$5}'))"
