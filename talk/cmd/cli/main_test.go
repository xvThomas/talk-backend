package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSystemPromptPath(t *testing.T) {
	p := defaultSystemPromptPath()
	if !strings.HasSuffix(p, "system_prompt.md") {
		t.Errorf("expected path ending with system_prompt.md, got: %s", p)
	}
}

func TestHistoryFilePath(t *testing.T) {
	p := historyFilePath()
	if !strings.HasSuffix(p, ".talks_history") {
		t.Errorf("expected path ending with .talks_history, got: %s", p)
	}
}

func TestStoreDBPath(t *testing.T) {
	p := storeDBPath()
	if !strings.HasSuffix(p, "talk.db") {
		t.Errorf("expected path ending with talk.db, got: %s", p)
	}
	// Should be in a .talk directory
	dir := filepath.Dir(p)
	if !strings.HasSuffix(dir, ".talk") {
		t.Errorf("expected parent dir .talk, got: %s", dir)
	}
}

func TestBuildPromptProvider(t *testing.T) {
	pp := buildPromptProvider("test_file.md")
	if pp == nil {
		t.Fatal("expected non-nil PromptProvider")
	}
}
