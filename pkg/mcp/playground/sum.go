package playground

import (
	"context"
	"talks/internal/domain"
)

type SumToolInput struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

type SumToolOutput struct {
	Sum float64 `json:"sum"`
}

type SumTool struct{}

var _ domain.TypedTool[SumToolInput, SumToolOutput] = (*SumTool)(nil) // ensure SumTool implements domain.TypedTool
var _ domain.MCPPromptProvider = (*SumTool)(nil)                 // ensure SumTool implements domain.MCPPromptProvider	

func NewSumTool() domain.TypedTool[SumToolInput, SumToolOutput] {
	return &SumTool{}
}

func (t *SumTool) Name() string {
	return "sum"
}

func (t *SumTool) Description() string {
	return "Calcule la somme de deux nombres a et b"
}

func (t *SumTool) Call(_ context.Context, input SumToolInput) (SumToolOutput, error) {
	return SumToolOutput{Sum: input.A + input.B}, nil
}

func (t *SumTool) Prompts() []domain.ToolPrompt {
	return []domain.ToolPrompt{
		{
			Name:        "sum_example",
			Description: "Exemple d'utilisation de l'outil sum",
			Messages: []domain.ToolPromptMessage{
				{Role: "user", Text: "Calcule 3 + 4"},
				{Role: "assistant", Text: "7"},
			},
		},
	}
}
