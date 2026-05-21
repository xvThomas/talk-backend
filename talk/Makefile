APP_NAME     := talk-cli
BINARY       := bin/$(APP_NAME)
CMD          := ./cmd/cli
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS      := -ldflags "-X github.com/xvThomas/LLMClientWrapper/talk-libs/version.Version=$(VERSION)"

.PHONY: all build run test cover vet clean help

COLOR_RESET  = \033[0m
COLOR_BOLD   = \033[1m
COLOR_GREEN  = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE   = \033[34m
CHECK        = [ok]

ECHO = printf '%b\n'

.DEFAULT_GOAL := help

all: vet test build ## Run vet, test and build

build: ## Build the CLI binary
	@$(ECHO) "$(COLOR_YELLOW)Building $(APP_NAME)...$(COLOR_RESET)"
	go build $(LDFLAGS) -o $(BINARY) $(CMD)
	@$(ECHO) "$(COLOR_GREEN)$(CHECK) Build successful: $(BINARY)$(COLOR_RESET)"

run: ## Run the CLI (MODEL=haiku-4.5)
	go run $(LDFLAGS) $(CMD) --model $(or $(MODEL),haiku-4.5) --system-file $(or $(SYSTEM_FILE),./system_prompt.md)

test: ## Run tests
	@$(ECHO) "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	go test ./...
	@$(ECHO) "$(COLOR_GREEN)$(CHECK) Tests passed$(COLOR_RESET)"

cover: ## Run tests with coverage
	@$(ECHO) "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@$(ECHO) "$(COLOR_GREEN)$(CHECK) Coverage done$(COLOR_RESET)"

vet: ## Run go vet
	@$(ECHO) "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	go vet ./...
	@$(ECHO) "$(COLOR_GREEN)$(CHECK) Vet passed$(COLOR_RESET)"

clean: ## Remove build artifacts
	rm -rf bin coverage.out

help: ## Show help
	@$(ECHO) "$(COLOR_BOLD)talk$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
