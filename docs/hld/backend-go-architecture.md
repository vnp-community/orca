# Orca Backend-Go — Go Microservices Platform (Control Plane, rewrite đang chạy song song)

**Nguồn:** Đọc trực tiếp `backend-go/**` (proto, `internal/{domain,usecase,adapter}`, `common/secrets`),
`deploy/dev/docker-compose.yml`, và `specs/backend-go/tdd/**` (tài liệu kiến trúc mục tiêu nội bộ của
team backend-go — xem cảnh báo "Proposed vs Implemented" ở §0).
**Cập nhật:** 2026-09-06 — tài liệu này mới, viết lần đầu vì `docs/hld/` trước đây (3 file gốc +
`v1/`) chỉ mô tả backend TypeScript (`backend/` + `desktop/`), không có dòng nào nhắc tới `backend-go/`
dù package này đã có 17 service Go thật đang chạy trong `deploy/dev/`.
**Scope:** Chỉ mô tả `backend-go/` — Control Plane thay thế `backend/src/main/**` (Node.js/Electron).
Data Plane (Dev Server Agent, `agent/`) **không đổi** — xem §7 và
[dev-server-architecture.md](./dev-server-architecture.md).

---

## 0. Đọc trước khi dùng tài liệu này

`backend-go/` là một **bản viết lại (rewrite) từ đầu bằng Go**, đang chạy song song với — không thay
thế hoàn toàn — backend TypeScript hiện hành (`backend/src/main/**`, mô tả tại
[backend-server-architecture.md](./backend-server-architecture.md)). Cả hai cùng tồn tại trong repo hôm
nay; migration là **strangler-fig, không phải big-bang** — xem
[`specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md`](../../specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md).

**Phân biệt hai lớp tài liệu backend-go, đừng nhầm lẫn:**

| Lớp | Ở đâu | Vai trò | Trạng thái |
|---|---|---|---|
| **`specs/backend-go/tdd/`** | `specs/backend-go/tdd/architecture/*.md`, `specs/backend-go/tdd/services/*.md` | Tài liệu **kiến trúc mục tiêu nội bộ** của team backend-go — viết TRƯỚC khi code tồn tại (chính `tdd/README.md:3` tự ghi "no Go code exists yet" — nay đã lỗi thời so với chính con số 17-service thật đang chạy) | Một phần đã implement đúng, một phần vẫn là target (ví dụ: K8s + Vault Agent sidecar, `grpc-gateway` cho REST edge, NATS JetStream outbox pattern đầy đủ) — **tài liệu này (root `docs/hld/`) chỉ khẳng định là "thật" những gì đã tự xác nhận qua code đọc trực tiếp trong phiên rà soát 2026-09-06** |
| **File này + `backend-go/**` thật** | `docs/hld/backend-go-architecture.md` | Kiến trúc **hiện hành, đã đối chiếu code** — vai trò tương đương `backend-server-architecture.md` (bên TS) cho phía Go | Nguồn sự thật cho "cái gì đang thật sự chạy" |

Quy ước trạng thái dùng trong tài liệu này (khớp với `backend-server-architecture.md`): ✅ đã implement
& khớp mô tả · ⚠️ có implement nhưng khác chi tiết so với `specs/backend-go/tdd/` · 🚧 target trong
`specs/backend-go/tdd/`, chưa xác nhận có trong code thật · ❌ mô tả target sai hẳn so với code.

---

## 1. Backend-Go là gì, và tại sao nó tồn tại song song với `backend/`

`backend-go/` là bản viết lại **Control Plane** (những gì `backend/src/main/**` làm hôm nay — auth,
routing RPC, quản lý Project/AI-Provider/Workflow/Task, fleet — xem
`specs/backend-go/tdd/README.md` §"Why this isn't a from-scratch design") thành **17 Go microservices**,
mỗi service một Postgres database riêng, giao tiếp nội bộ qua gRPC, đứng sau một `api-gateway` duy nhất
lộ ra ngoài. **Data Plane** (Dev Server Agent, thực thi PTY/git/fs trên máy remote) **nằm ngoài phạm vi**
rewrite này — backend-go vẫn nói chuyện với đúng execution plane `agent/` hiện có (§7).

