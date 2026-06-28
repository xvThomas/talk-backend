package agui

import (
	"context"
	"encoding/json"
	"errors"
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
// It receives the thread ID, model alias, user messages, and chat options
// containing the SSE writer for event emission via the pipeline.
type ChatFunc func(ctx context.Context, threadID string, modelAlias string, messages []types.Message, opts ChatOptions) error

// ChatOptions carries per-request options for the chat function.
type ChatOptions struct {
	SSEWriter      *SSEWriter
	ThinkingEffort domain.ThinkingEffort
}

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

	if len(input.Messages) == 0 && len(input.Resume) == 0 {
		http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
		return
	}

	// Handle resume after interrupt: validate and route.
	if len(input.Resume) > 0 {
		// ThreadID is required for resume — cannot correlate without it.
		if input.ThreadID == "" {
			http.Error(w, `{"error":"threadId is required when resuming an interrupt"}`, http.StatusBadRequest)
			return
		}

		// Validate all resume entries have a known status and no conflicts.
		hasResolved := false
		hasCancelled := false
		for _, entry := range input.Resume {
			switch entry.Status {
			case types.ResumeStatusResolved:
				hasResolved = true
			case types.ResumeStatusCancelled:
				hasCancelled = true
			default:
				http.Error(w, `{"error":"unknown resume status, expected resolved or cancelled"}`, http.StatusBadRequest)
				return
			}
		}
		if hasResolved && hasCancelled {
			http.Error(w, `{"error":"conflicting resume statuses: cannot mix resolved and cancelled"}`, http.StatusBadRequest)
			return
		}

		if hasCancelled {
			sse, sseErr := NewSSEWriter(w, h.log)
			if sseErr != nil {
				http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
				return
			}
			threadID := input.ThreadID
			runID := input.RunID
			if runID == "" {
				runID = uuid.New().String()
			}
			_ = sse.WriteEvent(r.Context(), events.NewRunStartedEvent(threadID, runID))
			_ = sse.WriteEvent(r.Context(), events.NewRunFinishedEvent(threadID, runID))
			return
		}

		// Resolved: inject a continuation user message if messages are empty.
		if len(input.Messages) == 0 {
			input.Messages = []types.Message{{
				Role:    "user",
				Content: "Please continue where you left off.",
			}}
		}
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
	if h.chatFn != nil {
		thinkingEffort := extractThinkingEffort(input.ForwardedProps)
		err = h.chatFn(ctx, threadID, modelAlias, input.Messages, ChatOptions{
			SSEWriter:      sse,
			ThinkingEffort: thinkingEffort,
		})
		if ctx.Err() != nil {
			h.log.Debug("client disconnected during chat", slog.String("thread_id", threadID))
			return
		}
		if err != nil {
			if errors.Is(err, domain.ErrMaxToolIterations) {
				interruptID := uuid.New().String()
				finishedEvent := events.NewRunFinishedEventWithOptions(threadID, runID,
					events.WithOutcome(events.RunFinishedOutcome{
						Type: events.RunFinishedOutcomeTypeInterrupt,
						Interrupts: []types.Interrupt{{
							ID:      interruptID,
							Reason:  "talk:max_iterations",
							Message: "I reached the tool call limit. Click Continue to let me keep working.",
						}},
					}),
				)
				_ = sse.WriteEvent(ctx, finishedEvent)
				return
			}
			_ = sse.WriteEvent(ctx, events.NewRunErrorEvent(err.Error()))
			return
		}
	}

	// RUN_FINISHED
	if ctx.Err() != nil {
		h.log.Debug("client disconnected before RUN_FINISHED", slog.String("thread_id", threadID))
		return
	}
	if err := sse.WriteEvent(ctx, events.NewRunFinishedEvent(threadID, runID)); err != nil {
		h.log.Error("writing RUN_FINISHED", slog.String("error", err.Error()))
		return
	}
}

// extractModelAlias extracts the model alias from forwardedProps.
// Returns an error if forwardedProps is not a map or the model key is missing/empty.
func extractModelAlias(forwardedProps any) (string, error) {
	if forwardedProps == nil {
		return "", fmt.Errorf("the model field is required")
	}

	props, ok := forwardedProps.(map[string]any)
	if !ok {
		return "", fmt.Errorf("the model field is required")
	}

	modelRaw, exists := props["model"]
	if !exists {
		return "", fmt.Errorf("the model field is required")
	}

	model, ok := modelRaw.(string)
	if !ok || model == "" {
		return "", fmt.Errorf("the model field is required")
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

// extractThinkingEffort extracts the thinking effort level from forwardedProps.
// Returns the zero value (empty string = off) for missing, invalid, or unrecognized values.
func extractThinkingEffort(forwardedProps any) domain.ThinkingEffort {
	if forwardedProps == nil {
		return ""
	}
	props, ok := forwardedProps.(map[string]any)
	if !ok {
		return ""
	}
	raw, exists := props["thinkingEffort"]
	if !exists {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	switch value {
	case "low":
		return domain.ThinkingLow
	case "medium":
		return domain.ThinkingMedium
	case "high":
		return domain.ThinkingHigh
	default:
		return ""
	}
}
