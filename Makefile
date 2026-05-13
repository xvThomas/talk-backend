BINARY-CLI   := bin/talk-cli
CMD      := ./cmd/cli
MODEL    ?= haiku-4.5
SYSTEM_FILE := ./system_prompt.md
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X talks/internal/version.Version=$(VERSION)"

.PHONY: all build run test cover cover-summary vet clean help

COLOR_RESET = \033[0m
COLOR_BOLD = \033[1m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE = \033[34m

ECHO = printf '%b\n'

.DEFAULT_GOAL := help

all: vet test cli-build ## Run vet, test and build

cli-build: ## Build the CLI binary
	@$(ECHO) "$(COLOR_YELLOW)Building CLI binary...$(COLOR_RESET)"
	go build $(LDFLAGS) -o $(BINARY-CLI) $(CMD)
	@$(ECHO) "$(COLOR_GREEN)✓ Build successful$(COLOR_RESET)"

cli-run: ## Run the CLI (MODEL=haiku-4.5, SYSTEM_FILE=./system_prompt.md)
	go run $(LDFLAGS) $(CMD) --model $(MODEL) --system-file $(SYSTEM_FILE)

owm-build: ## Build the OWM MCP server binary
	go build -o bin/mcp-owm cmd/mcp/owm/main.go

owm-run: ## Run the OWM MCP server
	go run cmd/mcp/owm/main.go

owm-dev: ## Run the OWM MCP server with hot-reload (air)
	@$(ECHO) "$(COLOR_YELLOW)Starting OWM MCP server with hot-reload...$(COLOR_RESET)"
	air -c ./cmd/mcp/owm/air.toml

lint: ## Run golangci-lint
	@$(ECHO) "$(COLOR_YELLOW)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		printf '%b\n' "$(COLOR_YELLOW)golangci-lint not installed. Install with:$(COLOR_RESET)"; \
		printf '%b\n' "  brew install golangci-lint  # macOS"; \
		printf '%b\n' "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

test: ## Run all tests
	@$(ECHO) "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	go test ./...
	@$(ECHO) "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

cover: ## Run tests with coverage and open HTML report
	@$(ECHO) "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@$(ECHO) "$(COLOR_GREEN)✓ Coverage report generated$(COLOR_RESET)"

cover-summary: ## Run tests with coverage summary
	@$(ECHO) "$(COLOR_YELLOW)Running tests with coverage summary...$(COLOR_RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@$(ECHO) "$(COLOR_GREEN)✓ Coverage summary generated$(COLOR_RESET)"

vet: ## Run go vet
	@$(ECHO) "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	go vet ./...
	@$(ECHO) "$(COLOR_GREEN)✓ go vet passed$(COLOR_RESET)"

clean: ## Remove build artifacts
	@$(ECHO) "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf bin coverage.out coverage.html
	@$(ECHO) "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

help: ## Show help
	@$(ECHO) "$(COLOR_BOLD)Talks monorepo$(COLOR_RESET)"
	@$(ECHO) "$(COLOR_GREEN)Available commands:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'