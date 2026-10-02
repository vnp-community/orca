package tools

import "context"

// ToolSession scopes the contexts of tool calls (and the ack-only drain of
// streamChannel handlers) to one owner, so closing it cancels everything
// in flight. Per-MCP-session instances (ToolSessions) additionally own the
// terminal/agent streams their tools started; pack 1/2 tools only need
// cancellation.
type ToolSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	// pty is set only on per-MCP-session sessions made by ToolSessions
	// (BE-MCP-SOL-009): the attached PTY streams and rings that session owns.
	pty *ptyState
}

// NewToolSession derives a session from parent.
func NewToolSession(parent context.Context) *ToolSession {
	ctx, cancel := context.WithCancel(parent)
	return &ToolSession{ctx: ctx, cancel: cancel}
}

// Context is cancelled by Close.
func (s *ToolSession) Context() context.Context { return s.ctx }

// Close cancels in-flight calls; it is idempotent.
func (s *ToolSession) Close() { s.cancel() }
