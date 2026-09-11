# BE-DB-SOL-004: Adapter MySQL/TiDB cho `issue-tracking-service`

> **✅ Implemented (2026-09-11).** Áp dụng lại nguyên vẹn pattern
> BE-DB-SOL-001/002 (`usage-service` pilot) cho `issue-tracking-service` —
> không thiết kế lại.

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/issue-tracking-service`
**Depends on:** `common/dbcapability` (BE-DB-SOL-001, dùng lại nguyên vẹn), `common/testutil.StartMySQL` (dùng lại nguyên vẹn)

---

## 1. Khác biệt với pilot — 2 repository thay vì 1

`issue-tracking-service` sở hữu **2 repository** trên cùng 1 struct
`Repository` (`internal/adapter/postgres/repository.go` +
`internal/adapter/postgres/connections.go`, cùng struct, 2 file), khớp 2
interface trong `internal/usecase/ports.go`:

- `usecase.OutboxEnqueuer` (`Enqueue`) + `common/outbox.Store`
  (`FetchUnpublished`/`MarkPublished`) — outbox transactional cho
  `LinkIssue` (Epic G).
- `usecase.ConnectionRepository` (`Upsert`, `Delete`, `GetStatus`,
  `SelectWorkspace`, `GetCredentialID`) — persistence cho
  Connect/Disconnect/SelectWorkspace/GetConnectionStatus, thêm sau Epic G
  (migration `0002_connections`).

`internal/adapter/mysql/` mới cũng implement CẢ 2 interface trên 1 struct
`Repository` duy nhất, chia thành `repository.go` (outbox) +
`connections.go` (connections) — khớp đúng cách chia file bản Postgres đã
dùng.

## 2. `impact()` đã chạy thật trước khi sửa symbol hiện có

- `impact({target:"ConnectionRepository", direction:"upstream", file_path:
  "...internal/usecase/ports.go", kind:"Interface"})` → **MEDIUM**,
  impactedCount 7 — toàn bộ depth=1 là quan hệ `IMPORTS` cấp file (mọi file
  trong `internal/adapter/*` import chung package `usecase` vì chúng cũng
  dùng các port khác, ví dụ `jira/client.go` import `usecase` chỉ để implement
  `IssueTrackerProvider`, không liên quan `ConnectionRepository`) — không
  phải 7 lời gọi thực sự phụ thuộc chữ ký `ConnectionRepository`. Không đổi
  chữ ký interface (chỉ thêm 1 implementation song song) nên an toàn tiến
  hành, nhưng ghi nhận rủi ro MEDIUM ở đây theo đúng yêu cầu AGENTS.md (không
  bỏ qua).
- `impact({target:"OutboxEnqueuer", ...})` → cùng kết quả MEDIUM/7, cùng lý
  do (file-level IMPORTS, không phải chữ ký thay đổi).
- `impact({target:"New", direction:"upstream", file_path:
  "...internal/adapter/postgres/repository.go"})` → **LOW**, impactedCount 2
  (`run` → `main`), 1 module `Usecase` ảnh hưởng trực tiếp.
- `impact({target:"run", direction:"upstream", file_path:
  "...cmd/server/main.go"})` → **LOW**, impactedCount 1 (`main`).

Không HIGH/CRITICAL nào ở cả 4 lần chạy — an toàn để thêm adapter mới +
sửa `main.go`'s `run` mà không đổi interface nào.

## 3. Migration dialect-safe

`migrations/` (2 file gốc: `0001_outbox`, `0002_connections`) tách thành
`migrations/postgres/` (di chuyển nguyên văn bằng `git mv`, không sửa nội
dung) + `migrations/mysql/` (mới):

- `0001_outbox`: bỏ `CREATE SCHEMA issuetracking`; `UUID` → `CHAR(36)`;
  `TIMESTAMPTZ` → `TIMESTAMP(6)`; `JSONB` → `JSON`; partial index
  (`WHERE published_at IS NULL`, MySQL không hỗ trợ) → composite index
  thường `(created_at, published_at)`; bỏ khối RLS (không tương đương).
- `0002_connections`: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` —
  xác nhận qua đọc `internal/adapter/postgres/connections.go` **không có
  code Go nào set/đọc cột `id`** (INSERT column list bỏ qua `id`, không
  query nào SELECT `id`), nên không phải trường hợp "ID sinh ở Go" như
  ghi chú CR gốc dự đoán — MySQL dùng `CHAR(36) PRIMARY KEY DEFAULT (UUID())`
  (MySQL 8.0.13+ hỗ trợ expression default, image test/prod `mysql:8` đủ
  mới). 4 cột `TEXT` nằm trong `UNIQUE KEY` composite
  (`tenant_id, user_id, provider, external_workspace_id`) phải đổi sang
  `VARCHAR` có độ dài cụ thể — InnoDB từ chối `TEXT`/`BLOB` trong khoá
  không có prefix length; độ dài chọn (64+64+16+255 ký tự × 4
  byte/ký tự utf8mb4 = 1596 byte) giữ dưới giới hạn 3072 byte của InnoDB.
  `credential_id UUID NOT NULL` → `CHAR(36) NOT NULL`. Bỏ khối RLS.

## 4. `internal/adapter/mysql/repository.go` + `connections.go`

Dịch 1:1 theo BE-DB-SOL-002 §3's mapping đã chốt (`$N`→`?`,
`errors.Is(pgx.ErrNoRows)`→`errors.Is(sql.ErrNoRows)`, `id = ANY($1)`→
`IN (?,...)` động cho `MarkPublished`). Một điểm **khác pilot, phát sinh
thật khi implement `Upsert`**: bản Postgres dùng biểu thức
`NOT EXISTS (SELECT 1 FROM issuetracking.connections WHERE ...)` ngay
trong mệnh đề `VALUES` của `INSERT ... ON CONFLICT` để tính `is_selected`
cho lần connect đầu tiên. MySQL's `INSERT ... ON DUPLICATE KEY UPDATE`
không có tương đương an toàn/ổn định cho subquery-trong-VALUES đọc lại
chính bảng đang insert (ngữ nghĩa version-fragile, không đáng tin cậy trên
cả MySQL lẫn TiDB) — thay vào đó, adapter MySQL tính `isFirstConnection`
bằng 1 `SELECT COUNT(*)` riêng trong CÙNG transaction trước khi `INSERT`,
rồi truyền kết quả như tham số thường. Cùng hiệu ứng ròng (lần connect đầu
cho `(tenant,user,provider)` tự động `is_selected=true`; workspace bổ sung
sau đó mặc định `false`), chỉ khác chỗ tính toán (SQL → Go+SQL 2 bước
trong 1 tx).

## 5. Wiring `cmd/server/main.go`

Đúng khuôn `usage-service/cmd/server/main.go`'s `run()`: `caps, err :=
dbcapability.DetectDialectFromDSN(dsn)`, `switch caps.Dialect` chọn
`pgxpool.New`/`issuetrackingpostgres.New` hoặc
`sql.Open("mysql", toMySQLDriverDSN(dsn))`/`issuetrackingmysql.New`, gán cả
3 biến interface (`repo usecase.OutboxEnqueuer`, `connectionRepo
usecase.ConnectionRepository`, `outboxStore outbox.Store`) từ CÙNG 1 giá
trị Repository cụ thể mỗi nhánh (postgres hoặc mysql) — khớp đúng cách
`usage-service`'s `run()` gán `repo, outboxStore` từ cùng 1 `pgRepo`/
`myRepo`. `toMySQLDriverDSN` copy nguyên văn từ `usage-service` (không có
package chung cho hàm này — mỗi `cmd/main` tự giữ bản riêng, đúng tiền lệ
`issue-status-sync`'s TASK-BE-DB-008 đã ghi nhận cùng lý do). Không thêm
biến env mới — vẫn `DATABASE_DSN`/`DATABASE_CREDENTIALS_FILE` sẵn có.
`healthSrv.Register(string(caps.Dialect), ...)` thay hard-code `"postgres"`.

## 6. Test tenant-isolation-without-RLS

`migrations/postgres/0002_connections.up.sql` VÀ `0001_outbox.up.sql` đều
có RLS policy (`ENABLE ROW LEVEL SECURITY` + `CREATE POLICY tenant_isolation`)
— khác `issue-status-sync` (không có RLS, xem BE-DB-SOL-003 §4) —
nên áp dụng TASK-BE-DB-003's pattern là đúng, không phải bỏ qua.
`outbox_events`'s `FetchUnpublished` không filter theo tenant (đúng thiết
kế — outbox relay đọc across-tenant rồi publish sự kiện đã mang sẵn
`tenant_id`, giống hệt `usage-service`), nên phạm vi cần test tenant-
isolation thật sự chỉ nằm ở `connections` table's read path:
`GetStatus`/`GetCredentialID`. 2 test mới
(`TestConnectionsRepository_GetStatus_DoesNotLeakAcrossTenants`,
`TestConnectionsRepository_GetCredentialID_DoesNotLeakAcrossTenants`) —
2 tenant connect CÙNG provider + CÙNG `external_workspace_id`, xác nhận
đọc theo tenant A không bao giờ thấy dữ liệu tenant B dù workspace-id trùng
hệt nhau.

## 7. Phát hiện phụ — bug tiền tồn tại ở `internal/adapter/postgres`, KHÔNG sửa (ngoài phạm vi)

Chạy `go test -tags=integration ./internal/adapter/postgres/... -v` (để
xác nhận việc tách `migrations/postgres/` không phá vỡ test hiện có) lộ ra
**4/5 test FAIL** — không liên quan tới việc tách thư mục migration (nội
dung SQL không đổi, chỉ đổi vị trí file bằng `git mv`):

- `credential_id UUID NOT NULL` (migration) nhưng
  `connections_test.go` dùng literal `"cred-1"`/`"cred-2"` (không phải
  UUID hợp lệ) → `ERROR: invalid input syntax for type uuid: "cred-1"
  (SQLSTATE 22P02)`.
- `tenant_id UUID NOT NULL` (migration `0001_outbox`) nhưng
  `repository_test.go` dùng literal `"tenant-1"` → cùng lỗi SQLSTATE 22P02.

Đây là bug tiền tồn tại trong code sản xuất (`internal/adapter/postgres`)
+ test fixture của chính CHÚNG (không phải migration MySQL mới hay adapter
MySQL mới của solution này) — xác nhận bằng cách đọc lại nội dung file
trước khi `git mv` (giống hệt sau khi di chuyển, không có sửa đổi nào).
Cùng dạng phát hiện như TASK-BE-DB-003/007 đã gặp ở `usage-service`'s
Postgres integration test (`ListSessions`'s `$2` type ambiguity) — tests
`internal/adapter/postgres` của `issue-tracking-service` dường như CHƯA
BAO GIỜ được chạy thật với testcontainers trước phiên này. **Không sửa ở
đây** — ngoài phạm vi TASK-BE-DB-009 (rollout MySQL, không phải fix bug
Postgres tiền tồn tại không liên quan tới multi-dialect), đúng nguyên tắc
"mỗi agent chỉ sửa 1 service, không mở rộng phạm vi" — ghi nhận rõ cho
người sở hữu `issue-tracking-service`'s Postgres adapter xử lý riêng. Hệ
quả trực tiếp: lane `postgres` của `.github/workflows/backend-go-issue-
tracking-service.yml` mới (giống hệt tình trạng ban đầu của
`backend-go-usage-service.yml` ở TASK-BE-DB-007) sẽ ĐỎ khi PR mở thật —
lane `mysql` thì xanh (đã xác nhận PASS 8/8 cục bộ, xem TASK-BE-DB-009's
"Kết quả thực tế").

## Không thuộc phạm vi solution này

- Sửa bug UUID-column-vs-non-UUID-literal ở `internal/adapter/postgres`
  (§7) — ghi nhận, không sửa.
- 13 service còn lại trong rollout — xem
  `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md`.

## Liên quan

- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md),
  [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go`
- `backend-go/services/issue-tracking-service/internal/adapter/postgres/{repository,connections}.go`
- [TASK-BE-DB-009](../tasks/TASK-BE-DB-009-issue-tracking-service-mysql-rollout.md)
