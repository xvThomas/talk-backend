package prompts

import "github.com/xvThomas/talk-backend/talk-libs/mcpserver"

// Sum instructs the LLM to use the sum tool for adding two integers.
var Sum = mcpserver.Prompt{
	Name:        "sum",
	Description: "Add two integers together using the sum tool",
	Arguments: []mcpserver.PromptArgument{
		{Name: "a", Description: "First integer", Required: true},
		{Name: "b", Description: "Second integer", Required: true},
	},
	Messages: []mcpserver.PromptMessage{
		{
			Role: "user",
			Text: "Compute the sum of {{a}} and {{b}} using the sum tool. " +
				"Return the result as a single integer value.",
		},
	},
}
