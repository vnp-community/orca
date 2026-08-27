package usecase

import "context"

type ReadFilePreviewUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewReadFilePreviewUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ReadFilePreviewUseCase {
	return &ReadFilePreviewUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadFilePreviewUseCase) Execute(ctx context.Context, worktreeID, path string, maxBytes int64) (content []byte, truncated bool, err error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, false, err
	}
	return exec.ReadFilePreview(ctx, conn.RepoPath, path, maxBytes)
}
