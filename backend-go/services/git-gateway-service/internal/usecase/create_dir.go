package usecase

import "context"

type CreateDirUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewCreateDirUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *CreateDirUseCase {
	return &CreateDirUseCase{resolver: resolver, local: local, relay: relay}
}

// Execute serves both files.createDir (noClobber=false) and
// files.createDirNoClobber (noClobber=true) — one usecase, one bool
// parameter, per SOL-009's proto-collapse note.
func (uc *CreateDirUseCase) Execute(ctx context.Context, worktreeID, path string, recursive, noClobber bool) error {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return err
	}
	return exec.CreateDir(ctx, conn.RepoPath, path, recursive, noClobber)
}
