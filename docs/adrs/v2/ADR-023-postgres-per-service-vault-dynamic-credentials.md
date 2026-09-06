# ADR-023 — Backend-go: Postgres database-per-service (vật lý) + Vault dynamic DB credentials cho mọi service

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-023 |
| **Trạng thái** | ✅ Accepted (database-per-service + fallback DSN) — ⚠️ Partial (Vault Agent sidecar production, xem "Trạng thái Implementation") |
| **Ngày** | 2026-09-06 |
| **HLD Ref** | [docs/hld/backend-go-architecture.md §6](../../hld/backend-go-architecture.md#6-data--secrets--postgres-per-service--vault-dynamic-credentials) |
| **Code Ref** | `backend-go/common/secrets/vault.go`, `deploy/dev/docker-compose.yml` (17× `migrate-<service>` job pattern) |
| **Amends** | [ADR-021](./ADR-021-unified-postgres-microservices-platform.md) — ADR-021 áp dụng cho TS system, ADR này áp dụng cho backend-go; **hai hệ thống, hai kết luận khác nhau về schema-per-service vs database-per-service, cả hai đều đúng cho bối cảnh riêng của nó (xem "Vì sao không thừa hưởng ADR-021 nguyên vẹn")** |
| **Không áp dụng cho** | TS `backend/` (vẫn theo ADR-021: 1 Postgres instance, schema-per-service, chưa physical tách) |

---

## Bối cảnh

ADR-021 (TS system) đứng trước 1 hệ thống **đã chạy production** với 25 bảng `orca_*` trong 1 Postgres
instance duy nhất, không tenant-aware, cộng 3 cơ chế lưu trữ phụ khác (JSON file, SQLite riêng cho
Orchestration, JSON usage). Với ràng buộc "không được phá vỡ runtime hiện tại", ADR-021 chọn
**schema-per-service trong CÙNG 1 Postgres instance** làm bước trung gian thực dụng (`ADR-021 §2`), rõ
ràng defer việc tách vật lý sang Phase 3 (chưa triển khai, `⏳`).

`backend-go/` không có ràng buộc đó — là 1 hệ thống viết mới hoàn toàn, không có dữ liệu production nào
cần giữ nguyên chỗ. Chính tài liệu kiến trúc nội bộ của team backend-go
(`specs/backend-go/tdd/architecture/02-microservices-decomposition.md`, design principle #2) đã nói rõ
quyết định **không thừa hưởng compromise của ADR-021**:

> *"Since this is a ground-up Go build with no existing shared instance to migrate incrementally, there's
> no reason to inherit that compromise — each service gets its own PostgreSQL database"*

## Quyết định

### 1. Database-per-service vật lý — mỗi trong 16 service có data đều có Postgres database riêng

Xác nhận qua code, không chỉ tài liệu: `deploy/dev/docker-compose.yml` định nghĩa 1 job `migrate-<name>`
riêng cho từng service (`migrate-auth`, `migrate-tenant`, `migrate-project`, `migrate-infra`,
`migrate-aiprovider`, `migrate-workflow`, `migrate-task`, `migrate-orchestration`, `migrate-automation`,
`migrate-annotation`, `migrate-notification`, `migrate-usage`, `migrate-credential`,
`migrate-issuetracking`, `migrate-scm` — 15 job cho 16 service có data; `git-gateway-service` và
`api-gateway` không sở hữu data riêng nên không có job migrate) — mỗi service tự chạy migration + đọc
DSN riêng của chính nó, không phải 1 migration runner dùng chung 1 database rồi tách theo schema.

### 2. Vault dynamic Postgres credentials là cơ chế MẶC ĐỊNH cho mọi service, không phải static password

`backend-go/common/secrets/vault.go`, doc comment đầu file: *"Every service uses DatabaseCredentials (via
the Vault Agent sidecar file, in production — see below)"*. `DatabaseCredentialsFromFile(path)` đọc file
Vault Agent sidecar render sẵn (credential ngắn hạn, tự xoay vòng theo lease) — không đọc password tĩnh
từ config/secret-manifest.

### 3. Tenant secret material CHỈ qua `credential-broker-service`, không service nào khác gọi thẳng Vault cho việc đó

Ngoại lệ ghi rõ trong chính code: `auth-service` tự gọi Vault Transit **chỉ** cho JWT signing key của
chính nó (service identity key, không phải secret của tenant/user) — `vault.go`'s doc comment: *"Epic
D... its JWT signing key is a service-wide signing identity, not tenant secret material, so it falls
outside that rule"*. Mọi OAuth token tích hợp (GitHub/GitLab/Jira/Linear/...), AI provider API key, VAPID
signing key đều đi qua `credential-broker-service`'s gRPC API
(`WriteCredential`/`ResolveCredential`/`RotateCredential`/`RevokeCredential`/`GetCredentialMetadata`/
`ResolveCredentialByOwner`/`SignVapidPayload`).

## Vì sao không thừa hưởng ADR-021 nguyên vẹn (cả hai quyết định đều đúng cho bối cảnh riêng)

| | ADR-021 (TS `backend/`) | ADR-023 (backend-go) |
|---|---|---|
| Ràng buộc | Hệ thống đang chạy production, không được downtime | Viết mới, chưa có traffic |
| Chọn | Schema-per-service, 1 Postgres instance | Database-per-service, N Postgres database |
| Vault | Chưa dùng (out of scope tại thời điểm ADR-021) | Dùng cho mọi service, ngày đầu |
| Multi-tenancy | `tenant_id` cột + `TenantContext` app-layer (chính) + RLS (phòng vệ thêm) | Chưa xác nhận cơ chế tenant-scoping thật của backend-go trong phiên rà soát này — 🚧 cần audit riêng, không giả định giống hệt ADR-021 |

## Hệ quả

- ✅ Mỗi service scale độc lập, backup/restore độc lập, migration history độc lập — không có rủi ro
  "1 migration lỗi ở service A khoá luôn service B" như khi dùng chung 1 instance.
- ✅ Compromise dữ liệu 1 service (leak DB credential) không tự động lộ dữ liệu service khác — mỗi
  service có Vault policy + DB credential riêng.
- ⚠️ **Vault Agent sidecar — pattern production — CHƯA thật sự nối dây trong `deploy/dev/`.** Đây là gap
  quan trọng nhất cần cảnh báo, không phải chi tiết nhỏ: `DatabaseCredentialsFromFile()` tự ghi trong
  doc comment: *"Falls back to the DATABASE_DSN env var when the file doesn't exist, which is what every
  service's local-dev/testcontainers path uses instead of a real Vault Agent"* — nghĩa là **con đường
  Vault Agent sidecar mô tả trong `specs/backend-go/tdd/architecture/06` là target đã có code sẵn sàng
  nhận nhưng KHÔNG phải con đường bất kỳ service nào trong `deploy/dev/` đang thực sự đi qua hôm nay** —
  chúng đều rơi vào fallback `DATABASE_DSN`.
- ⚠️ `deploy/dev/` tự chạy 1 Vault dev-mode **in-memory, mất dữ liệu mỗi lần restart** (không phải Vault
  Agent sidecar thật). Một sự cố thật đã xảy ra vì điều này: host reboot → Vault dev-mode mất sạch →
  `credential-broker-service`/`auth-service` crash-loop cho tới khi `orca-vault-init.service` (systemd
  auto-recovery, seed lại Vault dev) chạy. Đây là rủi ro vận hành thật của môi trường dev hiện tại, không
  phải giả thuyết.
- 🚧 Nhiều hơn 1 Postgres instance vật lý cần vận hành/backup/patch riêng biệt (so với schema-per-service
  chỉ 1 instance) — chi phí vận hành cao hơn, đổi lại cô lập tốt hơn. Chưa xác nhận `deploy/dev/` đã thật
  sự chạy 17 Postgres instance riêng hay 1 instance với 17 database logic (cả hai đều thoả "database-
  per-service" theo nghĩa Postgres `CREATE DATABASE`, khác nhau ở tách vật lý container) — **cần đọc
  `deploy/dev/docker-compose.yml`'s `postgres:` service definition kỹ hơn để xác nhận số container thật**
  (phiên này chỉ xác nhận có 1 `postgres:` service block trong compose, chưa xác nhận nó tạo bao nhiêu
  database logic bên trong).

## Trạng thái Implementation

| Phần | Trạng thái |
|---|---|
| Database-per-service (migration jobs riêng, DSN riêng) | ✅ Implemented |
| Vault dynamic credentials — code path đọc file sidecar | ✅ Implemented (code sẵn sàng) |
| Vault Agent sidecar thật trong production | 🚧 Chưa xác nhận có triển khai thật ngoài `deploy/dev/` — nằm ngoài phạm vi phiên rà soát này |
| Vault Agent sidecar trong `deploy/dev/` | ❌ Không có — mọi service dùng fallback `DATABASE_DSN` |
| `credential-broker-service` làm broker duy nhất cho tenant secret | ✅ Implemented (theo code + doc comment `vault.go`) |
| Tenant-scoping (RLS hay app-layer, tương đương ADR-021 §3) | 🚧 Chưa xác nhận trong phiên rà soát này |

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR-021](./ADR-021-unified-postgres-microservices-platform.md) | Quyết định tương ứng phía TS — schema-per-service, chưa Vault |
| [docs/hld/backend-go-architecture.md §6](../../hld/backend-go-architecture.md) | Bối cảnh đầy đủ + bảng 17 service |
| `specs/backend-go/tdd/architecture/05-data-architecture.md` | Target design đầy đủ: multi-tenancy, outbox/saga |
| `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md` | Target design đầy đủ Vault (K8s auth method, engines used) |
