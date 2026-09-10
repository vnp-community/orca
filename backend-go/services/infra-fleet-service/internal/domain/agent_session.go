package domain

import (
	"errors"
	"time"
)

// AgentStatus is an AgentSession's lifecycle state — see
// usecase/agent_output_classifier.go (TASK-AG-05-04) for how transitions
// are detected.
type AgentStatus string

const (
	AgentStatusSpawning  AgentStatus = "spawning" // between SpawnAgent accept and first idle signal
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusWaiting   AgentStatus = "waiting"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusError     AgentStatus = "error"
	AgentStatusStopped   AgentStatus = "stopped"
)

// ErrAgentAlreadyRunning is returned by AgentSessionRepository.Create when a
// partial unique constraint (BR-AG-01: one non-terminal agent session per
// worktree+user) rejects the insert.
var ErrAgentAlreadyRunning = errors.New("domain: an agent session is already running for this worktree+user")

var (
	// ErrAgentSessionExpired — BR-AG-08: a session that hasn't been active
	// within the resume window can no longer be resumed.
	ErrAgentSessionExpired = errors.New("domain: agent session has expired (BR-AG-08)")
	// ErrAgentVersionMismatch — BR-AG-09: resume must run against the same
	// agent build the session was originally spawned with.
	ErrAgentVersionMismatch = errors.New("domain: agent version differs from the original session (BR-AG-09)")
)

// AgentSession is a specialization of TerminalSession — it references
// terminal_sessions.pty_id via PtyID rather than duplicating PTY bookkeeping.
// ConnectionID is the resolution key (mirrors TerminalSession.ConnectionID —
// resolveAgentSession, TASK-AG-02-*, re-resolves through it via
// ConnectionResolver exactly like resolveTerminalSession does); DevServerID
// is a display-only snapshot of the dev server resolved at spawn time, not
// itself used for lookups.
type AgentSession struct {
	ID, PtyID, TenantID, ConnectionID, WorktreeID, DevServerID string
	UserID, ModelID, AccountID                                 string
	ResumeOfSessionID                                          string // "" for a fresh start
	AgentVersion                                               string // dev server's agent_version at spawn time
	Status                                                     AgentStatus
	StartedAt, LastActiveAt                                    time.Time
	StoppedAt                                                  *time.Time

	// ResumeProviderSessionKey/ID — the underlying CLI's OWN
	// conversation/session id, captured from an agent.hook notification
	// (TASK-AG-03-05) — distinct from ID (this row's own primary key).
	ResumeProviderSessionKey string // "session_id" | "conversation_id"; "" if never captured
	ResumeProviderSessionID  string // the CLI's OWN session/conversation id — distinct from ID
}

// UsesStreamJSON reports whether this session's spawn used
// --output-format stream-json (Claude Code fresh spawns only) — see
// TASK-AG-05-04's two-track classifier for why this matters.
func (s AgentSession) UsesStreamJSON() bool {
	return s.ModelID == "claude" && s.ResumeOfSessionID == ""
}
