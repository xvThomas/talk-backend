package main

import (
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/mcp"
)

// ModelSwitcher builds LLM clients by model alias.
type ModelSwitcher interface {
	Get(model domain.Model) (domain.LlmClient, error)
}

// App holds the shared session state for the CLI application.
type App struct {
	Printer
	Router       ModelSwitcher
	Manager      *domain.ConversationManager
	Scope        domain.SessionScope
	Messages     domain.MessageStore
	Sessions     domain.SessionBrowser
	PP           domain.PromptProvider
	MCPManager   *mcp.Manager
	MCPRegistry  mcp.Registry
	CurrentModel string
	LR           Reader
}
