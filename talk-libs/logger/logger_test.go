package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewColorHandler tests the creation of ColorHandler with various options
func TestNewColorHandler(t *testing.T) {
	tests := []struct {
		name string
		opts *slog.HandlerOptions
	}{
		{"with nil options", nil},
		{"with custom level", &slog.HandlerOptions{Level: slog.LevelDebug}},
		{"with warn level", &slog.HandlerOptions{Level: slog.LevelWarn}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewColorHandler(buf, tt.opts)

			if handler == nil {
				t.Fatal("expected non-nil handler")
			}
			if handler.writer != buf {
				t.Error("writer not set correctly")
			}
			if handler.opts == nil {
				t.Error("opts should not be nil after initialization")
			}
		})
	}
}

// TestColorHandler_Enabled tests whether handler enables correct log levels
func TestColorHandler_Enabled(t *testing.T) {
	tests := []struct {
		name         string
		handlerLevel slog.Level
		testLevel    slog.Level
		want         bool
	}{
		{"debug enabled for debug handler", slog.LevelDebug, slog.LevelDebug, true},
		{"info enabled for debug handler", slog.LevelDebug, slog.LevelInfo, true},
		{"debug disabled for info handler", slog.LevelInfo, slog.LevelDebug, false},
		{"info enabled for info handler", slog.LevelInfo, slog.LevelInfo, true},
		{"warn enabled for info handler", slog.LevelInfo, slog.LevelWarn, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewColorHandler(buf, &slog.HandlerOptions{Level: tt.handlerLevel})

			got := handler.Enabled(context.Background(), tt.testLevel)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetLevelColor tests color mapping for each log level
func TestGetLevelColor(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, colorGray},
		{slog.LevelInfo, colorBlue},
		{slog.LevelWarn, colorYellow},
		{slog.LevelError, colorRed},
		{slog.Level(-100), colorReset}, // Unknown level
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			got := getLevelColor(tt.level)
			if got != tt.want {
				t.Errorf("getLevelColor(%v) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

// TestColorHandler_Handle tests the main logging functionality with colors
func TestColorHandler_Handle(t *testing.T) {
	tests := []struct {
		name      string
		level     slog.Level
		message   string
		attrs     []slog.Attr
		wantColor string
		wantInMsg []string
	}{
		{
			name:      "info message",
			level:     slog.LevelInfo,
			message:   "test info message",
			attrs:     []slog.Attr{slog.String("key", "value")},
			wantColor: colorBlue,
			wantInMsg: []string{"test info message", "key=value", "level=INFO"},
		},
		{
			name:      "error message",
			level:     slog.LevelError,
			message:   "error occurred",
			attrs:     []slog.Attr{slog.Int("code", 500)},
			wantColor: colorRed,
			wantInMsg: []string{"error occurred", "code=500", "level=ERROR"},
		},
		{
			name:      "warn message",
			level:     slog.LevelWarn,
			message:   "warning",
			attrs:     nil,
			wantColor: colorYellow,
			wantInMsg: []string{"warning", "level=WARN"},
		},
		{
			name:      "debug message",
			level:     slog.LevelDebug,
			message:   "debug info",
			attrs:     []slog.Attr{slog.Bool("debug", true)},
			wantColor: colorGray,
			wantInMsg: []string{"debug info", "debug=true", "level=DEBUG"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewColorHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})

			record := slog.NewRecord(time.Now(), tt.level, tt.message, 0)
			for _, attr := range tt.attrs {
				record.AddAttrs(attr)
			}

			err := handler.Handle(context.Background(), record)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			output := buf.String()

			// Check color code presence
			if !strings.Contains(output, tt.wantColor) {
				t.Errorf("output missing color code %q", tt.wantColor)
			}

			// Check color reset
			if !strings.Contains(output, colorReset) {
				t.Error("output missing color reset code")
			}

			// Check expected content
			for _, want := range tt.wantInMsg {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}

			// Check timestamp format
			if !strings.Contains(output, "time=") {
				t.Error("output missing timestamp")
			}
		})
	}
}

// TestColorHandler_WithAttrs tests adding attributes to handler
func TestColorHandler_WithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, nil)

	attrs := []slog.Attr{
		slog.String("service", "shared"),
		slog.Int("version", 1),
	}

	newHandler := handler.WithAttrs(attrs)

	if newHandler == nil {
		t.Fatal("WithAttrs() returned nil")
	}

	// Verify it's a ColorHandler
	if _, ok := newHandler.(*ColorHandler); !ok {
		t.Error("WithAttrs() should return *ColorHandler")
	}
}

// TestColorHandler_WithGroup tests adding group to handler
func TestColorHandler_WithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, nil)

	newHandler := handler.WithGroup("database")

	if newHandler == nil {
		t.Fatal("WithGroup() returned nil")
	}

	// Verify it's a ColorHandler
	if _, ok := newHandler.(*ColorHandler); !ok {
		t.Error("WithGroup() should return *ColorHandler")
	}
}

