package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ReadDirUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewReadDirUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ReadDirUseCase {
	return &ReadDirUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadDirUseCase) Execute(ctx context.Context, worktreeID, path string) ([]domain.DirEntry, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.ReadDir(ctx, conn.RepoPath, path)
}
