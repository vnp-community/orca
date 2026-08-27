# BUG-FLEET-04: Onboarding wizard's connect+register steps work; platform/agent detection, preflight checks, and multi-server checklist do not

**Business Logic:** [BL-FLEET-04](../../../../docs/logic/fleet/BL-FLEET-04-dev-server-onboarding.md) — Dev Server Onboarding Wizard
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user (Carlos/Admin) walking through the 7-step onboarding wizard can get through "connect" (Step 1), "deploy relay" (Step 5), and "register" (Step 6) against backend-go. But Step 2's platform/arch/Node-version detection is discarded rather than shown to the user, Step 3 (detect AI agents) and Step 4 (real per-server preflight check) have no backend-go implementation at all, and Step 7 (multi-server checklist from a fleet import) has nothing to checklist against because fleet import doesn't exist (BUG-FLEET-01).

---

## Spec summary

BL-FLEET-04 is a 7-step wizard: (1) connect via SSH with a 10s timeout; (2) detect platform/arch/distro via `uname`/`lsb_release`; (3) detect installed AI agents (`claude`, `codex`, `gemini`, `openai`) via `which`; (4) preflight-check Git≥2.25, Node≥22, disk≥5GB, port availability, and `gh` CLI, allowing proceed-with-warnings; (5) SFTP-deploy the relay binary, SHA256-verify, start as daemon, HTTP-health-check; (6) register a `DevServer` record and persist it; (7) if onboarding came from a fleet import, show a multi-server checklist. State machine: `IDLE → CONNECTING → DETECTING → PREFLIGHT → DEPLOYING → REGISTERING → DONE`, with `FAILED` branches.

## What backend-go has

- **Step 1 (Connect):** `ssh.connect` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:383`) and the underlying `EstablishConnection` usecase (`backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go:29-72`) do a real SSH-target resolve, find-or-create `DevServer`, and confirm liveness via `agent.Health(...)` before marking the `Connection` `established` — this is a genuine, working connect step.
- **Step 2 (Detect Platform), partially:** the `agent.handshake` exchange in the relay-ssh provisioning pipeline captures `Platform`, `Arch`, `NodeVersion`, `AgentVersion`, `Capabilities` into `devserveragent.HandshakeInfo` (`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:117-155`) — but this data is never persisted onto `DevServer` or returned to any caller; `EstablishConnection`'s `Connection` result (`domain/connection.go:19-30`) carries only `{ID, DevServerID, Status, EstablishedAt}`, no platform fields. The wscompat layer's own comment confirms this: "none of the frontend's ...platform/arch/nodeVersion... fields exist server-side yet" (`channels.go:334-337`).
- **Step 5 (Deploy Relay):** real SFTP upload + SHA-256 verification + launch + handshake (`sshrelay/deploy.go:29-79`, `sshrelay/launch.go:18-40`, `sshrelay/provisioner.go:82-115`) — see also BUG-FLEET-02 for how this differs from the spec's daemon/HTTP-health model.
- **Step 6 (Register):** `RegisterDevServer` usecase persists a `DevServer` row (`backend-go/services/infra-fleet-service/internal/usecase/register_dev_server.go:34-50`), exposed via `devServer.add` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:453-477`).

## What's missing

- **Step 2 not surfaced:** platform/arch/distro detected during handshake is thrown away — no RPC returns it, no `DevServer` field stores it (`dev_server.go:48-54` has no platform/arch/nodeVersion fields).
- **Step 3 (Detect AI Agents) not implemented at all:** no `which claude codex gemini openai`-equivalent call anywhere in `backend-go/` — confirmed absent by grep for `detectAgent`/`DetectAgent` (zero hits in `infra-fleet-service`/`api-gateway`).
- **Step 4 (Preflight Check) not implemented for the onboarding flow:** the only `preflight.check` channel in backend-go (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:565-573`) is a **hardcoded local-machine stub** — `{git: {installed: true}, gh: {installed: false, authenticated: false}, glab: {installed: false, authenticated: false}}` — unrelated to a per-dev-server remote check of Git/Node version, disk space, or port availability on the target being onboarded. There is no remote `node --version`/`git --version`/`df -P`/port-probe check anywhere in `sshrelay`/`devserveragent`.
- **Step 6 field gaps:** `RegisterDevServerRequest` has no `relayPort`/`sshKeyPath` fields — the proto's `DevServer` message is `{id, tenant_id, host, mode, ssh_target_id}` only (`infrafleet.proto:78-84`), so the spec's `{id, hostname, user, relayPort, sshKeyPath, status: "online"}` record shape is not representable; `status` is always synthesized as `"disconnected"` client-side rather than stored (`channels.go:382`).
- **Step 7 (Multi-Server Checklist) not implemented:** there is no fleet-import flow to originate a multi-server onboarding session from — see BUG-FLEET-01 — so there is nothing to build this checklist UI/API against.
- **No explicit wizard state machine** (`IDLE → CONNECTING → DETECTING → PREFLIGHT → DEPLOYING → REGISTERING → DONE`) on the backend — each step above is either an independent RPC or entirely absent; no backend-go entity tracks onboarding progress/failure state across steps.

## See also

- `BUG-FLEET-01-fleet-inventory-not-implemented.md` — Step 7 has nothing to checklist without fleet import.
- `BUG-FLEET-02-bulk-provisioning-not-implemented.md` — Step 5's daemon/HTTP-health model gap, shared root cause with this file's Step 5 note.
- `specs/backend-go/bugs/missing-v1/BUG-024-ssh-channels-not-implemented.md` — historical; superseded, `ssh.connect`/`ssh.listTargets`/`ssh.getState`/`ssh.getUserAccount` are now wired (`channels_repo_ssh_status_workspace.go:331-383`).

## References

- `docs/logic/fleet/BL-FLEET-04-dev-server-onboarding.md`
- `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go:29-72` — Step 1
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:383` — `ssh.connect`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:117-155` — handshake captures platform info, discarded
- `backend-go/services/infra-fleet-service/internal/domain/connection.go:19-30` — `Connection` has no platform fields
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:334-337,382` — documented field-shape gap
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:565-573` — `preflight.check` hardcoded local stub, unrelated to remote onboarding preflight
- `backend-go/services/infra-fleet-service/internal/usecase/register_dev_server.go:34-50` — Step 6
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:78-84` — `DevServer` message, no relayPort/sshKeyPath
