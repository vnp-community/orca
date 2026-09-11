# BE-DB-SOL-007: Dialect capability layer + MySQL/TiDB adapter cho `scm-integration-service`

> **✅ Implemented (2026-09-11).** Áp dụng nguyên vẹn pattern đã xác lập ở
> BE-DB-SOL-001/002 (`usage-service`, pilot) — không thiết kế lại. Batch 2
> của rollout 15-service (xem
> [ROLLOUT-TRACKING.md](../tasks/ROLLOUT-TRACKING.md)), chạy song song với
> `notification-service`/`ai-provider-service`/`orchestration-service` (3
> agent khác, không đụng file của nhau — mỗi service 1 thư mục độc lập).

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/scm-integration-service`
**Depends on:** `backend-go/common/dbcapability` (đã có sẵn từ pilot, dùng
lại nguyên vẹn, không sửa), `backend-go/common/testutil.StartMySQL` (đã có
sẵn, dùng lại)
**Task tương ứng:** [TASK-BE-DB-012](../tasks/TASK-BE-DB-012-scm-integration-service-mysql-rollout.md)

---

## 1. Audit thật trước khi implement (đã Read đầy đủ, không suy đoán)

ROLLOUT-TRACKING.md's audit ghi **4 repository, 6 migration** cho
`scm-integration-service` — xác nhận đúng qua Read trực tiếp:

- **4 repository**, mỗi cái 1 struct/1 file riêng trong
  `internal/adapter/postgres/` (KHÔNG phải 1 struct gộp như
  `usage-service`/`annotation-service`):
  - `RateLimitCacheRepository` (`rate_limit_cache.go`) — implement
    `usecase.RateLimitCache` (2 method: `Get`, `Set`).
  - `IssueListCacheRepository` (`issue_list_cache.go`) — implement
    `usecase.IssueListCache` (2 method: `Get`, `Put`), BR-PI-01's
    5-phút-TTL cache trước `ListIssues`.
  - `OutboxRepository` (`outbox_repository.go`) — implement CẢ
    `usecase.OutboxEnqueuer` (`Enqueue`) LẪN `common/outbox.Store`
    (`FetchUnpublished`/`MarkPublished`) trên cùng 1 struct, giống hệt
    shape `usage-service`'s pilot.
  - `WebhookDeliveryRepository` (`webhook_delivery_repository.go`) —
    implement `usecase.WebhookDeliveryStore` (2 method: `Exists`,
    `Record`), idempotency cho webhook delivery (BUG-PI-03).
  - `internal/adapter/mysql/` mới giữ đúng shape này — 4 file, không gộp
    thành 1 `Repository` struct, để `cmd/server/main.go`'s dialect switch
    có thể gán từng biến (`rateLimitCache`, `issueListCache`, `outboxRepo`,
    `webhookDeliveries`) đối xứng giữa 2 nhánh.
- **6 file migration** (3 cặp up/down): `0001_init` (2 bảng:
  `rate_limit_cache`, `webhook_delivery_log`), `0002_issue_list_cache`,
  `0003_outbox_events` — khớp đúng số liệu ROLLOUT-TRACKING.md.
- **CÓ JSONB**: `issue_list_cache.issues_json` (0002) và
  `outbox_events.payload` (0003) — 2 cột JSONB, khác `usage-service`'s
  pilot vốn chỉ có 1 (`outbox_events.payload`).
- **CÓ RLS**: cả 4 bảng bật `ENABLE ROW LEVEL SECURITY` +
  `CREATE POLICY tenant_isolation` trong migration Postgres gốc — quan
  trọng cho tenant-isolation test ở §6.
- **CÓ `gen_random_uuid()` — VÀ thực sự "chết" theo 2 kiểu khác nhau**:
  - `issue_list_cache.id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
    (0002) — **THỰC SỰ ứng dụng dùng tới**: khác với
    `usage-service`/`annotation-service` (nơi Go luôn generate id trước
    khi INSERT), `IssueListCacheRepository.Put`'s INSERT KHÔNG liệt kê cột
    `id` — Postgres tự sinh qua DEFAULT. MySQL không cho phép `UUID()`
    trong DEFAULT (non-deterministic, bị từ chối) — bản MySQL phải generate
    `id` ở Go (`uuid.NewString()`) ngay trong `Put`, khác quyết định của 2
    service trước (nơi "DEFAULT chết, không mất gì" là kết luận đúng — ở
    đây DEFAULT **sống**, phải thay bằng logic Go tương đương, không phải
    bỏ hẳn). `id` không bao giờ được đọc lại (`Get` không SELECT `id`), nên
    không có rủi ro tương thích ngược nào.
  - `webhook_delivery_log.id UUID PRIMARY KEY` (0001, KHÔNG có DEFAULT) —
    Go đã generate qua `uuid.NewString()` trước khi INSERT
    (`WebhookDeliveryRepository.Record`) — giống pattern "DEFAULT chết"
    thông thường, ở đây thậm chí không có DEFAULT nào để bàn.
