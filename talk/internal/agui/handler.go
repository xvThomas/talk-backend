package agui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/google/uuid"
)

// Handler handles AG-UI protocol HTTP requests.
type Handler struct {
	log    *slog.Logger
	chatFn ChatFunc
}

// ChatFunc is the function signature for processing a conversation turn.
// It receives the thread ID and user messages, and returns the assistant response.
type ChatFunc func(ctx context.Context, threadID string, messages []types.Message) (string, error)

// NewHandler creates an AG-UI protocol handler.
// If chatFn is nil, a placeholder response is used.
func NewHandler(log *slog.Logger, chatFn ChatFunc) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{log: log, chatFn: chatFn}
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
		response, err = h.chatFn(ctx, threadID, input.Messages)
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
