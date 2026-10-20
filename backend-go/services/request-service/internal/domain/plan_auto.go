package domain

import (
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// PlanAutoState is where a request's automatic Plan generation stands. The job is stored so it survives a restart and so
// the Plan tab can show the outcome after a reload.
type PlanAutoState string

const (
	PlanAutoPending    PlanAutoState = "pending"
	PlanAutoRunning    PlanAutoState = "running"
	PlanAutoCommitting PlanAutoState = "committing"
	PlanAutoDone       PlanAutoState = "done"
	PlanAutoGaveUp     PlanAutoState = "gave_up"
	PlanAutoSkipped    PlanAutoState = "skipped"
	// PlanAutoSuperseded: a person got there first (the request moved on, has a Plan, or has a newer Plan run).
	PlanAutoSuperseded PlanAutoState = "superseded"
)

var planAutoMoves = map[PlanAutoState][]PlanAutoState{
	PlanAutoPending:    {PlanAutoRunning, PlanAutoSkipped, PlanAutoGaveUp, PlanAutoSuperseded, PlanAutoPending},
	PlanAutoRunning:    {PlanAutoRunning, PlanAutoCommitting, PlanAutoPending, PlanAutoGaveUp, PlanAutoSuperseded},
	PlanAutoCommitting: {PlanAutoDone, PlanAutoGaveUp, PlanAutoSuperseded, PlanAutoCommitting},
}

// IsActive reports whether the worker still has something to do for the job.
func (s PlanAutoState) IsActive() bool {
	return s == PlanAutoPending || s == PlanAutoRunning || s == PlanAutoCommitting
}

// IsFinal reports whether the job is over; a final job never moves again.
func (s PlanAutoState) IsFinal() bool {
	switch s {
	case PlanAutoDone, PlanAutoGaveUp, PlanAutoSkipped, PlanAutoSuperseded:
		return true
	}
	return false
}

// CanMove reports whether a job in state s may be written as state to. The three active states may stay put because a
// job is rescheduled in the same state (a retry delay, the next poll of a run).
func (s PlanAutoState) CanMove(to PlanAutoState) bool {
	for _, t := range planAutoMoves[s] {
		if t == to {
			return true
		}
	}
	return false
}

// PlanAutoOrigin says who made the job: the status_changed event of an approval, a person pressing Retry, or a change
// request on a Plan that already exists (CR-REQ-045), whose revised Plan the worker commits on the person's behalf.
type PlanAutoOrigin string

const (
	PlanAutoOriginEvent    PlanAutoOrigin = "event"
	PlanAutoOriginRetry    PlanAutoOrigin = "retry"
	PlanAutoOriginRevision PlanAutoOrigin = "revision"
)

// RevisionAnchor anchors the job that commits a revision on the run that wrote it, so one run has one job.
func RevisionAnchor(runID string) string { return "rev-" + runID }

// PlanAutoJob is one request's automatic Plan generation. At most one job per request is active at a time.
type PlanAutoJob struct {
	TenantID  string
	RequestID string
	// Anchor makes the job (and its run idempotency keys) unique per cause; see SolutionAnchor, TypeConfirmAnchor and RetryAnchor.
	Anchor      string
	Origin      PlanAutoOrigin
	ProjectID   string
	State       PlanAutoState
	Attempt     int
	MaxAttempts int
	// RunID is the Plan run the job follows; empty while pending.
	RunID          string
	ReasonCode     string
	CommitAttempts int
	NextCheckAt    time.Time
	LeaseOwner     string
	LeaseExpiresAt *time.Time
	// RequestedBy is the person who pressed Retry; empty for an event job.
	RequestedBy string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PlanAutoKeyMaxLen is the width of analysis_runs.idempotency_key on MySQL.
const PlanAutoKeyMaxLen = 128

const planAutoKeyPrefix = "plan-auto-"

// PlanAutoKey is the idempotency key of the run for the given attempt: a retry needs a new key, because the run store
// hands back the earlier (failed) run for a key it already has.
func PlanAutoKey(anchor string, attempt int) (string, error) {
	if anchor == "" || attempt < 1 {
		return "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_AUTO_KEY_INVALID", "a plan-auto key needs an anchor and an attempt of 1 or more", nil)
	}
	key := planAutoKeyPrefix + anchor
	if attempt > 1 {
		key = fmt.Sprintf("%s-%d", key, attempt)
	}
	if len(key) > PlanAutoKeyMaxLen {
		return "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_AUTO_KEY_INVALID", fmt.Sprintf("a plan-auto key is at most %d characters", PlanAutoKeyMaxLen), nil)
	}
	return key, nil
}

// SolutionAnchor anchors the job of an approved analysis on the Solution that was approved.
func SolutionAnchor(solutionID string) string { return solutionID }

// TypeConfirmAnchor anchors the job of a type confirmation (types without an Analysis) on the event that carried it.
func TypeConfirmAnchor(eventID string) string { return "ty-" + eventID }

// RetryAnchor anchors a manual Retry; the caller supplies a fresh UUID.
func RetryAnchor(id string) string { return "retry-" + id }

// Run error codes the plan-auto worker classifies. The set of transient codes lives only in IsTransientPlanRunCode: a
// code a new worker starts to write belongs in that table, nowhere else.
const (
	PlanAutoCodeAIUnavailable   = "REQUEST_PLAN_AI_UNAVAILABLE"
	PlanAutoCodeCompleteTimeout = "REQUEST_AI_COMPLETE_TIMEOUT"
	PlanAutoCodeAIBusy          = "REQUEST_AI_BUSY"
	PlanAutoCodeOpenSpecTimeout = "REQUEST_OPENSPEC_TIMEOUT"
	PlanAutoCodeOpenSpecFailed  = "REQUEST_OPENSPEC_AGENT_FAILED"
	PlanAutoCodeRunInterrupted  = "REQUEST_SOLUTION_RUN_INTERRUPTED"
	PlanAutoCodeCommitFailed    = "REQUEST_PLAN_COMMIT_FAILED"
	PlanAutoCodeDisabled        = "REQUEST_PLAN_AUTO_DISABLED"
)

var transientPlanRunCodes = map[string]struct{}{
	PlanAutoCodeCompleteTimeout: {},
	PlanAutoCodeAIUnavailable:   {},
	PlanAutoCodeAIBusy:          {},
	PlanAutoCodeOpenSpecTimeout: {},
	PlanAutoCodeOpenSpecFailed:  {},
	PlanAutoCodeRunInterrupted:  {},
}

// IsTransientPlanRunCode reports whether a failed Plan run is worth one more attempt. Everything else (invalid output,
// missing configuration, exhausted quota, an unknown or empty code) would fail the same way and costs another run.
func IsTransientPlanRunCode(code string) bool {
	_, ok := transientPlanRunCodes[code]
	return ok
}

// WantsAutoPlan decides whether a status change starts automatic Plan generation. Only the first Plan after a gate a
// person walked through qualifies; replans and revision loops never run on their own.
func WantsAutoPlan(req Request, flow FlowDefinition, trigger Trigger, to RequestStatus) bool {
	if to != RequestStatusPlanning || req.PlanTaskID != "" {
		return false
	}
	switch flow.PlanKind {
	case PlanNone, PlanSingleTask:
		return false
	}
	if PlanShapeFor(flow, req.Size) == ShapeNone {
		return false
	}
	switch trigger {
	case TriggerAnalysisApproved:
		return flow.AnalysisGate != GateNone
	case TriggerTypeConfirmed:
		return flow.AnalysisKind == AnalysisNone
	}
	return false
}
