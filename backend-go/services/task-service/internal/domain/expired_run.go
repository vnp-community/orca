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

// StuckTask is a task left at in_progress although the execution it points at
// (its active link) already ended. LinkStatus is that link's status_mirror.
type StuckTask struct {
	TenantID       string
	TaskID         string
	LinkID         string
	LinkStatus     string
	PreviousStatus string
}
