# TASK-054: Implement file search/list-family usecases (`SearchFiles`, `ListAllFiles`, `ListMarkdownDocuments`) + local `Search`/`Glob`

**From Solution:** SOL-009 (Design — `usecase/` layer; signature table's `files.search`/`files.listAll`/`files.listMarkdownDocuments` rows)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/search_files.go`, `list_all_files.go`, `list_markdown_documents.go` (all new), `backend-go/services/git-gateway-service/internal/adapter/localfs/executor.go` (extend — fills in TASK-050's `Search`/`Glob` placeholders)
**Depends on:** TASK-050, TASK-051
**Status:** `[x]` DONE — `localfs.Executor.Search`/`.Glob` implemented in TASK-050's pass directly (not left as placeholders); `SearchFilesUseCase`/`ListAllFilesUseCase`/`ListMarkdownDocumentsUseCase` all implemented. One deviation from this task's sketch: `ListMarkdownDocumentsUseCase` calls `Glob` with `maxResults=0` (unlimited) and applies the `.md`/`.mdx` filter + the caller's `maxResults` cap together, rather than passing the caller's `maxResults` straight to the raw (pre-filter) `Glob` call — the original sketch would under-return results by capping candidates before the markdown filter ever ran. `go build`/`go vet`/`go test` clean; `.git` directory correctly excluded from `Search`/`Glob` walks (tested).

---

## Context

Same `dispatchFilesystemExecutor` shape as TASK-052/TASK-053.
`ListMarkdownDocuments` is "the same underlying `glob`/walk as
`ListAllFiles`, filtered server-side to `*.md`/`*.mdx` — one usecase, thin
wrapper, not a duplicate walk implementation" (SOL-009).

TASK-050 left `localfs.Executor.Search`/`.Glob` undefined (or stubbed
with a `panic`) since they need more machinery than the other methods.
This task fills them in.

---

## Changes to make

### Step 1: `localfs.Executor.Search`/`.Glob` (fills in TASK-050's placeholders)

**File:** `internal/adapter/localfs/executor.go` — replace the placeholder
`Search`/`Glob` methods (or add them if TASK-050 only left a comment) with:

```go
import (
	"path/filepath"
	"regexp"
	"strings"
	// ...(existing imports)
)

func (e *Executor) Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	var matcher func(string) bool
	if opts.IsRegex {
		re, err := regexp.Compile(opts.Pattern)
		if err != nil {
			return nil, fmt.Errorf("localfs: invalid search pattern: %w", err)
		}
		matcher = re.MatchString
	} else {
		matcher = func(line string) bool { return strings.Contains(line, opts.Pattern) }
	}

	var matches []domain.SearchMatch
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if len(matches) >= opts.MaxResults && opts.MaxResults > 0 {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(repoPath, path)
		if opts.PathGlob != "" {
			if ok, _ := filepath.Match(opts.PathGlob, rel); !ok {
				return nil
			}
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable file (binary, permissions) — skip, don't fail the whole search
		}
		for i, line := range strings.Split(string(content), "\n") {
			if matcher(line) {
				matches = append(matches, domain.SearchMatch{Path: rel, Line: i + 1, LineText: line})
				if opts.MaxResults > 0 && len(matches) >= opts.MaxResults {
					break
				}
			}
		}
		return nil
	})
	return matches, err
}

func (e *Executor) Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error) {
	var out []string
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if maxResults > 0 && len(out) >= maxResults {
			return filepath.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		if ok, _ := filepath.Match(pattern, rel); ok {
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}
```

`filepath.SkipAll` requires Go 1.20+; `git-gateway-service`'s `go.mod`
already targets a newer Go version (verify with `go version` — if the
module's Go directive predates 1.20, use a sentinel `errStopWalk` and
check `errors.Is` instead).

### Step 2: Usecases

**File:** `internal/usecase/search_files.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type SearchFilesUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewSearchFilesUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *SearchFilesUseCase {
	return &SearchFilesUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *SearchFilesUseCase) Execute(ctx context.Context, worktreeID string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.Search(ctx, conn.RepoPath, opts)
}
```

**File:** `internal/usecase/list_all_files.go`

```go
package usecase

import "context"

type ListAllFilesUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewListAllFilesUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ListAllFilesUseCase {
	return &ListAllFilesUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ListAllFilesUseCase) Execute(ctx context.Context, worktreeID, pathGlob string, maxResults int) ([]string, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	return exec.Glob(ctx, conn.RepoPath, pathGlob, maxResults)
}
```

**File:** `internal/usecase/list_markdown_documents.go`

```go
package usecase

import (
	"context"
	"strings"
)

// ListMarkdownDocumentsUseCase is a thin wrapper over the same
// FilesystemExecutor.Glob ListAllFilesUseCase uses, filtered to
// *.md/*.mdx server-side — not a duplicate walk implementation.
type ListMarkdownDocumentsUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewListMarkdownDocumentsUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ListMarkdownDocumentsUseCase {
	return &ListMarkdownDocumentsUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ListMarkdownDocumentsUseCase) Execute(ctx context.Context, worktreeID string, maxResults int) ([]string, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	all, err := exec.Glob(ctx, conn.RepoPath, "**/*", maxResults)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, p := range all {
		if strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".mdx") {
			out = append(out, p)
		}
	}
	return out, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/...
go vet ./internal/...
```

Expected: clean build — this task completes `localfs.Executor`'s
implementation of `usecase.FilesystemExecutor`, so `internal/...` should
now build fully (assuming TASK-052/TASK-053/TASK-055 are also in place).
