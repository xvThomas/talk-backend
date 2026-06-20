package tools

import (
	"context"

	"github.com/xvThomas/talk-backend/talk-libs/mcpserver"
)

// SumToolInput holds the two integers to add.
type SumToolInput struct {
	A int `json:"a" description:"First integer"`
	B int `json:"b" description:"Second integer"`
}

// SumToolOutput holds the result of the addition.
type SumToolOutput struct {
	Result int `json:"result" description:"Sum of a and b"`
}

// SumTool computes the sum of two integers.
type SumTool struct{}

var _ mcpserver.MCPTool[SumToolInput, SumToolOutput] = (*SumTool)(nil)

func NewSumTool() *SumTool {
	return &SumTool{}
}

func (t *SumTool) Name() string        { return "sum" }
func (t *SumTool) Description() string { return "Compute the sum of two integers" }

func (t *SumTool) Call(_ context.Context, input SumToolInput) (SumToolOutput, error) {
	return SumToolOutput{Result: input.A + input.B}, nil
}
