.PHONY: test cover vet help

COLOR_RESET  = \033[0m
COLOR_GREEN  = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE   = \033[34m
COLOR_BOLD   = \033[1m

ECHO = printf '%b\n'

.DEFAULT_GOAL := help

test: ## Run tests
	@$(ECHO) "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	go test ./...
	@$(ECHO) "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

cover: ## Run tests with coverage
	@$(ECHO) "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@$(ECHO) "$(COLOR_GREEN)✓ Coverage done$(COLOR_RESET)"

vet: ## Run go vet
	@$(ECHO) "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	go vet ./...
	@$(ECHO) "$(COLOR_GREEN)✓ Vet passed$(COLOR_RESET)"

help: ## Show help
	@$(ECHO) "$(COLOR_BOLD)talk-libs$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
