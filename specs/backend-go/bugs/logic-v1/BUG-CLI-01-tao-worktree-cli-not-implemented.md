# BUG-CLI-01: No `orca worktree create` CLI surface reaches backend-go at all

**Business Logic:** [BL-CLI-01](../../../../docs/logic/cli-headless/BL-CLI-01-tao-worktree-cli.md) — Tạo Worktree qua Orca CLI
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A DevOps engineer scripting `orca worktree create --base main --agent claude --prompt "..." --json` in a CI/CD pipeline has no backend-go entry point to call at all. The real `orca` CLI binary that ships today (`desktop/src/cli/`) talks to the Electron desktop app's own local `PtyDaemon` Unix socket — it never calls backend-go's `api-gateway`. A CI runner with no desktop app installed has no way to drive backend-go's (real) worktree-creation RPC.

---

## Spec summary

`orca worktree create --base <branch> --agent <type> --prompt <text> --name <name> --json` should: send the command to the Orca daemon over a Unix socket, create the worktree (BL-WT-01), optionally start an agent (BL-AG-01) and inject the initial prompt, print worktree info as JSON to stdout, and exit 0/1/2 per BR-CLI-03. BR-CLI-01/02/04 require idempotency, valid JSON output, and dual human+machine-parseable error messages.

## What backend-go has

- The underlying worktree-creation capability this CLI command would need is real: `CreateWorktree.Execute` (`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:41-71`) runs `git worktree add` via local/relay `GitExecutor` and records bookkeeping via `project-service.RecordWorktreeCreated`, exposed as gRPC `CreateWorktree` (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:86`, `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:678-695`) and as the WebSocket JSON-RPC channel `worktree.create` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-73`, registered at `channels.go:119`). See `BUG-WT-01-tao-worktree-partial.md` for that saga's own gaps (no validation, no custom name/path input).
- `api-gateway` requires authentication on every route via `authMiddleware` (`backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-67`), including a real bearer-JWT path (`usecase.AuthValidator.Validate`, `backend-go/services/api-gateway/internal/usecase/validate_identity.go:65-79`) suitable in principle for a non-interactive CLI/CI caller — i.e. a token-based auth path a headless CLI could use does exist.
- No REST route exists for worktree creation: `mountGitRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:22-30`) only wires `/v1/git/status,/diff,/commit,/push,/pull,/commit-message`; `project_routes.go:37` (`POST /{id}/worktrees`) calls `RecordWorktreeCreated` only — the bookkeeping write, not the actual `git worktree add` (see `BUG-031`'s original description of this split, still accurate for the REST surface even though the WS surface has since caught up).

## What's missing

- **No CLI binary in backend-go's scope calls any of the above.** `find backend-go -iname "*cli*"` finds no CLI package; a repo-wide search for `orca worktree create`/`daemon.sock` under `backend-go/` returns nothing. The only real `orca worktree create` implementation (`desktop/src/cli/`) targets the Electron app's own `PtyDaemon` Unix socket (`~/.orca/orca.sock`, confirmed fixed per `specs/backend/bugs/cli-headless/BUG-BE-CLI-001-daemon-unix-socket-not-implemented.md`), not backend-go — so this CLI cannot be pointed at a backend-go deployment that has no desktop app running alongside it.
- **No REST endpoint performs the actual git worktree creation** — only the WebSocket `wscompat` JSON-RPC channel does, which requires a stateful WS connection speaking the frontend's `InboundMessage`/push-event framing, not something a shell script can `curl`.
- **`--agent <type>` has nothing real to bind to**: per `BUG-AG-01`, backend-go's only PTY-spawn primitive (`SpawnTerminalSession` → `pty.create`) spawns a bare login shell, not an AI agent binary — there is no `agent.spawn`/`AgentConfig` concept for a CLI-driven "start this agent type" request to invoke.
- **`--prompt <text>` injection**: the generic mechanism to send text into a running PTY (`terminal.send`, `channels_terminal.go:284-301`) is real, but nothing composes worktree-create → agent-spawn → prompt-send into the single atomic operation the CLI command implies, and there is still no agent to send the prompt to (see above).
- **No `--json` / exit-code contract (BR-CLI-02/03)** exists anywhere in backend-go, since there is no CLI process to define one in.
- **No idempotency guarantee (BR-CLI-01)**: `CreateWorktree.Execute` has no dedupe/idempotency-key handling — a repeated call with the same inputs runs `git worktree add` again and fails on the second attempt (per `BUG-WT-01`'s duplicate-path finding), rather than returning the existing worktree.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-WT-01-tao-worktree-partial.md` — the worktree-creation saga's own validation/business-rule gaps (name collision, base-branch-not-found, disk space, 20-worktree cap, no custom name/path)
- `specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md` — no real agent-spawn capability for the `--agent` flag to bind to
- `specs/backend-go/bugs/missing-v1/BUG-031-worktree-channels-not-implemented.md` — historical context on the worktree bookkeeping/git-operation split; stale for the WS surface (now wired) but still accurate that no REST route performs the git operation
- `specs/backend/bugs/cli-headless/BUG-BE-CLI-001-daemon-unix-socket-not-implemented.md` — confirms the real `orca` CLI's daemon target is the Electron app's own socket, not backend-go

## References

- `docs/logic/cli-headless/BL-CLI-01-tao-worktree-cli.md`
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:41-71`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:86`
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:678-695`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-73`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:119`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:22-30`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go:37`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-67`
- `backend-go/services/api-gateway/internal/usecase/validate_identity.go:65-79`
- `desktop/src/cli/` — the real `orca` CLI, targeting the desktop app's own daemon, not backend-go
