package usecase

import (
	"context"
)

// BindingStreamTarget describes a worktree binding used for routing agent stream events.
type BindingStreamTarget struct {
	TenantID      string
	DevServerID   string
	RepoBindingID string
	WorkspaceRoot string
	ProjectID     string
	WorktreeID    string
}

// BindingStreamTargets provides cross-tenant queries to resolve dev server streams to worktrees.
type BindingStreamTargets interface {
	ListTargets(ctx context.Context) ([]BindingStreamTarget, error)
	FindTarget(ctx context.Context, tenantID, devServerID, workspaceRoot string) (*BindingStreamTarget, error)
}

// QualityRunEventSink is notified when a quality evaluation run finishes (PQ-17).
type QualityRunEventSink interface {
	OnQualityFinished(ctx context.Context, tenantID, devServerID, workspaceRoot, runID, payloadJSON string) error
}

// NoopQualityRunEventSink provides a default implementation that does nothing.
type NoopQualityRunEventSink struct{}

func (n *NoopQualityRunEventSink) OnQualityFinished(ctx context.Context, tenantID, devServerID, workspaceRoot, runID, payloadJSON string) error {
	return nil
}
