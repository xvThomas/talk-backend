package playground

import (
	"context"
	"testing"
)

func TestSumTool_Metadata(t *testing.T) {
	tool := NewSumTool()
	if tool.Name() != "sum" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestSumTool_Call(t *testing.T) {
	tests := []struct {
		name    string
		a, b    float64
		wantSum float64
	}{
		{"positive integers", 3, 5, 8},
		{"floats", 1.5, 2.3, 3.8},
		{"zero values", 0, 0, 0},
		{"negative numbers", -4, 2, -2},
		{"both negative", -3, -7, -10},
	}

	tool := NewSumTool()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Call(context.Background(), SumToolInput{A: tt.a, B: tt.b})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Sum != tt.wantSum {
				t.Errorf("expected Sum %f, got %f", tt.wantSum, result.Sum)
			}
		})
	}
}

func TestSumTool_Prompts(t *testing.T) {
	tool := &SumTool{}
	prompts := tool.Prompts()

	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	p := prompts[0]
	if p.Name == "" {
		t.Error("prompt name should not be empty")
	}
	if p.Description == "" {
		t.Error("prompt description should not be empty")
	}
	if len(p.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	for i, m := range p.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			t.Errorf("messages[%d]: unexpected role %q", i, m.Role)
		}
		if m.Text == "" {
			t.Errorf("messages[%d]: text should not be empty", i)
		}
	}
}
