package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	prompt "github.com/c-bata/go-prompt"
)

func TestCommandSuggestionsTopLevel(t *testing.T) {
	suggestions := commandSuggestions("/m")
	if len(suggestions) == 0 {
		t.Fatal("expected top-level suggestions for /m")
	}

	foundModel := false
	foundMCP := false
	for _, suggestion := range suggestions {
		if suggestion.Text == "/model" {
			foundModel = true
		}
		if suggestion.Text == "/mcp" {
			foundMCP = true
		}
	}

	if !foundModel || !foundMCP {
		t.Fatalf("expected /model and /mcp in suggestions, got %+v", suggestions)
	}
}

func TestCommandSuggestionsSubcommands(t *testing.T) {
	suggestions := commandSuggestions("/mcp r")
	if len(suggestions) == 0 {
		t.Fatal("expected /mcp subcommand suggestions")
	}

	foundRemove := false
	foundRefresh := false
	for _, suggestion := range suggestions {
		if suggestion.Text == "remove" {
			foundRemove = true
		}
		if suggestion.Text == "refresh" {
			foundRefresh = true
		}
	}

	if !foundRemove || !foundRefresh {
		t.Fatalf("expected remove and refresh suggestions, got %+v", suggestions)
	}
}

func TestPersistHistorySkipsConsecutiveDuplicates(t *testing.T) {
	dir := t.TempDir()
	historyPath := filepath.Join(dir, "history")

	reader := &GoPromptReader{historyPath: historyPath}
	if _, err := reader.persistHistory("/help"); err != nil {
		t.Fatalf("persist first entry: %v", err)
	}
	if _, err := reader.persistHistory("/help"); err != nil {
		t.Fatalf("persist duplicate entry: %v", err)
	}
	if _, err := reader.persistHistory("hello"); err != nil {
		t.Fatalf("persist second unique entry: %v", err)
	}

	entries, err := loadHistoryEntries(historyPath)
	if err != nil {
		t.Fatalf("load history entries: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after dedupe, got %d: %+v", len(entries), entries)
	}
	if entries[0] != "/help" || entries[1] != "hello" {
		t.Fatalf("unexpected history entries: %+v", entries)
	}
}

func TestStripANSI(t *testing.T) {
	got := stripANSI("\x1b[31mHello\x1b[0m world")
	if got != "Hello world" {
		t.Fatalf("stripANSI() = %q, want %q", got, "Hello world")
	}
}

func TestGoPromptReaderComplete(t *testing.T) {
	gr := &GoPromptReader{}
	suggestions := gr.complete(*prompt.NewDocument())
	if suggestions != nil {
		t.Fatalf("expected nil suggestions for empty doc, got: %+v", suggestions)
	}
}

func TestNewGoPromptReader(t *testing.T) {
	dir := t.TempDir()
	historyPath := filepath.Join(dir, "history")
	if err := os.WriteFile(historyPath, []byte("/help\nhello\n"), 0o600); err != nil {
		t.Fatalf("write history: %v", err)
	}

	gr, err := NewGoPromptReader(historyPath)
	if err != nil {
		t.Fatalf("NewGoPromptReader() error = %v", err)
	}
	if len(gr.historyEntries) != 2 {
		t.Fatalf("historyEntries len = %d, want 2", len(gr.historyEntries))
	}
	if gr.lastPersisted != "hello" {
		t.Fatalf("lastPersisted = %q, want %q", gr.lastPersisted, "hello")
	}
}

func TestLoadHistoryEntries_ErrorOnDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := loadHistoryEntries(dir)
	if err == nil {
		t.Fatal("expected error when loading history from directory")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory-related error, got: %v", err)
	}
}
