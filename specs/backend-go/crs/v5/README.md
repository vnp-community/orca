# backend-go — Solutions cho CR v5 (MCP)

> **CR gốc:** [docs/crs/v5](../../../../docs/crs/v5/README.md) · **TDD áp dụng:** [specs/backend-go/tdd](../../tdd/README.md)
> **Hợp đồng với frontend:** [CONTRACT-mcp-ui-api.md](./CONTRACT-mcp-ui-api.md) (bắt buộc) · **Phía frontend:** [specs/frontend/crs/v5](../../../frontend/crs/v5/README.md)
> Trạng thái (cập nhật 2026-10-02): BE-MCP-SOL-001..007, 010..014 ✅ Implemented (unit/integration tests) — xem README từng service để biết khoảng trống; 008 🟡 pack 1-3 (pack 4 chưa); 009 🔲 đang được hiện thực riêng; 015 🟡 một phần (xem banner của nó). **Chưa có kiểm chứng end-to-end trên stack dev đầy đủ.**

## 1. Cấu trúc

```
specs/backend-go/crs/v5/<feature>/solutions/
    README.md                      # bảng solution, đối chiếu CR, thứ tự, phụ thuộc
    BE-MCP-SOL-NNN-<slug>.md       # NNN = số CR tương ứng (CR-MCP-NNN)
```

| Feature | Solution (BE) |
|---------|---------------|
| [mcp-service-foundation](./mcp-service-foundation/solutions/README.md) | BE-MCP-SOL-001, 002 |
| [mcp-protocol-server](./mcp-protocol-server/solutions/README.md) | BE-MCP-SOL-003, 004 |
| [mcp-authorization](./mcp-authorization/solutions/README.md) | BE-MCP-SOL-005, 006 |
| [mcp-tool-catalog](./mcp-tool-catalog/solutions/README.md) | BE-MCP-SOL-007, 008, 009 |
| [mcp-resources-prompts](./mcp-resources-prompts/solutions/README.md) | BE-MCP-SOL-010, 011 |
| [mcp-governance-safety](./mcp-governance-safety/solutions/README.md) | BE-MCP-SOL-012, 013 |
| [mcp-client-registry](./mcp-client-registry/solutions/README.md) | BE-MCP-SOL-014 |
| [mcp-quality-rollout](./mcp-quality-rollout/solutions/README.md) | BE-MCP-SOL-015 |

## 2. Quy ước từ TDD mà mọi solution phải tuân (đã đối chiếu `specs/backend-go/tdd`)

| Chủ đề | Quy ước | Nguồn TDD |
|--------|---------|-----------|
| Layout | `cmd/server`, `internal/{domain,usecase,adapter,config}`, `migrations/`, `deploy/`; domain chỉ stdlib; usecase có `ports.go`, mỗi usecase một type `Execute(ctx,in)`; `main.go` là composition root duy nhất | arch/03 |
| DB | database-per-service, pool + Vault dynamic creds + migration history riêng; **cấm** FK/join xuyên DB; tham chiếu service khác là logical FK kiểm bằng gọi API | arch/02, arch/05 |
| Tenant | `tenant_id UUID NOT NULL` mọi bảng; tenant đi qua gRPC metadata, không qua body; mọi truy vấn bind `tenant_id`; RLS `current_setting('app.tenant_id')` + `SET LOCAL`; cross-tenant ⇒ not-found | arch/05, arch/07 |
| Sự kiện | outbox cùng transaction; subject `orca.<service>.<entity>.<event>`; payload có event id, tenant id, occurred-at, schema version, chỉ đổi additive; consumer dedupe `processed_events` | arch/05, arch/08 |
| Lỗi | lỗi domain có kiểu → `common/apperrors` (`Kind`, `Code` UPPER_SNAKE) → status; protovalidate ở delivery | arch/03, arch/07 |
| Deadline | `context.WithTimeout` bắt buộc mọi call ra ngoài (mặc định 5s); retry chỉ với call idempotent; ghi chú lý do khi override | arch/08, arch/09 |
| Migration | golang-migrate, `0001_init.up/down.sql`, CI chạy up→down→up trên testcontainers; Postgres-only mặc định cho service mới (mcp-service: Postgres-only tạm thời — D2) | arch/04, arch/05 |
| Secret | service **không** nói chuyện Vault cho secret tenant — đi qua `credential-broker-service`; service chỉ dùng Vault cho DB creds | arch/06 |
| Authz | gateway kiểm thô; kiểm mịn trong service bằng OPA in-process; Rego ở bundle `orca-authz` (`package orca.authz.<x>`, `default allow := false`, có `*_test.rego`) | arch/07 |
| Audit | qua outbox → auth-service `audit_log` append-only | arch/07 |
| Quan sát | slog JSON (`trace_id`, `tenant_id`, `service`, `version`), OTel, RED metrics Prometheus, `/healthz` `/readyz` | arch/09 |
| Triển khai | Helm chart từ `orca-go-common-chart`, HPA, PDB, NetworkPolicy default-deny + allow theo đồ thị phụ thuộc, ServiceMonitor; compose + `postgres-init-databases.sh` | arch/10 |
| Test | unit (fake), integration `//go:build integration` (testcontainers), contract `buf breaking`, `opa test`, e2e ít kịch bản compose | arch/03, arch/04 |
| Lint repo | không `max-lines` disable; không đặt tên `helpers/utils/common`; Git ≥ 2.25; SSH-aware; tương thích GitLab | AGENTS.md |