- **CÓ `RETURNING`?** KHÔNG — không có method nào trong 4 repository dùng
  `RETURNING`. Mọi UPDATE (`Set` upsert qua `ON CONFLICT DO UPDATE`) không
  cần đọc lại giá trị vừa ghi trong cùng câu lệnh.
- **KHÔNG lưu credential/token nào** — xác nhận qua `ports.go`'s doc
  comment (`Credential` "never persisted... a structural guarantee") và
  qua chính 4 bảng migration: không cột nào chứa token/secret.
  `scm-integration-service` chỉ đọc/ghi credential qua gRPC tới
  `credential-broker-service` (`internal/adapter/credentialbroker`,
  KHÔNG chạm SQL) — mirror đúng phát hiện của `credential-broker-service`'s
  batch-1 rollout (BE-DB-SOL-006 §1: service đó có schema RIÊNG chỉ chứa
  con trỏ Vault, không secret nào; ở đây còn đơn giản hơn — không có bảng
  credential nào cả). Không cần cân nhắc mã hoá/che secret gì khi dịch
  migration hay viết adapter.

## 2. `common/dbcapability` — dùng lại nguyên vẹn

Không sửa `backend-go/common/dbcapability/capability.go`. Không phát hiện
bug nào trong package này khi dùng cho service thứ 5 của rollout (sau
`usage-service`, `issue-status-sync`, `issue-tracking-service`,
`annotation-service`, `credential-broker-service`).

## 3. Adapter MySQL — `internal/adapter/mysql/{rate_limit_cache,issue_list_cache,outbox_repository,webhook_delivery_repository}.go`

4 file, mirror 1:1 tên struct/constructor/method của
`internal/adapter/postgres/`'s 4 file tương ứng — không đổi signature nào
(xem §4 cho impact() xác nhận).

### 3.1 `RateLimitCacheRepository` — không có điểm dịch đặc biệt

`Get`'s freshness window đã tính `cutoff` ở Go (`time.Now().Add(-freshWithin)`)
ngay trong bản Postgres gốc — không có vấn đề interval-literal-string như
lo ngại ban đầu của BE-DB-SOL-002 §3 (mối lo đó không áp dụng ở đây vì
Postgres adapter đã tránh sẵn). `Set` dùng `INSERT ... ON DUPLICATE KEY
UPDATE` thay `ON CONFLICT ... DO UPDATE`, `` `limit` `` backtick-quoted
(MySQL reserved word, tương đương `"limit"` double-quoted ở Postgres).

### 3.2 `IssueListCacheRepository` — id sinh ở Go, KHÔNG mirror "DEFAULT chết"

Như §1 đã nêu, đây là điểm khác biệt thật so với 2 service trước: `Put`
phải tự gọi `uuid.NewString()` cho cột `id` vì Postgres's
`DEFAULT gen_random_uuid()` không dịch được sang MySQL DEFAULT. `Get`
không đổi gì (không SELECT `id`).

### 3.3 `OutboxRepository` — mirror `usage-service`'s pilot 1:1

`FetchUnpublished`/`MarkPublished`/`Enqueue` dịch y hệt
`usage-service/internal/adapter/mysql/repository.go`'s tương ứng (đã có
sẵn, không thiết kế lại) — `id = ANY($1)` → `IN (?,...)` động, guard
`len(ids) == 0`.

### 3.4 `WebhookDeliveryRepository` — `INSERT IGNORE` thay `ON CONFLICT DO NOTHING`

`Record`'s idempotency dựa vào UNIQUE key
`(provider, delivery_id)` — dịch sang `INSERT IGNORE`, cùng pattern
`usage-service.SaveSession`'s idempotent insert đã dùng.

