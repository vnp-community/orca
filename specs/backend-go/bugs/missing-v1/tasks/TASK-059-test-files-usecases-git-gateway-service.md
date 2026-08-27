# TASK-059: Test `files.*` usecases and `localfs`/relay executors

**From Solution:** SOL-009 (Test plan — `git-gateway-service` usecase/adapter items)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/read_file_test.go` (+ one per usecase file), `backend-go/services/git-gateway-service/internal/adapter/localfs/executor_test.go` (new)
**Depends on:** TASK-052, TASK-053, TASK-054, TASK-055
**Status:** `[x]` DONE — Covers this task's intent — local-vs-relay dispatch routing, `ReadFileChunkUseCase`/`RenameFileUseCase`/`CopyFileUseCase`'s "never call the relay executor" guarantee, and `localfs`'s real filesystem behavior including `ErrPathEscapesWorktree` — bundled into `internal/usecase/filesystem_dispatch_test.go` (one file, mirroring `dispatch_test.go`'s existing convention) rather than one test file per usecase as this task's sketch shows, and `fakeFilesystemExecutor` lives there rather than in `fakes_test.go` (no such shared fakes file exists in this service — `dispatch_test.go` itself holds `fakeConnectionResolver`/`fakeGitExecutor` inline). `internal/adapter/localfs/executor_test.go` added as specified (12 tests, real filesystem, no fakes). Closed the remaining gap: `ReadDirUseCase`/`StatFileUseCase`/`CreateDirUseCase`/`DeleteFileUseCase`/`WriteFileChunkUseCase`/`ListAllFilesUseCase` now each have their own Connected(relay)/NotConnected(local) dispatch-routing test pair in `filesystem_dispatch_test.go`, using new `calledReadDir`/`calledStat`/`calledCreateDir`/`calledDelete`/`calledWriteFileChunk` fields on `fakeFilesystemExecutor`. `go build`/`go vet`/`go test ./...` clean.

---

## Context

Regression coverage for the local-vs-relay dispatch decision (the part
this solution's design explicitly calls out as the thing worth testing
once, not per-RPC), plus the two known-gap usecases' "never call the relay"
guarantee, plus `localfs`'s real filesystem behavior including path-escape
rejection.

---

## Changes to make

### Fakes

**File:** `internal/usecase/fakes_test.go` (extend the existing fakes file
used by `read_test.go`/`commit_test.go`/etc. — follow its existing
`fakeConnectionResolver`/`fakeGitExecutor` naming convention)

```go
type fakeFilesystemExecutor struct {
	readFileFunc func(ctx context.Context, repoPath, relPath string) ([]byte, error)
	calls        []string // records method names called, for "never called" assertions
}

func (f *fakeFilesystemExecutor) ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error) {
	f.calls = append(f.calls, "ReadFile")
	if f.readFileFunc != nil {
		return f.readFileFunc(ctx, repoPath, relPath)
	}
	return []byte("stub"), nil
}
// ... implement the rest of FilesystemExecutor similarly, each appending
// its method name to f.calls before delegating to an optional func field
// or returning a zero value.

type fakeLocalOnlyFilesystemExecutor struct {
	calls []string
}

