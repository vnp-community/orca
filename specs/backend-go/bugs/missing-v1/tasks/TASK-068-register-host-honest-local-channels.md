# TASK-068: Register `host.*` honest local-answer channels

**From Solution:** SOL-011 (Design — Part 1: shippable now)
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[x]` DONE (verified) — implemented in the same new file as
TASK-046/TASK-066 (`channels_emulator_folderworkspace_host.go`), called
from `registerEmulatorFolderWorkspaceHostChannels`. `go build`/`go vet`
clean. Not yet wired into `RegisterRealChannels` — see TASK-066's note;
same one-line integration step covers all three namespaces in this file.

---

## Context

`host.wsl.isAvailable`, `host.wsl.listDistros`, `host.pwsh.isAvailable`,
and `host.gitBash.isAvailable` currently fall through to
`notImplementedHandler`. BUG-011 asked whether these should probe the
backend-go host itself or the caller's per-target dev server.

The answer is both, at different time horizons (mirrors SOL-008's
two-part shape): **now**, `backend-go`'s own host is a Linux container
(`10-deployment-infrastructure.md`'s deployment model) — none of
WSL/PowerShell/git-bash are meaningful on it, so `false`/`[]` is the
honest answer, not a placeholder, same posture as `preflight.check`'s
honest `gh`/`glab` `false` answers. **Later**, per-target resolution (does
*this worktree's* dev server have these tools) is blocked on an `agent/`
capability that doesn't exist today — see TASK-070, which documents that
target design without implementing it.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add at the end of the file:

```go
// ── host.* ──────────────────────────────────────────────────────────
//
// WSL/PowerShell/git-bash availability on the *backend-go host itself* —
// per BUG-011, the old backend probed only its own process host, never a
// per-target dev server. backend-go's own host is a Linux container
// (10-deployment-infrastructure.md's deployment model) with none of
// these three tools meaningful on it, so "false"/"[]" is the honest
// answer here, not a placeholder — same posture as preflight.check's
// honest gh/glab false answers below. Per-target (does the CALLER'S
// ACTIVE DEV SERVER have these) is a distinct, more useful question —
// see specs/backend-go/bugs/missing-v1/tasks/TASK-070 for that design,
// which is blocked on an agent/ capability that doesn't exist yet.
func registerHostChannels(r *Registry) {
	r.Register("host.wsl.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.wsl.listDistros", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return []string{}, nil
	})
	r.Register("host.pwsh.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.gitBash.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
}
```

Wire into `RegisterRealChannels`:

```go
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerHostChannels(r)
	// (alongside registerEmulatorChannels(r) / registerFilesChannels(r, gitClient) /
	// registerFolderWorkspaceChannels(r, projectClient) if those tasks already landed)
}
```

No new `RegisterRealChannels` parameter needed — `registerHostChannels`
takes only `r`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build.
