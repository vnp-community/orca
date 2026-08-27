# TASK-052: Implement file read-family usecases (`ReadFile`, `ReadFileChunk`, `ReadFilePreview`, `ReadDir`, `StatFile`)

**From Solution:** SOL-009 (Design — `usecase/` layer, representative `ReadFileUseCase`)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/read_file.go`, `read_file_chunk.go`, `read_file_preview.go`, `read_dir.go`, `stat_file.go` (all new)
**Depends on:** TASK-050, TASK-051
**Status:** `[x]` DONE — `ReadFileUseCase`/`ReadFileChunkUseCase`/`ReadFilePreviewUseCase`/`ReadDirUseCase`/`StatFileUseCase` all implemented as specified, including `ReadFileChunkUseCase`'s relay-target rejection (`ErrChunkedReadNotSupportedRemote`) BEFORE calling any executor. `go build`/`go vet`/`go test` clean; unit-tested in `filesystem_dispatch_test.go`.

---

## Context

Every simple read RPC follows the same three-step body: resolve the
executor via `dispatchFilesystemExecutor`, then call the corresponding
`FilesystemExecutor` method. `ReadFileChunk` is the one exception in this
family — it's unsupported for any relay target (BUG-009's known gap,
matching the old backend), so its usecase checks `conn.Connected` first
and returns a typed error **without** calling the executor at all.

---

## Changes to make

**File:** `internal/usecase/read_file.go`

```go
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
```

**File:** `internal/usecase/read_file_chunk.go`

```go
package usecase

import (
	"context"
	"errors"
)

// ErrChunkedReadNotSupportedRemote is returned by ReadFileChunkUseCase
// when dispatch resolves to a relay target — chunked reads are
// unsupported for any remote target by design, matching the old
// backend's own scope limit (BUG-009's known-gap finding). Preserved
// deliberately, not a TODO.
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
```

**File:** `internal/usecase/read_file_preview.go`

```go
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
```

**File:** `internal/usecase/read_dir.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ReadDirUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewReadDirUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ReadDirUseCase {
	return &ReadDirUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadDirUseCase) Execute(ctx context.Context, worktreeID, path string) ([]domain.DirEntry, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.ReadDir(ctx, conn.RepoPath, path)
}
```

**File:** `internal/usecase/stat_file.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StatFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewStatFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *StatFileUseCase {
	return &StatFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *StatFileUseCase) Execute(ctx context.Context, worktreeID, path string) (domain.FileStat, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return domain.FileStat{}, err
	}
	return exec.Stat(ctx, conn.RepoPath, path)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/usecase/...
go vet ./internal/usecase/...
```
