package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// StepApproval is a decision a running workflow step waits for from a person in
// Orca (TDD-WF-AGENT-001 §7.5) — e.g. a tdd-agent gate's approval. Unlike
// Approval (template publishing), it belongs to one step of one execution and
// is bound to the exact content shown (DecisionHash): a newer decision for the
// same step supersedes it instead of being approved blindly.
type StepApproval struct {
	ID           string
	TenantID     string
	ExecutionID  string
	StepID       string
	ProjectID    string
	Kind         string          // "tdd_agent"
	Subject      json.RawMessage // kind-specific: {runId, gate, role}
	Decision     json.RawMessage // what the approver reviews
	DecisionHash string
	Status       StepApprovalStatus
	ResolvedBy   string
	Rationale    string
	CreatedAt    time.Time
	ResolvedAt   *time.Time
	SubmittedAt  *time.Time // when the decision was delivered to the external system
}

type StepApprovalStatus string

const (
	StepApprovalPending    StepApprovalStatus = "pending"
	StepApprovalApproved   StepApprovalStatus = "approved"
	StepApprovalRejected   StepApprovalStatus = "rejected"
	StepApprovalSuperseded StepApprovalStatus = "superseded"
)

var (
	ErrStepApprovalInvalid     = errors.New("domain: step approval needs tenant, execution, step, kind and decision hash")
	ErrStepApprovalNotPending  = errors.New("domain: step approval is not pending")
	ErrStepApprovalStale       = errors.New("domain: the decision changed since it was shown; reload it")
	ErrStepApprovalNotFound    = errors.New("domain: step approval not found")
	ErrStepApprovalNoRationale = errors.New("domain: rationale is required")
)

func NewStepApproval(id, tenantID, executionID, stepID, projectID, kind string, subject, decision json.RawMessage, decisionHash string, now time.Time) (StepApproval, error) {
	if tenantID == "" || executionID == "" || stepID == "" || kind == "" || decisionHash == "" {
		return StepApproval{}, ErrStepApprovalInvalid
	}
	if len(subject) == 0 {
		subject = json.RawMessage("{}")
	}
	if len(decision) == 0 {
		decision = json.RawMessage("null")
	}
	return StepApproval{
		ID: id, TenantID: tenantID, ExecutionID: executionID, StepID: stepID, ProjectID: projectID, Kind: kind,
		Subject: subject, Decision: decision, DecisionHash: decisionHash, Status: StepApprovalPending, CreatedAt: now,
	}, nil
}

// Resolve records the person's decision. decisionHash must be the hash of the decision they were shown.
func (a *StepApproval) Resolve(approve bool, userID, decisionHash, rationale string, now time.Time) error {
	if a.Status != StepApprovalPending {
		return ErrStepApprovalNotPending
	}
	if decisionHash != a.DecisionHash {
		return ErrStepApprovalStale
	}
	if rationale == "" {
		return ErrStepApprovalNoRationale
	}
	a.Status = StepApprovalRejected
	if approve {
		a.Status = StepApprovalApproved
	}
	a.ResolvedBy, a.Rationale, a.ResolvedAt = userID, rationale, &now
	return nil
}

// Supersede retires a pending approval whose decision the external system replaced.
func (a *StepApproval) Supersede(now time.Time) {
	if a.Status == StepApprovalPending {
		a.Status, a.ResolvedAt = StepApprovalSuperseded, &now
	}
}

// MarkSubmitted records that an approved decision reached the external system.
func (a *StepApproval) MarkSubmitted(now time.Time) { a.SubmittedAt = &now }
