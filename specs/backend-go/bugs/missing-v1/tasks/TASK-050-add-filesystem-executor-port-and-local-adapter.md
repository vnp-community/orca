# TASK-050: Add `FilesystemExecutor` port and `adapter/localfs` implementation

**From Solution:** SOL-009 (Design — `usecase/` layer, local-vs-relay dispatch mechanism)
**Priority:** P0 — every `files.*` usecase task depends on this port
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`, `backend-go/services/git-gateway-service/internal/adapter/localfs/executor.go` (new)
**Depends on:** TASK-049
**Status:** `[x]` DONE — `FilesystemExecutor`/`LocalOnlyFilesystemExecutor` ports + `dispatchFilesystemExecutor` added to `ports.go`; `domain/file.go` (`DirEntry`/`FileStat`/`SearchOptions`/`SearchMatch`) added; `internal/adapter/localfs/executor.go` implements every method including `Search`/`Glob` directly (not left as TASK-054 placeholders — this pass wrote the full walk/grep/glob implementation in one step since the two tasks were done back-to-back). `go build`/`go vet`/`go test` clean.

---

## Context

`git-gateway-service`'s actual `usecase/ports.go` today (not the TDD's
`WorktreeResolver`/`HostResolver` split, which was never built) defines
`ConnectionResolver` (worktree -> `ResolvedConnection{Connected,
ConnectionID, RepoPath}`) and `GitExecutor`, selected per-call by
`dispatchExecutor(ctx, resolver, local, relay, worktreeID)`. This task
adds the file-I/O equivalent of that same pattern: a `FilesystemExecutor`
port, a `dispatchFilesystemExecutor` helper mirroring `dispatchExecutor`,
and a real `adapter/localfs` implementation mirroring `adapter/localgit`.
The relay implementation is TASK-051; the usecases that call
`dispatchFilesystemExecutor` are TASK-052 through TASK-055.

`RenameFile`/`CopyFile` are a known gap (BUG-009): the Dev Server Agent's
`fs.*` surface has no `rename`/`copy`. They live on a **separate**,
narrower `LocalOnlyFilesystemExecutor` interface so the type system
reflects the asymmetry — `adapter/grpcclient`'s relay implementation
(TASK-051) deliberately does not implement it.

---

## Changes to make

### Step 1: Extend `internal/usecase/ports.go`

Add below the existing `GitExecutor` interface:

```go
// FilesystemExecutor performs file I/O against a resolved worktree path.
// Two implementations exist, selected by dispatchFilesystemExecutor the
// same way dispatchExecutor selects a GitExecutor:
//   - internal/adapter/localfs: real os-backed implementation, used when
//     ConnectionResolver reports Connected=false.
//   - internal/adapter/grpcclient's RelayExecutor: relays to the Dev
//     Server Agent's fs.* methods, used when Connected=true.
type FilesystemExecutor interface {
	ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error)
	ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) (content []byte, truncated bool, err error)
	ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error)
	WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (bytesWritten int64, err error)
	WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (bytesWritten int64, err error)
	CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error
	Delete(ctx context.Context, repoPath, relPath string, recursive bool) error
	Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error)
	Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error)
	Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error)
}

// LocalOnlyFilesystemExecutor covers Rename/Copy — BUG-009's known gap:
// the Dev Server Agent's fs.* surface implements
// stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep but not
// rename/copy. Only adapter/localfs implements this interface;
// adapter/grpcclient's RelayExecutor does not, so
// dispatchLocalOnlyFilesystemExecutor (see rename_file.go / copy_file.go
// in TASK-055) can compile-time-guarantee it never calls a relay target.
type LocalOnlyFilesystemExecutor interface {
	Rename(ctx context.Context, repoPath, fromRel, toRel string) error
	Copy(ctx context.Context, repoPath, fromRel, toRel string) error
}

// dispatchFilesystemExecutor mirrors dispatchExecutor: resolve worktreeID's
// owning host, then return whichever FilesystemExecutor answers for it
// plus the resolved repo path and the raw ResolvedConnection (needed by
// TASK-052/TASK-055 to reject relay-only-unsupported operations before
// calling the executor at all).
func dispatchFilesystemExecutor(ctx context.Context, resolver ConnectionResolver, local, relay FilesystemExecutor, worktreeID string) (FilesystemExecutor, ResolvedConnection, error) {
	conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, ResolvedConnection{}, err
	}
	if conn.Connected {
		return relay, conn, nil
	}
	return local, conn, nil
}
```

### Step 2: Add domain types

**File:** `backend-go/services/git-gateway-service/internal/domain/file.go` (new)

```go
// Package domain — file I/O value objects, parallel to this package's
// existing git types (GitStatus, DiffResult, etc.).
package domain

// DirEntry is one entry returned by FilesystemExecutor.ReadDir.
type DirEntry struct {
	Name        string
	IsDirectory bool
}

// FileStat is FilesystemExecutor.Stat's result.
type FileStat struct {
	Exists          bool
	IsDirectory     bool
	SizeBytes       int64
	ModifiedAtUnixMs int64
}

