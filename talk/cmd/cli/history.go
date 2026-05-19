package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// History manages per-session command history with persistence to a file.
type History struct {
	entries []string // oldest first
	cursor  int      // len(entries) = new-entry position; 0..n-1 = navigating
	path    string
}

// NewHistory creates a History that loads from and saves to path.
func NewHistory(path string) *History {
	h := &History{path: path}
	_ = h.load()
	h.cursor = len(h.entries)
	return h
}

// Add appends a non-empty entry (skipping consecutive duplicates) and saves.
func (h *History) Add(entry string) {
	if entry == "" {
		return
	}
	if len(h.entries) == 0 || h.entries[len(h.entries)-1] != entry {
		h.entries = append(h.entries, entry)
	}
	h.cursor = len(h.entries)
	_ = h.save()
}

// Prev moves the cursor back (older) and returns the entry.
// Returns ("", false) when already at the oldest entry.
func (h *History) Prev() (string, bool) {
	if h.cursor == 0 {
		return "", false
	}
	h.cursor--
	return h.entries[h.cursor], true
}

// Next moves the cursor forward (newer).
// Returns ("", false) when past the most recent entry (back to empty line).
func (h *History) Next() (string, bool) {
	if h.cursor >= len(h.entries) {
		return "", false
	}
	h.cursor++
	if h.cursor == len(h.entries) {
		return "", false
	}
	return h.entries[h.cursor], true
}

// Reset places the cursor back at the new-line position.
func (h *History) Reset() {
	h.cursor = len(h.entries)
}

func (h *History) load() error {
	f, err := os.Open(h.path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line != "" {
			h.entries = append(h.entries, line)
		}
	}
	return sc.Err()
}

func (h *History) save() error {
	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range h.entries {
		_, _ = fmt.Fprintln(w, e)
	}
	return w.Flush()
}
