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
├── docker-compose.yml           # postgres, nats, 17 backend-go services, frontend, 14 migrate-* one-shots (Vault is external — see below)
├── docker-compose.local.yml     # OVERLAY cho chạy local: Vault dev-mode (in-memory) + vault-init; dùng chung với .env có VAULT_ADDR=http://vault:8200
├── .env.example / .env          # config (backend-go + frontend only — agent config is deploy/agent/.env.example)
├── orca-policy.hcl              # Vault policy applied to the SHARED vnp-domain Vault (172.20.2.21), not run by this compose file
├── VAULT-SHARED-MIGRATION.md    # migration record: throwaway local Vault -> shared persistent Vault
├── docker/
│   ├── postgres/init-databases.sh   # creates the 14 per-service databases on first boot
│   └── nginx/orca.conf              # frontend: serves the SPA + reverse-proxies /v1/* to api-gateway
└── scripts/
    ├── build-local.sh           # [LOCAL] cross-compile all 17 Go binaries + vite-build frontend (GOARCH mặc định amd64; máy arm64: GOARCH=arm64)
    ├── sync-to-server.sh        # [LOCAL] build, rsync, pull images, migrate, up -d — the whole flow in one command
    ├── migrate.sh               # [LOCAL or --remote] create missing databases, then run golang-migrate for one/all services
    ├── ensure-request-env.sh    # [LOCAL or --remote] sinh 3 token nội bộ của luồng Request còn thiếu trong .env (không ghi đè, không in giá trị)
    ├── redeploy-local.sh        # [LOCAL] build + migrate + restart đúng service đã đổi, restart cả nhóm khi container bị tạo lại, rồi smoke test
    ├── start-local-agent.sh     # [LOCAL] chạy Dev Server Agent trên máy này, nối vào stack local, KHÔNG có khoá API trong env của agent
    ├── ensure-mcp-env.sh        # [LOCAL or --remote] fill in missing MCP secrets in .env (never overwrites a set value)
    └── provision-vapid-key.sh   # [on server, or --remote] per-tenant Vault ecdsa-p256 key + public-key row for Web Push
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

### Local-only (không có server từ xa: chạy cả stack trên máy bạn)

Cần: Docker (Colima hoặc Docker Desktop, xem "Docker daemon" bên dưới), Go, Node + pnpm, `openssl`, `rsync`, Python 3.

```bash
cd /path/to/orca
cp deploy/dev/.env.example deploy/dev/.env
# Sửa deploy/dev/.env:
#   POSTGRES_PASSWORD=<tuỳ ý>
#   VAULT_ADDR=http://vault:8200       # Vault dev-mode của docker-compose.local.yml, KHÔNG phải Vault dùng chung 172.20.2.21
#   VAULT_TOKEN=dev-root-token
#   ORCA_AGENT_API_SECRET=<chuỗi ngẫu nhiên>          # cần cho start-local-agent.sh
#   BOOTSTRAP_ADMIN_EMAIL / BOOTSTRAP_ADMIN_PASSWORD  # tài khoản admin tạo ở lần khởi động đầu
#   REQUEST_FLOW_ENABLED=true  REQUEST_APPROVAL_ENABLED=true   # xem "Luồng Request" bên dưới
bash deploy/dev/scripts/ensure-request-env.sh        # sinh các token nội bộ của luồng Request còn thiếu (không in giá trị)
GOARCH=arm64 bash deploy/dev/scripts/build-local.sh  # máy Intel/amd64: bỏ GOARCH=arm64

cd deploy/dev
COMPOSE="docker compose -f docker-compose.yml -f docker-compose.local.yml"   # luôn kèm overlay, nếu không service sẽ tìm Vault ở 172.20.2.21
$COMPOSE up -d postgres nats vault
$COMPOSE up -d vault-init                            # bật transit và credential-secrets trong Vault dev-mode
bash scripts/migrate.sh                              # tạo DB còn thiếu rồi migrate mọi service
$COMPOSE up -d                                       # mọi service + frontend
```

Frontend ở `http://localhost:${FRONTEND_HTTP_PORT}`, api-gateway ở `http://localhost:${API_GATEWAY_PUBLIC_PORT}`. Đăng nhập bằng tài khoản
`BOOTSTRAP_ADMIN_*`.

**Vault local là dev-mode, nằm trong bộ nhớ**: tạo lại hoặc khởi động lại container `vault` là mất sạch khoá. Không bao giờ `down`/`--force-recreate`
`vault`, `postgres`, `nats` trên stack đang dùng; `redeploy-local.sh` từ chối khởi động lại ba service này.

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

