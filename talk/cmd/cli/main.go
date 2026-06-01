package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/config"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/llm/router"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/mcp"
	sqlitestore "github.com/xvThomas/LLMClientWrapper/talk/internal/memory/sqlite"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/prompt"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/usage"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		modelFlag      string
		systemFileFlag string
	)

	cmd := &cobra.Command{
		Use:   "talk-cli",
		Short: "Interactive LLM conversation session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), modelFlag, systemFileFlag)
		},
	}

	cmd.Flags().StringVar(&modelFlag, "model", "", "Model alias to use (e.g. sonnet-4.6, devstral)")
	cmd.Flags().StringVar(&systemFileFlag, "system-file", defaultSystemPromptPath(), "Path to a Markdown system prompt file")

	_ = cmd.MarkFlagRequired("model")

	return cmd
}

func run(ctx context.Context, modelAlias, systemFile string) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	r := router.NewLLMRouter(cfg)
	client, err := r.Get(domain.Model(modelAlias))
	if err != nil {
		return err
	}

	modelDescriptor, err := domain.Lookup(domain.Model(modelAlias))
	if err != nil {
		return err
	}

	pp := buildPromptProvider(systemFile)

	sessionID := domain.GenerateSessionID()
	const userID = "anonymous"

	dbPath := storeDBPath()
	store, err := sqlitestore.NewStore(dbPath, sessionID, userID)
	if err != nil {
		return fmt.Errorf("opening session store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// MCP server registry and manager.
	mcpRegistry, err := mcp.NewSQLiteRegistry(store.DB())
	if err != nil {
		return fmt.Errorf("initializing mcp registry: %w", err)
	}
	mcpManager := mcp.NewManager(mcpRegistry)
	mcpManager.ConnectAll(ctx)
	defer mcpManager.Close()

	// Create usage reporters based on configuration
	var reporters []domain.UsageReporter

	// Console reporter if enabled (default: true for backward compatibility)
	if cfg.ConsoleUsageReporter {
		reporters = append(reporters, &usage.ConsoleUsageReporter{})
	}

	// Langfuse reporter if both keys are present
	if cfg.LangfuseSecretKey != "" && cfg.LangfusePublicKey != "" {
		langfuseConfig := usage.LangfuseConfig{
			PublicKey: cfg.LangfusePublicKey,
			SecretKey: cfg.LangfuseSecretKey,
			BaseURL:   cfg.LangfuseBaseURL,
		}
		langfuseReporter := usage.NewLangfuseUsageReporter(langfuseConfig)
		reporters = append(reporters, langfuseReporter)
	}

	// Ensure at least one reporter is active
	if len(reporters) == 0 {
		reporters = append(reporters, &usage.ConsoleUsageReporter{})
	}

	manager := domain.NewConversationManager(
		client,
		modelAlias,
		modelDescriptor.OLTPProvider,
		store,
		store,
		pp,
		mcpManager.Tools,
		reporters,
		cfg.ToolsMaxConcurrent,
		cfg.ContextFullTurns,
	)

	history := NewHistory(historyFilePath())
	lr := NewLineReader(history)

	app := &App{
		Printer:      stdPrinter{},
		Router:       r,
		Manager:      manager,
		Store:        store,
		PP:           pp,
		MCPManager:   mcpManager,
		MCPRegistry:  mcpRegistry,
		CurrentModel: modelAlias,
		LR:           lr,
	}

	app.Printf("%s%s\n", cyan(bold+"Session started."+reset), faint(" "+version.Version))
	app.cmdHelp()
	for {
		app.Println()
		input, err := lr.ReadLine(green(bold+"You"+reset+":") + " ")
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			app.handleSlashCommand(ctx, input)
			continue
		}
		history.Add(input)

		answer, err := manager.Chat(ctx, input)
		if err != nil {
			app.Printf("\n%s %s\n", red("Error:"), err.Error())
			continue
		}

		app.Printf("\n%s %s\n", cyan(bold+"Assistant"+reset+":"), answer)
	}

	app.Println("\n" + faint("Session ended."))
	return nil
}

func buildPromptProvider(systemFile string) domain.PromptProvider {
	return prompt.NewFileProvider(systemFile)
}

func defaultSystemPromptPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "system_prompt.md"
	}
	return filepath.Join(filepath.Dir(exe), "system_prompt.md")
}

func historyFilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".talks_history")
	}
	return ".talks_history"
}

func storeDBPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".talk")
		_ = os.MkdirAll(dir, 0o700)
		return filepath.Join(dir, "talk.db")
	}
	return "talk.db"
}
