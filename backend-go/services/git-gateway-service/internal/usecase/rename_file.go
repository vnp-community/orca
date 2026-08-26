package usecase

import (
	"context"
	"errors"
)

// ErrFileOpNotSupportedOverRelay is returned by RenameFileUseCase and
// CopyFileUseCase when dispatch resolves to a relay target. Preserved
// deliberately from the old backend's own scope limit (BUG-009) — the Dev
// Server Agent's fs.* surface has no rename/copy method.
var ErrFileOpNotSupportedOverRelay = errors.New("usecase: rename/copy are not supported over a relay connection")

type RenameFileUseCase struct {
	resolver ConnectionResolver
	local    LocalOnlyFilesystemExecutor
}

// NewRenameFileUseCase deliberately takes no relay executor parameter —
// see ErrFileOpNotSupportedOverRelay above and this package's
// LocalOnlyFilesystemExecutor doc comment (ports.go).
func NewRenameFileUseCase(resolver ConnectionResolver, local LocalOnlyFilesystemExecutor) *RenameFileUseCase {
	return &RenameFileUseCase{resolver: resolver, local: local}
}

func (uc *RenameFileUseCase) Execute(ctx context.Context, worktreeID, from, to string) error {
	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return err
	}
	if conn.Connected {
		// Known gap, preserved deliberately — see this file's doc comment.
		// NOT falling back to a relay call the agent's fs.* surface doesn't
		// implement.
		return ErrFileOpNotSupportedOverRelay
	}
	return uc.local.Rename(ctx, conn.RepoPath, from, to)
}
