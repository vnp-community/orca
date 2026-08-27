# SOL-CLI-03: `orca daemon status` as a health-check client; `orca serve --daemon` as a local docker-compose supervisor — not a new monolithic process

**Resolves:** [BUG-CLI-03](../BUG-CLI-03-headless-mode-partial.md)
**Service:** `backend-go/cmd/orca-cli/` (new `daemon`/`serve` commands, built on [SOL-CLI-01](./SOL-CLI-01-orca-cli-worktree-create.md)) — no changes to any of the 17 backend-go services beyond what SOL-CLI-01/02 already propose
**Affected files (proposed):**
- `backend-go/cmd/orca-cli/internal/command/daemon_status.go`, `serve.go`, `daemon_stop.go`
- `backend-go/cmd/orca-cli/internal/localdaemon/compose_supervisor.go`, `pidfile.go`, `controlsocket.go`
- `backend-go/cmd/orca-cli/internal/apiclient/health.go`
- `docker-compose.yml` (repo-root Go workspace compose file per `10-deployment-infrastructure.md`'s "Local development" section — no change needed if it already exists; this solution reads it, does not redesign it)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD) — the spec's daemon model doesn't map onto backend-go's, and that's not a gap to fill by imitation

BUG-CLI-03's own "What backend-go has" section already establishes the load-bearing fact: every backend-go service is "inherently headless by construction" — a plain Go binary with zero GUI dependencies, real SIGTERM-triggered graceful shutdown (`main.go:71,351-352`), and real HTTP-API auth (`middleware.go:50-67`). BR-CLI-08/09/11/12 are **already satisfied**, per-service, today. What's actually missing — a single `orca serve`/daemon *process*, a PID file, a Unix socket — describes the Electron desktop app's `PtyDaemon`/`runtime.sock` model (confirmed: `desktop/src/cli/runtime/transport.test.ts:33`), not backend-go's.

**Building that model in backend-go would contradict the TDD, not fulfill it.** `04-tech-stack.md`'s "Explicitly rejected options" table is unambiguous: "A monolithic Go binary with internal packages instead of real services" was rejected in favor of "17 independently deployable services" because "the user's explicit requirement is microservices with independent production-readiness — a modular monolith doesn't meet 'must be organized as microservices'." A new `orca serve` binary that forks/supervises all 17 services as one daemon process *is* that rejected shape. `10-deployment-infrastructure.md`'s Kubernetes topology (§"Kubernetes topology") — Helm chart per service, `NetworkPolicy` default-deny, 3+ replicas each, GitOps-controlled promotion — has no "stop the daemon" operation at all in `staging`/`production`: the deployed state is whatever ArgoCD's config repo declares (§"CI/CD": "the deployed state is whatever's declared in the GitOps config repo, not whatever someone `kubectl apply`'d by hand"), and `api-gateway.md` §2/§8 establishes every replica is stateless and interchangeable — there is no single process whose PID a CLI could meaningfully record or SIGTERM.

**What the bug's own symptom actually needs, and where the TDD already has it.** The symptom is "a DevOps engineer trying to run `orca serve --daemon` ... in a Docker container" — i.e., a **local/CI single-host** convenience, not a production deployment primitive. `10-deployment-infrastructure.md`'s "Local development" section already specifies exactly this: a repo-root `docker-compose.yml` bringing up all 17 services + local Postgres + local Vault + local NATS, with `make dev-up`/`make dev-down` wrapping the lifecycle. This solution does not invent new orchestration — it makes `orca serve --daemon`/`orca daemon status`/`orca daemon stop` a **thin CLI wrapper around that existing compose lifecycle**, giving the DevOps engineer's Docker-container use case a real command without building a second, competing supervisor architecture.

**Production/staging get a different, honest answer, not a fake one.** For `dev`/`staging`/`production` (GitOps-managed), `orca daemon status` becomes a **read-only health check** against `api-gateway`'s already-real `/healthz`/`/readyz` (`common/health/health.go` — confirmed live, every service mounts it per `main.go:11-12`'s doc comment), and `orca daemon stop` in this mode returns a clear, non-destructive error directing the operator to `kubectl`/ArgoCD rather than attempting an operation the architecture doesn't support for a multi-replica stateless service. Pretending to support "stop the daemon" against a GitOps-managed 3-replica deployment would either do nothing meaningful or scale down a Deployment behind the GitOps system's back — actively harmful, not merely unimplemented.

---

## Design — `orca-cli daemon`/`serve`: two modes, one flag deciding which