> Ghi chú: TDD tham chiếu thư mục `standards/` và `migration/` nhưng **không tồn tại** — các quy tắc cấp mã (tag test, lint, tên metric/env) được lấy từ mã thật trong `backend-go/` (`.golangci.yml`, `Makefile`, `common/*`) và ghi "(code)" trong từng solution.

## 3. Các chỗ MCP **xung đột/khoảng trống** với TDD hiện hành → quyết định + sửa TDD kèm theo

Mỗi solution liên quan phải ghi mục "Sửa TDD kèm theo" trỏ vào bảng này. Đây là danh sách chuẩn:

| # | TDD nói | MCP cần | Quyết định của v5 | TDD cần cập nhật |
|---|---------|---------|-------------------|-------------------|
| T1 | api-gateway = "pure edge, zero business logic", không điều phối liên service (api-gateway.md §1,§2) | Endpoint giao thức `/mcp` viết tay | Chấp nhận như **một edge protocol adapter** (`adapter/mcpserver`), giống `wsbridge`/`wscompat`: chỉ dịch giao thức + gọi gRPC, **không** chứa quy tắc nghiệp vụ; policy/audit/approval nằm ở `mcp-service` | `api-gateway.md` §3 (thêm `/mcp`, `/oauth/*`, `/.well-known/*`), §6 (adapter mới); `arch/08` trách nhiệm gateway |
| T2 | Không nhắc `wscompat`; mọi việc miền đi qua gRPC có mTLS (arch/08) | `tools/call` gọi lại `Registry.Dispatch` | **Giữ D3** nhưng nêu rõ: `wscompat.Registry` chỉ là lớp dịch tới **gRPC client** của các service (không có nghiệp vụ trong gateway); `ToolExecutor` là adapter, quyết định allow/deny/approval luôn do `mcp-service` (gRPC) trả về trước khi dispatch | `api-gateway.md` §6 (mô tả `wscompat` + `ToolExecutor`); `arch/08` |
| T3 | auth-service không có vai trò OAuth AS (auth-service.md §3,§9; SSO/OIDC "chưa build") | `/authorize`,`/token`,`/register`,`/revoke`, metadata, PKCE, consent | **Thêm vào auth-service** (nó đã giữ khoá ký/JWKS + danh tính): nhóm RPC `OAuth*`, bảng `oauth_clients`, `oauth_auth_codes`, `oauth_refresh_tokens`; dữ liệu **consent/grant theo MCP** do `mcp-service` giữ qua gRPC (tránh auth-service phụ thuộc mcp trên đường login lõi — auth-service.md §7) | `auth-service.md` §3 (API), §4 (domain), §5 (data), §9 |
| T4 | Không có PAT; JWT ngắn hạn; không kiểm `aud` phía verifier | PAT hạn ≤ 90 ngày, `aud` = resource MCP | Thêm "MCP PAT" là **ngoại lệ có tài liệu**: lưu hash SHA-256, có `jti`, thu hồi tức thì, audit mỗi lần tạo/thu hồi/dùng đầu; verifier MCP **bắt buộc** `aud == resourceUrl`; verifier REST/WS **từ chối** token có `aud` MCP | `auth-service.md` §3,§5,§9; `arch/07` (bảng AuthN) |
| T5 | Fan-out chỉ qua JetStream; không có cross-replica ephemeral (api-gateway.md §5; notification §3) | SSE/progress/list_changed giữa replica | **Đã xác minh trong mã:** `common/eventbus.SubscribeEphemeral` là *JetStream ephemeral consumer* theo replica, **không** phải core-NATS. Thiết kế (BE-MCP-SOL-004): core-NATS ephemeral cho tín hiệu điều khiển + một stream JetStream giới hạn (`MCPSSE`) làm vòng đệm resume; cần thêm API vào `common/eventbus` (chủ package duyệt) | `arch/08` (mục Event conventions: thêm "ephemeral subjects"); `api-gateway.md` §5 |
| T6 | Rate limit per tenant/user trước routing; token bucket per-replica hoặc Redis (api-gateway.md §9) | Hạn mức theo (tenant,user,client,risk) + giới hạn số stream SSE | Mở rộng limiter hiện có bằng khoá `client`/`risk`; nhiều replica ⇒ dùng Redis (đã là lựa chọn build-time) hoặc chấp nhận sai số có ghi rõ | `api-gateway.md` §9; `auth-service.md` rate-tier data |
| T7 | "mcp-service không nên là wrapper mỏng" (arch/02 nguyên tắc 4) | Service mới | `mcp-service` sở hữu **trạng thái và quyết định thật**: grant/consent, session, policy tool, approval, audit nguồn, registry server ngoài ⇒ có DB riêng, có logic (đủ điều kiện) | `00-service-catalog.md` (thêm hàng), `arch/02` (bảng + đồ thị + số đếm service/DB), `arch/03..05,09,10` (số "17 services") |
| T9 | Broker chỉ cho 5 `CredentialCategory` (CHECK trong `0001_init.up.sql`); secret của MCP server ngoài chưa có chỗ | Secret env/header của server ngoài | Thêm category `mcp_external_secret` + migration broker (**cần**: CHECK `category` ở `0001_init.up.sql` của cả `postgres/` và `mysql/` chỉ có 5 giá trị — đã xác minh). **Đã chốt (D1):** không dùng phong bì client (broker coi nó là bytes mờ; `lib/credential-crypto.ts` dùng khoá dự phòng `'fallback-dev-token'`); UI gửi plaintext một lần qua WS/TLS (`mcp.externalServer.setSecret{serverId,kind,name,value}`), `mcp-service` ghi vào broker, broker Transit-encrypt; `value` bị che ở log/trace và không echo (CONTRACT C11) | `arch/06`, `credential-broker-service.md` |
| T10 | `arch/04`: multi-dialect là chuẩn; README này nói "Postgres-only mặc định" | DB cho mcp-service | **Đã chốt (D2, 2026-10-01):** `mcp-service` **Postgres-only tạm thời**, fail-fast với DSN khác (BE-MCP-SOL-001); không có `mysql/` bây giờ — đa dialect về sau cần `adapter/mysql` + `migrations/mysql` + thay SQL đặc thù Postgres + CI matrix | `arch/04`, `arch/05` |
| T11 | `/ws` đặt `InsecureSkipVerify: true` (không kiểm Origin) trong khi auth bằng cookie | `mcp.consent.decide`, `mcp.approval.decide` thành mục tiêu CSWSH | **Đã chốt & phê duyệt (D3), đang hiện thực:** env `WS_ALLOWED_ORIGINS` (CSV origin chính xác/host pattern) áp cho `wscompat.Handler`, `wsbridge.Handler` và `/mcp`; không rỗng ⇒ `OriginPatterns` thay `InsecureSkipVerify`; rỗng ⇒ hành vi cũ + WARN to khi khởi động. `MCP_ALLOWED_ORIGINS` mặc định = `WS_ALLOWED_ORIGINS` (BE-MCP-SOL-002 §D.3) | `api-gateway.md` §9, `arch/07` |
| T8 | Metric/env không quy định tiền tố | Tên metric/env MCP | Metric `orca_mcp_*`; env `MCP_*` (BE-002) theo `common/config` | `arch/09` (ghi quy ước tiền tố) |

