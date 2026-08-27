# TASK-CLI-03-04: `orca daemon status` — remote health-check vs. local compose modes

**From Solution:** SOL-CLI-03
**Priority:** P1
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/command/daemon_status.go`
**Depends on:** TASK-CLI-03-01 (`GetHealth`), TASK-CLI-03-03 (`ComposeSupervisor`)
**Status:** [x] DONE — added daemon_status.go (RunDaemonStatus), localdaemon/paths.go (DefaultPidFile/DefaultComposeFile XDG helpers), wired `daemon status --local` in root.go (nil cli/sup enforced per mode), added daemon_status_test.go proving each mode never touches the other's dependency via nil-pointer-panic guard; `go test ./cmd/orca-cli/internal/command/... -run TestDaemonStatus -v` passes.

---

## Context

Two genuinely different answers behind one command, decided by one flag: `--local` (this host's compose stack, `ComposeSupervisor.Status`) vs. the default remote mode (`api-gateway`'s `/healthz`/`/readyz` via `apiclient.GetHealth`). The two modes must never bleed into each other — `--local` makes zero HTTP calls, default mode never shells out to `docker`.

## Changes to make

`backend-go/cmd/orca-cli/internal/command/daemon_status.go`:

```go
package command

import (
	"context"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

type DaemonMode int

const (
	ModeRemote DaemonMode = iota // default: --api-url points at a deployed api-gateway
	ModeLocal                    // --local: this host's docker-compose stack
)

type DaemonStatusResult struct {
	Mode    string `json:"mode"`
	Status  string `json:"status"`
	Ready   bool   `json:"ready,omitempty"`
	PID     int    `json:"pid,omitempty"`
}

// RunDaemonStatus never lets the two modes bleed into each other: ModeLocal
// never calls cli (nil-safe — the caller passes nil for cli in --local
// mode so a coding mistake here fails loudly, not silently), ModeRemote
// never touches sup.
func RunDaemonStatus(ctx context.Context, mode DaemonMode, cli *apiclient.Client, sup *localdaemon.ComposeSupervisor) (DaemonStatusResult, error) {
	if mode == ModeRemote {
		health, err := cli.GetHealth(ctx)
		if err != nil {
			return DaemonStatusResult{}, err // exit 1 — unreachable gateway
		}
		status := "healthy"
		if !health.Healthy {
			status = "unhealthy"
		}
		return DaemonStatusResult{Mode: "remote", Status: status, Ready: health.Ready}, nil
	}

	status, err := sup.Status()
	if err != nil {
		return DaemonStatusResult{}, err
	}
	state := "stopped"
	if status.Running {
		state = "running"
	}
	return DaemonStatusResult{Mode: "local", Status: state, PID: status.PID}, nil
}
```

Wire `--local` as a bool flag in `internal/command/root.go`'s `daemon status` subcommand: when set, construct `sup := &localdaemon.ComposeSupervisor{...}` and pass `cli: nil`; when unset, construct `cli := apiclient.New(...)` from resolved credentials and pass `sup: nil`. Never construct both.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/command/... -run TestDaemonStatus -v
```

Expected new test `daemon_status_test.go`: `--local` mode never makes an HTTP call — pass a `cli` that panics/errors on any method call and assert `RunDaemonStatus(ctx, ModeLocal, panicyClient, fakeSupervisor)` never triggers it; default/remote mode never shells out to `docker` — pass a `sup` whose `Status`/`Start`/`Stop` all panic and assert `RunDaemonStatus(ctx, ModeRemote, fakeClient, panicySupervisor)` never triggers it. This is the regression guard against the two modes bleeding into each other.
