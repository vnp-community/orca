package domain

import "time"

type ClassificationRunStatus string

const (
	ClassificationRunRunning   ClassificationRunStatus = "running"
	ClassificationRunSucceeded ClassificationRunStatus = "succeeded"
	ClassificationRunFailed    ClassificationRunStatus = "failed"
)

// MaxClassificationRunClaims bounds how often a crashed run is re-claimed before it is
// failed, so a request that always kills the worker cannot loop forever.
const MaxClassificationRunClaims = 3

// ClassificationRun is the durable record of one background AI classification: the lease
// lets another instance take over after a crash, SourceEventID dedupes redelivered
// status_changed events.
type ClassificationRun struct {
	ID             string
	TenantID       string
	RequestID      string
	Trigger        string
	SourceEventID  string // empty for manual runs
	Manual         bool
	ActorID        string // user the AI call is attributed to (the reporter)
	Status         ClassificationRunStatus
	Claims         int
	LeaseOwner     string
	LeaseExpiresAt time.Time
	ErrorCode      string
	StartedAt      time.Time
	FinishedAt     *time.Time
}
