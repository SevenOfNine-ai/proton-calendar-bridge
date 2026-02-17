# Proton Calendar Bridge — Makefile
# Local documentation server + Go build targets

# --- Config ---
DOCS_PORT    ?= 3300
BINARY_NAME  := proton-calendar-bridge
GO_CMD       := go
MDBOOK_CMD   := mdbook

# --- Documentation ---

.PHONY: docs docs-build docs-serve docs-clean

## Build documentation (mdBook → book/build/)
docs-build:
	$(MDBOOK_CMD) build

## Serve documentation locally on http://localhost:$(DOCS_PORT)
docs-serve:
	@echo "📖 Serving docs at http://localhost:$(DOCS_PORT)"
	$(MDBOOK_CMD) serve --port $(DOCS_PORT) --open

## Alias: serve docs
docs: docs-serve

## Clean built documentation
docs-clean:
	rm -rf book/build

# --- Go Build ---

.PHONY: build run test lint clean

## Build the binary
build:
	$(GO_CMD) build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

## Run the binary
run: build
	./bin/$(BINARY_NAME)

## Run all tests
test:
	$(GO_CMD) test ./... -v

## Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

## Clean build artifacts
clean: docs-clean
	rm -rf bin/

# --- Helpers ---

.PHONY: help

## Show this help
help:
	@echo "Proton Calendar Bridge"
	@echo ""
	@echo "Documentation:"
	@echo "  make docs          Serve docs locally (port $(DOCS_PORT))"
	@echo "  make docs-build    Build static docs"
	@echo "  make docs-clean    Remove built docs"
	@echo ""
	@echo "Development:"
	@echo "  make build         Build Go binary"
	@echo "  make run           Build and run"
	@echo "  make test          Run tests"
	@echo "  make lint          Run linter"
	@echo "  make clean         Remove all artifacts"

.DEFAULT_GOAL := help
