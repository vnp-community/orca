package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ProposalWorkspace struct {
	// grpc client fields
}

func NewProposalWorkspace() *ProposalWorkspace {
	return &ProposalWorkspace{}
}

func (w *ProposalWorkspace) Ensure(ctx context.Context, in usecase.EnsureWorkspaceInput) (usecase.Workspace, error) {
	return usecase.Workspace{}, nil
}

func (w *ProposalWorkspace) ReadFile(ctx context.Context, worktreeID, path string) ([]byte, error) {
	return nil, nil
}

func (w *ProposalWorkspace) WriteFile(ctx context.Context, worktreeID, path string, content []byte) error {
	return nil
}

func (w *ProposalWorkspace) ChangedPaths(ctx context.Context, worktreeID string) ([]string, error) {
	return nil, nil
}

func (w *ProposalWorkspace) DiscardAll(ctx context.Context, worktreeID string) error {
	return nil
}

func (w *ProposalWorkspace) Commit(ctx context.Context, worktreeID, message string, paths []string) error {
	return nil
}
