APP_NAME     := go-sdk-server
BINARY       := bin/$(APP_NAME)
CMD          := .
DOCKER_IMAGE := $(APP_NAME)
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS      := -ldflags "-X talks/internal/version.Version=$(VERSION)"

.PHONY: all build run dev lint clean dockerize help

COLOR_RESET  = \033[0m
COLOR_BOLD   = \033[1m
COLOR_GREEN  = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE   = \033[34m

ECHO = printf '%b\n'

.DEFAULT_GOAL := help

all: lint build ## Run lint and build

build: ## Build the binary into bin/
	@$(ECHO) "$(COLOR_YELLOW)Building $(APP_NAME)...$(COLOR_RESET)"
	go build $(LDFLAGS) -o $(BINARY) $(CMD)
	@$(ECHO) "$(COLOR_GREEN)✓ Build successful: $(BINARY)$(COLOR_RESET)"

run: ## Run the server (TRANSPORT=http, ADDR=localhost:8080)
	go run $(LDFLAGS) $(CMD) --transport $(or $(TRANSPORT),http) --addr $(or $(ADDR),localhost:8080)

dev: ## Run with hot-reload (requires air)
	@$(ECHO) "$(COLOR_YELLOW)Starting $(APP_NAME) with hot-reload...$(COLOR_RESET)"
	@if command -v air >/dev/null 2>&1; then \
		air -c .air.toml; \
	else \
		$(ECHO) "$(COLOR_YELLOW)air not installed. Install with:$(COLOR_RESET)"; \
		$(ECHO) "  go install github.com/air-verse/air@latest"; \
	fi

lint: ## Run golangci-lint
	@$(ECHO) "$(COLOR_YELLOW)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		$(ECHO) "$(COLOR_YELLOW)golangci-lint not installed. Install with:$(COLOR_RESET)"; \
		$(ECHO) "  brew install golangci-lint  # macOS"; \
		$(ECHO) "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

clean: ## Remove build artifacts
	@$(ECHO) "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf bin/
	@$(ECHO) "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

dockerize: ## Build Docker image
	@$(ECHO) "$(COLOR_YELLOW)Building Docker image $(DOCKER_IMAGE):$(VERSION)...$(COLOR_RESET)"
	docker build -f Dockerfile -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest ../../..
	@$(ECHO) "$(COLOR_GREEN)✓ Docker image built: $(DOCKER_IMAGE):$(VERSION)$(COLOR_RESET)"

help: ## Show help
	@$(ECHO) "$(COLOR_BOLD)$(APP_NAME)$(COLOR_RESET)"
	@$(ECHO) "$(COLOR_GREEN)Available commands:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-15s$(COLOR_RESET) %s\n", $$1, $$2}'
