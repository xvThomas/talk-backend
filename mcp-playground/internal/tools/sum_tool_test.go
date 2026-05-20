package tools

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
	tool := NewSumTool()

	tests := []struct {
		name string
		a, b int
		want int
	}{
		{"positive numbers", 2, 3, 5},
		{"zeros", 0, 0, 0},
		{"negative numbers", -1, -2, -3},
		{"mixed signs", -5, 10, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tool.Call(context.Background(), SumToolInput{A: tc.a, B: tc.b})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Result != tc.want {
				t.Errorf("got %d, want %d", out.Result, tc.want)
			}
		})
	}
}