// TestGetLogLevel tests log level parsing from environment variable
func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     slog.Level
	}{
		{"debug uppercase", "DEBUG", slog.LevelDebug},
		{"info uppercase", "INFO", slog.LevelInfo},
		{"warn uppercase", "WARN", slog.LevelWarn},
		{"error uppercase", "ERROR", slog.LevelError},
		{"debug lowercase", "debug", slog.LevelDebug},
		{"info lowercase", "info", slog.LevelInfo},
		{"mixed case", "Debug", slog.LevelDebug},
		{"empty string defaults to info", "", slog.LevelInfo},
		{"invalid value defaults to info", "INVALID", slog.LevelInfo},
		{"random string defaults to info", "xyz", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				if err := os.Setenv("LOG_LEVEL", tt.envValue); err != nil {
					t.Fatalf("Failed to set LOG_LEVEL: %v", err)
				}
			} else {
				if err := os.Unsetenv("LOG_LEVEL"); err != nil {
					t.Fatalf("Failed to unset LOG_LEVEL: %v", err)
				}
			}
			defer func() {
				_ = os.Unsetenv("LOG_LEVEL") // Cleanup, ignore error
			}()

			got := getLogLevel()
			if got != tt.want {
				t.Errorf("getLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetLogger tests the global logger accessor
func TestGetLogger(t *testing.T) {
	logger := GetLogger()

	if logger == nil {
		t.Fatal("GetLogger() returned nil")
	}

	// Should return the same instance
	logger2 := GetLogger()
	if logger != logger2 {
		t.Error("GetLogger() should return same instance (singleton)")
	}
}

// TestColorHandler_MultipleMessages tests writing multiple log messages
func TestColorHandler_MultipleMessages(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	messages := []struct {
		level slog.Level
		msg   string
	}{
		{slog.LevelInfo, "first message"},
		{slog.LevelWarn, "second message"},
		{slog.LevelError, "third message"},
	}

	for _, m := range messages {
		record := slog.NewRecord(time.Now(), m.level, m.msg, 0)
		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
	}

	output := buf.String()

	// All messages should be present
	for _, m := range messages {
		if !strings.Contains(output, m.msg) {
			t.Errorf("output missing message %q", m.msg)
		}
	}

	// Should have 3 lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != len(messages) {
		t.Errorf("got %d lines, want %d", len(lines), len(messages))
	}
}

// syncBuffer is a thread-safe buffer for testing concurrent writes
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// TestColorHandler_ConcurrentWrites tests thread-safety of handler
func TestColorHandler_ConcurrentWrites(t *testing.T) {
	// Use a synchronized buffer for concurrent writes
	buf := &syncBuffer{}
	handler := NewColorHandler(buf, nil)

	var wg sync.WaitGroup
	numGoroutines := 10
	numMessages := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				record := slog.NewRecord(
					time.Now(),
					slog.LevelInfo,
					"concurrent message",
					0,
				)
				record.AddAttrs(slog.Int("goroutine", id), slog.Int("msg", j))

				err := handler.Handle(context.Background(), record)
				if err != nil {
					t.Errorf("Handle() error = %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	output := buf.String()

	// Verify output is not empty and contains expected content
	if len(output) == 0 {
		t.Error("Output is empty, expected log messages")
		return
	}

	// Basic sanity checks - output should contain level markers and goroutine IDs
	if !strings.Contains(output, "level=INFO") {
		t.Error("Output missing level markers")
	}
	if !strings.Contains(output, "goroutine=") {
		t.Error("Output missing goroutine IDs")
	}
}

// TestColorHandler_ComplexAttributes tests handling of various attribute types
func TestColorHandler_ComplexAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, nil)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(
		slog.String("string", "value"),
		slog.Int("int", 42),
		slog.Int64("int64", 9223372036854775807),
		slog.Float64("float", 3.14),
		slog.Bool("bool", true),
		slog.Duration("duration", time.Second),
		slog.Time("time", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	)

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()

	// Check all attributes are present
	expectations := []string{
		"string=value",
		"int=42",
		"int64=9223372036854775807",
		"float=3.14",
		"bool=true",
		"duration=1s",
		"time=",
	}

	for _, want := range expectations {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\nGot: %s", want, output)
		}
	}
}

// TestColorHandler_EmptyMessage tests handling of empty log message
func TestColorHandler_EmptyMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, nil)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "", 0)

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()

	// Should still have structure
	if !strings.Contains(output, "level=INFO") {
		t.Error("output missing level")
	}
	if !strings.Contains(output, "msg=") {
		t.Error("output missing msg field")
	}
}

// TestColorHandler_OutputFormat tests the complete output format
func TestColorHandler_OutputFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewColorHandler(buf, nil)

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()

	// Check format: color + time=... level=... msg="..." key=value + reset + newline
	if !strings.HasPrefix(output, colorBlue) {
		t.Error("output should start with color code")
	}
	if !strings.HasSuffix(output, colorReset+"\n") {
		t.Error("output should end with color reset and newline")
	}
	if !strings.Contains(output, "time=") {
		t.Error("output missing time field")
	}
	if !strings.Contains(output, "level=INFO") {
		t.Error("output missing level field")
	}
	if !strings.Contains(output, `msg="test message"`) {
		t.Error("output missing or incorrectly formatted msg field")
	}
}
