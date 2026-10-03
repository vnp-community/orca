package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SubjectTerminalClosed is published (INFRAFLEET stream, orca.infrafleet.>) by
// the outbox relay when a terminal session transitions open -> closed.
const SubjectTerminalClosed = "orca.infrafleet.terminal.closed"

// Close reasons carried on the event. Anything else is recorded as
// TerminalCloseReasonUser so the field stays a closed vocabulary.
const (
	TerminalCloseReasonUser          = "user"
	TerminalCloseReasonIdle          = "idle"
	TerminalCloseReasonSessionClosed = "session_closed"
)

var terminalClosedNamespace = uuid.MustParse("3c8e1d52-91b4-5f0a-8e47-6a2d9b17c0f3")

// NormalizeTerminalCloseReason maps unknown or empty reasons to "user".
func NormalizeTerminalCloseReason(reason string) string {
	switch reason {
	case TerminalCloseReasonIdle, TerminalCloseReasonSessionClosed, TerminalCloseReasonUser:
		return reason
	}
	return TerminalCloseReasonUser
}

// TerminalClosedEventID is a UUIDv5 of tenant+pty+"closed": a retried close
// maps to the same outbox primary key, so it can never enqueue twice.
func TerminalClosedEventID(tenantID, ptyID string) string {
	return uuid.NewSHA1(terminalClosedNamespace, []byte(tenantID+":"+ptyID+":closed")).String()
}

type terminalClosedOrigin struct {
	Type         string `json:"type"`
	ClientName   string `json:"client_name"`
	MCPSessionID string `json:"mcp_session_id"`
}

type terminalClosedPayload struct {
	TenantID string                `json:"tenant_id"`
	PtyID    string                `json:"pty_id"`
	UserID   string                `json:"user_id"`
	Reason   string                `json:"reason"`
	Actor    string                `json:"actor,omitempty"`
	Origin   *terminalClosedOrigin `json:"origin,omitempty"`
}

// NewTerminalClosedEvent builds the outbox row for a close. The payload names
// owner, origin and reason only — never the cwd, command or output, because
// consumers store notifications.
func NewTerminalClosedEvent(s TerminalSession, reason, actor string, now time.Time) (OutboxEvent, error) {
	userID := s.CreatedByUserID
	var origin *terminalClosedOrigin
	if !s.Origin.IsEmpty() {
		origin = &terminalClosedOrigin{Type: s.Origin.Type, ClientName: s.Origin.ClientName, MCPSessionID: s.Origin.MCPSessionID}
		if s.Origin.UserID != "" {
			userID = s.Origin.UserID
		}
	}
	payload, err := json.Marshal(terminalClosedPayload{
		TenantID: s.TenantID, PtyID: s.PtyID, UserID: userID, Reason: NormalizeTerminalCloseReason(reason), Actor: actor, Origin: origin,
	})
	if err != nil {
		return OutboxEvent{}, err
	}
	return OutboxEvent{
		ID: TerminalClosedEventID(s.TenantID, s.PtyID), TenantID: s.TenantID, Subject: SubjectTerminalClosed,
		OccurredAt: now.UTC(), PayloadJSON: payload,
	}, nil
}
