package domain

import "time"

// TerminalSession is the domain view of one PTY/terminal session spawned on
// a dev server's agent — mirrors orca.infrafleet.v1.TerminalSession and
// migrations/0005_terminal_sessions. ConnectionID is empty for a host-local
// session (see proto's SpawnTerminalSessionRequest doc comment); every
// session spawned through usecase.SpawnTerminalSession today is
// connection-bound, since host-local execution has no local pty adapter in
// this service (see that usecase's doc comment).
type TerminalSession struct {
	PtyID        string
	TenantID     string
	ConnectionID string
	Cwd          string
	CreatedAt    time.Time
	LastActiveAt time.Time
	// CreatedByUserID is threaded from SpawnTerminalSessionInput — empty for
	// pre-migration rows and for any caller that doesn't carry a resolved
	// user identity yet (see usecase.SpawnTerminalSession). Not included in
	// IsZero's check below: an old row legitimately has it empty without
	// being an absent session.
	CreatedByUserID string
	// ClosedAt is nil while the session is open — set by
	// usecase.KillTerminalSession, mirrors Connection/DevServer's convention
	// of a real Go zero value (nil pointer) over a sentinel time, per
	// InspectTerminalProcessResponse's "known" pattern elsewhere in this
	// service: an honest absence, not a fabricated value.
	ClosedAt *time.Time
}

// Touch bumps LastActiveAt to now — called by every usecase that observes
// activity on a session (write, resize, explicit focus) so
// ListTerminalSessions/idle-cleanup sweeps have a real signal to read.
func (t *TerminalSession) Touch(now time.Time) {
	t.LastActiveAt = now
}

// IsZero reports whether t is the zero-value TerminalSession — mirrors
// DevServer.IsZero/Connection.IsZero's convention for "not found" signaling
// without a pointer.
func (t TerminalSession) IsZero() bool {
	return t.PtyID == "" && t.TenantID == "" && t.ConnectionID == "" && t.Cwd == "" && t.CreatedAt.IsZero() && t.LastActiveAt.IsZero() && t.ClosedAt == nil
}

// TruncatedForMobile applies BR-MB-15's 500-char cap at the point of
// exposure — keeps the buffer's internal size independent of the mobile
// contract, so a future non-mobile consumer wanting more context isn't
// retroactively capped by this rule. Tail-truncated: keeps the MOST RECENT
// bytes, not the head.
func TruncatedForMobile(lastOutput []byte) string {
	s := string(lastOutput)
	if len(s) <= 500 {
		return s
	}
	return s[len(s)-500:]
}