### Redeploy một vài service trên stack local

```bash
bash deploy/dev/scripts/redeploy-local.sh request-service api-gateway      # build, migrate, up -d, restart, smoke
BUILD_FRONTEND=1 bash deploy/dev/scripts/redeploy-local.sh frontend api-gateway
SKIP_MIGRATE=1 SKIP_SMOKE=1 bash deploy/dev/scripts/redeploy-local.sh task-service
```

Tên là tên service trong compose (`request-service`, `project-service`, `issue-status-sync`, ...). Script tự chọn `GOARCH` theo kiến trúc Docker (arm64 trên
Apple Silicon), áp migration mới của service trước khi khởi động lại, và cuối cùng chạy suite `platform` của `orca-flows` làm smoke test.
Hai bẫy nó đã xử lý sẵn, đừng tự làm tay: (1) binary được bind-mount nên chỉ nhận bản mới khi container khởi động lại; (2) container bị **tạo lại** nhận IP mới
trên mạng bridge và các service khác vẫn giữ IP cũ ("unknown service ...", `REQUEST_NOT_IMPLEMENTED`), nên khi thấy "Recreate" script khởi động lại mọi
service Go (không đụng service có trạng thái). Nếu tự chạy `docker compose up -d` thì hãy tự `restart` các service Go sau đó.

## Luồng Request trên stack local

### Cờ hai tầng

Luồng Request (CR v6) chỉ chạy khi **cả hai** cờ bật; thiếu một cái hoặc không đọc được cờ tenant là tắt (fail closed):

| Tầng | Đặt ở đâu | Mặc định |
|---|---|---|
| Tổng | `REQUEST_FLOW_ENABLED` trong `deploy/dev/.env` (compose: `${REQUEST_FLOW_ENABLED:-false}`), đọc lúc `request-service` khởi động | `false` |
| Tenant | admin gửi kênh WS `request.flowSet {enabled:true}` (đọc lại bằng `request.flowStatus`); suite `orca-flows` tự bật ở `RQL-01` | không có dòng = tắt |

`REQUEST_APPROVAL_ENABLED=true` bật `ApprovalService` (cổng duyệt loại, Solution, Plan...). Đổi hai biến này cần `redeploy-local.sh request-service`
(hoặc `docker compose up -d request-service` rồi khởi động lại các service Go). Chi tiết vận hành: [Bật luồng Request](../../docs/guides/request/admin-enable-request-flow.md).

### Ngôn ngữ đầu ra của AI

`REQUEST_DEFAULT_OUTPUT_LANGUAGE` (CR-REQ-046) chọn ngôn ngữ AI viết phần phân tích (Solution, chẩn đoán, Findings, Answer) và Plan cho cả triển khai:
`vi` (mặc định, nếu không đặt biến) hoặc `en`. Khoá JSON, giá trị enum, id, đường dẫn và ngữ pháp OpenSpec luôn giữ tiếng Anh. Giá trị chỉ được đọc một lần lúc khởi động,
nên đổi xong cần `redeploy-local.sh request-service`. Đây là biến `REQUEST_DEFAULT_*` **duy nhất làm service không khởi động** khi sai giá trị; log báo
`REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en` và container thoát mã 1. Khởi động thành công thì log có dòng `output language code=vi`.

### Token nội bộ

Không có mTLS trong stack này, nên bốn token chia sẻ là kiểm tra caller duy nhất của các RPC nội bộ. Rỗng thì `request-service` từ chối mọi RPC bằng
`REQUEST_FORBIDDEN` và `/readyz` báo chưa sẵn sàng, mà triệu chứng lại xa nguyên nhân. `ensure-request-env.sh` chỉ điền chỗ thiếu, không bao giờ ghi đè hay in giá trị:

| Biến trong `.env` | Bên gửi → bên nhận | `request-service` đọc là |
|---|---|---|
| `REQUEST_INTERNAL_CALLER_TOKEN` | api-gateway → request-service | `GATEWAY_INTERNAL_TOKEN` |
| `REQUEST_SERVICE_INTERNAL_TOKEN` | issue-status-sync → request-service | `SERVICE_INTERNAL_TOKEN` |
| `PROJECT_INTERNAL_CALLER_TOKEN` | request-service → project-service (`ResolveProjectAccess`) | (cùng tên, project-service so khớp) |
| `AUTH_DIRECTORY_CALLER_TOKEN` | request-service → auth-service (`ListActiveTenantAdmins`, mở rộng approver `role:admin` khi phát thông báo duyệt) | (cùng tên, auth-service so khớp). Rỗng thì auth-service từ chối RPC và `/readyz` của request-service đỏ khi `REQUEST_APPROVAL_ENABLED=true` |

