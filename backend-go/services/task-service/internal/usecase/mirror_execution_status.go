package usecase

import "context"

// MirrorExecutionStatusInput mirrors the fields task-service's new
// orca.orchestration.task.statuschanged / orca.workflow.step.completed
// consumer (BE-SOL-003/TASK-FT-003-05) extracts from each subject's outbox
// payload.
type MirrorExecutionStatusInput struct {
	TenantID      string
	ExternalRefID string
	NewStatus     string
}

// MirrorExecutionStatus is the handler behind task-service's new
// orca.orchestration.task.statuschanged / orca.workflow.step.completed
// consumer (BE-SOL-003) — keeps execution_links.status_mirror
// (TASK-FT-001-01) roughly in sync with the owning engine's real state, for
// CR-FLOW-TASK-003's Activity Feed to read.
type MirrorExecutionStatus struct {
	links ExecutionLinkRepository
}

func NewMirrorExecutionStatus(links ExecutionLinkRepository) *MirrorExecutionStatus {
	return &MirrorExecutionStatus{links: links}
}

// Execute updates execution_links.status_mirror for the row whose
// external_ref_id matches in.ExternalRefID — a no-op (not an error) if no
// such row exists, the same idempotence posture SOL-TG-04's staleness guard
// already establishes for this codebase's cross-service callback handlers.
func (uc *MirrorExecutionStatus) Execute(ctx context.Context, in MirrorExecutionStatusInput) error {
	return uc.links.UpdateStatusMirror(ctx, in.TenantID, in.ExternalRefID, in.NewStatus)
}