```go
// internal/command/daemon_status.go
type DaemonMode int
const (
	ModeRemote DaemonMode = iota // default: --api-url points at a deployed api-gateway
	ModeLocal                    // --local: this host's docker-compose stack
)

func RunDaemonStatus(ctx context.Context, mode DaemonMode, cli *apiclient.Client, sup *localdaemon.ComposeSupervisor) (Result, error) {
	if mode == ModeRemote {
		// api-gateway's own /healthz+/readyz — a real, already-load-bearing
		// signal (Kubernetes liveness/readiness probes use the same
		// endpoint), not a new health concept invented for this CLI.
		health, err := cli.GetHealth(ctx) // GET {api-url}/healthz then /readyz
		if err != nil {
			return Result{}, err // exit 1 — unreachable gateway
		}
		return Result{Status: health.Status, Ready: health.Ready}, nil
	}
	status, err := sup.Status() // reads the local PID file, see below
	if err != nil {
		return Result{}, err
	}
	return Result{Status: status.State, PID: status.PID}, nil
}
```

```go
// internal/command/daemon_stop.go
func RunDaemonStop(ctx context.Context, mode DaemonMode, sup *localdaemon.ComposeSupervisor) (Result, error) {
	if mode == ModeRemote {
		// Deliberate refusal, not a stub — see "Design rationale" above.
		return Result{}, ErrStopUnsupportedInGitOpsMode // exit 1, message points to kubectl/ArgoCD
	}
	return sup.Stop(ctx) // docker compose down, see below
}
```

### `internal/localdaemon/compose_supervisor.go` — wraps the existing compose file, doesn't replace it

```go
// ComposeSupervisor shells out to `docker compose` against the repo-root
// docker-compose.yml (10-deployment-infrastructure.md's "Local
// development" section) — it never starts a Go process directly and never
// embeds service logic; the compose file remains the one source of truth
// for what "all 17 services" means, so this supervisor can't drift from
// `make dev-up`'s own definition of the stack.
type ComposeSupervisor struct {
	composeFile string // defaults to the workspace-root docker-compose.yml
	pidFile     string // ~/.local/share/orca/daemon.pid — BR-CLI-10
	sockFile    string // ~/.local/share/orca/daemon.sock — local-only control channel, see below
}

func (s *ComposeSupervisor) Start(ctx context.Context, opts StartOptions) error {
	if pid, running := readPIDFile(s.pidFile); running {
		return fmt.Errorf("orca serve already running (pid %d) — stop it first or use --force", pid)
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composeFile, "up", "-d")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting compose stack: %w", err)
	}
	// Records THIS supervisor process's own PID (it stays resident to hold
	// the control socket and forward SIGTERM->`docker compose down`), not
	// any individual service's PID — there is no single "the daemon
	// process" among 17 containers, so the PID file names the one process
	// that actually corresponds to "the orca serve invocation", per
	// BR-CLI-10's intent (prevent double-start) without fabricating a PID
	// that doesn't mean what the spec assumes it means.
	return writePIDFile(s.pidFile, os.Getpid())
}

func (s *ComposeSupervisor) Stop(ctx context.Context) error {
	defer os.Remove(s.pidFile)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composeFile, "down")
	return cmd.Run()
}
```

`serve --daemon` backgrounds this supervisor process (standard double-fork/`setsid` or `nohup`-equivalent, platform-specific per `AGENTS.md`'s cross-platform-support rule — Linux/macOS via `syscall.Setsid`, Windows via a detached process creation flag, not a POSIX-only assumption) and exits once the compose stack reports healthy; it does not itself become a 17-service-forking process, keeping the "monolithic Go binary" anti-pattern (`04-tech-stack.md`) entirely out of the picture — the actual services remain 17 independent containers, this is only the local convenience layer that starts/stops/asks about them together.

### BR-CLI-10 — PID file, scoped honestly

Written at `~/.local/share/orca/daemon.pid` as the spec names it, but documented (in `--help` and this solution) as "the local supervisor's PID, not an individual service's" — `daemon status --local` also cross-checks the PID is still alive (`syscall.Kill(pid, 0)` / equivalent) before reporting "running", so a crashed supervisor with a stale PID file self-corrects to "stopped" rather than reporting a false positive.

### `~/.local/share/orca/daemon.sock` — kept, but re-scoped to local-only IPC

The spec's Unix socket is preserved for exactly the case where it still adds value: `daemon status --local`/`daemon stop --local` querying the resident supervisor process without re-shelling to `docker compose ps` every call (cheaper, and lets the supervisor report richer state, e.g. "3/17 containers healthy"). It carries **no business traffic** — worktree/agent commands never touch it, they talk to `api-gateway` over REST+JWT (per SOL-CLI-01/02) exactly the same whether `api-gateway` is local-compose or remote-K8s. This keeps BR-CLI-08/12's "no GUI/display dependency" property intact (a Unix socket used only for local supervisor IPC has nothing to do with headlessness) while not resurrecting the Electron daemon's role as the transport for actual commands.

---

## Design — SLOs: measure, don't fabricate a mechanism

BUG-CLI-03 correctly flags the SLO table (`<2s` startup, `<150MB` idle, `<500ms` command response) as unmeasured, not unimplementable — these are properties of already-real Go service binaries, not missing code paths. This solution adds **measurement**, in the two places the TDD already has a slot for it, rather than new runtime instrumentation:

- `10-deployment-infrastructure.md`'s CI/CD pipeline step "build + scan container image (Trivy)" gains a startup-time smoke check per service: start the built image, poll `/healthz` until `200`, fail the build if it exceeds 2s (k6 or a simple Go test harness — `04-tech-stack.md`'s "Load testing: k6" row already names the tool for this class of check).
- `orca-cli daemon status --local` reports the compose stack's aggregate container memory (`docker stats --no-stream`, already-available data, no new agent/exporter) alongside health, so a DevOps engineer running this in CI gets the `<150MB`-idle signal for free without this solution inventing a memory-budget enforcement mechanism nothing in the TDD specifies where to enforce.

