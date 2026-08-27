package usecase

import "context"

type CopyFileUseCase struct {
	resolver ConnectionResolver
	local    LocalOnlyFilesystemExecutor
}

func NewCopyFileUseCase(resolver ConnectionResolver, local LocalOnlyFilesystemExecutor) *CopyFileUseCase {
	return &CopyFileUseCase{resolver: resolver, local: local}
}

func (uc *CopyFileUseCase) Execute(ctx context.Context, worktreeID, from, to string) error {
	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return err
	}
	if conn.Connected {
		return ErrFileOpNotSupportedOverRelay
	}
	return uc.local.Copy(ctx, conn.RepoPath, from, to)
}
