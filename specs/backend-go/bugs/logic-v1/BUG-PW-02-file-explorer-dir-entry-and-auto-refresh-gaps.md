# BUG-PW-02: File Explorer — directory listing lacks file size, and auto-refresh-after-agent has no trigger

**Business Logic:** [BL-PW-02](../../../../docs/logic/project-workspace/BL-PW-02-remote-file-explorer.md) — Remote File Explorer
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Low
**Symptom:** The file tree can be browsed, searched, and files can be viewed/edited/deleted for real end-to-end — but (1) every entry the tree renders is missing its file size (the backend directory-listing RPC simply doesn't return it, so the UI would need a separate `files.stat` round-trip per row to show one), and (2) "auto-refresh sau agent complete" (an explicit acceptance criterion) has no way to fire, because nothing in backend-go emits an event when an agent run finishes.

---

## Spec summary

BL-PW-02 specifies a lazy-loading remote file tree with inline git-status decorations, a read-only file viewer capped at 5MB, name+content search, a context menu (copy path, open in terminal, git actions), a hidden-files toggle, and a refresh mechanism that includes automatic refresh after an agent completes.

## What backend-go has

The `files.*` namespace (BUG-009 in `missing-v1` reported this as an 18/18 full-namespace gap) is now comprehensively wired for real:

- `files.read`, `files.stat`, `files.readDir`, `files.readChunk`, `files.readPreview`, `files.write`/`writeBase64`, `files.writeBase64Chunk`, `files.createDir`/`createDirNoClobber`, `files.delete`, `files.search`, `files.listAll`, `files.listMarkdownDocuments`, `files.rename`, `files.copy`, `files.commitUpload`, `files.unwatch` — all registered in `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:702-919` (`registerFilesChannels`), each calling a real `GitGatewayServiceClient` RPC backed by a real usecase (e.g. `internal/usecase/read_dir.go`, `search_files.go`, `stat_file.go` in `git-gateway-service`).
- `registerFilesChannels` is actually wired into the live dispatch path: `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:117` calls it from `RegisterRealChannels` (do not be misled by the stale "NOT called yet" doc comment at the top of `channels_git.go:7-9` — that comment predates the integration pass; line 117 confirms it is live).
- `workspace.refreshFileTree` (manual refresh) is a real implementation, not a stub — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:946-975`.
- Git status decorations are feasible client-side: `git.status`/`git.diff` are wired (`channels.go:267-281`), so the frontend can merge a separate status call with the directory listing, matching the spec's own `gitStatusMap.get(e.path)` client-side merge pattern.

## What's missing

- **No file size in directory listings.** `ReadDirRequest` (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:458-461`) takes only `worktree_id`/`path`, and `DirEntry` (`gitgateway.proto:462-465`) has only `name` and `is_directory` — no `size_bytes`. The spec's `expandDirectory()` explicitly maps `size: e.sizeBytes` onto every tree node (BL-PW-02 doc lines 31-38). Today the only way to get a size is a separate `files.stat` call per file (`StatFileResponse` does have `size_bytes` — `gitgateway.proto:510-515`), which is an N+1 round-trip the spec's single `fs.readDir` call doesn't require.
- **No `depth`/`includeDotFiles`/`sortBy`/`foldersFirst` on directory listing.** The spec's `fs.readDir` call takes `{ path, depth, includeDotFiles, sortBy, foldersFirst }` (BL-PW-02 doc lines 24-30); `ReadDirRequest` only has `path`, so multi-level expansion requires one call per directory level (acceptable for "lazy-load," per the AC) but hidden-file filtering and sort order must be done entirely client-side with no server hint either way — consistent with "lazy-load," but a real capability gap versus the documented contract.
- **"Refresh button + auto-refresh sau agent complete"** (BL-PW-02's own acceptance criterion) has no backend trigger: nothing publishes an "agent complete" event that the file explorer (or any panel) could subscribe to — see BUG-PW-04 for the full cross-panel event-bus gap this depends on. The manual refresh half of this AC is real (`workspace.refreshFileTree`); the automatic half is not.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — describes the state before `channels_git.go`'s `registerFilesChannels` landed; that finding is now stale/resolved for all 18 methods and should not be re-reported as a full-namespace gap.
- `specs/backend-go/bugs/logic-v1/BUG-PW-04-workspace-integration-not-implemented.md` — root cause of the "auto-refresh after agent complete" gap (no cross-service event bus wiring for `agent.complete`).

## References

- `docs/logic/project-workspace/BL-PW-02-remote-file-explorer.md:19-39,127-136` — `expandDirectory()` contract and acceptance criteria
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:458-468` — `ReadDirRequest`/`DirEntry`/`ReadDirResponse` (no size field)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:506-515` — `StatFileRequest`/`StatFileResponse` (has `size_bytes`, but per-file only)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:702-919` — `registerFilesChannels`, all 18 `files.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:117` — confirms `registerFilesChannels` is live in `RegisterRealChannels`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:946-975` — `workspace.refreshFileTree`
- `backend-go/services/git-gateway-service/internal/usecase/read_dir.go` — `ReadDirUseCase.Execute`
