package usecase

import "context"

type WriteFileChunkUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewWriteFileChunkUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *WriteFileChunkUseCase {
	return &WriteFileChunkUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *WriteFileChunkUseCase) Execute(ctx context.Context, worktreeID, path string, offsetBytes int64, content []byte, isFinal bool) (bytesWritten int64, err error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return 0, err
	}
	return exec.WriteFileChunk(ctx, conn.RepoPath, path, offsetBytes, content, isFinal)
}
