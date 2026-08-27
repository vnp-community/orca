# TASK-PW-02-04: `RelayExecutor.ReadDir` threads `size_bytes` from the agent's `fs.readDir` response

**From Solution:** SOL-PW-02
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
**Depends on:** TASK-PW-02-02
**Status:** `[x]` DONE — RelayExecutor.ReadDir now decodes the agent's real FileTreeNode shape (name/type/size) via an explicit intermediate struct, not the generic tag unmarshal; TestReadDir_MapsAgentFileTreeNodeShape passes

---

## Context

`RelayExecutor.ReadDir` (`relay_executor.go:829-837`) unmarshals the
agent's `fs.readDir` relay response directly into `[]domain.DirEntry` via
struct tags — once `domain.DirEntry` gains `SizeBytes` (TASK-PW-02-02),
whether this path "just works" depends entirely on whether the agent's
`fs.readDir` response already carries a size field on the wire.

**Verification step (do this first, before editing):** check
`specs/agent/api/agent-rpc-catalog-git-fs.md`'s `fs.readDir` response
shape.

- **If the agent's response already includes a per-entry size field**: add
  the matching `json` tag to `domain.DirEntry` (already done in
  TASK-PW-02-02 as `SizeBytes int64`; only the tag name may need
  adjusting here to match the agent's actual field name, e.g. `sizeBytes`
  vs `size_bytes`) — no other code change needed, since the struct-tag
  unmarshal in `ReadDir` already handles the rest generically.
- **If the agent's response does NOT include a size field**: this task's
  scope stops here. File a follow-up against `agent/` (not this
  solution's scope — `git-gateway-service` cannot fabricate a size the
  agent never sent) to add it to `fs.readDir`'s handler, and leave a
  `// TODO(SOL-PW-02):` comment on `RelayExecutor.ReadDir` noting relay-path
  entries report `SizeBytes: 0` until that lands. Do NOT silently drop
  this — surface it explicitly in the PR description.

## Changes to make

```go
func (r *RelayExecutor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	var result struct {
		Entries []domain.DirEntry `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "fs.readDir", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	return result.Entries, err
}
```

No change to this function's body is needed if the agent's field name
already matches `domain.DirEntry`'s new `json:"sizeBytes"` (or whatever
tag TASK-PW-02-02 used) — the existing generic struct unmarshal picks it
up automatically. Only add an explicit per-field mapping here if the
agent's wire field name differs from the Go struct tag.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/grpcclient/... -run TestReadDir -v
```

Expected: a contract test asserting the relay path's `size_bytes` mapping
against a fixture `fs.readDir` response payload (real shape, not
invented) — either passes because the agent already sends it, or is
committed as a documented-skip/TODO test if the agent-side gap is
confirmed real.