Lý do tồn tại song song với TS `backend/` (suy luận hợp lý từ tài liệu + code, không phải giả định):
`specs/backend-go/tdd/README.md` tự mô tả động lực là 2 mảng TS-system đã khảo sát kỹ càng và ghi lại là
có vấn đề cấu trúc: (a) 5 cơ chế lưu secret rời rạc, không nhất quán
(`specs/backend/models/05-credential-secret-stores.md`), và (b) một shared Postgres 25-bảng cho toàn hệ
thống chưa từng thật sự tách theo service (ADR-021 mới ở Phase 0/schema-scaffolding, xem
[docs/adrs/v2/ADR-021](../adrs/v2/ADR-021-unified-postgres-microservices-platform.md)). Backend-go giải
quyết cả hai bằng thiết kế ground-up thay vì tiếp tục retrofit TS system.

---

## 2. Topology — 17 services

Xác nhận trực tiếp qua `backend-go/services/` (17 thư mục) và `deploy/dev/docker-compose.yml` (17
service block dùng chung `*go-image`/`*go-defaults` YAML anchor, cộng `migrate-<service>` job riêng cho
16/17 service có DB — `api-gateway` không có job migrate vì không sở hữu data của riêng nó):

| # | Service | Nhóm | Sở hữu data gì | Thay thế phần TS nào (theo `specs/backend-go/tdd/services/00-service-catalog.md`) |
|---|---------|------|-----------------|----------------|
| 1 | `api-gateway` | Edge | Không (stateless routing + wscompat, §4) | `runtime/rpc/dispatcher.ts`, `WsSessionRouter`, `http-server.ts` |
| 2 | `auth-service` | Identity | Users, sessions, RBAC, audit, admin console | `AuthManager`, `backend/src/main/admin/` |
| 3 | `tenant-service` | Identity | Companies, departments, user profiles, teams | `ProfileResolver`, `TeamService` |
| 4 | `project-service` | Workspace | Projects, membership, project↔dev-server binding, project↔repo binding (`AssignRepoToProject`, xem §6) | `ProjectService` |
| 5 | `infra-fleet-service` | Workspace | SSH targets, dev server registry, fleet health, provider/terminal routing | `ssh/`, `providers/`, `FleetHealthMonitor` |
| 6 | `git-gateway-service` | Workspace | Không (dispatch: resolve host sở hữu worktree rồi relay tới Agent) | `git.ts` dynamic dispatch |
| 7 | `scm-integration-service` | Integration | Không (hệ thống nguồn bên ngoài — GitHub/GitLab/Bitbucket/Azure DevOps/Gitea) | `github/`, `gitlab/`, `bitbucket/`, `azure-devops/`, `gitea/`, `hosted-review/` |
| 8 | `issue-tracking-service` | Integration | Không (Jira/Linear) | `jira/`, `linear/` |
| 9 | `ai-provider-service` | AI | Provider account metadata, usage/quota | `AIProviderService`, `ProviderResolver`, `ProviderHealthChecker` |
| 10 | `workflow-service` | AI | Templates, executions, step executions | `WorkflowOrchestrator`, `DAGBuilder`, `StepExecutors` — **mô hình khác hẳn, xem §5** |
| 11 | `task-service` | AI | Task DAG, edges, grants, comments | `TaskService`, `TaskDAGValidator`, `TaskGrantService`, `TaskAIPlanner` |
| 12 | `orchestration-service` | AI | Multi-agent coordination gates | `PgOrchestrationDb`, `runtime/orchestration/` |
| 13 | `automation-service` | AI | Automation definitions + runs | `AutomationService` |
| 14 | `annotation-service` | Supporting | Review comments | `annotation-store.ts` |
| 15 | `notification-service` | Supporting | Push subscriptions, VAPID metadata, WS fan-out | `WebPushManager`, `PgWebPushStore` |
| 16 | `usage-service` | Supporting | AI-CLI usage/cost tracking | `ClaudeUsageStore`, `CodexUsageStore` |
| 17 | `credential-broker-service` | Supporting | Secret **metadata only** — mediates mọi read/write secret thật qua Vault | 5 cơ chế rời rạc, xem §6 |

**Không phải service riêng** (external infra, chạy trong cùng `deploy/dev/docker-compose.yml` nhưng
không phải 1 trong 17): `postgres` (1 instance, database-per-service — §6), `vault` + `vault-init`
(secrets), `nats` (`nats:2.10-alpine -js`, event bus — dùng cho domain event async, xem
`specs/backend-go/tdd/architecture/08-inter-service-communication.md` §"Event conventions"; **mức độ
publish/consume thật sự đã wire tới đâu trong 17 service — CHƯA xác nhận trong phiên rà soát này, cần
audit riêng trước khi khẳng định outbox pattern đã chạy đầy đủ**).