No new runtime code asserts these budgets — asserting them belongs in CI (build-time, catches regressions before merge) and in `09-observability-reliability.md`'s Prometheus/Grafana stack (runtime, per-service resource metrics already scraped via `/metrics`), neither of which this bug's scope (a CLI surface) should reimplement.

BR-CLI-12 (Docker-compatibility) — confirmed, not just asserted, by this solution's own `docker compose`-based `serve` command actually running the stack in a container-based flow; no separate verification step needed beyond the existing CI job building each service's container image (`10-deployment-infrastructure.md`'s CI/CD pipeline, "build + scan container image" step already present per-service).

---

## Test plan

- `orca-cli/internal/localdaemon/compose_supervisor_test.go` — `Start` refuses a second start while a live PID is recorded; a stale PID file (process no longer running) is detected and does not block a fresh `Start`; `Stop` removes the PID file even if `docker compose down` returns a non-zero exit (best-effort cleanup, logged not swallowed).
- `orca-cli/internal/localdaemon/pidfile_test.go` — round-trip write/read; corrupt/empty PID file treated as "not running", not a crash.
- `orca-cli/internal/apiclient/health_test.go` — `GetHealth` against a fake `/healthz`/`/readyz` pair: `200`/`200` → healthy; `200`/`503` (a `readyz` checker failing, matching `common/health/health.go`'s `handleReady`) → `ready:false`, distinct status from unreachable.
- `orca-cli/internal/command/daemon_status_test.go` — `--local` mode never makes an HTTP call (asserted against the fake `apiclient.Client`, zero invocations); default/remote mode never shells out to `docker` (asserted against a fake `ComposeSupervisor`, zero invocations) — regression guard against the two modes bleeding into each other.
- `orca-cli/internal/command/daemon_stop_test.go` — remote mode returns `ErrStopUnsupportedInGitOpsMode` and exit code 1 without attempting any HTTP call — the deliberate-refusal path is a first-class tested outcome, not an omission.
- Integration (docker-compose, matching `03-clean-architecture-guidelines.md`'s end-to-end tier): `orca serve --daemon --local` against the real repo-root compose file → `orca daemon status --local` reports running with a live PID → `orca daemon stop --local` → a second `status` call reports stopped; full round trip on Linux and (per `AGENTS.md`'s cross-platform rule) a Windows CI runner exercising the detached-process path.
- CI smoke check (per "Design — SLOs" above): each service's built container image starts and answers `/healthz` within 2s, wired into the existing per-service CI job, not a new pipeline.

## References

- `specs/backend-go/bugs/logic-v1/BUG-CLI-03-headless-mode-partial.md` — problem statement; "What backend-go has" section is the basis for this solution's "already satisfied" claims
- `specs/backend-go/tdd/architecture/10-deployment-infrastructure.md` — "Kubernetes topology" (no per-process stop primitive in GitOps-managed environments), "CI/CD" (GitOps-declared deploy state), "Local development" (existing `docker-compose.yml`/`make dev-up`/`make dev-down` this solution wraps)
- `specs/backend-go/tdd/architecture/04-tech-stack.md` — "Explicitly rejected options" table, "A monolithic Go binary... instead of 17 independently deployable services", the direct basis for rejecting a single-daemon-process redesign; "Load testing: k6" row
- `specs/backend-go/tdd/services/api-gateway.md:2` (§1, every backend-go service already GUI-free), `§2` (stateless, any replica), `§8` (availability/scaling posture underlying "no meaningful single-process stop")
- `backend-go/services/api-gateway/cmd/server/main.go:11-12,70-72,351-352` — `/healthz`+`/readyz` mount convention and real SIGTERM shutdown, cited as already-satisfying BR-CLI-09
- `backend-go/common/health/health.go` — `/healthz`/`/readyz` implementation this solution's `GetHealth` client call targets, reused as-is
- `backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-67` — existing auth requirement on the HTTP API, cited as already-satisfying BR-CLI-11
- `desktop/src/cli/runtime/transport.test.ts:33` — confirms the spec's Unix-socket daemon model is the Electron desktop app's, re-scoped here to local-only supervisor IPC
- `AGENTS.md` "Cross-Platform Support" — basis for the Linux/macOS-vs-Windows detached-process design note
- [`SOL-CLI-01`](./SOL-CLI-01-orca-cli-worktree-create.md) — the `orca-cli` binary and `apiclient` scaffolding this solution's `daemon`/`serve` commands are added to