```bash
bash deploy/dev/scripts/ensure-request-env.sh            # .env cục bộ
bash deploy/dev/scripts/ensure-request-env.sh --remote   # .env trên server qua SSH (bí mật không rời server)
```

### Dev server là chính máy này, AI qua Claude Code đã đăng nhập

Luồng Request gọi AI (phân loại, Solution, Plan) qua dev server của project, không cần khoá API. Trên máy chạy agent:

```bash
claude auth login                                         # một lần; không dùng `claude --bare` (bỏ qua OAuth/keychain)
node agent/build.mjs                                      # tạo agent/out/agent.js
bash deploy/dev/scripts/start-local-agent.sh              # DEV_SERVER_ID mặc định dev-local-mac; AGENT_WORK_DIR, AGENT_LOG đổi được
```

- Script gỡ `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY` khỏi môi trường agent, nên nguồn AI luôn là phiên Claude đã đăng nhập
  (`claude -p --output-format json --tools ""` trong thư mục tạm rỗng, prompt qua stdin). `ORCA_AI_CLI_SESSION=off` tắt nguồn này. Log mặc định ở `/tmp/orca-local-agent.log`.
- Lần đầu dev server hiện ở trạng thái chờ duyệt: admin duyệt trong Admin Console (kênh `devServer.approve`). Agent đăng ký dưới tenant
  `ORCA_AGENT_DEFAULT_TENANT_ID` (mặc định tenant sentinel `00000000-0000-0000-0000-000000000001`); nếu tenant của admin khác, đặt biến này trong `.env` rồi recreate `infra-fleet-service`
  và khởi động lại các service Go.
- Agent mất kết nối khi `infra-fleet-service` khởi động lại; build agent mới tự nối lại sau khoảng 6 giây, build cũ thì thoát hẳn (chạy lại script).
- AI không dùng được thì Request và run đứng yên kèm mã `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`,
  `REQUEST_AI_QUOTA_EXHAUSTED` hoặc `REQUEST_AI_BUSY`.

### Chạy bộ e2e nghiệm thu

```bash
python3 tests/stages/e2e/orca-flows/run_acceptance.py                  # toàn bộ, Claude thật (khoảng 8 phút)
python3 tests/stages/e2e/orca-flows/run_acceptance.py request matrix   # một phần
python3 tests/stages/e2e/orca-flows/run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02   # claude giả, như CI
```

`--ai-stub` tạm đổi agent sang bản thấy `claude` giả trước, chạy xong khôi phục agent như cũ (cùng thư mục làm việc, dev server và log). Chi tiết:
[`tests/stages/e2e/orca-flows/README.md`](../../tests/stages/e2e/orca-flows/README.md). CI: `.github/workflows/orca-flows-e2e.yml`.

### Docker daemon (Colima, Docker Desktop, Linux)

Lệnh `docker`/`docker compose` dùng context hiện hành nên chạy được với Colima. Test Go dùng testcontainers thì không đọc context, cần `DOCKER_HOST`:

```bash
export DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"   # Colima: unix://$HOME/.colima/default/docker.sock
export TESTCONTAINERS_RYUK_DISABLED=true                                                # cần trên Colima để testcontainers chạy được
```

Docker Desktop trên macOS: socket là `unix://$HOME/.docker/run/docker.sock` (hoặc `/var/run/docker.sock` khi bật tuỳ chọn mặc định); Linux: không cần đặt.
Colima trên Apple Silicon chạy container arm64 nên binary phải build `GOARCH=arm64`.

### Bẫy đã gặp

- Stack nói chuyện với Vault ở `VAULT_ADDR`; quên overlay `docker-compose.local.yml` (hoặc quên `VAULT_ADDR=http://vault:8200`) thì các service dùng Vault (`auth-service`, `credential-broker-service`) tìm Vault ở địa chỉ mặc định `172.20.2.21` và không tới được.
- `request-service` giới hạn ghi 5 lần/giây, burst 10 (`REQUEST_RATE_<READ|WRITE|AI|WEBHOOK>_PER_SECOND` và `_BURST`); client của `orca-flows` tự thử lại theo `retryAfterMs`.
- macOS không có `timeout` và `sha256sum`; zsh không tách từ biến `$var`. Các script trong thư mục này dùng bash và công cụ có sẵn trên macOS/Linux.

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

## MCP rollout (AI agents operating Orca)

