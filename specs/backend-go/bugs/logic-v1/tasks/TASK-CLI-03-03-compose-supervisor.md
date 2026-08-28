# TASK-CLI-03-03: `ComposeSupervisor` — wraps the repo-root `docker compose` lifecycle

**From Solution:** SOL-CLI-03
**Priority:** P0 — `daemon status`/`daemon stop --local` (TASK-CLI-03-04/05) depend on this
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/localdaemon/compose_supervisor.go`
**Depends on:** TASK-CLI-03-02
**Status:** [x] DONE — added compose_supervisor.go with a `runComposeCmd` injectable command-runner seam (never shells to real docker in tests) + compose_supervisor_test.go (refuses-second-start-no-extra-invoke, stale-pidfile-allows-restart, stop-removes-pidfile-on-failure, missing-compose-file-fails-fast-no-invoke); confirmed `backend-go/docker-compose.yml` exists (not repo-root — path resolution is TASK-CLI-03-04/05's job, unaffected here); `go test ./cmd/orca-cli/internal/localdaemon/... -run TestComposeSupervisor -v` passes.

---

## Context

`04-tech-stack.md`'s "Explicitly rejected options" table rejects "a monolithic Go binary... instead of 17 independently deployable services" — building a new process that forks/supervises all 17 services would be that rejected shape. This solution instead shells out to the **existing** repo-root `docker-compose.yml` (`10-deployment-infrastructure.md`'s "Local development" section) — the compose file stays the one source of truth for what "all services" means, so this supervisor can never drift from `make dev-up`'s own definition.

**Prerequisite check:** confirm `docker-compose.yml` exists at the repo root before writing this task's code (`ls /opt/repos/orca/docker-compose.yml`). It does not exist yet as of this task's authoring — if still absent when this task is picked up, creating/maintaining that compose file is explicitly out of this solution's scope (SOL-CLI-03 says it "reads it, does not redesign it"); in that case `ComposeSupervisor.Start` should fail fast with a clear `"docker-compose.yml not found at <path> — see 10-deployment-infrastructure.md's Local development section"` error rather than silently no-op, and this task's own tests should cover that failure path explicitly.

## Changes to make

`backend-go/cmd/orca-cli/internal/localdaemon/compose_supervisor.go`:

```go
package localdaemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// ComposeSupervisor shells out to `docker compose` against the repo-root
// docker-compose.yml — it never starts a Go process directly and never
// embeds service logic; the compose file remains the one source of truth
// for what "all services" means, so this supervisor can't drift from
// `make dev-up`'s own definition of the stack.
type ComposeSupervisor struct {
	ComposeFile string // repo-root docker-compose.yml
	PidFile     string // ~/.local/share/orca/daemon.pid — BR-CLI-10
}

type Status struct {
	Running bool
	PID     int
}

// Start brings the compose stack up. Records THIS supervisor process's own
// PID (os.Getpid()) — not any individual service's — see pidfile.go's doc
// comment for why: there is no single "the daemon process" among N
// containers.
func (s *ComposeSupervisor) Start(ctx context.Context) error {
	if pid, running := readPIDFile(s.PidFile); running {
		return fmt.Errorf("orca serve already running (pid %d) — stop it first or use --force", pid)
	}
	if _, err := os.Stat(s.ComposeFile); err != nil {
		return fmt.Errorf("compose file not found at %s — see 10-deployment-infrastructure.md's Local development section: %w", s.ComposeFile, err)
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.ComposeFile, "up", "-d")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting compose stack: %w", err)
	}
	return writePIDFile(s.PidFile, os.Getpid())
}

// Stop tears the compose stack down. Removes the PID file even if
// `docker compose down` fails — best-effort cleanup: a failed teardown
// should not permanently wedge Start into refusing to run again.
func (s *ComposeSupervisor) Stop(ctx context.Context) error {
	defer os.Remove(s.PidFile)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.ComposeFile, "down")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping compose stack: %w", err)
	}
	return nil
}

// Status reports the local supervisor's recorded PID and whether it's
// still alive — cross-checks readPIDFile's liveness probe so a crashed
// supervisor with a stale PID file self-corrects to "not running" rather
// than a false positive.
func (s *ComposeSupervisor) Status() (Status, error) {
	pid, running := readPIDFile(s.PidFile)
	return Status{Running: running, PID: pid}, nil
}
```

Default `ComposeFile`/`PidFile` paths (constructed by the command layer in TASK-CLI-03-04/05, not hardcoded here): `ComposeFile` = repo-root `docker-compose.yml` resolved relative to the working directory or an `ORCA_COMPOSE_FILE` env override; `PidFile` = `filepath.Join(os.UserHomeDir()-or-XDG-data-dir, "orca", "daemon.pid")` per BR-CLI-10 (`~/.local/share/orca/daemon.pid` on Linux — use `os.UserCacheDir`/a small XDG helper for cross-platform correctness, not a hardcoded `~/.local/share` string, per `AGENTS.md`'s file-paths rule).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/localdaemon/... -run TestComposeSupervisor -v
```

Expected new test `compose_supervisor_test.go` (fake `exec.Command` via a `PATH`-injected stub script, or an injectable command-runner seam — do not shell out to real `docker` in unit tests): `Start` refuses a second start while a live PID is recorded, without invoking `docker` a second time; a stale PID file (recorded PID not alive) does not block a fresh `Start`; `Stop` removes the PID file even when the stubbed `docker compose down` exits non-zero; `Start` against a missing compose file fails fast with the documented error message and never invokes `docker`.
