package usecase

import "context"

type WriteFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewWriteFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *WriteFileUseCase {
	return &WriteFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *WriteFileUseCase) Execute(ctx context.Context, worktreeID, path string, content []byte, createParents bool) (bytesWritten int64, err error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return 0, err
	}
	return exec.WriteFile(ctx, conn.RepoPath, path, content, createParents)
}
