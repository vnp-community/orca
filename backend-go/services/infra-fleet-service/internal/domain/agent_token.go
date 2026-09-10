package domain

import (
	"errors"
	"time"
)

// AgentToken is a persistent, named, per-DevServer agent bearer token
// (BL-AWS-03) — the durable counterpart to adapter/agentwsserver's
// ephemeral Registry/TokenIssuer bootstrap slots. Exactly one of
// TokenHash/CredentialRefID is set, mirroring
// infra.agent_tokens.exactly_one_secret_ref (migrations/0007_agent_tokens):
//   - TokenHash is set for direct-websocket rows — Orca only ever compares
//     an agent-presented value against this hash (see SOL-AWS-03).
//   - CredentialRefID is set for relay-websocket rows — Orca must itself
//     present the plaintext outbound, so the secret lives in
//     credential-broker-service/Vault, referenced here by id only (see
//     SOL-AWS-01).
type AgentToken struct {
	ID              string
	TenantID        string
	DevServerID     string
	Name            string
	TokenHash       string // set for direct-websocket rows only
	CredentialRefID string // set for relay-websocket rows only
	CreatedAt       time.Time
	LastUsedAt      *time.Time
	RevokedAt       *time.Time
}

// Active reports whether this token has not been revoked.
func (t AgentToken) Active() bool { return t.RevokedAt == nil }

var (
	// ErrEmptyAgentTokenName is returned when Name is empty — an unnamed
	// token can't be distinguished in the "10 named tokens" admin UI
	// (BL-AWS-03).
	ErrEmptyAgentTokenName = errors.New("domain: agent token name is required")
	// ErrAgentTokenLimitReached is returned when a DevServer already has
	// MaxActiveAgentTokensPerDevServer active tokens.
	ErrAgentTokenLimitReached = errors.New("domain: a DevServer may have at most 10 active agent tokens")
)

// MaxActiveAgentTokensPerDevServer is BL-AWS-03's cap on the admin UI's
// "10 named tokens" flexibility — not a per-connection concurrency limit
// (a relay-websocket DevServer dials with exactly one active token at a
// time; see SOL-AWS-01's ActiveForDevServer doc comment).
const MaxActiveAgentTokensPerDevServer = 10
