NAME := mdwiki
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "1.0.0")
REVISION := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date +%Y-%m-%dT%T%z)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(REVISION) -X main.date=$(BUILD_DATE)

.DEFAULT_GOAL := help

.PHONY: build
build: ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o $(NAME) .

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: lint
lint: ## Run linting
	golangci-lint run ./...

.PHONY: format
format: ## Format source code
	go fmt ./...

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(NAME)

.PHONY: help
help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