func (f *fakeLocalOnlyFilesystemExecutor) Rename(ctx context.Context, repoPath, fromRel, toRel string) error {
	f.calls = append(f.calls, "Rename")
	return nil
}
func (f *fakeLocalOnlyFilesystemExecutor) Copy(ctx context.Context, repoPath, fromRel, toRel string) error {
	f.calls = append(f.calls, "Copy")
	return nil
}
```

### Dispatch tests (representative — repeat this shape for each usecase file from TASK-052/TASK-053/TASK-054)

**File:** `internal/usecase/read_file_test.go`

```go
func TestReadFileUseCase_LocalDispatch(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	relay := &fakeFilesystemExecutor{}
	resolver := &fakeConnectionResolver{result: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	uc := NewReadFileUseCase(resolver, local, relay)

	if _, err := uc.Execute(context.Background(), "wt1", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if len(local.calls) != 1 || len(relay.calls) != 0 {
		t.Errorf("expected 1 local call, 0 relay calls; got local=%v relay=%v", local.calls, relay.calls)
	}
}

func TestReadFileUseCase_RelayDispatch(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	relay := &fakeFilesystemExecutor{}
	resolver := &fakeConnectionResolver{result: ResolvedConnection{Connected: true, ConnectionID: "c1", RepoPath: "/repo"}}
	uc := NewReadFileUseCase(resolver, local, relay)

	if _, err := uc.Execute(context.Background(), "wt1", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if len(relay.calls) != 1 || len(local.calls) != 0 {
		t.Errorf("expected 1 relay call, 0 local calls; got local=%v relay=%v", local.calls, relay.calls)
	}
}
```

### Known-gap regression guards

**File:** `internal/usecase/rename_file_test.go`

```go
func TestRenameFileUseCase_RelayDispatch_NeverCallsExecutor(t *testing.T) {
	local := &fakeLocalOnlyFilesystemExecutor{}
	resolver := &fakeConnectionResolver{result: ResolvedConnection{Connected: true, ConnectionID: "c1", RepoPath: "/repo"}}
	uc := NewRenameFileUseCase(resolver, local)

	err := uc.Execute(context.Background(), "wt1", "a.txt", "b.txt")
	if !errors.Is(err, ErrFileOpNotSupportedOverRelay) {
		t.Errorf("expected ErrFileOpNotSupportedOverRelay, got %v", err)
	}
	if len(local.calls) != 0 {
		t.Errorf("expected zero calls to local executor when relay-dispatched, got %v", local.calls)
	}
}

func TestRenameFileUseCase_LocalDispatch(t *testing.T) {
	local := &fakeLocalOnlyFilesystemExecutor{}
	resolver := &fakeConnectionResolver{result: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	uc := NewRenameFileUseCase(resolver, local)

	if err := uc.Execute(context.Background(), "wt1", "a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	if len(local.calls) != 1 {
		t.Errorf("expected 1 local call, got %v", local.calls)
	}
}
```

Mirror both tests in `copy_file_test.go` for `CopyFileUseCase`.

**File:** `internal/usecase/read_file_chunk_test.go`

```go
func TestReadFileChunkUseCase_RelayDispatch_ReturnsTypedError(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	resolver := &fakeConnectionResolver{result: ResolvedConnection{Connected: true, ConnectionID: "c1", RepoPath: "/repo"}}
	uc := NewReadFileChunkUseCase(resolver, local)

	_, err := uc.Execute(context.Background(), "wt1", "a.txt", 0, 10)
	if !errors.Is(err, ErrChunkedReadNotSupportedRemote) {
		t.Errorf("expected ErrChunkedReadNotSupportedRemote, got %v", err)
	}
	if len(local.calls) != 0 {
		t.Errorf("expected zero calls to local executor, got %v", local.calls)
	}
}
```

### `adapter/localfs` integration tests

**File:** `internal/adapter/localfs/executor_test.go`

```go
func TestExecutor_WriteReadStatDeleteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	e := New()
	ctx := context.Background()

	if _, err := e.WriteFile(ctx, dir, "a.txt", []byte("hello"), false); err != nil {
		t.Fatal(err)
	}
	content, err := e.ReadFile(ctx, dir, "a.txt")
	if err != nil || string(content) != "hello" {
		t.Fatalf("ReadFile: %v %q", err, content)
	}
	stat, err := e.Stat(ctx, dir, "a.txt")
	if err != nil || !stat.Exists || stat.SizeBytes != 5 {
		t.Fatalf("Stat: %+v %v", stat, err)
	}
	if err := e.Delete(ctx, dir, "a.txt", false); err != nil {
		t.Fatal(err)
	}
	if stat, _ := e.Stat(ctx, dir, "a.txt"); stat.Exists {
		t.Error("expected file to be gone after Delete")
	}
}

func TestExecutor_RenameCopy(t *testing.T) {
	dir := t.TempDir()
	e := New()
	ctx := context.Background()
	if _, err := e.WriteFile(ctx, dir, "a.txt", []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	if err := e.Copy(ctx, dir, "a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := e.Rename(ctx, dir, "b.txt", "c.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ReadFile(ctx, dir, "c.txt"); err != nil {
		t.Fatalf("expected c.txt to exist after rename: %v", err)
	}
}

func TestExecutor_PathEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	e := New()
	ctx := context.Background()

	_, err := e.ReadFile(ctx, dir, "../../etc/passwd")
	if !errors.Is(err, ErrPathEscapesWorktree) {
		t.Errorf("expected ErrPathEscapesWorktree, got %v", err)
	}
	if _, err := e.WriteFile(ctx, dir, "../escape.txt", []byte("x"), false); !errors.Is(err, ErrPathEscapesWorktree) {
		t.Errorf("expected ErrPathEscapesWorktree on write, got %v", err)
	}
}

func TestExecutor_SearchAndGlob(t *testing.T) {
	dir := t.TempDir()
	e := New()
	ctx := context.Background()
	if _, err := e.WriteFile(ctx, dir, "a.go", []byte("package main\n// TODO: fix\n"), false); err != nil {
		t.Fatal(err)
	}
	matches, err := e.Search(ctx, dir, domain.SearchOptions{Pattern: "TODO", MaxResults: 10})
	if err != nil || len(matches) != 1 {
		t.Fatalf("Search: %v %+v", err, matches)
	}
	paths, err := e.Glob(ctx, dir, "*.go", 10)
	if err != nil || len(paths) != 1 {
		t.Fatalf("Glob: %v %+v", err, paths)
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go test ./internal/usecase/... ./internal/adapter/localfs/... -count=1 -v
go vet ./internal/usecase/... ./internal/adapter/localfs/...
```

Expected: all tests pass, including the "zero calls" regression guards
for `RenameFile`/`CopyFile`/`ReadFileChunk` over relay dispatch.
