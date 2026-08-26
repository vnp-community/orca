package usecase

import (
	"context"
	"errors"
)

// ErrChunkedReadNotSupportedRemote is returned by ReadFileChunkUseCase when
// dispatch resolves to a relay target — chunked reads are unsupported for
// any remote target by design, matching the old backend's own scope limit
// (BUG-009's known-gap finding). Preserved deliberately, not a TODO.
var ErrChunkedReadNotSupportedRemote = errors.New("usecase: chunked file reads are not supported over a relay connection")

type ReadFileChunkUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
}

// NewReadFileChunkUseCase takes only a local executor — there is
// deliberately no relay FilesystemExecutor parameter, since this usecase
// must never attempt a relay call for this operation (see
// ErrChunkedReadNotSupportedRemote above).
func NewReadFileChunkUseCase(resolver ConnectionResolver, local FilesystemExecutor) *ReadFileChunkUseCase {
	return &ReadFileChunkUseCase{resolver: resolver, local: local}
}

func (uc *ReadFileChunkUseCase) Execute(ctx context.Context, worktreeID, path string, offsetBytes, lengthBytes int64) ([]byte, error) {
	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, err
	}
	if conn.Connected {
		// Check dispatch target BEFORE attempting any relay call — the
		// agent's fs.* surface doesn't implement chunked reads anyway.
		return nil, ErrChunkedReadNotSupportedRemote
	}
	full, err := uc.local.ReadFile(ctx, conn.RepoPath, path)
	if err != nil {
		return nil, err
	}
	end := offsetBytes + lengthBytes
	if end > int64(len(full)) {
		end = int64(len(full))
	}
	if offsetBytes > int64(len(full)) {
		return []byte{}, nil
	}
	return full[offsetBytes:end], nil
}