`sync-to-server.sh` now does most of this for you. **MCP stays off until you set `MCP_ENABLED=true`** in the
server's `.env` — everything below can be deployed first and switched on later.

### What the deploy does automatically
| Step | Where | Effect |
|------|-------|--------|
| `ensure-mcp-env.sh --remote` | server `.env` | generates `MCP_INTERNAL_CALLER_TOKEN`, `OAUTH_INTERNAL_CALLER_TOKEN`, `AUTH_MCP_PRINCIPAL_CALLER_TOKEN`, `MCP_CURSOR_KEY` (random, created **on the server**, never printed) and derives `OAUTH_RESOURCE_URL=<PUBLIC_BASE_URL>/mcp`. Only fills missing/empty keys. |
| `migrate.sh --remote` | Postgres | creates any missing database (e.g. `mcp` — `init-databases.sh` only runs on a brand-new volume), then migrates. |
| `provision-vapid-key.sh --remote --all` | Vault + `notification` DB | optional pre-warm: creates `vapid-signing-<tenant_id>` (ecdsa-p256) and stores its public key for existing tenants. Keys are also created automatically on first use (see below). Non-fatal if it can't. |
| compose defaults | containers | `WS_ALLOWED_ORIGINS` and `VAPID_SUBJECT` default to `PUBLIC_BASE_URL`; `mcp-service` gets the OPA bundle mounted at `/policy/orca-authz`. |

### Decisions baked in (and why)
- **`WS_ALLOWED_ORIGINS` = `PUBLIC_BASE_URL`, rolled out in `WS_ORIGIN_MODE=report` first.** Browsers authenticate with a cookie, so without an Origin check any web page a user visits could drive `/ws`. Report mode logs what *would* be rejected (`grep "would be rejected"` in the api-gateway log) without blocking anyone, because some clients' Origin is unknown (an Electron renderer may send `file://`). Once the log shows only things you want blocked, set `WS_ORIGIN_MODE=enforce`. nginx forwards `Host $http_host` for the two WebSocket locations so a direct `http://<ip>:<port>` visit still counts as same-origin; any *other* address must be added to `WS_ALLOWED_ORIGINS` (comma-separated).
- **Internal tokens are generated, not defaulted.** There is no mTLS/mesh here (see "Known limitations"), so these shared secrets are the only check on internal-only RPCs. A guard stays off — with a startup warning — while its token is empty.
- **`VAPID_SUBJECT` = `PUBLIC_BASE_URL`** (must be `mailto:` or `https:`).

### Manual steps (one-time; need rights this deploy does not have)
1. **Apply the Vault policy** on the shared Vault (172.20.2.21) so the orca token can create VAPID signing keys. The
   live policy is named **`orca`** (verified with `lookup-self` on the `.env` token: policies `[default, orca]`) and today it
   lacks `create` on `transit/keys/vapid-signing-*` (verified: `read, update`). Writing a policy needs an **admin** token —
   the orca token is denied `sys/policies/acl/*`, and the root token was revoked after the migration (a temporary one comes
   from `vault operator generate-root`, 3 of 5 unseal-key holders). Then:
   ```bash
   read -rs VAULT_ADMIN_TOKEN && export VAULT_ADMIN_TOKEN       # not echoed, not in shell history
   deploy/dev/scripts/apply-vault-policy.sh                      # dry run: shows backup + exactly what would change
   deploy/dev/scripts/apply-vault-policy.sh --apply              # writes it, then verifies with the orca token
   vault token revoke -self                                      # if the admin token was a temporary root token
   ```
   `vault policy write` replaces the whole policy, so the script **refuses** (exit 3) when the live policy has rules the repo
   file lacks, backs the live policy up (mode 600) first, and only needs `curl` + `python3`. Without this step
   `provision-vapid-key.sh` reports a 403 and Web Push stays off; everything else works.
   The credential-broker key for external-server secrets (`credential-broker-mcp_external_secret`) needs **no** action — it auto-creates on first use under the existing `transit/encrypt/*` `create` grant.
2. **TLS front proxy** (the host-level gateway that terminates `PUBLIC_BASE_URL`; not in this repo) must forward, to the `frontend` port, these paths **unbuffered with a long read timeout** (SSE) and preserving `Host`, `X-Forwarded-Proto`, `Authorization`, `Mcp-Session-Id`, `MCP-Protocol-Version`, `Last-Event-ID`:
   `/mcp`, `/.well-known/oauth-authorization-server`, `/.well-known/oauth-protected-resource`, `/oauth/*` (except `/oauth/consent`, which is an SPA page and must NOT be proxied to the API).
