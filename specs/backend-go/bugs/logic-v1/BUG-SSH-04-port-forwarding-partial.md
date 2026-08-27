# BUG-SSH-04: Port scan/kill relay RPCs exist, but no auto port-forwarding (tunnel creation, local-port allocation, well-known-port exclusion, per-worktree namespacing, or cleanup) is implemented anywhere in backend-go

**Business Logic:** [BL-SSH-04](../../../../docs/logic/remote-development/BL-SSH-04-port-forwarding.md) — Tự động Port Forwarding từ Remote
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A QA engineer or remote-dev user who starts a dev server on a remote host gets no automatic `localhost:3001 → remote:3000` tunnel and no "Port 3001 → remote:3000 [Open Browser]" notification. Orca's backend can only be asked to scan for open ports on-demand (pull-based) and relay a kill signal — the entire forwarding mechanism (periodic scan, new-port event, local-port allocation, tunnel setup, per-worktree namespacing, cleanup) does not exist in backend-go.

---

## Spec summary

BL-SSH-04 describes the relay scanning localhost ports on the remote host every 2 seconds, pushing a `{port, process}` event to Orca when a new port opens, Orca picking a free local port (3001–9999, excluding well-known ports 22/25/53/80/443), establishing an SSH tunnel `localhost:<local> → remote:<port>`, labeling it by worktree, notifying the user, and tearing the tunnel down when the remote port closes or the worktree is deleted — with per-worktree port namespacing so two worktrees using the same remote port (e.g. 3000) don't collide locally.

## What backend-go has

A pull-based scan/kill relay pair exists and is real, wired plumbing:

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:18,45,239-258` — `ScanWorkspacePorts`/`KillWorkspacePort` RPCs and request/response messages.
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:25-63` — resolves the connection, and when a `connectionId` is bound, relays to the Dev Server Agent's `ports.scan` method (`scan_workspace_ports.go:50`); returns `[]` for a genuinely local/unconnected worktree (deliberately, not a silent swallow — see its doc comment).
- `backend-go/services/infra-fleet-service/internal/usecase/kill_workspace_port.go:24-72` — identical resolve→relay shape for `ports.kill`.
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:480-520` — `workspacePorts.scan`/`workspacePorts.kill` wscompat channels wire both RPCs, reachable from the frontend.

## What's missing

Everything that makes this "auto port forwarding" rather than "a manual scan button":

- **No periodic 2-second scan loop anywhere in backend-go** — `ScanWorkspacePorts` is purely on-demand/pull (called once per `workspacePorts.scan` invocation, `scan_workspace_ports.go:34`); there is no scheduler, ticker, or background poller in `infra-fleet-service` that re-scans and diffs port state over time.
- **No push notification of newly opened ports** — no event/notification channel exists to tell the frontend "port 3000 just opened, process node" the moment it happens; the client would have to poll `workspacePorts.scan` itself, and even then gets a flat `open_ports []int32` list (`infrafleet.proto:244-246`), not the spec's `{port, process}` shape — there's no process-name field at all.
- **No local-port allocation (BR-SSH-17, range 3001–9999)** — confirmed via repo-wide search: no code picks a free local port in that range anywhere in `backend-go/services`.
- **No well-known-port exclusion (BR-SSH-16: 22/25/53/80/443)** — no such filter exists in `scan_workspace_ports.go` or anywhere else; `decodeOpenPorts` (`scan_workspace_ports.go:69-86`) passes through whatever the agent reports unfiltered.
- **No SSH tunnel establishment at all** — there is no `net.Listen` + forwarded-copy loop, no `ssh.Client.Dial`-based local-to-remote forwarding, nor any other tunnel primitive in `sshconn`, `sshrelay`, or `infra-fleet-service` more broadly. `sshconn.Connection` exposes only `RunCommand`/`NewSession`/`SFTPClient` (`sshconn/connector.go:198-262`) — no `Listen`/port-forward method exists on it.
- **No per-worktree port namespacing (BR-SSH-19)** — `ScanWorkspacePortsRequest`/`KillWorkspacePortRequest` both carry a `worktree_id` field (`infrafleet.proto:240-241,248-251`) that reaches the agent relay call (`scan_workspace_ports.go:50`), but since no tunnel is ever created, there is no local-port-per-worktree mapping to keep separate in the first place.
- **No cleanup-on-close (BR-SSH-18)** — no teardown hook exists for "remote port closed" or "worktree deleted" since no tunnel exists to tear down.
- **No `127.0.0.1`-only scan-scope enforcement (BR-SSH-15)** — moot in backend-go since the scan itself is entirely delegated to the (out-of-scope, TS) Dev Server Agent's own `ports.scan` handler; nothing in backend-go enforces or verifies this constraint.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-027-workspaceports-channels-not-implemented.md` — reported `workspacePorts.scan`/`kill` as unwired `wscompat` channels. This is now **stale for the wiring gap**: both channels are wired (see above); `specs/backend-go/bugs/missing-v1/solutions/SOL-027-workspaceports-channels.md`'s proposed fix has been implemented. The remaining gap this report describes (no actual forwarding/tunnel mechanism) is a different, much larger gap that BUG-027/SOL-027 never claimed to close — SOL-027 was scoped only to wiring the existing scan/kill RPCs, not to building port forwarding.

## References

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:18,45,239-258`
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:1-87`
- `backend-go/services/infra-fleet-service/internal/usecase/kill_workspace_port.go:1-73`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:456-522`
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go:198-262` (no forwarding primitive)
- `docs/logic/remote-development/BL-SSH-04-port-forwarding.md`
