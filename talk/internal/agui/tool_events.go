package agui

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

var _ domain.ToolCallEventHandler = (*ToolCallEmitter)(nil)

// ToolCallEmitter emits AG-UI tool call lifecycle events to an SSE stream.
type ToolCallEmitter struct {
	sse    *SSEWriter
	logger *slog.Logger
}

// NewToolCallEmitter creates a ToolCallEmitter that writes tool call events to the given SSE writer.
func NewToolCallEmitter(sse *SSEWriter, logger *slog.Logger) *ToolCallEmitter {
	return &ToolCallEmitter{sse: sse, logger: logger}
}

// HandleToolCallStart emits TOOL_CALL_START and TOOL_CALL_ARGS events before tool execution.
func (e *ToolCallEmitter) HandleToolCallStart(ctx context.Context, event domain.ToolCallEvent) error {
	if ctx.Err() != nil {
		return nil
	}
	if err := e.sse.WriteEvent(ctx, events.NewToolCallStartEvent(event.ToolCall.ID, event.ToolCall.Name)); err != nil {
		return err
	}
	argsJSON, _ := json.Marshal(event.ToolCall.Input)
	return e.sse.WriteEvent(ctx, events.NewToolCallArgsEvent(event.ToolCall.ID, string(argsJSON)))
}

// HandleToolCallEnd emits a TOOL_CALL_END event after tool execution completes.
func (e *ToolCallEmitter) HandleToolCallEnd(ctx context.Context, event domain.ToolCallEndEvent) error {
	if ctx.Err() != nil {
		return nil
	}
	return e.sse.WriteEvent(ctx, events.NewToolCallEndEvent(event.ToolCall.ID))
}
