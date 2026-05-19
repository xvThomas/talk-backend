package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// LineReader reads a line of input with raw-mode terminal support,
// including history navigation via up/down arrow keys.
type LineReader struct {
	history *History
	lastLen int // visible length of the last drawn input (for line clearing)
}

// NewLineReader creates a LineReader backed by the given History.
func NewLineReader(h *History) *LineReader {
	return &LineReader{history: h}
}

// ReadLine displays prompt, then reads a line with full history navigation.
// Returns io.EOF when the user signals end-of-input (Ctrl+D on an empty line).
// On Ctrl+C the current line is discarded and a new prompt is shown.
func (lr *LineReader) ReadLine(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Not a TTY — fall back to a simple bufio read.
		return lr.readLineFallback(prompt)
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	fmt.Print(prompt)
	lr.history.Reset()
	lr.lastLen = 0

	var buf []rune

	for {
		b := [1]byte{}
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return "", io.EOF
		}

		switch b[0] {
		case 13: // CR — Enter (raw mode sends \r not \n)
			line := string(buf)
			fmt.Print("\r\n")
			return line, nil

		case 3: // Ctrl+C — discard line, show new prompt
			fmt.Print("^C\r\n" + prompt)
			buf = buf[:0]
			lr.lastLen = 0
			lr.history.Reset()

		case 4: // Ctrl+D — EOF only when line is empty
			if len(buf) == 0 {
				fmt.Print("\r\n")
				return "", io.EOF
			}

		case 127, 8: // Backspace / DEL
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				lr.redraw(prompt, string(buf))
			}

		case 27: // ESC — start of an escape sequence
			seq := [2]byte{}
			n, _ := os.Stdin.Read(seq[:])
			if n >= 2 && seq[0] == '[' {
				switch seq[1] {
				case 'A': // up arrow → older history entry
					if entry, ok := lr.history.Prev(); ok {
						buf = []rune(entry)
						lr.redraw(prompt, entry)
					}
				case 'B': // down arrow → newer history entry
					if entry, ok := lr.history.Next(); ok {
						buf = []rune(entry)
						lr.redraw(prompt, entry)
					} else {
						buf = buf[:0]
						lr.redraw(prompt, "")
					}
				}
			}

		default:
			if b[0] >= 32 { // printable byte — handle multi-byte UTF-8
				r := lr.readRune(b[0])
				buf = append(buf, r)
				fmt.Print(string(r))
				lr.lastLen = len(buf)
			}
		}
	}
}

// readRune assembles a full UTF-8 rune starting with firstByte.
func (lr *LineReader) readRune(firstByte byte) rune {
	size := utf8.RuneLen(rune(firstByte))
	if size <= 1 {
		return rune(firstByte)
	}
	raw := make([]byte, size)
	raw[0] = firstByte
	_, _ = io.ReadFull(os.Stdin, raw[1:])
	r, _ := utf8.DecodeRune(raw)
	return r
}

// redraw clears the current input line and redraws prompt + content.
func (lr *LineReader) redraw(prompt, content string) {
	// \033[2K clears the entire terminal line; \r moves to column 0.
	fmt.Printf("\r\033[2K%s%s", prompt, content)
	lr.lastLen = len([]rune(content))
}

// readLineFallback is used when stdin is not a TTY (e.g. piped input).
func (lr *LineReader) readLineFallback(prompt string) (string, error) {
	fmt.Print(prompt)
	var line string
	n, err := fmt.Scanln(&line)
	if n == 0 && err != nil {
		return "", io.EOF
	}
	return strings.TrimSpace(line), nil
}
