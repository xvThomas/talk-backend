package main

import (
	"path/filepath"
	"testing"
)

func TestHistory_AddAndNavigate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(path)

	h.Add("first")
	h.Add("second")
	h.Add("third")

	// Navigate backward
	entry, ok := h.Prev()
	if !ok || entry != "third" {
		t.Errorf("Prev() = (%q, %v), want (\"third\", true)", entry, ok)
	}
	entry, ok = h.Prev()
	if !ok || entry != "second" {
		t.Errorf("Prev() = (%q, %v), want (\"second\", true)", entry, ok)
	}
	entry, ok = h.Prev()
	if !ok || entry != "first" {
		t.Errorf("Prev() = (%q, %v), want (\"first\", true)", entry, ok)
	}

	// At oldest — should return false
	_, ok = h.Prev()
	if ok {
		t.Error("Prev() should return false at oldest entry")
	}

	// Navigate forward
	entry, ok = h.Next()
	if !ok || entry != "second" {
		t.Errorf("Next() = (%q, %v), want (\"second\", true)", entry, ok)
	}
	entry, ok = h.Next()
	if !ok || entry != "third" {
		t.Errorf("Next() = (%q, %v), want (\"third\", true)", entry, ok)
	}

	// Past newest — back to new-line position
	_, ok = h.Next()
	if ok {
		t.Error("Next() should return false past newest entry")
	}
}

func TestHistory_SkipsDuplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(path)

	h.Add("hello")
	h.Add("hello")
	h.Add("hello")

	// Should have only one entry
	h.Reset()
	entry, ok := h.Prev()
	if !ok || entry != "hello" {
		t.Errorf("Prev() = (%q, %v), want (\"hello\", true)", entry, ok)
	}
	_, ok = h.Prev()
	if ok {
		t.Error("expected only one entry after consecutive duplicates")
	}
}

func TestHistory_SkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(path)

	h.Add("")
	h.Add("actual")
	h.Add("")

	h.Reset()
	entry, ok := h.Prev()
	if !ok || entry != "actual" {
		t.Errorf("Prev() = (%q, %v), want (\"actual\", true)", entry, ok)
	}
	_, ok = h.Prev()
	if ok {
		t.Error("expected only one entry after skipping empty strings")
	}
}

func TestHistory_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")

	// First instance
	h1 := NewHistory(path)
	h1.Add("persisted-entry")

	// Second instance loading from same file
	h2 := NewHistory(path)
	entry, ok := h2.Prev()
	if !ok || entry != "persisted-entry" {
		t.Errorf("Prev() after reload = (%q, %v), want (\"persisted-entry\", true)", entry, ok)
	}
}

func TestHistory_Reset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	h := NewHistory(path)

	h.Add("a")
	h.Add("b")
	h.Prev()
	h.Prev()

	h.Reset()

	// After reset, Prev should return the newest entry
	entry, ok := h.Prev()
	if !ok || entry != "b" {
		t.Errorf("after Reset(), Prev() = (%q, %v), want (\"b\", true)", entry, ok)
	}
}

func TestHistory_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "missing")
	h := NewHistory(path)

	// Should work fine with empty history
	_, ok := h.Prev()
	if ok {
		t.Error("expected no entries for non-existent history file")
	}

	// Adding should not panic even if directory doesn't exist
	h.Add("test")
}

func TestHistory_NextAtBoundary(t *testing.T) {
	dir := t.TempDir()
	h := NewHistory(filepath.Join(dir, "h"))
	h.Add("only")

	// Go to oldest
	h.Prev()
	// Now Next should move cursor to len(entries) and return "", false
	entry, ok := h.Next()
	if ok {
		t.Errorf("Next() from last entry should return false, got (%q, true)", entry)
	}

	// Calling Next again when already past end
	_, ok = h.Next()
	if ok {
		t.Error("Next() when already past end should return false")
	}
}
