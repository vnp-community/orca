package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type SearchFilesUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewSearchFilesUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *SearchFilesUseCase {
	return &SearchFilesUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *SearchFilesUseCase) Execute(ctx context.Context, worktreeID string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.Search(ctx, conn.RepoPath, opts)
}