---

## 3. Internal architecture pattern — hexagonal/clean architecture, nhất quán

Mỗi service theo cùng 1 layout (xác nhận trên `project-service`, `workflow-service`, `api-gateway`,
`credential-broker-service` — không phải audit toàn bộ 17/17, xem cảnh báo cuối mục):

```
backend-go/services/<name>/
├── cmd/                       ← main package, khởi tạo DI, đọc config, mở gRPC server
├── internal/
│   ├── domain/                ← entity + value object thuần Go, không phụ thuộc framework/DB
│   ├── usecase/                ← application logic, orchestrate domain + port interfaces
│   └── adapter/
│       ├── grpc/               ← gRPC server, implement <Service>ServiceServer từ proto
│       └── postgres/            ← implement port interface bằng pgx/sqlc, migration riêng service
└── (api-gateway thêm: internal/adapter/wscompat/ — xem §4)
```

Đây đúng là **Ports & Adapters (hexagonal)**: `usecase` phụ thuộc vào interface trong `domain`/của chính
`usecase`, còn `adapter/grpc` và `adapter/postgres` là 2 adapter độc lập implement các interface đó —
domain logic không import trực tiếp `pgx` hay generated gRPC stub.

⚠️ **Chưa audit toàn bộ 17/17 service để xác nhận layout giống hệt nhau** — chỉ xác nhận trên 4 service
kể trên qua đọc trực tiếp cấu trúc thư mục trong phiên này. Coi đây là pattern **phổ biến, nhiều khả
năng đúng cho phần lớn**, không phải đã kiểm chứng từng service.

---

## 4. Internal contract: gRPC + `api-gateway`'s wscompat bridge tới browser/Electron

### 4.1 Nội bộ (service ↔ service, và `api-gateway` ↔ service): gRPC thuần

Mỗi service export proto package `orca.<service>.v1` (`backend-go/proto/orca/<service>/v1/*.proto`,
generated stub tại `backend-go/proto/gen/go/orca/<service>/v1/`). `api-gateway` giữ 1
`<Service>ServiceClient` cho từng service nó cần gọi.

### 4.2 Ngoài (browser/Electron client) ↔ `api-gateway`: **KHÔNG phải gRPC/gRPC-Web trực tiếp**

