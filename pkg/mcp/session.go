package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// mcpSession represents an active Streamable HTTP session (MCP 2025-03-26).
type mcpSession struct {
	id       string
	sseConns sync.Map // map[chan []byte]struct{}
}

func (s *mcpSession) addConn(ch chan []byte) {
	s.sseConns.Store(ch, struct{}{})
}

func (s *mcpSession) removeConn(ch chan []byte) {
	s.sseConns.Delete(ch)
}

// hasConn reports whether at least one SSE GET /mcp connection is open.
func (s *mcpSession) hasConn() bool {
	found := false
	s.sseConns.Range(func(_, _ any) bool {
		found = true
		return false // stop iteration on first hit
	})
	return found
}

// push delivers a message to all registered SSE connections.
func (s *mcpSession) push(msg []byte) {
	s.sseConns.Range(func(key, _ any) bool {
		ch := key.(chan []byte)
		select {
		case ch <- msg:
		default:
		}
		return true
	})
}

// closeAll signals all registered SSE connections to close.
func (s *mcpSession) closeAll() {
	s.sseConns.Range(func(key, _ any) bool {
		close(key.(chan []byte))
		return true
	})
}

// sessionStore is a thread-safe map of session ID -> *mcpSession.
type sessionStore struct {
	sessions sync.Map
}

func (st *sessionStore) create() *mcpSession {
	var b [16]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s := &mcpSession{id: id}
	st.sessions.Store(id, s)
	return s
}

func (st *sessionStore) get(id string) (*mcpSession, bool) {
	v, ok := st.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*mcpSession), true
}

func (st *sessionStore) delete(id string) {
	if v, ok := st.sessions.LoadAndDelete(id); ok {
		v.(*mcpSession).closeAll()
	}
}
