# BUG-SSH-01: SSH connect flow exists end-to-end, but auth/config model is a different design than the spec (Vault-cert fleet auth, no `~/.ssh/config`)

**Business Logic:** [BL-SSH-01](../../../../docs/logic/remote-development/BL-SSH-01-ket-noi-ssh.md) — Kết nối SSH Host
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user cannot point Orca at an arbitrary host using their own `~/.ssh/config` (`Host`/`HostName`/`IdentityFile`/`ProxyJump`/`Include`), their own SSH key, agent forwarding, a password, or 2FA — `ssh.connect` only works for a host that has been pre-registered as an `SshTarget` with a Vault SSH secrets-engine role (`vault_ssh_role`) already provisioned out-of-band. There is no config-parsing, no key/password/keyboard-interactive negotiation, no `ProxyJump`, no keepalive, and no concurrent-connection cap.

---

## Spec summary

BL-SSH-01 describes a user adding an SSH host in Orca, at which point Orca parses the user's `~/.ssh/config` (including `Include` directives), resolves `HostName`/`Port`/`User`/`IdentityFile`/`ProxyJump`, and negotiates authentication in order (key file → agent forwarding → password → keyboard-interactive/2FA). It sets up a keepalive (`ServerAliveInterval` = 30s, BR-SSH-03), enforces a 10-second unreachable-host timeout with a specific error message (A2), supports `ProxyJump` chaining (A3), caps concurrent connections to the same host at 10 (BR-SSH-04), and never stores credentials in plaintext (BR-SSH-01).

## What backend-go has

A real, wired connection-establishment path exists, but built on a completely different trust/auth model:

- `infrafleet.proto:16,37,40,44` — `CreateSshTarget`, `ListSshTargets`, `GetSshState`, `EstablishConnection` RPCs are all defined and implemented (this closes the gap `specs/backend-go/bugs/missing-v1/BUG-024-ssh-channels-not-implemented.md` reported as fully missing — see "See also" below).
- `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go:32-48`, `list_ssh_targets.go:21-27`, `get_ssh_state.go:35-50`, `establish_connection.go:31-77` — real usecases, Postgres-backed (`internal/adapter/postgres/repository.go`'s `SshTargetStore`), tenant-scoped.
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:331-402` — `registerSshChannels` wires `ssh.listTargets`, `ssh.getUserAccount` (derived client-side from `ListSshTargets`), `ssh.getState`, and `ssh.connect` (→ `EstablishConnection`, 20s timeout) — all reachable from the frontend today.
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go:107-196` — `Connector.Connect` performs the actual SSH dial: generates an ephemeral ed25519 keypair in-memory, exchanges it for a short-lived Vault-signed certificate (`SSHSignPublicKey`), and authenticates with `ssh.PublicKeys(certSigner)` only.
- `domain/ssh_target.go:19-31` — `SshTarget{ID, TenantID, Host, UserName, VaultSSHRole}` — no port, no `IdentityFile`, no `ProxyJump`, no known-hosts fingerprint.

This satisfies BR-SSH-01's *intent* (no raw key material stored — `ssh_target.go:12-16`'s invariant enforces a Vault role pointer only) via a different, arguably stronger mechanism than "OS keychain", but the rest of the spec's flow does not exist as described.

## What's missing

- **No `~/.ssh/config` parsing at all** — no `Host`/`HostName`/`Port`/`User`/`IdentityFile` resolution, no `Include` directive support. `domain.SshTarget` has no port field at all (`ssh_target.go:19-24`'s doc comment: "Port ... from the fuller design-doc entity are not modeled in this scaffold"); every target dials port 22 (`sshconn/connector.go:46-52`, `defaultSSHPort`).
- **No `ProxyJump` support** (A3) — `Connector.Connect` (`connector.go:137-196`) dials `target.Host` directly; no jump-host chaining exists anywhere in `sshconn`.
- **No key-file / SSH-agent-forwarding / password / keyboard-interactive auth negotiation** (main flow step 2c, A1) — `clientConfig.Auth` is hardcoded to `ssh.PublicKeys(certSigner)` only (`connector.go:175-179`); there is no fallback chain, so A1's "key fails → try agent forwarding → try password → error with fix hint" flow cannot occur — there is only one auth method, period.
- **No 10-second unreachable-host timeout with the spec's exact UX** (A2) — `DialTimeout` defaults to 10s (`connector.go:87`, `DefaultConfig`), which coincidentally matches, but no `"Connection refused: <host>:<port>"`-shaped error surface exists; errors are wrapped generically (`connector.go:186`).
- **No keepalive / `ServerAliveInterval`** (BR-SSH-03) — no `ssh.ClientConfig` keepalive/ping loop anywhere in `sshconn` or `devserveragent`.
- **No max-10-concurrent-connections-per-host enforcement** (BR-SSH-04) — no connection pool or per-host counter exists; `sshconn.Connector` opens a fresh connection per `Connect` call with no cap.
- **Host-key verification is explicitly disabled** — `connector.go:178`, `ssh.InsecureIgnoreHostKey()` — a real, acknowledged (not spec-covered) security gap.
- **No OS keychain integration** — moot given the Vault-cert model, but flagged since BR-SSH-01 literally names "OS keychain" as the mechanism and none exists; Vault is the closest analog implemented (`sshconn/connector.go:1-26`'s package doc comment).

## See also

- `specs/backend-go/bugs/missing-v1/BUG-024-ssh-channels-not-implemented.md` — reported `ssh.listTargets`/`ssh.getState`/`ssh.getUserAccount`/`ssh.connect` as entirely unwired with no backing RPC. This is now **stale**: `ListSshTargets`, `GetSshState`, and `EstablishConnection` all exist and are wired (see above) — the proposed fix in `specs/backend-go/bugs/missing-v1/solutions/SOL-024-ssh-channels.md` has since been implemented. The remaining gap this report describes is a design-model mismatch (Vault-cert fleet auth vs. user's own `~/.ssh/config`), not a wiring gap.

## References

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:13,16,37,40,44,211-296` — RPC signatures and `SshTarget`/`GetSshStateResponse`/`EstablishConnectionRequest` messages
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:1-48`
- `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go:32-48`, `list_ssh_targets.go:21-27`, `get_ssh_state.go:35-50`, `establish_connection.go:31-77`
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go:1-263`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:324-403`
- `backend-go/services/infra-fleet-service/cmd/server/main.go:90-121` — Vault-based wiring of `sshconn.Connector`/`sshrelay.Provisioner`
- `docs/logic/remote-development/BL-SSH-01-ket-noi-ssh.md`
