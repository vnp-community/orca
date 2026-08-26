# TASK-058: Wire `files.rename`/`files.copy`/local bookkeeping channels and register `files.*` in `RegisterRealChannels`

**From Solution:** SOL-009 (Design — `wscompat` wiring, `files.rename`/`files.copy`/`files.commitUpload`/`files.unwatch` sections)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-057
**Status:** `[partial]` `files.rename`/`files.copy`/`files.commitUpload`/`files.unwatch` all registered, completing `registerFilesChannels` in `channels_git.go` — 16 of 18 `files.*` frontend methods now backed (the other 2, `writeBase64`/`createDirNoClobber`, collapse onto existing RPCs per TASK-049's design, not separate channels). NOT done: the actual `RegisterRealChannels`/`main.go` wiring call (`registerFilesChannels(r, gitClient)`) — deliberately left for the integration pass per the parent scope's explicit instruction not to edit `channels.go`/`main.go` directly. `go build`/`go vet`/`go test` clean; `gitgatewayv1.GitGatewayServiceClient` confirmed to include all 16 new methods.

---

## Context

Completes `registerFilesChannels` (started in TASK-057) with the two
known-gap RPCs and the two always-local bookkeeping channels, then wires
the whole function into `RegisterRealChannels`. `files.commitUpload` and
`files.unwatch` are renderer-side bookkeeping in the old backend (no fs
I/O) — registered as local no-op acks, matching
`crashReports.getLatestPending`'s in-process pattern, not forwarded to
`git-gateway-service`.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Append to the end of `registerFilesChannels` (added in TASK-057)

Replace the trailing comment left by TASK-057:

```go
	// files.rename / files.copy — known-gap RPCs; forwarded as-is, the
	// NOT_SUPPORTED-over-relay decision happens usecase-side (TASK-055),
	// wscompat just relays whatever status comes back.
```

with the real registrations:

```go
	type renameArgs struct {
		WorktreeID string `json:"worktreeId"`
		From       string `json:"fromPath"`
		To         string `json:"toPath"`
	}
	simpleFileOp(r, "files.rename", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.RenameFile(ctx, &gitgatewayv1.RenameFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err // FAILED_PRECONDITION over relay surfaces as-is
		}
		return resp, nil
	})
	simpleFileOp(r, "files.copy", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.CopyFile(ctx, &gitgatewayv1.CopyFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── Always-local bookkeeping — no gRPC call, matches
	// crashReports.getLatestPending's in-process pattern. ────────────
	r.Register("files.commitUpload", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	r.Register("files.unwatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
}
```

### Step 2: Wire into `RegisterRealChannels`

Find:

```go
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Replace with (add `registerFilesChannels(r, gitClient)` — reuses the
existing `gitClient` parameter, no new dial or new `RegisterRealChannels`
parameter needed, same client `registerGitChannels` already uses):

```go
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerFilesChannels(r, gitClient)
}
```

If TASK-046 (`emulator.*`) already landed and added
`registerEmulatorChannels(r)` to this same block, add
`registerFilesChannels(r, gitClient)` alongside it rather than replacing
that line.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go vet ./...
```

Expected: clean build across `api-gateway`. Confirm
`gitgatewayv1.GitGatewayServiceClient` (the generated interface) now
includes all 16 new methods from TASK-049's regenerated stubs — if it
doesn't, `client.ReadFile`/etc. calls above will fail to compile, which
means TASK-049's `buf generate proto` step needs to be re-run.
