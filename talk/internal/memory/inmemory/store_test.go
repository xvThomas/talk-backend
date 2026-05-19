package inmemory

import (
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"testing"
)

func TestStore_AddAndAll(t *testing.T) {
	s := NewInMemoryStore("sess-1", "anonymous")
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hello"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "world"})

	msgs := s.All()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Error("unexpected message contents")
	}
}
