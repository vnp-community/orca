# TASK-051: Extend `grpcclient.RelayExecutor` with `fs.*` relay methods

**From Solution:** SOL-009 (Design — `usecase/` layer, relay half of the local-vs-relay dispatch mechanism)
**Priority:** P0 — read/write/search usecase tasks depend on this
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
**Depends on:** TASK-049, TASK-050
**Status:** `[x]` DONE — all `fs.*` relay methods added to `RelayExecutor` (`ReadFile`/`ReadFilePreview`/`ReadDir`/`WriteFile`/`WriteFileChunk`/`CreateDir`/`Delete`/`Stat`/`Search`/`Glob`); no `Rename`/`Copy`/`ReadFileChunk` methods added, confirmed by a compile-time test (`TestRelayExecutor_ImplementsFilesystemExecutorNotLocalOnly` in `grpcclient_test.go`) that `*RelayExecutor` satisfies `usecase.FilesystemExecutor`. `go build`/`go vet`/`go test` clean.

---

## Context

`RelayExecutor` already implements `usecase.GitExecutor` (`GetStatus`,
`GetDiff`, `Commit`, `Push`, `Pull`) and `usecase.AICompleter` by calling
`infra-fleet-service`'s generic `Relay` RPC with method names like
`"git.status"` and JSON params, via its private `relay(ctx, connectionID,
method, params, out)` helper. This task adds `usecase.FilesystemExecutor`
to the same type, using the identical `relay(...)` helper with `"fs.*"`
method names — no new relay plumbing needed, this file's own doc comment
already documents that params/result field names are best-effort against
`agent-rpc-catalog-git-fs.md`'s Part B contract, not verified against a
live agent; the same caveat applies to the `fs.*` methods added here.

`RelayExecutor` deliberately does **not** implement
`usecase.LocalOnlyFilesystemExecutor` (`Rename`/`Copy`) — per BUG-009's
finding, the Dev Server Agent's `fs.*` surface has no `rename`/`copy`
method. That absence is what makes TASK-055's compile-time-plus-runtime
guard work: `RelayExecutor` simply has no `Rename`/`Copy` methods to call.

`ReadFileChunk` is likewise **not** added to `RelayExecutor` — it is
unsupported for any relay target by design (matching the old backend);
TASK-052's usecase checks `conn.Connected` before ever reaching this
executor, so no relay method for it is needed here.

---

## Changes to make

**File:** `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`

Add these methods to `RelayExecutor` (after the existing `Pull` method,
before `Complete`):

```go
// ── usecase.FilesystemExecutor ──────────────────────────────────────────
//
// Relays to the Dev Server Agent's fs.* methods. Per this file's package
// doc comment, field names below are named to match this service's own
// domain types, not verified against a live agent — reconcile against
// the real handler contract (specs/agent/api/agent-rpc-catalog-git-fs.md)
// before removing this comment, same caveat as the git.* methods above.

func (r *RelayExecutor) ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error) {
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := r.relay(ctx, repoPath, "fs.readFile", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result); err != nil {
		return nil, err
	}
	return decodeFileContent(result.Content, result.Encoding)
}

func (r *RelayExecutor) ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) ([]byte, bool, error) {
	var result struct {
		Content   string `json:"content"`
		Encoding  string `json:"encoding"`
		Truncated bool   `json:"truncated"`
	}
	if err := r.relay(ctx, repoPath, "fs.readFile", map[string]any{
		"path":     filepath.Join(repoPath, relPath),
		"maxBytes": maxBytes,
	}, &result); err != nil {
		return nil, false, err
	}
	content, err := decodeFileContent(result.Content, result.Encoding)
	return content, result.Truncated, err
}

func (r *RelayExecutor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	var result struct {
		Entries []domain.DirEntry `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "fs.readDir", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	return result.Entries, err
}

func (r *RelayExecutor) WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (int64, error) {
	var result struct {
		BytesWritten int64 `json:"bytesWritten"`
	}
	err := r.relay(ctx, repoPath, "fs.writeFile", map[string]any{
		"path":          filepath.Join(repoPath, relPath),
		"content":       base64.StdEncoding.EncodeToString(content),
		"encoding":      "base64",
		"createParents": createParents,
	}, &result)
	if err != nil {
		return 0, err
	}
	if result.BytesWritten == 0 {
		result.BytesWritten = int64(len(content))
	}
	return result.BytesWritten, nil
}

func (r *RelayExecutor) WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (int64, error) {
	var result struct {
		BytesWritten int64 `json:"bytesWritten"`
	}
	err := r.relay(ctx, repoPath, "fs.writeFile", map[string]any{
		"path":        filepath.Join(repoPath, relPath),
		"offsetBytes": offsetBytes,
		"content":     base64.StdEncoding.EncodeToString(content),
		"encoding":    "base64",
		"isFinal":     isFinal,
	}, &result)
	if err != nil {
		return 0, err
	}
	if result.BytesWritten == 0 {
		result.BytesWritten = int64(len(content))
	}
	return result.BytesWritten, nil
}

func (r *RelayExecutor) CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error {
	return r.relay(ctx, repoPath, "fs.mkdir", map[string]any{
		"path":      filepath.Join(repoPath, relPath),
		"recursive": recursive,
		"noClobber": noClobber,
	}, nil)
}

func (r *RelayExecutor) Delete(ctx context.Context, repoPath, relPath string, recursive bool) error {
	return r.relay(ctx, repoPath, "fs.rmdir", map[string]any{
		"path":      filepath.Join(repoPath, relPath),
		"recursive": recursive,
	}, nil)
}

func (r *RelayExecutor) Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error) {
	var result domain.FileStat
	err := r.relay(ctx, repoPath, "fs.stat", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	return result, err
}

func (r *RelayExecutor) Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	var result struct {
		Matches []domain.SearchMatch `json:"matches"`
	}
	err := r.relay(ctx, repoPath, "fs.grep", map[string]any{
		"repoPath":   repoPath,
		"pattern":    opts.Pattern,
		"isRegex":    opts.IsRegex,
		"pathGlob":   opts.PathGlob,
		"maxResults": opts.MaxResults,
	}, &result)
	return result.Matches, err
}

func (r *RelayExecutor) Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error) {
	var result struct {
		Paths []string `json:"paths"`
	}
	err := r.relay(ctx, repoPath, "fs.glob", map[string]any{
		"repoPath":   repoPath,
		"pattern":    pattern,
		"maxResults": maxResults,
	}, &result)
	return result.Paths, err
}

// decodeFileContent turns the agent's {content, encoding} pair into raw
// bytes, matching WriteFile/WriteFileChunk's own base64-on-the-wire
// convention above.
func decodeFileContent(content, encoding string) ([]byte, error) {
	if encoding == "base64" {
		return base64.StdEncoding.DecodeString(content)
	}
	return []byte(content), nil
}
```

Add `"encoding/base64"` and `"path/filepath"` to the file's import block
if not already present (the existing file imports `"context"`,
`"encoding/json"`, `"fmt"`, plus the `infrafleetv1` and `domain`
packages).

Note: `RelayExecutor` intentionally gains no `Rename`/`Copy`/`ReadFileChunk`
methods here — see Context above.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/adapter/grpcclient/...
go vet ./internal/adapter/grpcclient/...
```

Expected: clean build. `var _ usecase.FilesystemExecutor = (*RelayExecutor)(nil)`
compiles; `var _ usecase.LocalOnlyFilesystemExecutor = (*RelayExecutor)(nil)`
must NOT compile (add this as a commented-out line in a scratch file to
manually confirm during review, then remove it — do not commit it).
