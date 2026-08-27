package usecase

import "context"

type ReadFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewReadFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ReadFileUseCase {
	return &ReadFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadFileUseCase) Execute(ctx context.Context, worktreeID, path string) ([]byte, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.ReadFile(ctx, conn.RepoPath, path)
}
