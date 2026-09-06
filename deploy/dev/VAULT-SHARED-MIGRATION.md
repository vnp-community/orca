# Migrating deploy/dev off its throwaway dev-mode Vault → the shared Vault (172.20.2.21)

## Why

`deploy/dev`'s own `vault` service runs in dev mode (`VAULT_DEV_ROOT_TOKEN_ID`, in-memory
storage) — every host reboot wipes it, requiring `vault-init` to re-run before
`auth-service`/`credential-broker-service` stop crash-looping. This is exactly what happened on
2026-09-06 (`orca-vault-init.service` auto-recovered, but the box was in a degraded state for
several minutes). `vnp-domain/vault/` already runs a real, persistent HashiCorp Vault
(file storage, seal/unseal, backed up) on `172.20.2.21` for Bifrost — reusing it removes this
whole failure class for Orca.

## What's needed — and what's NOT done yet

**Confirmed via direct inspection of `vnp-domain/vault/` (not assumed):**

- The shared Vault currently has **only the `secret/` KV v2 mount** active (used by
  `bifrost-policy.hcl`'s `secret/data/bifrost/*` / `secret/metadata/bifrost/*` paths). It does
  **not** have a `transit/` engine or a `credential-secrets/` KV mount — the two engines Orca's
  `common/secrets` client (`backend-go/common/secrets/vault.go`) requires. Grep across all of
  `vnp-domain` for "orca" returns zero hits — there is no existing Orca integration to build on.
- The Vault container's port is bound `127.0.0.1:8200:8200` (localhost-only on `172.20.2.21`).
  The README documents opening it to `172.20.2.39` via `ufw allow from 172.20.2.39 to any port
  8200` — but only for the **metrics** use case (`obs-token`'s `read-metrics` policy). Reusing
  this for real secret traffic is the same firewall rule (Vault's access control is
  token/policy-based, not path-filtered at the network layer), but it has not been done for
  application traffic yet.
- Vault uses `file` storage (not dev mode) — **restarting the container reseals it**, requiring
  `make vault-unseal` (holder of the unseal keys) afterward. Any change to the container's port
  binding requires a restart.

**This means fully wiring Orca to the shared Vault requires, in order:**

1. **(vnp-domain side)** Enable two new secret engines on the shared Vault:
   ```
   vault secrets enable transit
   vault secrets enable -path=credential-secrets kv-v2
   ```
   (Mirrors exactly what Orca's own dev Vault already does on first boot — see
   `backend/vault` boot logs: `successful mount: path=transit/`, `path=credential-secrets/`.)
2. **(vnp-domain side)** Add `orca-policy.hcl` (drafted below) and apply it:
   ```
   vault policy write orca /path/to/orca-policy.hcl
   vault token create -policy=orca -period=8760h -display-name=orca-backend-go
   ```
3. **(vnp-domain side)** Open the firewall for real (not metrics-only) traffic:
   ```
   sudo ufw allow from 172.20.2.39 to any port 8200
   ```
   (Same rule as `obs-token`'s comment already documents — no new firewall mechanism, just a
   broader justification for the same port.)
4. **(vnp-domain side)** Rebind the Vault container's port from `127.0.0.1:8200:8200` to
   `0.0.0.0:8200:8200` (or a specific interface) in `vnp-domain/vault/docker-compose.yml`, then
   `docker compose up -d vault` — **this reseals Vault**; whoever holds `.vault-keys` must run
   `make vault-unseal` immediately after. Bifrost's own access is via the internal `vault-net`
   docker network and is unaffected by the port rebind itself, only by the reseal window.
5. **(orca deploy/dev side, after 1-4 are live)** Remove the local `vault`/`vault-init` services
   from `docker-compose.yml`, set:
   ```
   VAULT_ADDR=http://172.20.2.21:8200
   VAULT_TOKEN=<the orca-scoped token from step 2>
   ```

## Draft policy (step 2) — NOT yet applied anywhere

See `orca-policy.hcl` in this same directory, mirroring `vnp-domain/vault/bifrost-policy.hcl`'s
shape but scoped to exactly what `backend-go/common/secrets` (Transit encrypt/decrypt +
`credential-secrets/` KV read/write/destroy-metadata) and `auth-service`'s own direct Transit
JWT-signing use (per `common/secrets/vault.go`'s doc comment: "auth-service is the one other
direct Transit caller ... its JWT signing key is a service-wide signing identity") actually touch.
No dynamic-DB-credential paths are included — `backend-go` still falls back to `DATABASE_DSN`
for that (no Vault Agent sidecar wired up yet, confirmed in `vault.go`'s own doc comment), so
requesting broader database-secrets-engine access now would be unused scope.

## Why steps 1-4 are NOT executed in this session

These steps modify a **live Vault instance serving Bifrost** (a different application) on a host
this session has SSH access to but does not own — enabling new secret engines, rewriting the
container's port binding (forcing a reseal), and opening a broader firewall rule are all
cross-team, hard-to-reverse changes with real blast radius on Bifrost's availability. Per this
repo's own operating principle (confirm before hard-to-reverse, outward-facing changes), these
are left as a reviewed, ready-to-run runbook for whoever owns `vnp-domain/vault/` to execute (or
to explicitly authorize this session to execute), rather than done unilaterally.

## Status

- [x] Investigated shared Vault's actual mount/network state (not assumed)
- [x] Drafted `orca-policy.hcl`
- [x] Wrote this runbook
- [ ] Steps 1-4 (vnp-domain side, live Vault changes) — **not executed, needs explicit go-ahead**
- [ ] Step 5 (orca deploy/dev side) — **not executed**, depends on 1-4 being live first
