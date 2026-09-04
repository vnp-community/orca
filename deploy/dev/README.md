# Orca Dev Server — backend-go + frontend

> This directory now deploys **`backend-go/`** (17 Go microservices) +
> **`frontend/`**. The previous TypeScript-backend deploy set that used to
> live here has moved to [`deploy/old/`](../old/README.md) — it's untouched
> and still works, kept as a reference / rollback path while `backend-go/`
> is a scaffold (see [`backend-go/docs/execution-plan.md`](../../backend-go/docs/execution-plan.md)
> for what "scaffold" means concretely). Deploy Agent (Dev Server Agent),
> Desktop, and Mobile are unaffected — see `deploy/agent/`, `deploy/desktop/`,
> `deploy/mobile/`.

## The flow

```
[Máy Developer / CI]                              [Orca Server]
──────────────────────                            ─────────────────────────────────
1. build-local.sh                                  4. docker compose pull (public images
   → cross-compile 17 Go binaries                     only — nothing custom-built)
     (CGO_ENABLED=0, linux/amd64)                   5. migrate.sh --remote (one-shot
   → vite build frontend/                              migrate/migrate containers, profile
                                                        "migrate")
2. sync-to-server.sh <version>                      6. docker compose up -d
   → rsync binaries + migrations +                     ┌──────────────────────────────┐
     frontend build + deploy config                    │ 17 backend-go containers      │
     (NOT source — backend-go/ source                  │  gcr.io/distroless/static     │ ← binary
     never touches the server)                         │  (~2MB, no shell) + BIND-      │   bind-
                                                         │  MOUNTED binary, read-only     │   mounted
                                                         │ frontend (nginx:alpine) +      │ ← static
                                                         │  BIND-MOUNTED dist/            │   assets
                                                         │ postgres / vault / nats        │   mounted
                                                         └──────────────────────────────┘
3. (sync-to-server.sh calls the above for you)
```

**No image is built for backend-go or the frontend at all** — every
container runs a stock public image
(`gcr.io/distroless/static-debian12:nonroot` for all 17 Go services,
`nginx:1.27-alpine` for the frontend) with the locally-built binary /
static bundle **bind-mounted** in read-only. This is the literal
"smallest possible image" answer: there's no smaller option than not
building a custom one. See `docker-compose.yml`'s header comment for the
full rationale.

## Structure

```
deploy/dev/
├── docker-compose.yml           # postgres, vault, vault-init, nats, 17 backend-go services, frontend, 14 migrate-* one-shots
├── .env.example / .env          # config (backend-go + frontend only — agent config is deploy/agent/.env.example)
├── docker/
│   ├── postgres/init-databases.sh   # creates the 14 per-service databases on first boot
│   └── nginx/orca.conf              # frontend: serves the SPA + reverse-proxies /v1/* to api-gateway
└── scripts/
    ├── build-local.sh           # [LOCAL] cross-compile all 17 Go binaries + vite-build frontend
    ├── sync-to-server.sh        # [LOCAL] build, rsync, pull images, migrate, up -d — the whole flow in one command
    └── migrate.sh               # [LOCAL or --remote] run golang-migrate for one/all services
```

## Quick start

```bash
cd /path/to/orca

# 1. Config
cp deploy/dev/.env.example deploy/dev/.env
nano deploy/dev/.env    # set POSTGRES_PASSWORD, SERVER_HOST, SERVER_KEY

# 2. Deploy (builds locally, syncs, migrates, starts)
bash deploy/dev/scripts/sync-to-server.sh 0.1.0
```

### Local-only (no remote server — test the stack on your own machine)

```bash
cp deploy/dev/.env.example deploy/dev/.env   # POSTGRES_PASSWORD is enough locally
bash deploy/dev/scripts/build-local.sh
cd deploy/dev
docker compose up -d postgres vault nats
../../deploy/dev/scripts/migrate.sh
docker compose up -d
```

### Redeploy after a code change

```bash
bash deploy/dev/scripts/sync-to-server.sh 0.1.1   # any changed services' binaries + frontend
```

Every run rebuilds and re-syncs **all** 17 services' binaries — cheap
(static Go builds, seconds each) compared to the old Node/Vite flow, so
there's no per-service incremental-build machinery here. If that stops
being true at some point (build time becomes a real bottleneck),
`build-local.sh <service-name>` already supports building one service only
— wire that into `sync-to-server.sh` as an optional argument then.

## Networking