3. Set `MCP_ENABLED=true` in the server's `.env`, then `docker compose up -d api-gateway`.

### Verify
```bash
curl -si "$PUBLIC_BASE_URL/mcp" | head -3                                   # 401 + WWW-Authenticate (not 404) once enabled
curl -s  "$PUBLIC_BASE_URL/.well-known/oauth-protected-resource"            # resource = $PUBLIC_BASE_URL/mcp
curl -s  "$PUBLIC_BASE_URL/.well-known/oauth-authorization-server" | head -c 300
docker compose logs api-gateway mcp-service | grep -iE "warn|WS_ALLOWED_ORIGINS|internal.?caller"   # no "token empty" warnings
```
Then Settings → MCP in the UI (needs an admin to create a first token / connect a client).

### Rollback
`MCP_ENABLED=false` + `docker compose up -d api-gateway` turns the whole surface off (`/mcp` → 404, channels → `MCP_DISABLED`). A tenant-wide emergency stop that keeps the rest of MCP up: Settings → MCP → Kill switch.

### Known limits
- VAPID keys are provisioned automatically: the first VAPID public-key request (or first push) of a tenant makes notification-service ask credential-broker-service to create `vapid-signing-<tenant_id>` (ecdsa-p256) and store its public key, so tenants created after a deploy need no script. `provision-vapid-key.sh` is an optional pre-warm. The Vault policy step above remains required.
- `OAUTH_RESOURCE_URL` must equal `<MCP_PUBLIC_BASE_URL>/mcp` exactly; if you change `PUBLIC_BASE_URL`, clear `OAUTH_RESOURCE_URL` in the server `.env` and redeploy so it is re-derived.

## issue-status-sync (Jira status sync)

`issue-status-sync` is a NATS (JetStream) consumer with no public gRPC API: it reacts to worktree/SCM events and transitions the linked Jira issue through `issue-tracking-service`. It has its own database, `issuestatussync`.
- `./scripts/migrate.sh issuestatussync` (or `--remote`) creates the database if missing, then migrates it; safe on an already-running server (`init-databases.sh` only covers fresh volumes). Then `docker compose up -d issue-status-sync`.
- It only acts for projects with `IssueStatusSyncEnabled` set and for users who have connected Jira; otherwise events are ignored.

## Known limitations (read before treating this as production)

- **Vault is the shared, persistent vnp-domain instance (172.20.2.21), not a
  local dev-mode container** (as of 2026-09-07 — see
  [`VAULT-SHARED-MIGRATION.md`](./VAULT-SHARED-MIGRATION.md) for the
  migration record and [`orca-policy.hcl`](./orca-policy.hcl) for exactly
  what's granted). This replaced the original throwaway dev-mode Vault
  (`VAULT_DEV_ROOT_TOKEN_ID`, in-memory, wiped on every restart) after a real
  incident: a 172.20.2.39 reboot wiped it, and `auth-service`/
  `credential-broker-service` crash-looped until manual recovery. The shared
  instance uses `file` storage (persists across restarts) and requires
  `VAULT_TOKEN` in `.env` to be set to the real orca-scoped token — there is
  no insecure default anymore (`docker compose up` fails loudly if unset,
  see `x-go-common-env` in docker-compose.yml). This is still not the full
  HA/auto-unseal setup
  [`specs/backend-go/architecture/06-secrets-vault-architecture.md`](../../specs/backend-go/architecture/06-secrets-vault-architecture.md)
  specifies for real production, but it no longer loses data on reboot.
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

## Vault (shared instance) — no local re-init needed anymore

Orca's own `vault`/`vault-init` services and the `orca-vault-init.service`
systemd unit that used to recover them after a host reboot are **gone**
(removed 2026-09-07, see `VAULT-SHARED-MIGRATION.md`) — there is nothing
local to re-initialize on this host anymore. The shared Vault at
172.20.2.21 is a separate, persistent instance managed by whoever owns
`vnp-domain/vault/`; if IT is ever sealed (its own host reboot, an operator
action), `deploy/dev`'s services will fail closed against it (transit/
credential-secrets calls error) until someone runs `vault-unseal` **over
there** — that recovery is out of this deploy's scope, same as any other
shared dependency this deploy doesn't own (see vnp-domain/vault/README.md).

If `orca-vault-init.service` is still installed/enabled on an older host
from before this migration, disable it — it references a `vault-init`
compose service that no longer exists and will fail on the next boot:
```bash
sudo systemctl disable --now orca-vault-init.service
sudo rm -f /etc/systemd/system/orca-vault-init.service
sudo systemctl daemon-reload
```
