package domain

import (
	"errors"
	"time"
	"unicode/utf8"
)

type RunKind string

const (
	RunKindSolution  RunKind = "solution"
	RunKindDiagnosis RunKind = "diagnosis"
	RunKindFindings  RunKind = "findings"
	RunKindAnswer    RunKind = "answer"
)

type AnalysisMode string

const (
	AnalysisModeComplete      AnalysisMode = "complete"
	AnalysisModeAgentReadonly AnalysisMode = "agent_readonly"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

var ErrTooManyAttempts = errors.New("too many attempts")

// AnalysisRun tracks the lifecycle and output of an AI generation or analysis task.
type AnalysisRun struct {
	ID             string
	TenantID       string
	RequestID      string
	Kind           RunKind
	Mode           AnalysisMode
	Status         RunStatus
	IdempotencyKey *string
	Attempt        int
	LeaseOwner     *string
	LeaseExpiresAt *time.Time
	ErrorCode      *string
	ErrorMessage   *string
	RawOutput      *string
	StartedAt      time.Time
	FinishedAt     *time.Time
	// SolutionID is the draft the run fills; ProjectID and ActorID let a restarted worker rebuild its context.
	SolutionID string
	ProjectID  string
	ActorID    string
	Feedback   string
	// Enforcement and RepoCheck are only set by agent_readonly runs.
	Enforcement string
	RepoCheck   string
}

// Run enforcement and repo-check values recorded on agent_readonly runs.
const (
	EnforcementAgent      = "agent_enforced"
	EnforcementPromptOnly = "prompt_only"
	RepoCheckSkipped      = "skipped"
	RepoCheckClean        = "clean"
	RepoCheckModified     = "modified"
)

// RecentRunsLimit is how many past runs ListSolutions shows so the UI can surface a failure.
const RecentRunsLimit = 10

// RecordAttempt increments the attempt counter. Fails if exceeding 2.
func (r *AnalysisRun) RecordAttempt() error {
	if r.Attempt >= 2 {
		return ErrTooManyAttempts
	}
	r.Attempt++
	return nil
}

// Succeed marks the run as successfully completed.
func (r *AnalysisRun) Succeed(now time.Time) {
	r.Status = RunStatusSucceeded
	r.FinishedAt = &now
	r.LeaseOwner = nil
	r.LeaseExpiresAt = nil
}

// Fail marks the run as failed with an error code and message.
func (r *AnalysisRun) Fail(code, msg string, now time.Time) {
	r.Status = RunStatusFailed
	r.FinishedAt = &now
	r.ErrorCode = &code
	r.ErrorMessage = &msg
	r.LeaseOwner = nil
	r.LeaseExpiresAt = nil
}

// TruncateRaw limits the raw output string to 256 KB, ensuring it is split at a valid UTF-8 rune boundary.
func TruncateRaw(s string) string {
	const limit = 256 * 1024
	if len(s) <= limit {
		return s
	}

	for i := limit; i >= limit-4 && i > 0; i-- {
		if utf8.ValidString(s[:i]) {
			return s[:i]
		}
	}
	// Fallback if no boundary found within the last 4 bytes (rare but possible with invalid UTF-8)
	return ""
}
