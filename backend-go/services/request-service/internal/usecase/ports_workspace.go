package usecase

import "context"

type Workspace struct {
	WorktreeID string
	Path       string
	Branch     string
	BaseRef    string
}

type EnsureWorkspaceInput struct {
	TenantID      string
	ProjectID     string
	RepoID        string
	RequestNumber int64
	ChangeID      string
}

type ProposalWorkspace interface {
	Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error)
	ReadFile(ctx context.Context, worktreeID, path string) ([]byte, error)
	WriteFile(ctx context.Context, worktreeID, path string, content []byte) error
	ChangedPaths(ctx context.Context, worktreeID string) ([]string, error)
	DiscardAll(ctx context.Context, worktreeID string) error
	Commit(ctx context.Context, worktreeID, message string, paths []string) error
}
