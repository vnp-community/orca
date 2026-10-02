package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Session state values (mirror mcp-service mcp.sessions.state).
const (
	SessionInitializing = "initializing"
	SessionReady        = "ready"
	SessionClosed       = "closed"
)

// Close reasons recorded with a closed session.
const (
	CloseClientDelete = "client_delete"
	CloseIdle         = "idle"
	CloseUser         = "user"
	CloseAdmin        = "admin"
	CloseKillSwitch   = "kill_switch"
)

var (
	// ErrSessionNotFound: unknown session (also used for "not yours" - callers
	// must never distinguish the two).
	ErrSessionNotFound = errors.New("mcpserver: session not found")
	// ErrStreamLimit: the cluster-wide SSE stream cap was hit (HTTP 429).
	ErrStreamLimit = errors.New("mcpserver: too many open streams")
	// ErrResumeGap: the replay buffer no longer holds the requested events.
	ErrResumeGap = errors.New("mcpserver: resume gap")
)

// SecretHash is what stores keep of an Mcp-Session-Id: the id itself is a bearer
// secret and never leaves the gateway (not in the DB, NATS subjects or logs).
func SecretHash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// SessionRecord is the durable view of one session. ID is the non-secret row
// id used by the UI, audit, kill switch and NATS subjects.
type SessionRecord struct {
	ID                                  string
	TenantID, UserID                    string
	ClientID, ClientName, ClientVersion string
	GrantID, TokenID, ProtocolVersion   string
	Capabilities                        json.RawMessage
	LogLevel, State                     string
}

// NewSession is what the gateway learns at `initialize`.
type NewSession struct {
	ClientName, ClientVersion, ProtocolVersion string
	Capabilities                               json.RawMessage
}

// SessionTouch updates last-seen and counters.
type SessionTouch struct {
	Ready          bool
	ToolCallsDelta int64
	LogLevel       string
}

// SessionStore is the durable session registry shared by all replicas
// (BE-MCP-SOL-004: grpcSessionStore over mcp-service; in-memory fallback).
// The SDK keeps the live connection in process memory; the store is what makes
// identity binding, idle expiry and cross-replica adoption possible.
type SessionStore interface {
	Create(ctx context.Context, p Principal, secretHash []byte, ns NewSession) (SessionRecord, error)
	// Lookup returns the record including State=="closed"; ErrSessionNotFound if unknown.
	Lookup(ctx context.Context, p Principal, secretHash []byte) (SessionRecord, error)
	// Touch returns the resulting state (closed => drop the session).
	Touch(ctx context.Context, p Principal, rowID string, t SessionTouch) (string, error)
	Close(ctx context.Context, p Principal, rowID, reason string) error
}

// StreamRegistry is an OPTIONAL capability of a SessionStore: cluster-exact
// SSE stream accounting (mcp-service session_streams).
type StreamRegistry interface {
	OpenStream(ctx context.Context, p Principal, rowID string, maxPerUser, maxPerTenant int) (streamID string, err error)
	HeartbeatStream(ctx context.Context, p Principal, streamID string) error
	CloseStream(ctx context.Context, p Principal, streamID string) error
}

// SignalBus carries non-durable control signals between replicas (core NATS).
// Loss is tolerated: signals are self-healing (idle expiry, client retry).
// common/eventbus.Ephemeral satisfies it.
type SignalBus interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, fn func(subject string, data []byte)) (unsubscribe func(), err error)
}

// Signal subjects (BE-MCP-SOL-004 section 1): session signals are keyed by row id.
const (
	signalSessionPrefix = "orca.ephemeral.mcp.session."
	signalTenantPrefix  = "orca.ephemeral.mcp.tenant."
)

// ResumableEventStore is an mcp.EventStore whose streams can also be followed
// from ANY replica, which is what lets a client that lost its POST stream on
// replica A resume (Last-Event-ID) on replica B and receive exactly the rest.
type ResumableEventStore interface {
	mcp.EventStore
	// Check reports ErrResumeGap if events after afterIdx were already dropped
	// (or the stream is unknown); it lets the caller answer 404 before it
	// commits to an SSE response.
	Check(ctx context.Context, sessionID, streamID string, afterIdx int) error
	// Follow delivers events of (session, stream) with ordinal > afterIdx in
	// order and then live ones, until fn returns stop=true/err or ctx ends.
	// It returns ErrResumeGap, before delivering anything, if events after
	// afterIdx were already dropped from the bounded buffer.
	Follow(ctx context.Context, sessionID, streamID string, afterIdx int, fn func(idx int, data []byte) (stop bool, err error)) error
	// Purge drops every buffered event of the session (authoritative close only).
	Purge(ctx context.Context, sessionID string) error
}

// SessionRecorder is an OPTIONAL extension of Recorder (type-asserted) so
// existing Recorder implementations keep compiling. BE-MCP-SOL-015 maps these to
// orca_mcp_sessions_active, orca_mcp_sse_streams_active, orca_mcp_resume_total{result}.
type SessionRecorder interface {
	SessionOpened()
	SessionClosed(reason string)
	StreamOpened()
	StreamClosed()
	Resume(result string) // ok | gap | unsupported
	RequestCancelled()
	ProgressCoalesced(dropped int)
}

// RequestInfo is the verified per-request context handed to Deps.RequestContext.
type RequestInfo struct {
	SessionID  string // non-secret row id (empty before the session is persisted)
	ClientName string
	Depth      int    // from the verified token's mcp_depth claim
	Root       string // from the verified token's mcp_root claim
}

// SessionClock lets tests drive idle expiry.
type SessionClock func() time.Time
