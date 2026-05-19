package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/config"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/llm/router"
	inmemorystore "github.com/xvThomas/LLMClientWrapper/talk/internal/memory/inmemory"
	langfusestore "github.com/xvThomas/LLMClientWrapper/talk/internal/memory/langfuse"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/prompt"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/usage"

	"github.com/spf13/cobra"
)

// ANSI colour helpers — no external dependency required.
const (
	reset       = "\033[0m"
	bold        = "\033[1m"
	dim         = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

func cyan(s string) string      { return colorCyan + s + reset }
func green(s string) string     { return colorGreen + s + reset }
func yellow(s string) string    { return colorYellow + s + reset }
func red(s string) string       { return colorRed + s + reset }
func faint(s string) string     { return dim + s + reset }
func emphasize(s string) string { return bold + s + reset }

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
	var tools []domain.Tool // TODO: implement MCP client to connect to tool servers

	sessionID := domain.GenerateSessionID()
	const userID = "anonymous"

	var store domain.MessageStore
	if cfg.LangfuseSecretKey != "" && cfg.LangfusePublicKey != "" {
		store = langfusestore.NewLangfuseStore(sessionID, userID, langfusestore.Config{
			PublicKey: cfg.LangfusePublicKey,
			SecretKey: cfg.LangfuseSecretKey,
			BaseURL:   cfg.LangfuseBaseURL,
		})
	} else {
		store = inmemorystore.NewInMemoryStore(sessionID, userID)
	}

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
		// Fallback to console reporter if no other reporters are configured
		reporters = append(reporters, &usage.ConsoleUsageReporter{})
	}

	manager := domain.NewConversationManager(client, modelAlias, modelDescriptor.Provider, store, pp, tools, reporters, cfg.ToolsMaxConcurrent)
	currentModel := modelAlias

	fmt.Print(cyan(bold+"Session started."+reset) + faint(" "+version.Version) + `
` +
		faint(" Commands:\n") +
		faint("  /model   — switch models\n") +
		faint("  /memory  — show current session history\n") +
		faint("  /session — switch conversation session\n") +
		faint("  /prompt  — show system prompt\n") +
		faint("  /tools   — list available tools\n") +
		faint("  /q       — quit\n"))
	history := NewHistory(historyFilePath())
	lr := NewLineReader(history)
	for {
		fmt.Println()
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
			handleSlashCommand(ctx, input, r, pp, store, manager, &currentModel, lr, tools)
			continue
		}
		history.Add(input)

		answer, err := manager.Chat(ctx, input)
		if err != nil {
			fmt.Printf("\n%s %s\n", red("Error:"), err.Error())
			continue
		}

		fmt.Printf("\n%s %s\n", cyan(bold+"Assistant"+reset+":"), answer)
	}

	fmt.Println("\n" + faint("Session ended."))
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

func handleSlashCommand(ctx context.Context, input string, r *router.Router, pp domain.PromptProvider, store domain.MessageStore, manager *domain.ConversationManager, currentModel *string, lr *LineReader, tools []domain.Tool) {
	cmd := strings.Fields(input)[0]
	switch cmd {
	case "/model":
		cmdModel(r, manager, currentModel, lr)
	case "/memory":
		cmdMemory(ctx, store)
	case "/session":
		cmdSession(ctx, store, lr)
	case "/prompt":
		cmdPrompt(ctx, pp)
	case "/tools":
		cmdTools(tools)
	case "/q":
		cmdQuit()
	default:
		fmt.Printf("Unknown command %s. Available: %s, %s, %s, %s, %s, %s\n",
			red(cmd), yellow("/model"), yellow("/memory"), yellow("/session"), yellow("/prompt"), yellow("/tools"), yellow("/q"))
	}
}

func cmdTools(tools []domain.Tool) {
	if len(tools) == 0 {
		fmt.Println(faint("(no tools registered)"))
		return
	}
	fmt.Println("\n" + emphasize("Available tools:"))
	for _, t := range tools {
		fmt.Printf("  %s\n    %s\n", cyan(t.Name()), t.Description())
	}
}

func cmdQuit() {
	fmt.Println(faint("Exiting session."))
	os.Exit(0)
}

