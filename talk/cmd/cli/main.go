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

	"github.com/google/uuid"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/config"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/llm/router"
	internalmcp "github.com/xvThomas/LLMClientWrapper/talk/internal/mcp"
	sqlitestore "github.com/xvThomas/LLMClientWrapper/talk/internal/memory/sqlite"
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

	sessionID := domain.GenerateSessionID()
	const userID = "anonymous"

	dbPath := storeDBPath()
	store, err := sqlitestore.NewStore(dbPath, sessionID, userID)
	if err != nil {
		return fmt.Errorf("opening session store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// MCP server registry and manager.
	mcpRegistry, err := internalmcp.NewSQLiteRegistry(store.DB())
	if err != nil {
		return fmt.Errorf("initializing mcp registry: %w", err)
	}
	mcpManager := internalmcp.NewManager(mcpRegistry)
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
		// Fallback to console reporter if no other reporters are configured
		reporters = append(reporters, &usage.ConsoleUsageReporter{})
	}

	manager := domain.NewConversationManager(client, modelAlias, modelDescriptor.Provider, store, pp, mcpManager.Tools, reporters, cfg.ToolsMaxConcurrent)
	currentModel := modelAlias

	fmt.Print(cyan(bold+"Session started."+reset) + faint(" "+version.Version) + "\n")
	cmdHelp()
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
			handleSlashCommand(ctx, input, r, pp, store, manager, &currentModel, lr, mcpManager, mcpRegistry)
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

func storeDBPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".talk")
		_ = os.MkdirAll(dir, 0o700)
		return filepath.Join(dir, "talk.db")
	}
	return "talk.db"
}

func handleSlashCommand(ctx context.Context, input string, r *router.Router, pp domain.PromptProvider, store domain.MessageStore, manager *domain.ConversationManager, currentModel *string, lr *LineReader, mcpMgr *internalmcp.Manager, mcpReg internalmcp.Registry) {
	cmd := strings.Fields(input)[0]
	switch cmd {
	case "/model":
		cmdModel(r, manager, currentModel, lr)
	case "/memory":
		cmdMemory(ctx, store)
	case "/sessions":
		cmdSessions(ctx, store)
	case "/session":
		args := strings.TrimSpace(strings.TrimPrefix(input, "/session"))
		cmdSession(ctx, store, lr, args)
	case "/prompt":
		cmdPrompt(ctx, pp)
	case "/mcp":
		args := strings.TrimSpace(strings.TrimPrefix(input, "/mcp"))
		cmdMCP(ctx, args, mcpMgr, mcpReg, lr)
	case "/help":
		cmdHelp()
	case "/q":
		cmdQuit()
	default:
		fmt.Printf("Unknown command %s. Type %s for available commands.\n",
			red(cmd), yellow("/help"))
	}
}

func cmdMCP(ctx context.Context, args string, mgr *internalmcp.Manager, reg internalmcp.Registry, lr *LineReader) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		// Default to list.
		cmdMCPList(mgr)
		return
	}
	switch parts[0] {
	case "list":
		cmdMCPList(mgr)
	case "add":
		cmdMCPAdd(ctx, mgr, reg, lr)
	case "remove":
		cmdMCPRemove(ctx, mgr, reg, lr)
	case "refresh":
		cmdMCPRefresh(ctx, mgr)
	default:
		fmt.Printf("Unknown /mcp subcommand %s. Available: %s, %s, %s, %s\n",
			red(parts[0]), yellow("list"), yellow("add"), yellow("remove"), yellow("refresh"))
	}
}

func cmdMCPList(mgr *internalmcp.Manager) {
	statuses := mgr.Statuses()
	if len(statuses) == 0 {
		fmt.Println(faint("(no MCP servers registered)"))
		return
	}
	fmt.Println("\n" + emphasize("MCP Servers:"))
	for _, st := range statuses {
		stateLabel := green("connected")
		if !st.Connected {
			stateLabel = red("disconnected")
		}
		serverInfo := ""
		if st.ServerName != "" {
			serverInfo = faint(" (" + st.ServerName)
			if st.ServerVersion != "" {
				serverInfo += " " + st.ServerVersion
			}
			serverInfo += ")"
		}
		fmt.Printf("  %s  %s  %s%s\n", cyan(st.Config.Name), faint(st.Config.URL), stateLabel, serverInfo)
		if st.Error != "" {
			fmt.Printf("    %s\n", red(st.Error))
		}
		for _, toolName := range st.Tools {
			fmt.Printf("    • %s\n", toolName)
		}
	}
}

