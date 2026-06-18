package main

import (
	"path/filepath"
	"testing"
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
