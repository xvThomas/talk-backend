package domain

import sharedDomain "github.com/xvThomas/LLMClientWrapper/talk-libs/domain"

// Re-export shared domain types used by the conversation engine.
type Tool = sharedDomain.Tool
type TypedTool[TInput any, TOutput any] = sharedDomain.TypedTool[TInput, TOutput]
type ToolPrompt = sharedDomain.ToolPrompt
type ToolPromptMessage = sharedDomain.ToolPromptMessage
