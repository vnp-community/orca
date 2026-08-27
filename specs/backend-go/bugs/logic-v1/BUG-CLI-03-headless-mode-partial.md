# BUG-CLI-03: No `orca serve`/`daemon status`/`daemon stop` control surface — backend-go is headless by construction but has no daemon lifecycle command

**Business Logic:** [BL-CLI-03](../../../../docs/logic/cli-headless/BL-CLI-03-headless-mode.md) — Chạy Orca ở Headless Mode
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A DevOps engineer trying to run `orca serve --port 7777 --daemon` / `orca daemon status` / `orca daemon stop` against backend-go in a Docker container has no such commands to run — backend-go's services are already GUI-free Go server binaries (satisfying the spirit of "no display required"), but there is no single `orca` daemon process, no PID file, no Unix socket, and no `daemon status`/`stop` verbs anywhere in backend-go.

---

## Spec summary

`orca serve --port 7777 --daemon` should fork an Electron-main-process-based daemon with no renderer, open a Unix socket at `~/.local/share/orca/daemon.sock`, optionally start an HTTP API on the given port, write a PID file at `~/.local/share/orca/daemon.pid` (BR-CLI-10), and shut down gracefully on SIGTERM (BR-CLI-09). It must never require a display (BR-CLI-08), stay Docker-compatible with no GUI dependencies (BR-CLI-12), and require an auth token on the exposed HTTP API (BR-CLI-11), with startup <2s and idle memory <150MB (SLO table).

## What backend-go has

- **Inherently headless by construction** — every backend-go service (`api-gateway`, `git-gateway-service`, `infra-fleet-service`, etc., `backend-go/services/*/cmd/server/main.go`) is a plain Go binary with an HTTP/gRPC server and zero GUI dependencies — this trivially satisfies BR-CLI-08 and BR-CLI-12's intent (no X11/Wayland/Xvfb, no Electron/GUI framework in the dependency tree at all), even though it was never built "as a headless mode" — it is the only mode that exists.
- **Real graceful shutdown on SIGTERM**: `api-gateway`'s `run()` builds its lifecycle context via `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` (`backend-go/services/api-gateway/cmd/server/main.go:71`) and calls `publicServer.Shutdown(shutdownCtx)` / `healthServer.Shutdown(shutdownCtx)` (`main.go:351-352`) on that signal — this is a real implementation of BR-CLI-09's requirement, per-service.
- **Real HTTP-API auth-token requirement**: `authMiddleware` (`backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-67`) runs ahead of every route and rejects unauthenticated requests with `401 UNAUTHENTICATED`; the bearer-token path validates a real RS256 JWT against `auth-service`'s JWKS (`backend-go/services/api-gateway/internal/usecase/validate_identity.go:58-79`) — a genuine implementation of BR-CLI-11's "HTTP API phải require authentication token", not a stub.

## What's missing

- **No single `orca serve`/daemon process exists.** backend-go is 17 independently-deployed services (`find backend-go -path "*/cmd/*" -type d` lists 17 `cmd/server` dirs), not the one-process "Orca daemon" the spec describes; there is no orchestrating binary that a `--daemon`/`--port` flag pair would configure.
- **No Unix socket anywhere in backend-go** — `grep -rli "unix socket\|\.sock" backend-go --include="*.go"` returns no hits; the CLI/daemon transport the spec assumes (`~/.local/share/orca/daemon.sock`) does not exist in this codebase. (The real Unix-socket daemon, `PtyDaemon.ts`, lives in the Electron desktop app per `specs/backend/bugs/cli-headless/BUG-BE-CLI-001-daemon-unix-socket-not-implemented.md`, and is unrelated to backend-go.)
- **No PID file mechanism (BR-CLI-10)** — no code under `backend-go/` writes a PID file to `~/.local/share/orca/daemon.pid` or any equivalent path; nothing prevents running the same service binary twice.
- **No `orca daemon status` / `orca daemon stop` commands** — these are CLI verbs with no backend-go implementation to call, consistent with `BUG-CLI-01`/`BUG-CLI-02`'s finding that no CLI binary in backend-go's scope exists at all.
- **SLOs are unmeasured, not enforced**: no startup-time instrumentation, no memory budget/limit configuration (`< 150MB` idle), and no automated check of `< 500ms` command-response time anywhere in `backend-go/` — these targets exist only in the spec doc, with nothing in the codebase asserting or reporting against them.
- **Docker-compatibility (BR-CLI-12) is asserted, not verified in this audit** — no `Dockerfile`/CI job was inspected as part of this pass to confirm an actual container build exists for these services; flagged as unverified rather than assumed.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-CLI-01-tao-worktree-cli-not-implemented.md` — no CLI binary in backend-go's scope to be the client of a headless daemon
- `specs/backend-go/bugs/logic-v1/BUG-CLI-02-quan-ly-agent-cli-not-implemented.md` — same root cause, agent-management side
- `specs/backend/bugs/cli-headless/BUG-BE-CLI-001-daemon-unix-socket-not-implemented.md` — the real (now-fixed) Unix-socket daemon lives in the Electron desktop app, not backend-go; this bug documents that backend-go never grew an equivalent

## References

- `docs/logic/cli-headless/BL-CLI-03-headless-mode.md`
- `backend-go/services/api-gateway/cmd/server/main.go:70-72,351-352`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-67`
- `backend-go/services/api-gateway/internal/usecase/validate_identity.go:58-79`
- `backend-go/services/*/cmd/server/` — 17 independent service entry points, no single daemon process
