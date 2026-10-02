package wscompat

import "context"

// ToolStreamScope is the terminal stream registry of ONE MCP tool session, the
// counterpart of the per-WebSocket-connection registry (handler.go). terminal.*
// and agent.* handlers need a registry on ctx; /mcp has no WS connection, so
// the tool layer owns one scope per MCP session and closes it with the session.
// Closing ends the AttachPty streams only; killing the PTYs is the caller's job
// (the daemon keeps a PTY alive without any attached stream).
type ToolStreamScope struct{ reg *terminalStreamRegistry }

func NewToolStreamScope() *ToolStreamScope { return &ToolStreamScope{reg: newTerminalStreamRegistry()} }

// Context returns ctx carrying this scope's registry.
func (s *ToolStreamScope) Context(ctx context.Context) context.Context {
	return terminalStreamsContext(ctx, s.reg)
}

// Attached reports whether a live AttachPty stream exists for ptyID.
func (s *ToolStreamScope) Attached(ptyID string) bool {
	_, ok := s.reg.get(ptyID)
	return ok
}

// Detach cancels and forgets one stream.
func (s *ToolStreamScope) Detach(ptyID string) {
	if e, ok := s.reg.remove(ptyID); ok {
		e.cancel()
	}
}

// Close cancels every stream of the scope.
func (s *ToolStreamScope) Close() {
	s.reg.mu.Lock()
	entries := make([]*terminalStreamEntry, 0, len(s.reg.streams))
	for _, e := range s.reg.streams {
		entries = append(entries, e)
	}
	s.reg.streams = map[string]*terminalStreamEntry{}
	s.reg.mu.Unlock()
	for _, e := range entries {
		e.cancel()
	}
}
