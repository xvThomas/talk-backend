package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

// ColorHandler wraps a slog.Handler to add color to log levels
type ColorHandler struct {
	handler slog.Handler
	writer  io.Writer
	opts    *slog.HandlerOptions
}

// NewColorHandler creates a new colored log handler for terminal output.
func NewColorHandler(w io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ColorHandler{
		handler: slog.NewTextHandler(w, opts),
		writer:  w,
		opts:    opts,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// WithAttrs returns a new ColorHandler with the given attributes added.
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithAttrs(attrs),
		writer:  h.writer,
		opts:    h.opts,
	}
}

// WithGroup returns a new ColorHandler with the given group name added.
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithGroup(name),
		writer:  h.writer,
		opts:    h.opts,
	}
}

// Handle formats and writes a log record with color coding.
func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	// Get color based on level
	levelColor := getLevelColor(r.Level)

	// Use custom formatting with color for entire line
	buf := make([]byte, 0, 1024)

	// Start with color code for entire line
	buf = append(buf, levelColor...)

	buf = append(buf, fmt.Sprintf("time=%s level=%s msg=%q",
		r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		r.Level.String(),
		r.Message)...)

	// Add attributes
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, fmt.Sprintf(" %s=%v", a.Key, a.Value)...)
		return true
	})

	// End with color reset
	buf = append(buf, colorReset...)
	buf = append(buf, '\n')

	_, err := h.writer.Write(buf)
	return err
}

func getLevelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return colorGray
	case slog.LevelInfo:
		return colorBlue
	case slog.LevelWarn:
		return colorYellow
	case slog.LevelError:
		return colorRed
	default:
		return colorReset
	}
}

// Logger is the global logger instance used throughout the application.
var Logger *slog.Logger

func init() {
	_ = godotenv.Load()
	level := getLogLevel()

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Choose the format based on the env variable LOG_FORMAT
	var handler slog.Handler
	logFormat := os.Getenv("LOG_FORMAT")
	noColor := os.Getenv("NO_COLOR") == "TRUE"

	switch {
	case logFormat == "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case noColor:
		// Use standard text handler without colors
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		// Use colored handler (default)
		handler = NewColorHandler(os.Stdout, opts)
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

func getLogLevel() slog.Level {
	// Default log level is INFO, can be overridden by LOG_LEVEL env variable
	switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// GetLogger returns the global logger instance.
func GetLogger() *slog.Logger {
	return Logger
}
