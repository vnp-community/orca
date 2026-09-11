# BE-DB-SOL-012: Dialect capability layer + MySQL/TiDB adapter cho `automation-service`

> **✅ Implemented (2026-09-11).** Áp dụng nguyên vẹn pattern đã xác lập ở
> BE-DB-SOL-001/002 (`usage-service`, pilot) — không thiết kế lại, nhưng
> `automation-service` có 1 điểm khác biệt cấu trúc buộc phải giải quyết
> riêng: partial unique index bảo vệ BR-AT-08 ("chỉ 1 run đang chạy mỗi
> automation") — xem §3 dưới đây.

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/automation-service`
**Depends on:** `backend-go/common/dbcapability` (đã có sẵn từ pilot, dùng
lại nguyên vẹn, không sửa), `backend-go/common/testutil.StartMySQL` (đã có
sẵn, dùng lại)
**Task tương ứng:** [TASK-BE-DB-017](../tasks/TASK-BE-DB-017-automation-service-mysql-rollout.md)

---

## 1. Audit thật trước khi implement (đã Read đầy đủ, không suy đoán)

`automation-service` audit trực tiếp (khớp số liệu ROLLOUT-TRACKING.md: 1
repository, 16 migration):

- **1 "repository"** thực ra là 2 struct chia sẻ 1 `*pgxpool.Pool`/`*sql.DB`
  trong `internal/adapter/postgres/repository.go`: `AutomationRepository`
  (implement `usecase.AutomationRepository` VÀ `usecase.DueAutomationClaimer`
  — `ClaimDue`'s `SELECT ... FOR UPDATE SKIP LOCKED`) và
  `AutomationRunRepository` (implement `usecase.AutomationRunRepository` VÀ
  `common/outbox.Store`).
- **16 file migration** (8 cặp up/down): `0001_init`, `0002_scheduler_columns`,
  `0003_action_chain`, `0004_one_running_run`, `0005_trigger_columns`,
  `0006_worktree_cleanup_log`, `0007_outbox`, `0008_project_and_actions`.
- **CÓ JSONB**: `automations.actions_json`/`trigger_filter_json`,
  `automation_runs.action_results_json`, `outbox_events.payload` — 4 cột,
  nhiều hơn hẳn `usage-service`/`annotation-service` (0-1 cột).
- **CÓ RLS**: 3 policy (`automations`, `automation_runs`,
  `worktree_cleanup_log`) — cùng kết luận đã lặp lại ở mọi service trước:
  chưa từng active thật (không `SET LOCAL app.tenant_id`, không
  `FORCE ROW LEVEL SECURITY`).
- **CÓ `gen_random_uuid()`** (default trên `automations.id`,
  `automation_runs.id`, `worktree_cleanup_log.id`) nhưng **không được dùng**:
  `internal/usecase/create_automation.go:112` và `run_now.go:135` luôn gọi
  `uuid.NewString()` ở Go trước khi insert — giống hệt phát hiện lặp lại ở
  mọi service trước trong rollout này.
- **KHÔNG dùng `RETURNING`** ở bất kỳ đâu trong
  `internal/adapter/postgres/repository.go` — khác `annotation-service`
  (có 2 chỗ dùng `RETURNING`, cần dịch "UPDATE rồi SELECT lại"). Điều này
  giúp `automation-service`'s migration dịch đơn giản hơn ở điểm này,
  nhưng lại phức tạp hơn hẳn ở điểm khác (xem §3).
- **CÓ 1 partial unique index LOAD-BEARING**:
  `idx_automation_runs_one_running` (`WHERE status = 'running'`,
  migration `0004`) — enforce BR-AT-08 ("tối đa 1 run đang `running` mỗi
  automation"). Khác hẳn 3 partial index còn lại
  (`idx_automations_due`/`idx_automations_trigger`/
  `idx_automation_outbox_events_unpublished`, chỉ là tối ưu hiệu năng —
  mất partiality ở MySQL chỉ làm index to hơn, không đổi kết quả query),
  index này **không thể** dịch thành "full unique index" ở MySQL — sẽ giới
  hạn còn ĐÚNG 1 row (bất kể status) cho mỗi `automation_id`, phá vỡ hoàn
  toàn lịch sử run. Đây là điểm khác biệt cấu trúc lớn nhất so với mọi
  service trước trong rollout — xem §3.

## 2. `common/dbcapability` — dùng lại nguyên vẹn

Không sửa `backend-go/common/dbcapability/capability.go`. Không phát hiện
bug nào khi dùng cho service thứ 6 trong rollout này (sau usage-service,
issue-status-sync, issue-tracking-service, annotation-service,
credential-broker-service).

## 3. Adapter MySQL — `internal/adapter/mysql/repository.go`

Dịch từng method sang `database/sql`/MySQL, cùng khuôn BE-DB-SOL-002. Các
điểm khác biệt quan trọng của service này:

### 3.1. `idx_automation_runs_one_running` — partial unique index LOAD-BEARING, không chỉ hiệu năng

MySQL/InnoDB không có cú pháp partial index (không có `CREATE UNIQUE INDEX
... WHERE ...`). Giải pháp chuẩn của MySQL cho "unique chỉ trong tập con
hàng" là 1 cột "shadow" — `NULL` ở mọi hàng KHÔNG thuộc tập con cần ràng
buộc, giá trị cần unique ở những hàng thuộc tập con — rồi `UNIQUE INDEX`
trên cột đó. MySQL's UNIQUE INDEX coi `NULL` là "không bằng bất kỳ NULL nào
khác" (khác Postgres nhưng đúng ngữ nghĩa cần).

**Thử đầu tiên (SAI, đã phát hiện qua chạy thật)**: dùng
`GENERATED ALWAYS AS (...) STORED` để cột tự tính, giống pattern SQL thuần
tuý hay dùng:

```sql
-- KHÔNG dùng — lỗi thật khi chạy trên mysql:8:
ALTER TABLE automation_runs
    ADD COLUMN running_slot CHAR(36)
        GENERATED ALWAYS AS (CASE WHEN status = 'running' THEN automation_id ELSE NULL END) STORED;
-- ERROR 1215 (HY000): Cannot add foreign key constraint
```

Xác nhận qua chạy thật (không suy đoán từ tài liệu): InnoDB (MySQL 8.4,
image `mysql:8` dùng cho test) từ chối `ADD COLUMN ... GENERATED ... STORED`
khi biểu thức generated tham chiếu 1 cột (`automation_id`) đang mang
`FOREIGN KEY` (`fk_automation_runs_automation` → `automations.id`) — kể cả
khi DROP FK trước, ADD COLUMN generated, rồi RE-ADD FK sau đều thất bại ở
bước re-add với cùng lỗi 1215. Đây không phải lỗi cú pháp của migration,
mà là giới hạn thật của InnoDB engine.

**Giải pháp thật (đã chạy PASS)**: `running_slot` là cột **THƯỜNG** (không
generated), do **application layer** (`internal/adapter/mysql.
AutomationRunRepository.UpdateStatus`) gán giá trị tường minh trong cùng
câu `UPDATE` đã ghi `status`:

```sql
-- migrations/mysql/0004_one_running_run.up.sql
ALTER TABLE automation_runs
    ADD COLUMN running_slot CHAR(36) NULL;

CREATE UNIQUE INDEX idx_automation_runs_one_running ON automation_runs (running_slot);
```

```go
// internal/adapter/mysql/repository.go — AutomationRunRepository.UpdateStatus
var runningSlot any
if run.Status == domain.RunStatusRunning {
    runningSlot = run.AutomationID
}
// ... UPDATE automation_runs SET status = ?, ..., running_slot = ? WHERE id = ? AND tenant_id = ?
```

Kết quả: bất kỳ số lượng hàng non-running nào cũng tồn tại song song (đều
`running_slot = NULL`), nhưng 2 hàng `status='running'` cùng
`automation_id` sẽ đụng độ trên `running_slot` — đúng ngữ nghĩa Postgres's
`CREATE UNIQUE INDEX ... WHERE status = 'running'`, không còn phụ thuộc
tính năng generated column (và giới hạn FK đi kèm). Đánh đổi duy nhất: nếu
tương lai có thêm 1 method khác tự ý UPDATE `status` mà quên đồng bộ
`running_slot`, bất biến sẽ bị phá — chấp nhận được vì `UpdateStatus` là
API DUY NHẤT ghi `status` trong toàn bộ codebase (xác nhận qua
`impact()`/đọc `ports.go`: không method nào khác của
`AutomationRunRepository` ghi `status`). Xác nhận PASS thật qua
`TestAutomationRunRepository_OneRunningPartialUniqueIndex` (mysql variant,
§Kết quả thực tế).

### 3.2. `isDuplicateKeyError` — dịch phát hiện unique-violation sang MySQL

`internal/adapter/postgres.isUniqueViolation` dùng `pgconn.PgError.Code ==
"23505"` + `ConstraintName` (field riêng). `go-sql-driver/mysql`'s
`*mysql.MySQLError` chỉ có `Number == 1062` + `Message` dạng free-text
("Duplicate entry '...' for key '<index>'") — `isDuplicateKeyError` dùng
`strings.Contains(myErr.Message, indexName)` thay vì so sánh bằng nhau. Map
sang cùng sentinel `usecase.ErrConcurrentRunActive` — usecase layer
(`run_now.go`) không cần biết dialect nào đang chạy.

### 3.3. `AutomationRepository.Update`/`AutomationRunRepository.UpdateStatus` — CÙNG cạm bẫy RowsAffected của BE-DB-SOL-005, giải pháp KHÁC

`Update`/`UpdateStatus` (cả 2, không chỉ 1 như `annotation-service`) đều
kiểm tra `RowsAffected() == 0` để quyết định "not found" — y hệt cạm bẫy
BE-DB-SOL-005 §3.1 đã phát hiện (driver mặc định đếm "dòng ĐỔI giá trị",
không phải "dòng khớp WHERE"). `annotation-service` chọn "UPDATE rồi SELECT
lại". **Service này chọn giải pháp khác, ở tầng driver**: thêm
`clientFoundRows=true` vào chuỗi DSN của `database/sql.Open("mysql", ...)`
(`cmd/server/main.go`'s `toMySQLDriverDSN`) — set cờ `CLIENT_FOUND_ROWS`
của giao thức MySQL, khôi phục đúng ngữ nghĩa "đếm dòng khớp WHERE" của
Postgres cho **toàn bộ connection**, không cần sửa từng method riêng lẻ.
Lý do chọn khác `annotation-service`: `automation-service` có 2 method
cùng cạm bẫy (`Update` VÀ `UpdateStatus`) thay vì 1, nên 1 fix ở tầng driver
áp dụng tự động cho cả 2 (và mọi UPDATE tương lai) rẻ hơn viết SELECT-lại 2
lần. Xác nhận PASS thật qua
`TestAutomationRepository_Update_NoopRetryStillSucceeds` (retry với cùng
giá trị hệt như trước, không bị báo nhầm not-found).

Rủi ro đã cân nhắc: `clientFoundRows=true` chỉ đổi ngữ nghĩa `RowsAffected()`
của UPDATE (không đổi INSERT/DELETE) — `AcquireRunLock` (UPDATE có
điều kiện, `SET running_since = NOW(6)` luôn đổi giá trị thật do timestamp
luôn tiến) và `Delete` (không phụ thuộc RowsAffected theo "đổi giá trị")
không bị ảnh hưởng bởi cờ này — đã verify qua test suite đầy đủ, không có
test nào failed vì thay đổi ngữ nghĩa này.

### 3.4. `PruneOldRuns`/`PruneRuns` — MySQL cấm SELECT trực tiếp từ bảng đang DELETE

`DELETE FROM automation_runs WHERE ... id NOT IN (SELECT id FROM
automation_runs WHERE ...)` hợp lệ ở Postgres nhưng MySQL từ chối với lỗi
"You can't specify target table 'automation_runs' for update in FROM
clause". Giải pháp chuẩn MySQL: bọc subquery trong 1 derived table thêm
(`SELECT id FROM (SELECT id FROM automation_runs WHERE ...) AS keep`) —
buộc MySQL materialize kết quả trước, phá vỡ tham chiếu trực tiếp tới bảng
đích. Áp dụng cho cả `PruneOldRuns` (deprecated nhưng vẫn giữ, theo
`ports.go`'s doc comment) và `PruneRuns`.

### 3.5. `UpdateStatus`'s outbox write — không dùng lại được `eventbus.RunCompletedPublisher`

`internal/adapter/eventbus.RunCompletedPublisher.PublishRunCompleted(ctx,
tx pgx.Tx, run)` nhận thẳng `pgx.Tx` — Postgres-specific, adapter MySQL's
`*sql.Tx` không thể truyền vào. Đây là 1 coupling dialect có thật, tiền tồn
tại trong `automation-service`'s OWN `eventbus` package (không phải
`common/`), nhưng **flag không fix** — sửa `PublishRunCompleted` để nhận 1
interface trừu tượng hơn cần thiết kế lại chữ ký (khác placeholder `$N`
vs `?`, không chỉ khác kiểu transaction), ngoài phạm vi rollout MySQL của
CR này. Giải pháp: `internal/adapter/mysql.AutomationRunRepository.UpdateStatus`
tự viết INSERT outbox row trực tiếp trong `*sql.Tx` của chính nó, dùng lại
`eventbus.RunCompletedSubject` (hằng số export, an toàn import) và 1 struct
`runCompletedPayload` cục bộ khớp CHÍNH XÁC field JSON
(`automation_id`/`run_id`/`status`) với package `eventbus`'s bản
unexported — đảm bảo consumer (`notification-service`) nhận event giống hệt
bất kể dialect nào ghi ra.

### 3.6. `AcquireRunLock`'s TTL — `make_interval` → `DATE_SUB(... INTERVAL ? MICROSECOND)`

Postgres: `running_since < now() - make_interval(secs => $4)` (nhận
`ttl.Seconds()` dạng float, không mất độ chính xác). MySQL: dùng
`DATE_SUB(NOW(6), INTERVAL ? MICROSECOND)` với `ttl.Microseconds()` (int64)
— chọn đơn vị MICROSECOND thay vì SECOND để tránh nhị nghĩa khi bind 1 giá
trị không nguyên (MySQL's `INTERVAL ? SECOND` với tham số thập phân có hành
vi làm tròn không tài liệu hoá rõ ràng; MICROSECOND luôn nhận số nguyên,
loại bỏ hoàn toàn nhập nhằng).

## 4. Compensating control cho tenant isolation khi RLS vắng mặt

Mọi query trong `internal/adapter/postgres/repository.go` đã filter tường
minh theo `tenant_id` — giống mọi service trước, RLS chưa từng active thật.
Test `TestAutomationRepository_List_ScopesToTenant` (mirror TASK-BE-DB-003's
pattern) xác nhận `List` không rò rỉ chéo tenant trên MySQL.

## 5. Migration dialect-safe — điểm khác biệt so với các service trước

- **4 cột JSONB → JSON** (nhiều nhất trong rollout tới nay):
  `automations.actions_json`/`trigger_filter_json`,
  `automation_runs.action_results_json`, `outbox_events.payload`.
  `actions_json`/`action_results_json` cần `DEFAULT (JSON_ARRAY())` —
  MySQL 8.0.13+ chấp nhận function-expression default trong ngoặc đơn,
  xác nhận chạy thật không lỗi trên `mysql:8` (§Kết quả thực tế).
- **3 partial index → full index** (chỉ ảnh hưởng hiệu năng):
  `idx_automations_due` (`WHERE enabled`), `idx_automations_trigger`
  (`WHERE trigger_type = 'event'`), `idx_automation_outbox_events_unpublished`
  (`WHERE published_at IS NULL`).
- **1 partial index → shadow-column unique index, application-maintained**
  (ảnh hưởng đúng-sai, không chỉ hiệu năng): `idx_automation_runs_one_running`
  — xem §3.1 (bao gồm lý do KHÔNG dùng generated column: giới hạn InnoDB
  thật, "Error 1215").
- **`request_id` (automation_runs) bound thành `VARCHAR(255)`** (Postgres:
  `TEXT` không giới hạn) — cần cho `UNIQUE KEY uniq_tenant_request` mà
  không phải dùng prefix-index (như `annotation-service`'s `file_path`/
  `repo_id`) — khớp `usage-service`'s tiền lệ `VARCHAR(128)` cho cùng vai
  trò "caller-supplied idempotency key".
- **2 Foreign Key** (`automation_runs.automation_id` →
  `automations.id ON DELETE CASCADE`, `worktree_cleanup_log.run_id` →
  `automation_runs.id ON DELETE CASCADE`) — Postgres dùng `REFERENCES`
  inline trong `CREATE TABLE`, MySQL cần `CONSTRAINT ... FOREIGN KEY`
  tường minh (cú pháp khác nhưng cùng hành vi CASCADE — xác nhận qua
  `TestAutomationRepository_Delete_CascadesToRunsAndFailsForWrongTenant`,
  PASS thật cả 2 dialect).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `run`/`Config`/`Load`/`AutomationRepository`/`AutomationRunRepository`/`DueAutomationClaimer` | Đã chạy — LOW cho `run`/`Load`/`Config` (impactedCount 1/2/3); **MEDIUM** cho 3 interface (`ports.go`, impactedCount 7 mỗi cái — phản ánh số usecase tiêu thụ interface, KHÔNG phải rủi ro đổi signature, vì signature giữ nguyên 100%) | Xem TASK-BE-DB-017 mục 1 |
| `idx_automation_runs_one_running`'s shadow-column emulation dịch sai | Đã tránh — test riêng xác nhận cả 2 nhánh (đụng độ khi cùng running + giải phóng khi terminal); thử đầu tiên dùng GENERATED column thất bại thật (Error 1215), đã đổi sang cột application-maintained | Xem §3.1 |
| `running_slot` chỉ đồng bộ đúng nếu MỌI nơi ghi `status` đều đi qua `UpdateStatus` | Thấp — xác nhận `UpdateStatus` là API duy nhất ghi `status` trong `AutomationRunRepository` (đọc `ports.go` đầy đủ) | Xem §3.1 |
| `clientFoundRows=true` đổi ngữ nghĩa RowsAffected() ngoài dự kiến cho method khác | Thấp — đã audit toàn bộ UPDATE trong adapter, chỉ `Update`/`UpdateStatus` phụ thuộc "đếm dòng khớp", `AcquireRunLock`/`Delete` không bị ảnh hưởng | Xem §3.3 |
| `eventbus.RunCompletedPublisher`'s `pgx.Tx`-typed signature (coupling dialect tiền tồn tại) | Thấp, flag không fix | Xem §3.5 — ngoài phạm vi rollout MySQL |
| Driver TiDB thật chưa test | Trung bình | Kế thừa nguyên trạng thái đã ghi ở BE-DB-SOL-002 — chưa có service nào trong rollout 15-service này chạy thật trên `pingcap/tidb` |

## Không thuộc phạm vi solution này

- 14 service còn lại của rollout 15-service (ROLLOUT-TRACKING.md) — nhân
  rộng, ngoài phạm vi.
- Sửa `internal/adapter/eventbus.RunCompletedPublisher`'s `pgx.Tx` coupling
  — flagged ở §3.5, không fix (cần thiết kế lại chữ ký, ngoài phạm vi CR-DB).
- Data migration tool Postgres→MySQL cho khách hàng thật — theo loại trừ
  chung của CR-DB-003.

## Liên quan

- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RowsAffected pitfall gốc, giải pháp khác ở §3.3)
- `backend-go/services/automation-service/internal/usecase/ports.go`
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go`
- `backend-go/services/automation-service/internal/adapter/mysql/repository.go` (mới)
- [TASK-BE-DB-017](../tasks/TASK-BE-DB-017-automation-service-mysql-rollout.md)