func cmdMCPAdd(ctx context.Context, mgr *internalmcp.Manager, reg internalmcp.Registry, lr *LineReader) {
	name, err := lr.ReadLine("Server name: ")
	if err != nil || strings.TrimSpace(name) == "" {
		fmt.Println(yellow("Cancelled."))
		return
	}
	name = strings.TrimSpace(name)

	url, err := lr.ReadLine("Server URL: ")
	if err != nil || strings.TrimSpace(url) == "" {
		fmt.Println(yellow("Cancelled."))
		return
	}
	url = strings.TrimSpace(url)

	authChoice, err := lr.ReadLine("Auth type [none/apikey/oauth] (default: apikey): ")
	if err != nil {
		fmt.Println(yellow("Cancelled."))
		return
	}
	authChoice = strings.TrimSpace(strings.ToLower(authChoice))
	if authChoice == "" {
		authChoice = "apikey"
	}

	cfg := internalmcp.ServerConfig{
		ID:       uuid.NewString(),
		Name:     name,
		URL:      url,
		AuthType: internalmcp.AuthType(authChoice),
	}

	switch cfg.AuthType {
	case internalmcp.AuthTypeNone:
		// No credentials needed.
	case internalmcp.AuthTypeAPIKey:
		key, err := lr.ReadLine("API Key: ")
		if err != nil {
			fmt.Println(yellow("Cancelled."))
			return
		}
		cfg.APIKey = strings.TrimSpace(key)
	case internalmcp.AuthTypeOAuth:
		clientID, _ := lr.ReadLine("Client ID: ")
		clientSecret, _ := lr.ReadLine("Client Secret: ")
		tokenURL, _ := lr.ReadLine("Token URL: ")
		scopes, _ := lr.ReadLine("Scopes (comma-separated): ")
		cfg.OAuth = &internalmcp.OAuthConfig{
			ClientID:     strings.TrimSpace(clientID),
			ClientSecret: strings.TrimSpace(clientSecret),
			TokenURL:     strings.TrimSpace(tokenURL),
		}
		if s := strings.TrimSpace(scopes); s != "" {
			for _, sc := range strings.Split(s, ",") {
				cfg.OAuth.Scopes = append(cfg.OAuth.Scopes, strings.TrimSpace(sc))
			}
		}
	default:
		fmt.Printf("%s\n", yellow("Invalid auth type. Use 'none', 'apikey', or 'oauth'."))
		return
	}

	// Test connection before persisting.
	fmt.Printf("Testing connection to %s...\n", faint(cfg.URL))
	status, err := mgr.Connect(ctx, cfg)
	if err != nil {
		fmt.Printf("%s %s\n", red("Connection failed:"), err.Error())
		return
	}

	// Persist.
	if err := reg.Add(ctx, cfg); err != nil {
		fmt.Printf("%s %s\n", red("Failed to save:"), err.Error())
		return
	}

	fmt.Printf("%s %s", green("✓ Server added:"), cyan(name))
	if status.ServerName != "" {
		fmt.Printf(" %s", faint("("+status.ServerName+" "+status.ServerVersion+")"))
	}
	fmt.Printf(" — %d tools\n", len(status.Tools))
}

