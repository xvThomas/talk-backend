package agui

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/google/uuid"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

var _ domain.MessageEventHandler = (*AGUIEmitter)(nil)

// AGUIEmitter emits all AG-UI content events (text messages and tool calls)
// to an SSE stream. It implements domain.MessageEventHandler.
type AGUIEmitter struct {
	sse *SSEWriter
	log *slog.Logger
}

// NewAGUIEmitter creates an AGUIEmitter that writes content events to the given SSE writer.
func NewAGUIEmitter(sse *SSEWriter, log *slog.Logger) *AGUIEmitter {
	if log == nil {
		log = slog.Default()
	}
	return &AGUIEmitter{sse: sse, log: log}
}

// HandleMessageEvent emits REASONING_* events for thinking content (any assistant message),
// then TEXT_MESSAGE_START → TEXT_MESSAGE_CONTENT → TEXT_MESSAGE_END for final assistant messages
// (no tool calls, non-empty content).
func (e *AGUIEmitter) HandleMessageEvent(ctx context.Context, event domain.MessageEvent) error {
	if event.Role != domain.RoleAssistant {
		return nil
	}

	// Emit reasoning events if thinking content is present (independent of tool calls).
	if event.Thinking != "" {
		reasoningID := uuid.New().String()
		if err := e.writeEvent(ctx, events.NewReasoningStartEvent(reasoningID)); err != nil {
			return nil
		}
		if err := e.writeEvent(ctx, events.NewReasoningMessageStartEvent(reasoningID, "reasoning")); err != nil {
			return nil
		}
		if err := e.writeEvent(ctx, events.NewReasoningMessageContentEvent(reasoningID, event.Thinking)); err != nil {
			return nil
		}
		if err := e.writeEvent(ctx, events.NewReasoningMessageEndEvent(reasoningID)); err != nil {
			return nil
		}
		if err := e.writeEvent(ctx, events.NewReasoningEndEvent(reasoningID)); err != nil {
			return nil
		}
	}

	// Emit text message events only for final messages (no tool calls, non-empty content).
	if len(event.ToolCalls) > 0 {
		return nil
	}
	if event.Content == "" {
		return nil
	}

	messageID := uuid.New().String()

	if err := e.writeEvent(ctx, events.NewTextMessageStartEvent(messageID, events.WithRole("assistant"))); err != nil {
		return nil
	}
	if err := e.writeEvent(ctx, events.NewTextMessageContentEvent(messageID, event.Content)); err != nil {
		return nil
	}
	if err := e.writeEvent(ctx, events.NewTextMessageEndEvent(messageID)); err != nil {
		return nil
	}
	return nil
}

// HandleTurnEvent is a no-op for the SSE emitter.
func (e *AGUIEmitter) HandleTurnEvent(_ context.Context, _ domain.TurnEvent) error {
	return nil
}

// HandleToolCallStart emits TOOL_CALL_START and TOOL_CALL_ARGS events before tool execution.
func (e *AGUIEmitter) HandleToolCallStart(ctx context.Context, event domain.ToolCallEvent) error {
	if err := e.writeEvent(ctx, events.NewToolCallStartEvent(event.ToolCall.ID, event.ToolCall.Name)); err != nil {
		return nil
	}
	argsJSON, _ := json.Marshal(event.ToolCall.Input)
	if err := e.writeEvent(ctx, events.NewToolCallArgsEvent(event.ToolCall.ID, string(argsJSON))); err != nil {
		return nil
	}
	return nil
}

// HandleToolCallEnd emits a TOOL_CALL_END event after tool execution completes.
func (e *AGUIEmitter) HandleToolCallEnd(ctx context.Context, event domain.ToolCallEndEvent) error {
	if err := e.writeEvent(ctx, events.NewToolCallEndEvent(event.ToolCall.ID)); err != nil {
		return nil
	}
	return nil
}

// writeEvent checks for context cancellation, writes the event, and handles errors
// according to the best-effort policy: log unexpected errors, return nil always.
func (e *AGUIEmitter) writeEvent(ctx context.Context, event events.Event) error {
	if ctx.Err() != nil {
		e.log.Debug("skipping SSE write, context canceled")
		return ctx.Err()
	}
	if err := e.sse.WriteEvent(ctx, event); err != nil {
		if ctx.Err() != nil {
			e.log.Debug("SSE write failed due to client disconnect")
		} else {
			e.log.Warn("unexpected SSE write error", slog.String("error", err.Error()))
		}
		return err
	}
	return nil
}
