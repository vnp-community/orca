package domain

import (
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Session states and close reasons (BE-MCP-SOL-004).
const (
	SessionInitializing = "initializing"
	SessionReady        = "ready"
	SessionClosed       = "closed"

	SubjectSessionClosed = "orca.mcp.session.closed"

	CloseReasonClientDelete = "client_delete"
	CloseReasonIdle         = "idle"
	CloseReasonUser         = "user"
	CloseReasonAdmin        = "admin"
	CloseReasonKillSwitch   = "kill_switch"
	CloseReasonTokenRevoked = "token_revoked"

	CodeStreamLimit = "MCP_STREAM_LIMIT"

	// StreamLiveWindow: a stream whose heartbeat is older is not counted.
	StreamLiveWindow = 90 * time.Second
)

var closeReasons = map[string]bool{
	CloseReasonClientDelete: true, CloseReasonIdle: true, CloseReasonUser: true,
	CloseReasonAdmin: true, CloseReasonKillSwitch: true, CloseReasonTokenRevoked: true,
}

// ValidCloseReason reports whether r is one of the documented reasons.
func ValidCloseReason(r string) bool { return closeReasons[r] }

// Session is one MCP session. The Mcp-Session-Id secret is not part of it.
type Session struct {
	ID, TenantID, UserID                        string
	SecretHash                                  []byte
	ClientID, ClientName, ClientVersion         string
	GrantID, TokenID, ProtocolVersion, LogLevel string
	CapabilitiesJSON                            []byte
	State, CloseReason                          string
	ToolCalls                                   int64
	CreatedAt, LastSeenAt                       time.Time
	ActiveStreams                               int
}

// ErrStreamLimit is mapped by the gateway to HTTP 429.
func ErrStreamLimit() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeStreamLimit, "too many open streams", nil)
}

// SessionClosedEvent is what the idle reaper and CloseSession enqueue.
type ClosedSession struct {
	ID, TenantID, UserID string
}
