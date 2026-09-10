# TASK-PW-02-02: `domain.DirEntry` gains `SizeBytes`; `localfs.Executor.ReadDir` populates it

**From Solution:** SOL-PW-02
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/domain/file.go`, `backend-go/services/git-gateway-service/internal/adapter/localfs/executor.go`
**Depends on:** TASK-PW-02-01
**Status:** `[x]` DONE — domain.DirEntry.SizeBytes added; localfs.Executor.ReadDir populates it via entry.Info(); TestReadDir_ReportsSizeBytes passes

---

## Context

`domain.DirEntry` (`internal/domain/file.go:5-9`) currently has only
`Name`/`IsDirectory`. `localfs.Executor.ReadDir` (`executor.go:73-87`)
builds entries from `os.ReadDir` without calling `entry.Info()`, so no
size is ever computed on the host-local dispatch path. This task adds the
field and the local-exec population; the relay path is TASK-PW-02-03.

## Changes to make

In `internal/domain/file.go`:

```go
// DirEntry is one entry returned by FilesystemExecutor.ReadDir.
type DirEntry struct {
	Name        string
	IsDirectory bool
	// SizeBytes is 0 for directories. Populated from the same source
	// FileStat.SizeBytes already uses (os.Stat locally, the agent's
	// fs.stat/fs.readDir response over relay) — added SOL-PW-02.
	SizeBytes int64
}
```

In `internal/adapter/localfs/executor.go`'s `ReadDir`:

```go
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
	for _, ent := range entries {
		var size int64
		if !ent.IsDir() {
			info, err := ent.Info()
			if err != nil {
				continue // unreadable entry (permissions/race) — skip, don't fail the whole ReadDir
			}
			size = info.Size()
		}
		out = append(out, domain.DirEntry{Name: ent.Name(), IsDirectory: ent.IsDir(), SizeBytes: size})
	}
	return out, nil
}
```

Add/extend a test in `internal/adapter/localfs/executor_test.go` (or
create it if `ReadDir` has no dedicated test file yet): a real temp-dir
with a file of known byte length reports the exact size; a subdirectory
reports `0`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/localfs/... -run TestReadDir -v
go test ./services/git-gateway-service/internal/usecase/... -run TestReadDir -v
```

Expected: clean build; new/updated tests pass, including the "directory
reports 0" and "unreadable entry is skipped, not fatal" cases.
