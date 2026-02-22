# Coolify Patrol Makefile
.PHONY: build test clean docker-build run help

# Build info
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -ldflags="-w -s -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GO = go

build:
	@echo "Building coolify-patrol $(VERSION)..."
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/coolify-patrol ./cmd/patrol

test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/ coverage.out

docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t coolify-patrol:$(VERSION) \
		-t coolify-patrol:latest \
		.

run: build
	@echo "Starting coolify-patrol..."
	./bin/coolify-patrol

dry-run: build
	@echo "Running dry-run check..."
	./bin/coolify-patrol --log-format text --dry-run --once

discover: build
	@echo "Discovering Coolify applications..."
	./bin/coolify-patrol discover

version: build
	./bin/coolify-patrol --version

help:
	@echo "Coolify Patrol - Available targets:"
	@echo "  build      - Build the binary"
	@echo "  test       - Run all tests"
	@echo "  clean      - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  run        - Run the application"
	@echo "  dry-run    - Run once in dry-run mode"
	@echo "  discover   - Discover Coolify applications"
	@echo "  version    - Show version"

.DEFAULT_GOAL := help