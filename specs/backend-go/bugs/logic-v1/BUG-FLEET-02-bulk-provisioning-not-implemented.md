# BUG-FLEET-02: No bulk/fleet-wide server provisioning — only a single-server, on-demand relay deploy exists

**Business Logic:** [BL-FLEET-02](../../../../docs/logic/fleet/BL-FLEET-02-bulk-provisioning.md) — Bulk Server Provisioning
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** An Admin cannot provision "the whole fleet, or a project's subset of it, in parallel" against backend-go. There is no `orca fleet provision [--project] [--concurrency]` entry point, no fan-out across multiple servers, no prerequisite checks (Node/Git version, disk space), no retry-3-times policy, and no `degraded`/`unhealthy` status persisted per server. What exists is a single-server relay deploy pipeline that only ever runs lazily, one dev server at a time, the first time something needs to talk to it.

---

## Spec summary

BL-FLEET-02 describes `orca fleet provision [--project <name>] [--concurrency 5]`: load the server list (optionally filtered by project), provision each server in parallel up to a concurrency cap — SSH connect, check Node.js/Git/disk-space prerequisites, SFTP-deploy and SHA256-verify the relay binary, start it as a daemon, health-check it over HTTP, and update the server's status in the fleet store — then print a `{success, failed, skipped}` summary. Errors are handled per-server (log + continue), with a 3-retry policy on relay-deploy failure and idempotent re-runs.

## What backend-go has

A real **single-server** deploy pipeline exists in `sshrelay.Provisioner.Provision` (`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:82-115`):
1. Resolves the `SshTarget`, dials it (`sshconn.Connector`).
2. SFTP-uploads the relay bundle (`agent/out/agent.js`) and SHA-256-verifies it against a remote-computed checksum (`deploy.go:29-79`) — a real integrity check equivalent to the spec's "Verify relay binary hash" step.
3. Launches it via `node agent.js --stdio` over an SSH exec channel (`launch.go:18-40`).
4. Performs the receiver-side `agent.handshake` exchange to confirm the agent is alive (`provisioner.go:117-155`).

This is invoked from `devserveragent.Client.getOrProvisionSession` (`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:138`) — i.e. lazily, on first use of a connection, for exactly one dev server, not as a fleet-wide batch operation.

## What's missing

- No `orca fleet provision` CLI or equivalent RPC that takes a list of servers and fans them out — `infra-fleet-service/cmd/server/main.go` has no CLI subcommands (confirmed: no `flag.`/`cobra`/`Command(` hits).
- No `--project` filter or `--concurrency`-bounded parallel loop anywhere (`grep` for `bulkProvision`, `BulkProvision`, `--concurrency` across `backend-go/` returns zero hits).
- No prerequisite checks: no `node --version`/`git --version`/`df -h` probes anywhere in `sshrelay`/`devserveragent` — the pipeline goes straight from SSH-connect to SFTP-upload with no version/disk-space gating.
- No "Start Relay" as a **daemon**: the actual launch model is one SSH exec channel running `node agent.js --stdio` in the foreground (`launch.go:18` doc comment: "no detach/nohup/Unix-socket reattach model... a dropped SSH connection just ends the session; the next call re-provisions from scratch") — the opposite of the spec's `--daemon` + PID-file + independent-of-connection model.
- No HTTP health-check step (`GET http://localhost:<relayPort>/health`) — liveness is confirmed only via the `agent.handshake` JSON-RPC exchange, not an HTTP probe, and there is no `relayPort` concept at all in this pipeline.
- No retry-3-times policy on deploy failure, no `degraded`/`unhealthy` status write-back — `domain.DevServer` has no status field (`backend-go/services/infra-fleet-service/internal/domain/dev_server.go:48-54`) to persist one into.
- No `{success, failed, skipped}` summary output — nothing aggregates per-server outcomes since there is no fleet-wide entry point to aggregate them under.

## See also

BL-FLEET-01's gap (no fleet inventory/import) compounds this: even if bulk provisioning existed, there is no server list to filter by `--project` from — see `BUG-FLEET-01-fleet-inventory-not-implemented.md`.

## References

- `docs/logic/fleet/BL-FLEET-02-bulk-provisioning.md`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:82-115` — single-server `Provision` pipeline
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:29-79` — SFTP upload + SHA-256 verification (real)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:18-40` — foreground exec launch, no daemon/PID file
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:138` — `getOrProvisionSession`, lazy single-connection trigger
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:48-54` — no status field to write `degraded`/`unhealthy` into
- `backend-go/services/infra-fleet-service/cmd/server/main.go` — no CLI subcommands
