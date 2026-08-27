package usecase

import "context"

type DeleteFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewDeleteFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *DeleteFileUseCase {
	return &DeleteFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *DeleteFileUseCase) Execute(ctx context.Context, worktreeID, path string, recursive bool) error {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return err
	}
	return exec.Delete(ctx, conn.RepoPath, path, recursive)
}
