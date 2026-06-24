package agui

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
)

// SSEWriter writes AG-UI events to an HTTP response as Server-Sent Events.
type SSEWriter struct {
	w      http.ResponseWriter
	writer *sse.SSEWriter
}

// NewSSEWriter creates an SSEWriter after setting the appropriate SSE headers.
// Returns an error if the ResponseWriter does not support flushing.
func NewSSEWriter(w http.ResponseWriter, log *slog.Logger) (*SSEWriter, error) {
	if _, ok := w.(http.Flusher); !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if log == nil {
		log = slog.Default()
	}

	return &SSEWriter{
		w:      w,
		writer: sse.NewSSEWriter().WithLogger(log),
	}, nil
}

// WriteEvent serializes an AG-UI event and writes it as an SSE data frame.
func (s *SSEWriter) WriteEvent(ctx context.Context, event events.Event) error {
	return s.writer.WriteEvent(ctx, s.w, event)
}
