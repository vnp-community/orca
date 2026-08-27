# TASK-CLI-03-05: `orca serve --daemon --local` + `orca daemon stop` (local start/stop, remote refusal)

**From Solution:** SOL-CLI-03
**Priority:** P1
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/command/serve.go`, `daemon_stop.go`
**Depends on:** TASK-CLI-03-03 (`ComposeSupervisor`), TASK-CLI-03-04 (`DaemonMode`)
**Status:** `[ ]` TODO

---

## Context

`orca serve --daemon --local` starts the compose stack and backgrounds the supervisor process; `orca daemon stop --local` tears it down. In remote/GitOps mode, `stop` is a **deliberate refusal**, not a stub: `10-deployment-infrastructure.md`'s Kubernetes topology has no "stop the daemon" operation for a stateless, multi-replica, GitOps-managed deployment — attempting one would either do nothing meaningful or scale down a Deployment behind ArgoCD's back.

## Changes to make

**1. `backend-go/cmd/orca-cli/internal/command/daemon_stop.go`:**

```go
package command

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

// ErrStopUnsupportedInGitOpsMode is RunDaemonStop's remote-mode result —
// a deliberate refusal (see this task's Context), not a fallback stub.
var ErrStopUnsupportedInGitOpsMode = errors.New("stopping a GitOps-managed deployment from the CLI is not supported — use kubectl/ArgoCD instead")

func RunDaemonStop(ctx context.Context, mode DaemonMode, sup *localdaemon.ComposeSupervisor) error {
	if mode == ModeRemote {
		return ErrStopUnsupportedInGitOpsMode
	}
	return sup.Stop(ctx)
}
```

**2. `backend-go/cmd/orca-cli/internal/command/serve.go`:**

```go
package command

import (
	"context"
	"errors"
	"os"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

var errServeRequiresLocal = errors.New("orca serve --daemon requires --local — there is no single daemon process to start against a GitOps-managed deployment")

// RunServe starts the local compose stack. --daemon backgrounds the
// process (handled by main.go's platform-specific daemonize step, see
// below); RunServe itself is the foreground body either way — Start
// already returns once `docker compose up -d` reports the containers
// launched, matching "exits once the compose stack reports healthy" per
// SOL-CLI-03's design (a fuller health-poll-before-returning loop can be
// layered on here using apiclient.GetHealth once TASK-CLI-03-01 is
// available to this package without an import cycle).
func RunServe(ctx context.Context, local bool, sup *localdaemon.ComposeSupervisor) error {
	if !local {
		return errServeRequiresLocal
	}
	return sup.Start(ctx)
}
```

Daemonizing (`--daemon` backgrounding the process) is a `main.go`-level concern, not `RunServe`'s — keep `RunServe` synchronous/testable, and fork/detach around it:

`backend-go/cmd/orca-cli/internal/localdaemon/daemonize_unix.go` (`//go:build !windows`):

```go
//go:build !windows

package localdaemon

import (
	"os"
	"os/exec"
	"syscall"
)

// Daemonize re-execs the current process detached from the controlling
// terminal (setsid) — the standard POSIX double-fork-equivalent for a Go
// process, since Go's exec.Cmd has no direct "daemonize in place"
// primitive. Returns immediately in the parent; the child continues as
// the backgrounded supervisor.
func Daemonize(args []string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd.Start()
}
```

`backend-go/cmd/orca-cli/internal/localdaemon/daemonize_windows.go` (`//go:build windows`):

```go
//go:build windows

package localdaemon

import (
	"os"
	"os/exec"
	"syscall"
)

// Daemonize on Windows uses CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS —
// the Windows-native equivalent of setsid-based detaching, per AGENTS.md's
// cross-platform-support rule (no POSIX-only assumption).
func Daemonize(args []string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x00000008} // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	return cmd.Start()
}
```

**3. `backend-go/cmd/orca-cli/internal/command/root.go`** — wire `serve --daemon --local` (calling `localdaemon.Daemonize` with the same args minus `--daemon` when backgrounding, else running `RunServe` in the foreground) and `daemon stop [--local]`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/command/... -run 'TestDaemonStop|TestServe' -v
```

Expected: `daemon_stop_test.go` — remote mode returns `ErrStopUnsupportedInGitOpsMode` without calling `sup.Stop` (assert on a fake/panicking `ComposeSupervisor`) — the refusal is a first-class tested outcome, not an omission. `serve_test.go` — `--local` false returns `errServeRequiresLocal` without calling `sup.Start`.

Integration (manual or CI-gated, per `03-clean-architecture-guidelines.md`'s end-to-end tier, once `docker-compose.yml` exists per TASK-CLI-03-03's prerequisite note): `orca serve --daemon --local` -> `orca daemon status --local` reports running with a live PID -> `orca daemon stop --local` -> a second `status` call reports stopped.
