package agui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/google/uuid"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

// Handler handles AG-UI protocol HTTP requests.
type Handler struct {
	log             *slog.Logger
	chatFn          ChatFunc
	supportedModels []string
}

// ChatFunc is the function signature for processing a conversation turn.
// It receives the thread ID, model alias, user messages, and an optional tool call event handler
// for emitting tool call lifecycle events to the SSE stream.
type ChatFunc func(ctx context.Context, threadID string, modelAlias string, messages []types.Message, toolHandler domain.ToolCallEventHandler) (string, error)

// NewHandler creates an AG-UI protocol handler.
// supportedModels lists valid model aliases for error messages.
// If chatFn is nil, a placeholder response is used.
func NewHandler(log *slog.Logger, chatFn ChatFunc, supportedModels []string) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{log: log, chatFn: chatFn, supportedModels: supportedModels}
}

// ServeHTTP handles POST /agent requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var input types.RunAgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	if len(input.Messages) == 0 {
		http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
		return
	}

	// Extract model alias from forwardedProps.
	modelAlias, err := extractModelAlias(input.ForwardedProps)
	if err != nil {
		sse, sseErr := NewSSEWriter(w, h.log)
		if sseErr != nil {
			http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
			return
		}
		msg := fmt.Sprintf("%s Available models: %s.", err.Error(), strings.Join(h.supportedModels, ", "))
		_ = sse.WriteEvent(r.Context(), events.NewRunErrorEvent(msg))
		return
	}

	// Validate model is in the supported list.
	if !h.isModelSupported(modelAlias) {
		sse, sseErr := NewSSEWriter(w, h.log)
		if sseErr != nil {
			http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
			return
		}
		msg := fmt.Sprintf("Unknown model %q. Available models: %s.", modelAlias, strings.Join(h.supportedModels, ", "))
		_ = sse.WriteEvent(r.Context(), events.NewRunErrorEvent(msg))
		return
	}

	sse, err := NewSSEWriter(w, h.log)
	if err != nil {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	threadID := input.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
	}

	runID := input.RunID
	if runID == "" {
		runID = uuid.New().String()
	}

	ctx := r.Context()

	// RUN_STARTED
	if err := sse.WriteEvent(ctx, events.NewRunStartedEvent(threadID, runID)); err != nil {
		h.log.Error("writing RUN_STARTED", slog.String("error", err.Error()))
		return
	}

	// Get response from chat function.
	var response string
	if h.chatFn != nil {
		toolEmitter := NewToolCallEmitter(sse, h.log)
		response, err = h.chatFn(ctx, threadID, modelAlias, input.Messages, toolEmitter)
		if ctx.Err() != nil {
			h.log.Debug("client disconnected during chat", slog.String("thread_id", threadID))
			return
		}
		if err != nil {
			_ = sse.WriteEvent(ctx, events.NewRunErrorEvent(err.Error()))
			return
		}
	} else {
		response = "Server is running but no chat function is configured."
	}

	messageID := uuid.New().String()

	// TEXT_MESSAGE_START
	if err := sse.WriteEvent(ctx, events.NewTextMessageStartEvent(messageID, events.WithRole(string(types.RoleAssistant)))); err != nil {
		h.log.Error("writing TEXT_MESSAGE_START", slog.String("error", err.Error()))
		return
	}

	// TEXT_MESSAGE_CONTENT
	if err := sse.WriteEvent(ctx, events.NewTextMessageContentEvent(messageID, response)); err != nil {
		h.log.Error("writing TEXT_MESSAGE_CONTENT", slog.String("error", err.Error()))
		return
	}

	// TEXT_MESSAGE_END
	if err := sse.WriteEvent(ctx, events.NewTextMessageEndEvent(messageID)); err != nil {
		h.log.Error("writing TEXT_MESSAGE_END", slog.String("error", err.Error()))
		return
	}

	// RUN_FINISHED
	if err := sse.WriteEvent(ctx, events.NewRunFinishedEvent(threadID, runID)); err != nil {
		h.log.Error("writing RUN_FINISHED", slog.String("error", err.Error()))
		return
	}
}

// extractModelAlias extracts the model alias from forwardedProps.
// Returns an error if forwardedProps is not a map or the model key is missing/empty.
func extractModelAlias(forwardedProps any) (string, error) {
	if forwardedProps == nil {
		return "", fmt.Errorf("the model field is required.")
	}

	props, ok := forwardedProps.(map[string]any)
	if !ok {
		return "", fmt.Errorf("the model field is required.")
	}

	modelRaw, exists := props["model"]
	if !exists {
		return "", fmt.Errorf("the model field is required.")
	}

	model, ok := modelRaw.(string)
	if !ok || model == "" {
		return "", fmt.Errorf("the model field is required.")
	}

	return model, nil
}

func (h *Handler) isModelSupported(alias string) bool {
	for _, m := range h.supportedModels {
		if m == alias {
			return true
		}
	}
	return false
}
