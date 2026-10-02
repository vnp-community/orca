package domain

// ExpiredRun is a direct_agent execution whose lease ran out — the process that
// owned it died or lost contact. ClaimExpired has already marked its link
// failed; the recovery usecase reverts the task.
type ExpiredRun struct {
	TenantID       string
	LinkID         string
	TaskID         string
	PreviousStatus string
}
