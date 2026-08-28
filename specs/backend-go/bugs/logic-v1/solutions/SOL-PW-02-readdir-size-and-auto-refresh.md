# SOL-PW-02: Add `size_bytes` to `ReadDir`, and consume SOL-PW-04's event bridge for auto-refresh

**Resolves:** [BUG-PW-02](../BUG-PW-02-file-explorer-dir-entry-and-auto-refresh-gaps.md)
**Service:** `git-gateway-service` (proto/usecase/adapter extension) + `api-gateway` (no new work — see "Auto-refresh" below)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`DirEntry`, `ReadDirRequest`)
- `backend-go/services/git-gateway-service/internal/domain/` (`DirEntry` value object, if one exists separately from the proto type — verify before adding a parallel one)
- `backend-go/services/git-gateway-service/internal/usecase/read_dir.go`
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (`GitExecutor.ReadDir`/`FilesystemExecutor.ReadDir` signature, per whichever exists post-SOL-009)
- `backend-go/services/git-gateway-service/internal/adapter/localfs/` (populate `size_bytes`/`is_directory` from `os.Stat`)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (thread `stat`'s size field through `fs.readDir`'s relay response, if the agent already returns it — verify against `agent-rpc-catalog-git-fs.md`'s `fs.readDir` response shape before assuming a client-side N+1 is still needed)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go` (`registerFilesChannels`'s `files.readDir`/`registerGitDeepChannels`'s `workspace.refreshFileTree` — passthrough only, no logic change)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

This bug is two unrelated gaps bundled under one BL; they get two
independent, differently-sized fixes.

### Gap 1 — `size_bytes` on `ReadDir`: a real, small proto/usecase change

`git-gateway-service.md` §4 describes this service's domain types as
"value objects mirrored from the Dev Server Agent's wire protocol... no
constructor enforces a git invariant" — `DirEntry` is exactly this kind of
type, so adding a field is a pure mechanical extension, not a design
decision. §2 already establishes the dispatch shape (resolve → local exec
or relay → translate) every `ReadDir` call already follows; this solution
adds one field to the translate step, nothing else.

`StatFileResponse` already has `size_bytes` (`gitgateway.proto:510-515`,
cited directly in BUG-PW-02) — proving the *value* is already computable
on both the local (`os.Stat`) and relay (agent's `fs.stat`) paths, per
`git-gateway-service.md` §2 steps 3/3'. The gap is purely that
`ReadDirResponse.DirEntry` never asks for it. BL-PW-02's own
`expandDirectory()` contract (`docs/logic/project-workspace/BL-PW-02-remote-file-explorer.md:19-39`)
maps `size: e.sizeBytes` directly off the directory-listing response, not
off a follow-up `files.stat` call — so this is a genuine contract gap
versus the documented frontend consumer, not a nice-to-have.

**Depth calibration**: BUG-PW-02 rates this gap Low severity precisely
because a client-side N+1 `files.stat`-per-entry workaround exists today
(both `files.stat` and `files.readDir` are already real, wired channels
per BUG-PW-02's own "What backend-go has" section) — this solution closes
the gap at the source rather than leaving the N+1 as the permanent
answer, but does not warrant `depth`/`includeDotFiles`/`sortBy`/
`foldersFirst` server-side filtering in the same pass: BUG-PW-02 itself
frames those as "a real capability gap versus the documented contract"
but consistent with lazy-load acceptance criteria already being met
client-side — so this solution ships `size_bytes` now and flags the
filter/sort fields as a follow-up, not silently drops them.

### Gap 2 — Auto-refresh after agent complete: not this service's fix

BUG-PW-02 already correctly attributes the root cause to
[BUG-PW-04](../BUG-PW-04-workspace-integration-not-implemented.md): "the
automatic half is not [implemented] — see BUG-PW-04 for the full
cross-panel event-bus gap this depends on." [SOL-PW-04](./SOL-PW-04-workspace-integration-event-bus.md)'s
`api-gateway` workspace-event bridge is the actual fix — once a
`orca.task.task.statuschanged` (or the workflow-completion equivalent)
event reaches a connected session as a `workspace.event` WS frame, the
frontend's existing, already-real `workspace.refreshFileTree` channel
(`channels_git.go:946-975`, confirmed real by both BUG-PW-02 and
BUG-PW-01) is the trigger target — **no git-gateway-service change is
needed for the automatic half**, only the frontend subscribing to the
frame SOL-PW-04 introduces and calling the channel it already has. This
solution's scope for Gap 2 is therefore just this citation, not
duplicate design — implementing it here would fork the event-bus design
across two solution docs for one mechanism.

## Design — Proto changes (`gitgateway.proto`)

```protobuf
message DirEntry {
  string name = 1;
  bool is_directory = 2;
  // Added (SOL-PW-02): 0 for directories (matches StatFileResponse's own
  // convention — a directory's "size" isn't a meaningful git-gateway-service
  // concept, the frontend already doesn't render one for folder rows per
  // BL-PW-02's expandDirectory() contract). Populated from the same
  // os.Stat/fs.stat source StatFile already uses on both dispatch paths.
  int64 size_bytes = 3;
}
```

No new RPC, no request-shape change — `ReadDirRequest` is untouched.
`ReadDirResponse` gains the field on its existing `entries` type, which is
purely additive on the wire (proto3 field addition, safe for any client
still on the old shape to ignore).

## Design — `usecase`/adapter layer

```go
// internal/usecase/read_dir.go — the executor call already returns
// []domain.DirEntry; extending that struct's shape (adapter/local +
// adapter/grpcclient) is the entire change here. The usecase's own body
// (resolve worktree → resolve host → dispatch → translate) is unchanged.
```

```go
// internal/adapter/localfs/... — wherever ReadDir currently builds
// domain.DirEntry from os.ReadDir's entries, add:
info, err := entry.Info()
if err != nil {
    continue // unreadable entry (permissions/race) — already the existing
              // skip behavior for a failed os.Stat-equivalent, per whatever
              // this package's current error handling does for entries it
              // can't stat; do not regress that by treating a stat failure
              // as a hard ReadDir failure.
}
size := int64(0)
if !info.IsDir() {
    size = info.Size()
}
```

```go
// internal/adapter/grpcclient/relay_executor.go — ReadDir's relay call
// already gets back whatever the agent's fs.readDir response carries.
// Verify against specs/agent/api/agent-rpc-catalog-git-fs.md's fs.readDir
// entry (not read for this solution) whether the agent's DirEntry
// equivalent already includes a size field:
//   - If yes: thread it straight through, matching every other relay
//     field-mapping in this file (e.g. FastForward/RebaseFromBase's 1:1
//     param passthrough).
//   - If no: the agent's fs.readDir handler needs a small size_bytes
//     addition too (an agent/ change) — flag this explicitly per this
//     task's instruction, since it would make this solution's scope
//     cross into agent/ for the relay-dispatch case only (the host-local
//     case above needs no agent change regardless).
```

## Test plan

- `git-gateway-service/internal/usecase/read_dir_test.go` — fake
  `FilesystemExecutor`/`GitExecutor` returning entries with known sizes;
  assert the usecase passes `size_bytes` through unchanged for files and
  zero for directories.
- `adapter/localfs/read_dir_test.go` (or equivalent existing test file) —
  real temp-dir integration test: a file of known byte length reports the
  exact size; a subdirectory reports `0`; an unreadable entry (permission-
  denied fixture) is skipped, not fatal.
- `adapter/grpcclient` — contract test asserting the relay path's
  `size_bytes` mapping, once the agent-side answer above is resolved.
- `wscompat/channels_git_test.go` — `files.readDir`'s existing test
  updated to assert `sizeBytes` appears in the returned entry shape (a
  passthrough regression guard, not new logic to test).
- Explicitly **not** in this solution's test plan: auto-refresh-after-
  agent-complete tests — those belong to SOL-PW-04's test plan.

## References

- `docs/logic/project-workspace/BL-PW-02-remote-file-explorer.md:19-39,127-136` — `expandDirectory()` contract mapping `size: e.sizeBytes`, and the refresh acceptance criterion
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:458-468` — current `ReadDirRequest`/`DirEntry`/`ReadDirResponse` (no size field)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:506-515` — `StatFileResponse` (already has `size_bytes`), proving the value is already computable on both dispatch paths
- `specs/backend-go/tdd/services/git-gateway-service.md:142-160` (§4 domain model — "value objects... no constructor enforces a git invariant", why this is a mechanical extension), `:36-74` (§2 resolve→dispatch→translate, the shape this solution's translate step extends)
- `specs/backend-go/bugs/logic-v1/BUG-PW-04-workspace-integration-not-implemented.md` and [SOL-PW-04](./SOL-PW-04-workspace-integration-event-bus.md) — the actual fix for this bug's Gap 2 (auto-refresh trigger)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:946-975` — `workspace.refreshFileTree`, the already-real trigger target Gap 2's fix calls into
- `backend-go/services/git-gateway-service/internal/adapter/` — `localfs`/`grpcclient` package names confirmed via directory listing (`ls services/git-gateway-service/internal/adapter/`)
