# BUG-CLI-02: `orca agent status/wait/send` and `orca snapshot` have no CLI-reachable backend-go surface

**Business Logic:** [BL-CLI-02](../../../../docs/logic/cli-headless/BL-CLI-02-quan-ly-agent-cli.md) — Quản lý Agent qua Orca CLI
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A DevOps engineer scripting `orca agent wait --worktree <id> --timeout 300` or `orca snapshot --worktree <id> --output result.txt` for a CI pipeline has no way to invoke these against backend-go: the underlying primitives (agent-running check, wait-for-exit, send-input, scrollback capture) are real gRPC/WS operations, but every one of them is keyed by an internal `ptyId` reachable only over the stateful WebSocket JSON-RPC protocol `wscompat` speaks with the frontend — there is no `--worktree <id>`-addressable REST/CLI surface, and no `orca` binary in backend-go's scope calls any of them.

---

## Spec summary

`orca agent status --worktree <id> --json`, `orca agent wait --worktree <id> --timeout 300`, `orca agent send "<prompt>" --worktree <id>`, and `orca snapshot --worktree <id> --output result.txt` should let a CI script poll an agent's status every 5s (BR-CLI-05: `wait` must time out and exit code 2 if the agent never finishes), send it a prompt, and capture its full scrollback — all while the Orca GUI may be open in parallel (BR-CLI-07).

## What backend-go has

- **`terminal.agentStatus` / `terminal.isRunningAgent`** — real WS channels backed by `GetTerminalAgentStatus` gRPC (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:434-478`), returning `agentRunning`/`agentKind`/`readyForInput`. This is the closest real equivalent to `orca agent status`.
- **`terminal.wait`** — real WS channel backed by `WaitTerminalSession` gRPC (`channels_terminal.go:392-418`), returning `{exited, exitCode, timedOut}` given a `ptyId` and `timeoutMs` — structurally the same shape `orca agent wait --timeout` needs.
- **`terminal.send`** — real WS channel that writes into a live `AttachPty` stream for a given `ptyId` (`channels_terminal.go:284-301`) — the mechanism `orca agent send "<prompt>"` would need.
- **On-demand scrollback capture exists** in the terminal multiplex protocol: `SnapshotRequest`/recovery (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:18-28,257`) — the real backing for BR-CLI-06's "capture toàn bộ scrollback hiện tại", but delivered as binary multiplex frames over `terminal.multiplex`, not a plain-text snapshot endpoint.
- All four are real, gRPC-backed, and exercised by tests (not stubs) — this is a "no CLI wrapper exists", not a "no backend capability exists" gap, unlike e.g. `emulator.*`/`browser.*` (see `BUG-008`/`BUG-006`).

## What's missing

- **No `--worktree <id>` → `ptyId` resolution surface for CLI use.** Every one of the four primitives above is keyed by `ptyId`, an internal terminal-session identifier, not a worktree id. `terminal.list` (`channels_terminal.go:376`) can enumerate sessions for a connection, but nothing maps "the CLI's single active agent PTY for worktree X" the way `--worktree <id>` implies — a caller would have to already know which `ptyId` on which connection corresponds to "the agent" for that worktree.
- **No REST route exposes any of `terminal.agentStatus`/`terminal.wait`/`terminal.send`/scrollback-snapshot.** `httpgateway`'s route files (`git_routes.go`, `project_routes.go`, `infra_routes.go`, etc.) have no `/v1/terminal/*` or `/v1/agent/*` endpoints — a shell script cannot `curl` any of this; it would need a full WebSocket client speaking `wscompat`'s `InboundMessage` framing.
- **No CLI binary in backend-go's scope calls any of these channels.** As in `BUG-CLI-01`, the only real `orca agent status/wait/send`/`orca snapshot` implementation lives in `desktop/src/cli/`, targeting the Electron app's own local daemon — not backend-go.
- **No plain-text scrollback-snapshot output.** `SnapshotRequest`'s payload travels as binary multiplex frames (`terminal_stream_frame.go`) for live-viewport reconstruction, not as the flat text file `orca snapshot --output result.txt` implies; nothing in backend-go assembles a plain scrollback string a CLI could redirect to a file.
- **No exit-code / timeout contract (BR-CLI-03/05)**: `WaitTerminalSession`'s `timedOut` boolean is real, but no code anywhere maps that to a process exit code of `2`, since no CLI process exists to make that mapping in.
- **BR-CLI-07 ("mọi command phải hoạt động khi GUI đang mở song song") is unverifiable** — there is no independent CLI client to test concurrently against the same worktree; multiplex viewport-claiming (`ClaimViewport`, `channels_terminal_multiplex.go:18`) suggests multi-client concern was designed for, but nothing confirms a non-GUI caller was validated against it.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-CLI-01-tao-worktree-cli-not-implemented.md` — sibling gap: no CLI binary in backend-go's scope at all
- `specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md` — `terminal.agentStatus` reports on whatever PTY is running, but backend-go can only spawn a generic shell, not a real agent binary, which limits what "agent status" can mean in practice
- `specs/backend-go/bugs/missing-v1/BUG-029-terminal-channels-not-implemented.md` — historical context; stale for `terminal.agentStatus`/`terminal.wait`/`terminal.send` (now wired), still accurate that no REST equivalent exists

## References

- `docs/logic/cli-headless/BL-CLI-02-quan-ly-agent-cli.md`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:284-301,392-418,434-478`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:18-28,257`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:376` — `terminal.list`
- `desktop/src/cli/` — the real `orca agent`/`orca snapshot` CLI, targeting the desktop app's own daemon, not backend-go