func cmdPrompt(ctx context.Context, pp domain.PromptProvider) {
	text, err := pp.SystemPrompt(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error loading prompt: ")+err.Error())
		return
	}
	if text == "" {
		fmt.Println(faint("(no system prompt)"))
		return
	}
	fmt.Printf("\n%s\n%s\n%s\n", faint("--- system prompt ---"), text, faint("--- end ---"))
}

func cmdModel(r *router.Router, manager *domain.ConversationManager, currentModel *string, lr *LineReader) {
	models := domain.SupportedModels()
	slices.Sort(models)

	fmt.Println("\n" + emphasize("Available models:"))
	for i, m := range models {
		d, _ := domain.Lookup(m)
		if string(m) == *currentModel {
			fmt.Printf("  [%d] %s %s %s\n", i+1, cyan(fmt.Sprintf("%-14s", m)), faint("("+string(d.Provider)+")"), green("← current"))
		} else {
			fmt.Printf("  [%d] %-14s %s\n", i+1, m, faint("("+string(d.Provider)+")"))
		}
	}

	choice, err := lr.ReadLine(fmt.Sprintf("Choose [1-%d]: ", len(models)))
	if err != nil {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || n < 1 || n > len(models) {
		fmt.Println(yellow("Invalid choice, keeping current model."))
		return
	}

	selected := models[n-1]
	client, err := r.Get(selected)
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error building client: ")+err.Error())
		return
	}
	manager.SetClient(client, string(selected))
	*currentModel = string(selected)
	fmt.Printf("Switched to %s.\n", green(string(selected)))
}

func cmdMemory(ctx context.Context, store domain.MessageStore) {
	sb, ok := store.(domain.SessionBrowser)
	if !ok {
		fmt.Println(faint("(session history not available)"))
		return
	}
	turns, err := sb.LoadSession(ctx, store.SessionID())
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	if len(turns) == 0 {
		fmt.Println(faint("(no history for current session)"))
		return
	}
	fmt.Printf("\n%s  %s\n", emphasize("Session history:"), faint(store.SessionID()))
	for i, t := range turns {
		turnIDStr := ""
		if t.TurnID != "" {
			turnIDStr = "  " + faint(t.TurnID)
		}
		fmt.Printf("\n%s  %s%s\n",
			emphasize(fmt.Sprintf("Turn %d", i+1)),
			faint(t.At.Format("2006-01-02 15:04:05")),
			turnIDStr)
		fmt.Printf("  %s %s\n", green(bold+"You"+reset+":"), t.Question)
		fmt.Printf("  %s %s\n", cyan(bold+"Assistant"+reset+":"), t.Answer)
		if t.CallCount > 1 {
			fmt.Printf("  %s\n", faint(fmt.Sprintf("(%d LLM calls)", t.CallCount)))
		}
	}
}

func cmdSession(ctx context.Context, store domain.MessageStore, lr *LineReader) {
	sb, ok := store.(domain.SessionBrowser)
	if !ok {
		fmt.Println(faint("(session switching not available)"))
		return
	}
	sessions, err := sb.ListSessions(ctx, store.UserID())
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	fmt.Printf("\n%s  %s\n", emphasize("Sessions:"), faint("user: "+store.UserID()))
	if len(sessions) == 0 {
		fmt.Println(faint("  (no past sessions found)"))
		return
	}
	for i, s := range sessions {
		label := shortID(s.ID)
		marker := ""
		if s.ID == store.SessionID() {
			marker = " " + green("← current")
		}
		fmt.Printf("  [%d] %s  %s%s\n", i+1, cyan(label), faint(s.CreatedAt.Format("2006-01-02 15:04")), marker)
	}
	choice, err := lr.ReadLine(fmt.Sprintf("Choose [1-%d] (Enter to cancel): ", len(sessions)))
	if err != nil || strings.TrimSpace(choice) == "" {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || n < 1 || n > len(sessions) {
		fmt.Println(yellow("Invalid choice."))
		return
	}
	selected := sessions[n-1].ID
	if err := sb.SetSession(ctx, selected); err != nil {
		fmt.Fprintln(os.Stderr, red("Error switching session: ")+err.Error())
		return
	}
	fmt.Printf("Switched to session %s.\n", green(shortID(selected)))
}

// shortID returns a concise display form of a session UUID.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}
