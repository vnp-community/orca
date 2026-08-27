# BUG-SSH-02: Relay deploy pipeline is real but always re-deploys (no version check), has no retry/diagnostics, and deploys a Node script, not a native `orca-relay` binary

**Business Logic:** [BL-SSH-02](../../../../docs/logic/remote-development/BL-SSH-02-deploy-relay.md) — Deploy Orca Relay Binary
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** Every `ssh.connect` on a relay-ssh dev server re-uploads and re-launches the full agent bundle over SFTP, even if an identical, already-running relay process from a previous session would have sufficed — there is no `--version` check, no OS/arch-specific binary selection, no retry-with-backoff on a flaky network, and no diagnostic collection if the relay process dies immediately after launch.

---

## Spec summary

BL-SSH-02 describes checking an existing remote relay's version against the local one, and only uploading (via SFTP to `~/.local/bin/orca-relay`, `chmod +x`, SHA256-verified) when missing or outdated; detecting remote OS/arch to pick the right native binary; retrying network failures 3x; re-uploading and ultimately refusing to connect on a persistent hash mismatch; and collecting stderr diagnostics if the relay crashes immediately after start.

## What backend-go has

A real deploy+launch+handshake pipeline exists and is wired into the live connection path (not dead code):

- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:32-81` — `deploy()` SFTP-uploads `cfg.BundlePath` (the local `agent/out/agent.js`) to `.orca-relay/agent.js` on the target (`deploy.go:22-24,52-63`), then computes a remote SHA-256 checksum via a `node -e` one-liner and aborts with an error if it doesn't match the local hash (`deploy.go:65-78`) — this is a real BR-SSH-05 hash-verify-before-execute implementation.
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:18-45` — `launch()` opens a fresh SSH exec channel and runs `node agent.js --stdio` in the deployed directory.
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:82-172` — `Provisioner.Provision` chains `Connect → deploy → launch → receiveHandshake` end-to-end, closing whatever was opened on any failure (`provisioner.go:96-114`).
- `backend-go/services/infra-fleet-service/cmd/server/main.go:106-121` — this `Provisioner` is actually wired into `devserveragent.Client` via `WithRelaySSH`, and `EstablishConnection`'s `agent.Health` call (`establish_connection.go:66-68`) drives it through `getOrProvisionSession` (`devserveragent/client.go:179-200`) — i.e. `ssh.connect` really does trigger this pipeline, not just a stub.

## What's missing

- **No version check before upload (BR-SSH-07 violated)** — the spec requires uploading "chỉ khi version mismatch"; `Provisioner.Provision` (`provisioner.go:82-115`) calls `deploy()` unconditionally on every provision, with no `orca-relay --version` probe or any check for an already-running/already-deployed relay of the correct version. Per-package doc comment (`provisioner.go:14-21`): "one exec channel per session ... the next call re-provisions from scratch" — deploy always happens.
- **No remote OS/arch detection or multi-binary selection** (main flow 3a-3b) — `deploy.go` uploads one fixed local `cfg.BundlePath` (a single Node.js bundle) with no `uname -m`/`uname -s` probe and no linux-x64/linux-arm64 selection; the spec's "native `orca-relay` binary" model isn't what's built (this pass ships one JS bundle run via `node`, not an architecture-specific compiled binary — acknowledged in `sshconn/connector.go`'s and `sshrelay`'s own package doc comments as a smaller scope than the TS reference).
- **No retry-3x on SFTP/network upload failure (A1)** — `deploy()` (`deploy.go:32-81`) returns the first error immediately (e.g. `fmt.Errorf("sshrelay: uploading relay bundle: %w", err)`, `deploy.go:59`); no retry loop exists anywhere in `sshrelay`.
- **No hash-mismatch re-upload-then-reject flow (A2)** — a checksum mismatch (`deploy.go:76-78`) returns an error on the first attempt; there is no re-upload-and-recheck step before giving up.
- **No relay-crash diagnostic collection (A3)** — `launch()` (`launch.go:18-45`) starts the process and returns a transport; if the handshake never arrives, `receiveHandshake` (`provisioner.go:128-172`) times out (`HandshakeTimeout`, `config.go:15-19`) and returns a generic wrapped error — no stderr capture or "check remote OS requirements" diagnostic is surfaced. `session.Stderr` is never wired in `launch.go`.
- **No random, session-scoped, expiring relay token (BR-SSH-06)** — relay-ssh's handshake explicitly has none: `provisioner.go:118-127`'s doc comment states "No dedicated Handshake frame type... this exchange... No token check — the SSH connection is already the trust boundary" — by design, not by omission, but it means BR-SSH-06 as literally specified (a random, session-expiring token) isn't implemented for this mode.
- **No explicit non-root enforcement or reporting** (BR-SSH-08/BR-SSH-09) — the deploy path never checks the remote user's privilege level or reports a version-mismatch diagnosis to the caller distinctly from a generic failure.

## See also

None — no prior missing-v1/api-v1 bug covers relay deploy/provisioning specifically; the closest sibling reports (BUG-024 in missing-v1) only covered `ssh.*` wscompat wiring, not the deploy pipeline this report addresses.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:1-101`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:1-45`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:1-179`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/config.go:1-36`
- `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go:60-68`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:179-200`
- `backend-go/services/infra-fleet-service/cmd/server/main.go:90-121`
- `docs/logic/remote-development/BL-SSH-02-deploy-relay.md`
