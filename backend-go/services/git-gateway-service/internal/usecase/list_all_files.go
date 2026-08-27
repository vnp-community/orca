package usecase

import "context"

type ListAllFilesUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewListAllFilesUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ListAllFilesUseCase {
	return &ListAllFilesUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ListAllFilesUseCase) Execute(ctx context.Context, worktreeID, pathGlob string, maxResults int) ([]string, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.Glob(ctx, conn.RepoPath, pathGlob, maxResults)
}