// SearchOptions parameterizes FilesystemExecutor.Search.
type SearchOptions struct {
	Pattern    string
	IsRegex    bool
	PathGlob   string
	MaxResults int
}

// SearchMatch is one result line from FilesystemExecutor.Search.
type SearchMatch struct {
	Path     string
	Line     int
	LineText string
}
```

### Step 3: New package `internal/adapter/localfs/executor.go`

```go
// Package localfs implements usecase.FilesystemExecutor (and
// usecase.LocalOnlyFilesystemExecutor) against the host filesystem
// directly — the host-local case, parallel to adapter/localgit. Every
// method resolves relPath against repoPath and rejects any resolved path
// that escapes it, per git-gateway-service.md §3's "never trust a
// client-supplied host path" posture, applied one level deeper since file
// I/O takes an additional relative path the git RPCs don't.
package localfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ErrPathEscapesWorktree is returned when relPath, once cleaned and
// joined to repoPath, resolves outside repoPath.
var ErrPathEscapesWorktree = errors.New("localfs: path escapes worktree root")

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

// resolve joins repoPath+relPath and rejects any result that escapes
// repoPath — called first by every method below.
func resolve(repoPath, relPath string) (string, error) {
	full := filepath.Clean(filepath.Join(repoPath, relPath))
	repoPath = filepath.Clean(repoPath)
	if full != repoPath && !strings.HasPrefix(full, repoPath+string(filepath.Separator)) {
		return "", ErrPathEscapesWorktree
	}
	return full, nil
}

func (e *Executor) ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (e *Executor) ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) ([]byte, bool, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return nil, false, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	// Truncated iff more data remained after maxBytes.
	_, peekErr := f.Read(make([]byte, 1))
	truncated := peekErr == nil
	return buf[:n], truncated, nil
}

func (e *Executor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, domain.DirEntry{Name: e.Name(), IsDirectory: e.IsDir()})
	}
	return out, nil
}

func (e *Executor) WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (int64, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return 0, err
	}
	if createParents {
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return 0, err
		}
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return 0, err
	}
	return int64(len(content)), nil
}

func (e *Executor) WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (int64, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.WriteAt(content, offsetBytes); err != nil {
		return 0, err
	}
	return int64(len(content)), nil
}

func (e *Executor) CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return err
	}
	if noClobber {
		if _, statErr := os.Stat(full); statErr == nil {
			return fmt.Errorf("localfs: %s already exists", relPath)
		}
	}
	if recursive {
		return os.MkdirAll(full, 0o755)
	}
	return os.Mkdir(full, 0o755)
}

func (e *Executor) Delete(ctx context.Context, repoPath, relPath string, recursive bool) error {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return err
	}
	if recursive {
		return os.RemoveAll(full)
	}
	return os.Remove(full)
}

func (e *Executor) Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error) {
	full, err := resolve(repoPath, relPath)
	if err != nil {
		return domain.FileStat{}, err
	}
	info, err := os.Stat(full)
	if errors.Is(err, os.ErrNotExist) {
		return domain.FileStat{Exists: false}, nil
	}
	if err != nil {
		return domain.FileStat{}, err
	}
	return domain.FileStat{
		Exists:           true,
		IsDirectory:      info.IsDir(),
		SizeBytes:        info.Size(),
		ModifiedAtUnixMs: info.ModTime().UnixMilli(),
	}, nil
}

// Rename and Copy implement usecase.LocalOnlyFilesystemExecutor — see
// ports.go's doc comment on why this is a separate interface.
func (e *Executor) Rename(ctx context.Context, repoPath, fromRel, toRel string) error {
	fromFull, err := resolve(repoPath, fromRel)
	if err != nil {
		return err
	}
	toFull, err := resolve(repoPath, toRel)
	if err != nil {
		return err
	}
	return os.Rename(fromFull, toFull)
}

func (e *Executor) Copy(ctx context.Context, repoPath, fromRel, toRel string) error {
	fromFull, err := resolve(repoPath, fromRel)
	if err != nil {
		return err
	}
	toFull, err := resolve(repoPath, toRel)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(fromFull)
	if err != nil {
		return err
	}
	return os.WriteFile(toFull, content, 0o644)
}

// Search and Glob are implemented by TASK-054's usecases calling out to
// this executor — see that task for the walk/grep implementation, kept
// out of this file to keep this task's diff focused on the read/write/
// stat/rename/copy primitives every other usecase needs first.
```

Note: `Search`/`Glob` method bodies are added in TASK-054, which extends
this same `Executor` type — this task only needs the struct and the
methods above to exist so `FilesystemExecutor`/`LocalOnlyFilesystemExecutor`
compile against a real `*Executor`. If you prefer, stub them here with
`panic("implemented in TASK-054")` bodies so the package compiles before
TASK-054 lands; do not skip declaring the methods, since Go requires the
full interface to be satisfied wherever `*Executor` is passed as a
`usecase.FilesystemExecutor`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/... 2>&1 | grep -v "TASK-054" || true
```

Expected: package compiles once TASK-054's `Search`/`Glob` bodies (or
placeholder stubs per the note above) are present.
