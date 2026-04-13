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