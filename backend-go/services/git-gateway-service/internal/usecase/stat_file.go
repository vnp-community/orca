package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StatFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewStatFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *StatFileUseCase {
	return &StatFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *StatFileUseCase) Execute(ctx context.Context, worktreeID, path string) (domain.FileStat, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return domain.FileStat{}, err
	}
	return exec.Stat(ctx, conn.RepoPath, path)
}