### Quyết định đã chốt (2026-10-01)

| # | Quyết định | Áp dụng ở |
|---|-----------|-----------|
| D1 | **Secret MCP ngoài:** không dùng phong bì client. `mcp.externalServer.setSecret{serverId,kind,name,value}` gửi plaintext một lần qua WS/TLS; gateway ⇒ `mcp-service` ⇒ broker `WriteCredential` (category `mcp_external_secret`, Vault Transit); plaintext không lưu/log, `value` bị che ở log WS + trace store, response không echo (CONTRACT C11, có test); lúc spawn `mcp-service` `ResolveCredential` rồi chỉ đưa vào env tiến trình con. Điểm yếu `'fallback-dev-token'` của `CredentialInput` AI Provider là follow-up riêng, ngoài phạm vi. FE copy: "sent over TLS and encrypted at rest by the server", không nói mã hoá đầu cuối. Migration CHECK broker là **cần thiết** (đã xác minh) | T9, BE-014, FE-011, CONTRACT |
| D2 | `mcp-service` **Postgres-only tạm thời**, fail-fast DSN khác; chưa có `mysql/` | T10, BE-001, CR-001 |
| D3 | Allow-list Origin được phê duyệt, đang hiện thực: `WS_ALLOWED_ORIGINS` (+ `MCP_ALLOWED_ORIGINS` mặc định theo nó); rỗng ⇒ hành vi cũ + WARN | T11, BE-002 §D.3, CR-002 |
| D4 | Go SDK chính thức cho server; **Python SDK chính thức** (`mcp`) làm client tham chiếu trong CI (`backend-go/ci/mcp-conformance/`, phiên bản ghim; API chưa xác minh); harness Go giữ cho test in-process | BE-003, BE-015, FE-012, CR-015 |
| D5 | Web Push deep link `/?section=mcp&tab=approvals&approval=<id>`; service worker `frontend/src/renderer/public/service-worker.js`; hộp thoại trong app là đường chính; push thật phụ thuộc **CR-NOTIF-002** (`DeliverPush` chưa có) | CONTRACT §4, BE-013, FE-009 |
| D6 | Tenant mới **bật mặc định** qua `MCP_TENANT_DEFAULT_ENABLED` (mặc định `true`, `mcp-service` đọc); `MCP_ENABLED` vẫn là công tắc tổng (mặc định `false` tới gate CR-015); mặc định rủi ro/hard-deny/scope/kill switch không bị cờ này đổi (có test) | BE-002, BE-012, BE-015, CONTRACT, FE-001/008, CR-002/012 |

## 4. Mẫu nội dung một BE solution

Giữ đúng khuôn `crs/v4/notification/solutions` (tiếng Việt, định danh tiếng Anh): banner trạng thái → CR/Service/TDD tham chiếu → **1. Trạng thái hiện tại (re-verify)** → **Quyết định khác/thêm so với CR gốc** → **2. Giải pháp** (A, B, C… kèm snippet Go/proto/SQL/Rego) → **Hợp đồng với frontend** (kênh nào từ CONTRACT, ánh xạ lỗi) → **Sửa TDD kèm theo** (mã T# ở mục 3) → **Kiểm thử** → **Rủi ro & phụ thuộc** → **Không thuộc phạm vi** → **Liên quan** (file links). Phần *impact analysis* (`gitnexus impact`) chỉ ghi lệnh cần chạy trước khi sửa và rủi ro dự kiến — **không bịa số liệu**.