func cmdMCPRemove(ctx context.Context, mgr *internalmcp.Manager, reg internalmcp.Registry, lr *LineReader) {
	servers, err := reg.List(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	if len(servers) == 0 {
		fmt.Println(faint("(no MCP servers registered)"))
		return
	}

	fmt.Println("\n" + emphasize("Registered MCP servers:"))
	for i, s := range servers {
		fmt.Printf("  [%d] %s  %s\n", i+1, cyan(s.Name), faint(s.URL))
	}

	choice, err := lr.ReadLine(fmt.Sprintf("Remove [1-%d] (Enter to cancel): ", len(servers)))
	if err != nil || strings.TrimSpace(choice) == "" {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || n < 1 || n > len(servers) {
		fmt.Println(yellow("Invalid choice."))
		return
	}

	selected := servers[n-1]
	if err := reg.Remove(ctx, selected.ID); err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	mgr.Disconnect(selected.ID)
	fmt.Printf("Removed %s.\n", green(selected.Name))
}

func cmdMCPRefresh(ctx context.Context, mgr *internalmcp.Manager) {
	count := mgr.Refresh(ctx)
	fmt.Printf("%s %d tools available.\n", green("✓ Tools refreshed:"), count)
}

func cmdHelp() {
	fmt.Println(emphasize("Commands:"))
	fmt.Println(faint("  /help     — show this help"))
	fmt.Println(faint("  /model    — switch models"))
	fmt.Println(faint("  /memory   — show current session history"))
	fmt.Println(faint("  /sessions — list all sessions"))
	fmt.Println(faint("  /session  — new session or switch (usage: /session [id])"))
	fmt.Println(faint("  /prompt   — show system prompt"))
	fmt.Println(faint("  /mcp      — manage MCP servers (add, remove, refresh, list)"))
	fmt.Println(faint("  /q        — quit"))
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
	// Resolve session title
	title := ""
	if sessions, err := sb.ListSessions(ctx, store.UserID()); err == nil {
		for _, s := range sessions {
			if s.ID == store.SessionID() {
				title = s.Title
				break
			}
		}
	}
	header := emphasize("Session history:")
	if title != "" {
		header += "  " + cyan(title)
	}
	fmt.Printf("\n%s  %s\n", header, faint(shortID(store.SessionID())))
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

func cmdSessions(ctx context.Context, store domain.MessageStore) {
	sb, ok := store.(domain.SessionBrowser)
	if !ok {
		fmt.Println(faint("(sessions not available)"))
		return
	}
	sessions, err := sb.ListSessions(ctx, store.UserID())
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	if len(sessions) == 0 {
		fmt.Println(faint("(no sessions)"))
		return
	}
	fmt.Printf("\n%s\n", emphasize("Sessions:"))
	for _, s := range sessions {
		marker := ""
		if s.ID == store.SessionID() {
			marker = " " + green("← current")
		}
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Printf("  %s  %s  %s  %s%s\n",
			cyan(s.ID),
			title,
			faint(s.CreatedAt.Format("2006-01-02 15:04")),
			faint(fmt.Sprintf("%d turns", s.TurnCount)),
			marker)
	}
	fmt.Println()
}

func cmdSession(ctx context.Context, store domain.MessageStore, lr *LineReader, args string) {
	sb, ok := store.(domain.SessionBrowser)
	if !ok {
		fmt.Println(faint("(session switching not available)"))
		return
	}

	// /session <id> — switch to an existing session by prefix match
	if args != "" {
		sessions, err := sb.ListSessions(ctx, store.UserID())
		if err != nil {
			fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
			return
		}
		var match string
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, args) {
				match = s.ID
				break
			}
		}
		if match == "" {
			fmt.Println(yellow("No session found matching: ") + faint(args))
			return
		}
		if err := sb.SetSession(ctx, match); err != nil {
			fmt.Fprintln(os.Stderr, red("Error switching session: ")+err.Error())
			return
		}
		fmt.Printf("Switched to session %s.\n", green(shortID(match)))
		return
	}

	// /session — show sessions and offer to create new or switch
	sessions, err := sb.ListSessions(ctx, store.UserID())
	if err != nil {
		fmt.Fprintln(os.Stderr, red("Error: ")+err.Error())
		return
	}
	fmt.Printf("\n%s  %s\n", emphasize("Sessions:"), faint("user: "+store.UserID()))
	if len(sessions) == 0 {
		fmt.Println(faint("  (no past sessions found)"))
	} else {
		for i, s := range sessions {
			label := shortID(s.ID)
			marker := ""
			if s.ID == store.SessionID() {
				marker = " " + green("← current")
			}
			turns := ""
			if s.TurnCount > 0 {
				turns = fmt.Sprintf(" (%d turns)", s.TurnCount)
			}
			fmt.Printf("  [%d] %s  %s%s%s\n", i+1, cyan(label), faint(s.CreatedAt.Format("2006-01-02 15:04")), faint(turns), marker)
		}
	}
	choice, err := lr.ReadLine(fmt.Sprintf("Choose [1-%d] or 'new' (Enter to cancel): ", len(sessions)))
	if err != nil || strings.TrimSpace(choice) == "" {
		return
	}
	choice = strings.TrimSpace(choice)
	if choice == "new" {
		newID := domain.GenerateSessionID()
		if err := sb.SetSession(ctx, newID); err != nil {
			fmt.Fprintln(os.Stderr, red("Error creating session: ")+err.Error())
			return
		}
		fmt.Printf("New session created: %s\n", green(shortID(newID)))
		return
	}
	n, err := strconv.Atoi(choice)
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
