package domain

import "time"

// ExecutionLink is one task.execution_links row — see the
// 0004_execution_links migration for the full column set this mirrors.
// BE-SOL-001/CR-FLOW-TASK-001: one row is written per ExecuteTask dispatch,
// across all three engines, giving CR-FLOW-TASK-003's Activity Feed
// (BE-SOL-003) a history row even for the synchronous Engine 1 path.
type ExecutionLink struct {
	ID            string
	TenantID      string
	TaskID        string
	Engine        ExecutionEngine
	ExternalRefID string
	StatusMirror  string
	StartedAt     time.Time
	CompletedAt   *time.Time
}
