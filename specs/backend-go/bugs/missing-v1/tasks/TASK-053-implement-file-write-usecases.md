# TASK-053: Implement file write-family usecases (`WriteFile`, `WriteFileChunk`, `CreateDir`, `DeleteFile`)

**From Solution:** SOL-009 (Design — `usecase/` layer)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/write_file.go`, `write_file_chunk.go`, `create_dir.go`, `delete_file.go` (all new)
**Depends on:** TASK-050, TASK-051
**Status:** `[x]` DONE — `WriteFileUseCase`/`WriteFileChunkUseCase`/`CreateDirUseCase`/`DeleteFileUseCase` all implemented as specified. `go build`/`go vet`/`go test` clean.

---

## Context

Same `dispatchFilesystemExecutor` shape as the read family (TASK-052) —
no known gaps in this family, both local and relay executors implement
every method.

`files.writeBase64` does not get its own usecase — per SOL-009's proto
note, base64-vs-utf8 is a wire-encoding choice the wscompat layer decodes
before calling `WriteFileUseCase.Execute` (see TASK-057); this usecase
always receives raw bytes.

---

## Changes to make

**File:** `internal/usecase/write_file.go`

```go
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
```

**File:** `internal/usecase/write_file_chunk.go`

```go
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
```

**File:** `internal/usecase/create_dir.go`

```go
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
```

**File:** `internal/usecase/delete_file.go`

```go
package usecase

import "context"

type DeleteFileUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewDeleteFileUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *DeleteFileUseCase {
	return &DeleteFileUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *DeleteFileUseCase) Execute(ctx context.Context, worktreeID, path string, recursive bool) error {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return err
	}
	return exec.Delete(ctx, conn.RepoPath, path, recursive)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/usecase/...
go vet ./internal/usecase/...
```
