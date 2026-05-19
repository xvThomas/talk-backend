package domain

// Role represents the author of a message in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall describes a tool invocation requested by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult holds the output of an executed tool call.
type ToolResult struct {
	ToolCallID string
	Content    string
}

// Message is a single entry in a conversation.
type Message struct {
	Role        Role
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
	TurnID      string // unique turn identifier used for reconciliation across sources
}