- Only **`frontend`** (nginx, port `FRONTEND_HTTP_PORT`, default 8080) and
  **`api-gateway`** (port `API_GATEWAY_PUBLIC_PORT`, default 8081) are
  exposed to the host. Every other service is reachable only on the
  internal `orca-go-net` bridge network, per
  [`specs/backend-go/architecture/08-inter-service-communication.md`](../../specs/backend-go/architecture/08-inter-service-communication.md).
- The frontend nginx config proxies `/v1/*` (REST) and
  `/v1/notifications/stream` (WebSocket) to `api-gateway` — the browser
  never talks to an individual backend-go service directly.
- TLS termination is **not** handled by either container here — it's
  expected to sit in front of `frontend` at the host/gateway level (same
  layering `deploy/old/gateway/` used). Add it there, or put a TLS-terminating
  proxy in this compose file, once this deploy needs to be reachable outside
  a trusted network.

## Known limitations (read before treating this as production)

- **Vault runs in dev mode** (`VAULT_DEV_ROOT_TOKEN_ID`, in-memory storage,
  single node) — secrets do not survive a container restart, and this is
  explicitly not the HA/auto-unseal setup
  [`specs/backend-go/architecture/06-secrets-vault-architecture.md`](../../specs/backend-go/architecture/06-secrets-vault-architecture.md)
  specifies for real production. Fine for a dev server; do not point real
  tenant secrets at this. **This means `vault-init` (the one-shot service
  that re-enables `transit/`+`credential-secrets/`) must re-run after every
  Vault restart, including a bare host reboot — see below.**
- **No mTLS / service mesh** — containers talk to each other in plaintext
  over the `orca-go-net` bridge network. `architecture/07-security-architecture.md`
  specifies mTLS via a service mesh for production; this Docker Compose
  deploy has no mesh at all (that's a Kubernetes-shaped concern, not a
  Compose one — see `specs/backend-go/architecture/10-deployment-infrastructure.md`
  for the Kubernetes target this eventually graduates to).
- **`read_only: true` on every backend-go container** — distroless images
  have no writable layer need for a stateless-by-design Go service, but if
  a future service genuinely needs to write local scratch files, add a
  `tmpfs:` mount for that path rather than flipping `read_only` off
  wholesale.
- **Every backend-go service in this repository is itself a scaffold** —
  several cross-service calls are stubs (see each service's own README and
  [`backend-go/docs/execution-plan.md`](../../backend-go/docs/execution-plan.md)).
  This deploy set makes the *infrastructure* real; it doesn't make the
  *application* feature-complete.
- **SSO (CR-LOGIN-001) is off by default and single-tenant-only when on.**
  Every `SSO_*`/`AUTH_MODE`/`PUBLIC_BASE_URL` var in `.env.example` is
  optional — leaving them unset keeps local-password login working exactly
  as before. Turning SSO on requires `PUBLIC_BASE_URL` to be this
  deployment's real externally-reachable URL (never guessed from a request)
  and each configured provider's redirect URI registered to match it
  exactly. A brand-new SSO user's tenant is auto-resolved only when exactly
  one company/tenant exists in this deployment — see
  `backend-go/services/auth-service/README.md`'s SSO entry for the full
  account-linking policy and remaining known gaps.

## Vault re-initialization after a host reboot

`docker compose up -d` (the normal deploy flow, `scripts/sync-to-server.sh`)
always re-evaluates every service, so it re-runs `vault-init` correctly on
every deploy. A **bare host reboot with no deploy afterward does not**:
Docker only auto-restarts containers whose restart policy says to
(`vault` itself is `unless-stopped` and comes back — empty, per dev mode
above — on its own), but `vault-init` is a one-shot job (`restart: "no"`)
that already exited successfully once, so Docker never re-invokes it just
because `vault` came back fresh. Left alone, `auth-service` and
`credential-broker-service` crash-loop on 404s against `transit/`/
`credential-secrets/` until someone notices and runs `vault-init` by hand
(this happened live on 172.20.2.39 — see git history for the incident that
prompted this section).

[`systemd/orca-vault-init.service`](./systemd/orca-vault-init.service) closes
this gap: a systemd unit that runs `docker compose run --rm vault-init`
after `docker.service` on every boot (idempotent — safe to run again even
when Vault already has its mounts). One-time install on the server:

```bash
sudo cp deploy/dev/systemd/orca-vault-init.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now orca-vault-init.service
```

Verify: `systemctl is-enabled orca-vault-init.service` should print
`enabled`; `systemctl status orca-vault-init.service` / `journalctl -u
orca-vault-init.service` show the last run's outcome.