Frontend web/Electron hiện có (mô tả tại
[web-server-architecture.md §5.1](./web-server-architecture.md#51-websocketrpcclient)) đã nói một giao
thức riêng: WebSocket JSON envelope kiểu ipcRenderer (`{ id, type: 'invoke', channel, args }` →
`{ id, type: 'result', result }`), không phải JSON-RPC 2.0, không phải gRPC.

`api-gateway` **không bắt frontend đổi giao thức**. Thay vào đó,
`backend-go/services/api-gateway/internal/adapter/wscompat/` là một lớp dịch: mỗi file
`channels_<domain>.go` (ví dụ `channels_workflow.go`, `channels_git.go`, `channels_tenant_project.go` —
xác nhận ~25 file `channels_*.go` trong thư mục này) đăng ký handler cho từng `channel` name y hệt tên
namespace RPC cũ (`workflow.execute`, `workflow.getExecution`, ...), decode `args` JSON, gọi đúng
`<Service>ServiceClient` method tương ứng qua gRPC nội bộ, rồi trả kết quả (raw proto message,
`encoding/json`-marshalled — xem cảnh báo camelCase/snake_case bên dưới) lại đúng envelope WS mà frontend
đã biết cách parse.

```
Browser/Electron WebSocketRpcClient
    │  { id, type:'invoke', channel:'workflow.getExecution', args:[...] }
    ▼
api-gateway :  wscompat.Registry.Register("workflow.getExecution", handler)
    │  handler decode args → gRPC WorkflowServiceClient.GetExecution(ctx, req)
    ▼
workflow-service (gRPC, Postgres riêng)
    │
    ▼ trả về *workflowv1.GetExecutionResponse
api-gateway :  encode lại thành { id, type:'result', result }
    ▼
Browser/Electron — không biết/không cần biết phía sau là gRPC
```

**Nguồn:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go:1-46` —
docblock của `registerWorkflowChannels()` tự giải thích chính xác cơ chế này, gồm cả một cảnh báo thật
đáng chú ý (giữ nguyên, không phải suy diễn):

> mọi handler trả `*workflowv1.X` proto message qua `encoding/json` thuần (không phải `protojson`), nên
> field name serialize ra là `snake_case` (`template_id`) — khác `camelCase` mà frontend TypeScript
> (`WorkflowExecution`/`WorkflowTemplate` type) mong đợi. Đây là **lỗi/gap thật đang tồn tại**, tự file
> đó ghi rõ là biết nhưng chưa sửa (nằm ngoài phạm vi CR đã thêm 7 channel còn thiếu) — không suy đoán,
> trích trực tiếp comment trong code.

**Vì sao chọn wscompat thay vì client nói thẳng gRPC-Web/`grpc-gateway`:** xem
[ADR-022](../adrs/v2/ADR-022-wscompat-protocol-bridge.md) — một quyết định kiến trúc thật (code đã chọn
và implement hướng này), tuy `specs/backend-go/tdd/architecture/08-inter-service-communication.md` §"API
Gateway responsibilities" mô tả target dùng `grpc-gateway` cho REST — **hai điều này không khớp nhau**,
ADR-022 ghi lại rõ khác biệt.

---

## 5. Hai workflow engine song song, KHÔNG tương thích nhau

Đây là một sự thật kiến trúc hiện tại quan trọng, không phải lỗi tài liệu — xem
[ADR-024](../adrs/v2/ADR-024-dual-workflow-engines-migration.md) cho phân tích đầy đủ. Tóm tắt:

| | Legacy TS engine | Go `workflow-service` |
|---|---|---|
| Vị trí | `backend/src/main/workflow/` + `desktop/src/main/workflow/` | `backend-go/services/workflow-service/` |
| Model | DAG/wave-based, `WorkflowOrchestrator` + `DAGBuilder`, step data đầy đủ | Phẳng hơn — `WorkflowExecution{id, template_id, status, root_trace_id, project_id}` (`backend-go/proto/orca/workflow/v1/workflow.proto:91-97`), **không trả step/DAG data qua gRPC hiện tại** dù bảng `workflow.step_executions` tồn tại ở tầng DB (chưa expose qua RPC — chưa xác nhận lý do: scaffold-chưa-hoàn-thiện hay chủ đích) |
| Ai gọi tới nó | Electron/local runtime — RPC namespace `workflow.*` đi thẳng vào TS dispatcher, không qua backend-go | Web-mode client (browser) — cùng RPC namespace `workflow.*`, nhưng đi qua `api-gateway`'s wscompat (§4) tới gRPC `workflow-service` |
| Data store | Postgres/SQLite `orca_workflow_executions` (bảng chung TS system) | Postgres riêng của `workflow-service` (database-per-service, §6) |

**Hệ quả thật, chưa có giải pháp khắc phục nào trong code:** một workflow execution tạo qua Electron
(engine TS) hoàn toàn vô hình với Web-mode (engine Go) và ngược lại — 2 database riêng biệt, không đồng
bộ. Người dùng chuyển giữa Electron và trình duyệt cho "cùng" một project sẽ thấy 2 lịch sử workflow
tách biệt.

**Bằng chứng trực tiếp trong code** (không suy diễn): docblock của `registerWorkflowChannels()`
(`channels_workflow.go:23-29`, CR-PW-005) tự ghi: *"Electron/local mode already worked (it talks to the
legacy TS workflow-rpc-handler directly)"* trong khi Web-mode **404'd với "unknown channel"** cho 7/11
RPC method cho tới khi CR-PW-005 backfill — nghĩa là chính bản thân code xác nhận 2 con đường tách biệt,
và xác nhận Web-mode từng bị thiếu tính năng hoàn toàn (không chỉ khác data) trước bản vá đó.

---

## 6. Data & Secrets — Postgres per-service + Vault dynamic credentials

Xem chi tiết đầy đủ tại
[`specs/backend-go/tdd/architecture/05-data-architecture.md`](../../specs/backend-go/tdd/architecture/05-data-architecture.md)
và
[`specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md`](../../specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md).
Tóm tắt phần **đã xác nhận là thật** trong phiên rà soát này — xem
[ADR-023](../adrs/v2/ADR-023-postgres-per-service-vault-dynamic-credentials.md) cho phân tích quyết định
đầy đủ:

- **Database-per-service thật, không phải schema-per-service-trong-1-instance** như ADR-021 (bên TS)
  từng chọn làm bước trung gian thực dụng. `deploy/dev/docker-compose.yml` có 1 job `migrate-<service>`
  riêng cho từng service (`migrate-auth`, `migrate-tenant`, ..., `migrate-scm` — 16 job, khớp 16/17
  service có data riêng) — mỗi service tự chạy migration + kết nối DSN riêng, không phải 1 migration
  runner chung chạy 1 database chia schema.
- **Vault dynamic Postgres credentials cho MỌI service** — không phải static password trong config.
  `backend-go/common/secrets/vault.go`'s doc comment (dòng đầu file): *"Every service uses
  DatabaseCredentials (via the Vault Agent sidecar file, in production)"*.
  - ⚠️ **Nhưng sidecar này chưa thật sự nối dây trong `deploy/dev/`**: `DatabaseCredentialsFromFile()`
    (`vault.go`) đọc file do Vault Agent sidecar render — nếu không tồn tại, **fallback đọc thẳng env
    var `DATABASE_DSN`**, và chính doc comment của hàm này tự thừa nhận: *"Falls back to the DATABASE_DSN
    env var when the file doesn't exist, which is what every service's local-dev/testcontainers path uses
    instead of a real Vault Agent"* — nghĩa là con đường "thật" (sidecar) hiện là **target đã code sẵn
    nhưng không phải con đường mọi service dev/test đang chạy hôm nay**; `DATABASE_DSN` fallback mới là
    con đường thật đang chạy trong `deploy/dev/`.
  - `deploy/dev/` tự chạy 1 Vault **dev-mode riêng, in-memory, mất dữ liệu mỗi lần restart** — không phải
    Vault Agent sidecar production như tài liệu target mô tả. Đây từng gây sự cố thật: một lần reboot
    host xoá sạch Vault, khiến `credential-broker-service`/`auth-service` crash-loop cho tới khi
    `orca-vault-init.service` (systemd unit tự khôi phục) chạy lại.
- **Tenant secret material CHỈ qua `credential-broker-service`** — mọi OAuth token
  tích hợp, AI provider API key, VAPID signing key đều qua Vault Transit/KV v2 **mediated bởi
  `credential-broker-service`'s gRPC API** (`WriteCredential`, `ResolveCredential`, `RotateCredential`,
  `RevokeCredential`, `GetCredentialMetadata`, `ResolveCredentialByOwner`, `SignVapidPayload`). Ngoại lệ
  duy nhất ngoài "dynamic DB credential cho chính mình": `auth-service` tự gọi Vault Transit cho JWT
  signing key của chính nó (service-identity key, không phải tenant secret) — xác nhận qua
  `vault.go`'s doc comment: *"auth-service is the one other direct Transit caller (Epic D) — its JWT
  signing key is a service-wide signing identity, not tenant secret material, so it falls outside that
  rule"*.
- **`project-service`: hai cơ chế multi-repo/multi-project riêng biệt, đừng nhầm** —
  `Repo.ProjectID` (RPC `AssignRepoToProject`, thêm gần đây) cho phép 1 Project sở hữu nhiều Repo; đây
  KHÁC với `OrcaProjectSourceProject`/`orcaProjects.*` (cơ chế cũ hơn, join view của cả 1 Project vào
  Project khác) — hai cơ chế cùng tồn tại song song, phục vụ 2 nhu cầu khác nhau.

---

## 7. Backend-go ↔ Dev Server Agent (execution plane) — KHÔNG đổi

`agent/` (Dev Server Agent, mô tả đầy đủ tại
[dev-server-architecture.md](./dev-server-architecture.md)) **nằm ngoài phạm vi** rewrite backend-go —
xác nhận tại `specs/backend-go/tdd/architecture/02-microservices-decomposition.md` §"What's deliberately
not a separate service": *"The Dev Server Agent's role ... stays out of this decomposition ... a
different system (agent/ today)"*.

`infra-fleet-service` và `git-gateway-service` là 2 service Go duy nhất nói chuyện với Agent, và theo
`specs/backend-go/tdd/architecture/08-inter-service-communication.md` §"Talking to the Dev Server Agent",
họ **giữ nguyên** giao thức wire hiện có của TS system (3 connection mode `relay-ssh` /
`relay-websocket` / `direct-websocket`, khung nhị phân 13-byte) thay vì đổi sang gRPC streaming
("Option A" được chọn làm default, "Option B — modernize to gRPC" bị hoãn vì đổi cả `agent/` nằm ngoài
scope). 🚧 **Chưa tự xác nhận trong phiên này** rằng `infra-fleet-service`/`git-gateway-service` đã có
Go client implementation thật cho giao thức 13-byte đó (chỉ xác nhận đây là quyết định/target đã ghi lại
— chưa đọc code Go client thật của 2 service này để verify wire-format implementation).

---

## 8. Deployment — `deploy/dev/`

`deploy/dev/docker-compose.yml` chạy toàn bộ 17 service như **distroless Go binary bind-mount**, dùng
chung 1 YAML anchor image build (`&go-image`) — không build image riêng từng service. Hạ tầng đi kèm
trong cùng compose file: 1 Postgres instance (database-per-service, §6), 1 Vault dev-mode, `vault-init`
job (seed Vault dev), `nats:2.10-alpine` (JetStream bật qua `-js`), `frontend` container, và các job
`migrate-<service>` chạy dưới profile `migrate` (không tự chạy khi `docker compose up` thường —
`profiles: ["migrate"]`, dùng qua `scripts/migrate.sh`).

Đây là mô hình triển khai **khác hẳn** bất kỳ mô tả deployment nào trong `docs/hld/v1/deployment.md`
(viết cho TS system: Electron installer / Node server / Docker đơn instance) — `docs/hld/v1/deployment.md`
**không đề cập backend-go**, chưa được cập nhật trong phiên rà soát này (ngoài phạm vi task hiện tại; xem
mục "Chưa xác nhận" ở README chính).

🚧 **Chưa xác nhận**: mô hình deployment production (ngoài `deploy/dev/`) — ví dụ Kubernetes/Helm như
`specs/backend-go/tdd/architecture/10-deployment-infrastructure.md` mô tả — nằm ngoài phạm vi phiên rà
soát này (chỉ đọc `deploy/dev/`), nên **không khẳng định** production đã chạy trên K8s hay chưa.

---

## 9. Điều CHƯA xác nhận / cần audit thêm

Ghi rõ theo đúng quy ước của codebase — không đoán:

- Layout hexagonal có nhất quán trên toàn bộ 17/17 service hay không (chỉ xác nhận 4/17, §3).
- Mức độ NATS JetStream thật sự được publish/consume tới đâu (hạ tầng có chạy, chưa audit code
  publisher/consumer, §2).
- Go client implementation thật cho giao thức Agent 13-byte trong `infra-fleet-service`/
  `git-gateway-service` (§7) — chỉ xác nhận quyết định giữ nguyên protocol, chưa đọc code client.
- Mô hình deployment production ngoài `deploy/dev/` (§8).
- Lý do sản phẩm/kỹ thuật cụ thể đằng sau lựa chọn wscompat thay vì `grpc-gateway` (ADR-022) — suy luận
  hợp lý từ code, không phải trích dẫn quyết định có ghi lại từ team.

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [backend-server-architecture.md](./backend-server-architecture.md) | Backend TS hiện hành (song song với backend-go) |
| [web-server-architecture.md](./web-server-architecture.md) | Frontend browser — client của cả wscompat (Web-mode) lẫn TS dispatcher (Electron) |
| [dev-server-architecture.md](./dev-server-architecture.md) | Dev Server Agent — execution plane, không đổi bởi backend-go |
| [ADR-021](../adrs/v2/ADR-021-unified-postgres-microservices-platform.md) | Quyết định tương ứng phía TS (schema-per-service, chưa physical DB-per-service) |
| [ADR-022](../adrs/v2/ADR-022-wscompat-protocol-bridge.md) | wscompat làm lớp dịch giao thức browser↔gRPC |
| [ADR-023](../adrs/v2/ADR-023-postgres-per-service-vault-dynamic-credentials.md) | Postgres-per-service + Vault dynamic credentials cho backend-go |
| [ADR-024](../adrs/v2/ADR-024-dual-workflow-engines-migration.md) | Hai workflow engine song song trong giai đoạn migration |
| [`specs/backend-go/tdd/README.md`](../../specs/backend-go/tdd/README.md) | Tài liệu kiến trúc mục tiêu nội bộ team backend-go (một phần đã implement, một phần vẫn target) |
| [`specs/backend-go/tdd/services/00-service-catalog.md`](../../specs/backend-go/tdd/services/00-service-catalog.md) | Bảng chi tiết service↔TS-equivalent↔migration-phase |
| [`specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md`](../../specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md) | Chiến lược strangler-fig, phase sequencing |