**Không có pitfall RowsAffected-sau-UPDATE (annotation-service's finding)
ở service này** — khác annotation-service (có `UpdateAnnotation` dựa vào
RowsAffected==0 để phát hiện not-found), không method nào trong 4
repository ở đây UPDATE rồi kiểm tra RowsAffected để quyết định
not-found/error; mọi UPDATE ở đây là upsert (`ON DUPLICATE KEY UPDATE`)
hoặc không cần biết đã match hay chưa (`MarkPublished`). Đã rà soát kỹ
từng method, không bỏ sót.

## 4. `impact()` — chạy trước khi sửa mọi symbol hiện có

Repo `orca`, tất cả risk **LOW** trừ 4 interface `ports.go` (xem cảnh báo
dưới):

| Symbol | risk | impactedCount | Ghi chú |
|---|---|---|---|
| `run` (`cmd/server/main.go`) | LOW | 1 | 1 caller (`main`) |
| `Load` (`internal/config/config.go`) | LOW | 2 | |
| `Config` (struct) | LOW | 3 | |
| `RateLimitCache`, `IssueListCache`, `OutboxEnqueuer`, `WebhookDeliveryStore` (4 interface, `ports.go`) | **HIGH** | 19 mỗi cái | Xem cảnh báo dưới |

**Cảnh báo HIGH đã đánh giá, không phải tín hiệu chặn implement**: cả 4
interface báo `impactedCount=19`/`risk=HIGH`, nhưng chi tiết (`byDepth`)
cho thấy toàn bộ 19 "impacted" là quan hệ `IMPORTS` ở **cấp file** — mọi
file trong `internal/adapter/{github,gitlab,bitbucket,...}` import chung
package `usecase` (vì `ports.go` định nghĩa 19 interface trong 1 file), nên
BẤT KỲ symbol nào trong `ports.go` cũng báo fan-out 19 file — đây là hiệu
ứng "file lớn, nhiều interface dùng chung 1 file", không phải dấu hiệu
riêng của 4 interface này rủi ro cao. Không đổi signature interface nào
(4 adapter MySQL implement lại y hệt) — khác với sửa/xoá 1 method trên
interface (trường hợp đó impact 19 file sẽ thực sự đáng lo). Đã confirm
bằng cách đọc `byDepth` thật (không chỉ tin `summaryOnly`) trước khi tiếp
tục — theo đúng yêu cầu "warn nếu HIGH/CRITICAL trước khi sửa".

## 5. Migration `migrations/postgres/` (`git mv`, nội dung y hệt) + `migrations/mysql/` (mới, dialect-safe)

Điểm dịch không tầm thường, khác `usage-service`'s pilot:

- **`CREATE SCHEMA scm`** → bỏ (database `scm` riêng, như mọi service
  khác trong rollout).
- **`UNIQUE` constraint trên cột `TEXT`** — 2 trường hợp, dịch khác nhau
  tuỳ theo TEXT đó có nằm trong 1 UNIQUE constraint hay không:
  - `webhook_delivery_log`'s `UNIQUE(provider, delivery_id)` — `delivery_id`
    là `TEXT` ở Postgres. Prefix index kiểu annotation-service's
    `repo_id(255)` **SAI ở đây**: annotation-service dùng prefix cho 1
    index thường (chỉ tăng tốc WHERE-scan, không cần unique tuyệt đối); ở
    đây UNIQUE constraint backing một idempotency guarantee thật —
    prefix-index unique chỉ đảm bảo 255 byte đầu không trùng, không phải
    toàn bộ giá trị. → đổi hẳn `TEXT` thành `VARCHAR(255)` (bounded, index
    trực tiếp, đúng ngữ nghĩa unique tuyệt đối) thay vì `TEXT` +
    prefix-index.
  - `issue_list_cache`'s `UNIQUE(tenant_id, provider, repo, filter_hash)`
    — cùng lý do, `repo` → `VARCHAR(255)`, `filter_hash` → `CHAR(64)`
    (sha256 hex digest, độ dài cố định — chính xác hơn `VARCHAR`).
- **`gen_random_uuid()` sống** (§1) — `issue_list_cache.id` không có
  DEFAULT ở bản MySQL, generate ở Go (`internal/adapter/mysql/issue_list_cache.go`).
- **`WHERE published_at IS NULL` partial index** (`outbox_events`) — không
  có tương đương MySQL, index đầy đủ `(created_at, published_at)` thay
  thế — cùng dịch `usage-service`'s pilot đã làm.
- **RLS DROP với comment giải thích** — cả 4 bảng có RLS trên Postgres,
  không bảng nào có RLS trên MySQL migration mới — mỗi file để lại đoạn
  SQL RLS gốc dạng comment (không xoá âm thầm), theo đúng tiền lệ
  `annotation-service`/`usage-service`.
- **`payload JSONB` → `JSON`** (`outbox_events`) — không mất chức năng,
  `common/outbox.Relay` chỉ đọc nguyên khối. **`issues_json JSONB` →
  `JSON`** (`issue_list_cache`) — tương tự,
  `IssueListCacheRepository.Get`/`Put` chỉ marshal/unmarshal nguyên khối,
  không dùng toán tử JSONB nào (`->`/`@>`).

## 6. Test — tenant-isolation-without-RLS cho cả 4 bảng (TASK-BE-DB-003 pattern)

Cả 4 bảng có RLS trên Postgres → cả 4 cần test xác nhận tenant-isolation
vẫn đúng dù không còn RLS backstop trên MySQL:

- `RateLimitCacheRepository`: mirror bản Postgres's
  `TestRateLimitCacheRepository_ScopedByTenantAndProvider` — PK
  `(tenant_id, provider, bucket)` tự nhiên scope theo tenant, test xác
  nhận `Get` không đọc nhầm row tenant khác.
- `IssueListCacheRepository`: `TestIssueListCacheRepository_DoesNotLeakAcrossTenants`
  (mới) — 2 tenant cache CÙNG `(provider, repo, filter)`, xác nhận `Get`
  của tenant này không trả issues của tenant kia.
- `WebhookDeliveryRepository`: `TestWebhookDeliveryRepository_ScopedByProviderNotJustDeliveryID`
  — UNIQUE key thật là `(provider, delivery_id)`, không phải chỉ
  `delivery_id`; xác nhận 2 provider khác nhau với CÙNG `delivery_id`
  không bị coi là trùng.
- `OutboxRepository`: `TestOutboxRepository_FetchUnpublished_ScopedAcrossTenants`
  — outbox không tenant-scope theo thiết kế (relay publish xuyên tenant),
  nên test ở đây xác nhận mỗi row giữ đúng `tenant_id` của chính nó qua
  round-trip, không phải "không đọc được row tenant khác" (khác 3 bảng
  trên) — ghi rõ trong doc comment của test để không gây hiểu lầm.

Xem "Kết quả thực tế" ở [TASK-BE-DB-012](../tasks/TASK-BE-DB-012-scm-integration-service-mysql-rollout.md)
cho output test thật.

## 7. Wiring `cmd/server/main.go`

Khác `usage-service`'s pilot (1 biến `repo usecase.Repository` +
1 biến `outboxStore outbox.Store` riêng): ở đây có 4 biến port
(`rateLimitCache`, `issueListCache`, `outboxRepo`, `webhookDeliveries`),
mỗi biến gán trong cả 2 nhánh `switch caps.Dialect`. `outboxRepo` cần 1
interface cục bộ mới (`outboxStore`, định nghĩa ngay trong `main.go`) hợp
nhất `usecase.OutboxEnqueuer` + `common/outbox.Store` — vì
`OutboxRepository` (cả 2 dialect) implement cả 2 interface trên cùng 1
struct nhưng `usecase`/`common/outbox` không có sẵn 1 interface gộp nào
để khai biến. `healthSrv.Register` chuyển từ cố định "postgres" sang theo
nhánh dialect, giống `usage-service`. `toMySQLDriverDSN` copy nguyên vẹn
từ `usage-service` (thuần DSN-plumbing, không đặc thù service nào).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `issue_list_cache.id`'s DEFAULT sống thật (khác pilot) | Trung bình | Đã xử lý bằng Go-side `uuid.NewString()` trong `Put` — xem §1/§3.2 |
| `webhook_delivery_log`/`issue_list_cache`'s TEXT-trong-UNIQUE | Trung bình | Prefix-index annotation-service's precedent KHÔNG áp dụng được (sai ngữ nghĩa unique) — dùng `VARCHAR`/`CHAR` bounded thay vì `TEXT`+prefix, xem §5 |
| `impact()` báo HIGH cho 4 interface `ports.go` | Thấp (đã đánh giá) | File-level fan-out do `ports.go` chứa 19 interface dùng chung, không phải tín hiệu riêng của 4 interface này — xem §4 |
| Driver TiDB thật chưa test | Trung bình | Kế thừa nguyên rủi ro đã ghi ở BE-DB-SOL-002 §Rủi ro — chưa service nào trong rollout chạy thật trên `pingcap/tidb` |

## Không thuộc phạm vi solution này

- 3 service khác trong batch 2 (`notification-service`,
  `ai-provider-service`, `orchestration-service`) — agent khác, song song,
  không đụng tới.
- Provider adapter (`internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea}`)
  — không chạm SQL, ngoài phạm vi rollout database này hoàn toàn.
- `credential-broker-service` — đã rollout ở batch 1 (BE-DB-SOL-006),
  không đụng lại.

## Liên quan

- `backend-go/services/scm-integration-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/common/outbox/*.go` (`Store` interface, `Record`)
- `backend-go/common/secrets/*.go` (`DatabaseCredentialsFromFile` — không đổi)
- [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RLS-drop + UUID handling precedent gần nhất)
