# TASK-055: Implement known-gap usecases (`RenameFile`, `CopyFile`) with NOT_SUPPORTED-over-relay guard

**From Solution:** SOL-009 (Design — `usecase/` layer, `RenameFileUseCase` sketch + "Known gaps carried forward" section)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/rename_file.go`, `copy_file.go` (both new)
**Depends on:** TASK-050, TASK-051
**Status:** `[x]` DONE — `RenameFileUseCase`/`CopyFileUseCase` implemented taking only `LocalOnlyFilesystemExecutor` (no relay param), both returning `ErrFileOpNotSupportedOverRelay` before ever calling the executor when `Connected=true`. One deviation from this task's sketch: instead of a manual `errors.Is` switch added to `server.go`'s existing error-mapping (there wasn't one — this service uses `apperrors.ToGRPCStatus` uniformly), a new `toFileGRPCStatus` helper in `server.go` maps `ErrFileOpNotSupportedOverRelay`/`ErrChunkedReadNotSupportedRemote` to `FailedPrecondition` and falls back to `apperrors.ToGRPCStatus` for typed `AppError`s from the other files.* usecases — used by every files.* RPC handler. `go build`/`go vet`/`go test` clean.

---

## Context

`RenameFile`/`CopyFile` are BUG-009's known gap: the Dev Server Agent's
`fs.*` surface has no `rename`/`copy` (only
`stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep`). Both RPCs work
for the host-local dispatch branch, but must return `NOT_SUPPORTED`
(mapped to gRPC `FAILED_PRECONDITION` at the adapter layer) when dispatch
resolves to a relay target — **without ever attempting a relay call**,
since `RelayExecutor` (TASK-051) doesn't implement
`usecase.LocalOnlyFilesystemExecutor` in the first place. These two
usecases therefore take only a `LocalOnlyFilesystemExecutor`, never a
relay one — the type signature itself prevents the mistake.

---

## Changes to make

**File:** `internal/usecase/rename_file.go`

```go
package usecase

import (
	"context"
	"errors"
)

// ErrFileOpNotSupportedOverRelay is returned by RenameFileUseCase and
// CopyFileUseCase when dispatch resolves to a relay target. Preserved
// deliberately from the old backend's own scope limit (BUG-009) — the
// Dev Server Agent's fs.* surface has no rename/copy method.
var ErrFileOpNotSupportedOverRelay = errors.New("usecase: rename/copy are not supported over a relay connection")

type RenameFileUseCase struct {
	resolver ConnectionResolver
	local    LocalOnlyFilesystemExecutor
}

// NewRenameFileUseCase deliberately takes no relay executor parameter —
// see ErrFileOpNotSupportedOverRelay above and this package's
// LocalOnlyFilesystemExecutor doc comment (ports.go, added in TASK-050).
func NewRenameFileUseCase(resolver ConnectionResolver, local LocalOnlyFilesystemExecutor) *RenameFileUseCase {
	return &RenameFileUseCase{resolver: resolver, local: local}
}

func (uc *RenameFileUseCase) Execute(ctx context.Context, worktreeID, from, to string) error {
	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return err
	}
	if conn.Connected {
		// Known gap, preserved deliberately — see this file's doc
		// comment. NOT falling back to a relay call the agent's fs.*
		// surface doesn't implement.
		return ErrFileOpNotSupportedOverRelay
	}
	return uc.local.Rename(ctx, conn.RepoPath, from, to)
}
```

**File:** `internal/usecase/copy_file.go`

```go
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
```

### Map `ErrFileOpNotSupportedOverRelay` / `ErrChunkedReadNotSupportedRemote` to gRPC status

**File:** `internal/adapter/grpc/server.go` (wherever this service's
`apperrors.ToGRPCStatus`-style mapping table lives — follow the existing
convention for `domain.ErrProjectNotFound`-style sentinel mapping in this
service)

Add both new sentinels to the mapping so they surface as
`codes.FailedPrecondition`, not an unmapped `codes.Unknown`:

```go
case errors.Is(err, usecase.ErrFileOpNotSupportedOverRelay),
	errors.Is(err, usecase.ErrChunkedReadNotSupportedRemote):
	return status.Error(codes.FailedPrecondition, err.Error())
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/...
go vet ./internal/...
```

Manually confirm (do not commit): a line like
`var _ usecase.FilesystemExecutor = (*grpcclient.RelayExecutor)(nil)` compiles,
while `var _ usecase.LocalOnlyFilesystemExecutor = (*grpcclient.RelayExecutor)(nil)`
does NOT — this is what guarantees `RenameFileUseCase`/`CopyFileUseCase`
can never be constructed with a relay-backed executor.
