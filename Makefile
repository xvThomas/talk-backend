BINARY-CLI   := bin/talk-cli
CMD      := ./cmd/cli
MODEL    ?= haiku-4.5
SYSTEM_FILE := ./system_prompt.md
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X talks/internal/version.Version=$(VERSION)"

.PHONY: all build run test cover cover-summary vet clean help

.DEFAULT_GOAL := help

all: vet test cli-build ## Run vet, test and build

cli-build: ## Build the CLI binary
	go build $(LDFLAGS) -o $(BINARY-CLI) $(CMD)

cli-run: ## Run the CLI (MODEL=haiku-4.5, SYSTEM_FILE=./system_prompt.md)
	go run $(LDFLAGS) $(CMD) --model $(MODEL) --system-file $(SYSTEM_FILE)

owm-build: ## Build the OWM MCP server binary
	go build -o bin/mcp-owm cmd/mcp/owm/main.go

owm-run: ## Run the OWM MCP server
	go run cmd/mcp/owm/main.go

owm-dev: ## Run the OWM MCP server with hot-reload (air)
	air -c ./cmd/mcp/owm/air.toml

test: ## Run all tests
	go test ./...

cover: ## Run tests with coverage and open HTML report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

cover-summary: ## Run tests with coverage summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -rf bin coverage.out coverage.html

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
