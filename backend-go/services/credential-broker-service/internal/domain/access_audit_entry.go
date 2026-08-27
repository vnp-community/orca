package domain

import (
	"errors"
	"time"
)

// Action is the operation an AccessAuditEntry records — mirrors the four
// RPCs credentialbrokerv1.CredentialBrokerService exposes (see
// proto/orca/credentialbroker/v1/credentialbroker.proto).
type Action string

const (
	ActionWrite   Action = "write"
	ActionResolve Action = "resolve"
	ActionRotate  Action = "rotate"
	ActionRevoke  Action = "revoke"
)

// Valid reports whether a is one of the known action values.
func (a Action) Valid() bool {
	switch a {
	case ActionWrite, ActionResolve, ActionRotate, ActionRevoke:
		return true
	default:
		return false
	}
}

var (
	ErrEmptyCredentialID    = errors.New("domain: credential_id is required")
	ErrEmptyAccessorService = errors.New("domain: accessor_service is required")
	ErrInvalidAction        = errors.New("domain: invalid audit action")
)

// AccessAuditEntry is an append-only record of one access to a credential —
// never a secret value, never updated or deleted after insert. Per
// credential-broker-service.md §8: "A credential access without a
// corresponding access_audit_log row is a compliance gap, not a
// degraded-but-acceptable outcome" — internal/usecase treats a failure to
// persist one of these as a failure of the whole operation, never a
// best-effort side effect (see internal/usecase's appendAudit helper).
type AccessAuditEntry struct {
	ID              int64
	CredentialID    string
	AccessorService string // resolved from mTLS/JWT identity, never client-asserted — see internal/adapter/grpc
	Action          Action
	OccurredAt      time.Time
}

// NewAccessAuditEntry constructs an AccessAuditEntry, enforcing the
// invariants an audit row must satisfy to be a meaningful compliance record.
func NewAccessAuditEntry(credentialID, accessorService string, action Action, occurredAt time.Time) (AccessAuditEntry, error) {
	if credentialID == "" {
		return AccessAuditEntry{}, ErrEmptyCredentialID
	}
	if accessorService == "" {
		return AccessAuditEntry{}, ErrEmptyAccessorService
	}
	if !action.Valid() {
		return AccessAuditEntry{}, ErrInvalidAction
	}
	return AccessAuditEntry{
		CredentialID:    credentialID,
		AccessorService: accessorService,
		Action:          action,
		OccurredAt:      occurredAt,
	}, nil
}
