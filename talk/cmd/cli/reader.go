package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	prompt "github.com/c-bata/go-prompt"
	"golang.org/x/term"
)

var ansiEscapeSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Reader abstracts line input for testability.
type Reader interface {
	ReadLine(prompt string) (string, error)
}

var topLevelCommandSuggestions = []prompt.Suggest{
	{Text: "/help", Description: "show available commands"},
	{Text: "/model", Description: "switch model"},
	{Text: "/memory", Description: "show current session history"},
	{Text: "/session", Description: "manage sessions"},
	{Text: "/prompt", Description: "show system prompt"},
	{Text: "/mcp", Description: "manage MCP servers"},
	{Text: "/q", Description: "quit"},
}

var mcpSubcommandSuggestions = []prompt.Suggest{
	{Text: "list", Description: "list MCP servers"},
	{Text: "add", Description: "add MCP server"},
	{Text: "remove", Description: "remove MCP server"},
	{Text: "refresh", Description: "refresh MCP tools"},
}

var sessionSubcommandSuggestions = []prompt.Suggest{
	{Text: "list", Description: "list sessions"},
	{Text: "new", Description: "create new session"},
	{Text: "remove", Description: "remove session"},
}

// GoPromptReader wraps go-prompt and provides persistent history.
type GoPromptReader struct {
	readMu sync.Mutex

	historyPath    string
	historyEntries []string
	lastPersisted  string

	nonTTY bool
}

// NewGoPromptReader creates a Reader backed by go-prompt.
func NewGoPromptReader(historyPath string) (*GoPromptReader, error) {
	entries, err := loadHistoryEntries(historyPath)
	if err != nil {
		return nil, err
	}

	gr := &GoPromptReader{
		historyPath:    historyPath,
		historyEntries: append([]string(nil), entries...),
		nonTTY:         !term.IsTerminal(int(os.Stdin.Fd())),
	}
	if len(entries) > 0 {
		gr.lastPersisted = entries[len(entries)-1]
	}

	return gr, nil
}

// ReadLine reads one line of input.
func (gr *GoPromptReader) ReadLine(promptText string) (string, error) {
	gr.readMu.Lock()
	defer gr.readMu.Unlock()

	if gr.nonTTY {
		line, err := readLineFallback(promptText)
		if err != nil {
			return "", err
		}
		normalized := strings.TrimSpace(line)
		if normalized != "" {
			if added, _ := gr.persistHistory(normalized); added {
				gr.historyEntries = append(gr.historyEntries, normalized)
			}
		}
		return line, nil
	}

	p := prompt.New(
		func(string) {},
		gr.complete,
		prompt.OptionPrefix(stripANSI(promptText)),
		prompt.OptionHistory(gr.historyEntries),
		prompt.OptionAddKeyBind(
			prompt.KeyBind{Key: prompt.ControlA, Fn: prompt.GoLineBeginning},
			prompt.KeyBind{Key: prompt.ControlE, Fn: prompt.GoLineEnd},
		),
	)

	line := p.Input()
	normalized := strings.TrimSpace(line)
	if normalized != "" {
		if added, _ := gr.persistHistory(normalized); added {
			gr.historyEntries = append(gr.historyEntries, normalized)
		}
	}

	return line, nil
}

func stripANSI(text string) string {
	return ansiEscapeSequence.ReplaceAllString(text, "")
}

func (gr *GoPromptReader) complete(doc prompt.Document) []prompt.Suggest {
	return commandSuggestions(doc.TextBeforeCursor())
}

func commandSuggestions(beforeCursor string) []prompt.Suggest {
	input := strings.TrimSpace(beforeCursor)
	if input == "" || !strings.HasPrefix(input, "/") {
		return nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	if len(parts) == 1 {
		return prompt.FilterHasPrefix(topLevelCommandSuggestions, parts[0], true)
	}

	if len(parts) == 2 {
		switch parts[0] {
		case "/mcp":
			return prompt.FilterHasPrefix(mcpSubcommandSuggestions, parts[1], true)
		case "/session":
			return prompt.FilterHasPrefix(sessionSubcommandSuggestions, parts[1], true)
		}
	}

	return nil
}

func (gr *GoPromptReader) persistHistory(entry string) (bool, error) {
	if entry == "" || entry == gr.lastPersisted {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(gr.historyPath), 0o700); err != nil {
		return false, err
	}

	f, err := os.OpenFile(gr.historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		return false, err
	}

	gr.lastPersisted = entry
	return true, nil
}

func loadHistoryEntries(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	entries := make([]string, 0, 64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries = append(entries, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func readLineFallback(promptText string) (string, error) {
	_, _ = os.Stdout.WriteString(promptText)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				return "", io.EOF
			}
			return line, nil
		}
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}
