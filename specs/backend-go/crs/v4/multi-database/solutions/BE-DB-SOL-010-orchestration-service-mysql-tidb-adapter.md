# BE-DB-SOL-010: Adapter MySQL/TiDB thật cho `orchestration-service`

**CR:** CR-DB-002, CR-DB-003
**Service:** `orchestration-service` (batch 2 của rollout 15 service —
xem [ROLLOUT-TRACKING.md](../tasks/ROLLOUT-TRACKING.md))
**Depends on:** `common/dbcapability` (BE-DB-SOL-001, dùng lại nguyên vẹn),
pattern BE-DB-SOL-002 (`usage-service` pilot)

---

## 1. Khác biệt so với pilot — quan trọng

`orchestration-service` khác `usage-service`/batch-1 ở 2 điểm:

1. **4 repository interface riêng biệt** trong `internal/usecase/ports.go`
   (`OrchestrationTaskRepository`, `DispatchContextRepository`,
   `GateRepository`, `CoordinatorRunRepository`), không phải 1 interface
   `Repository` gộp như pilot — cả 4 đều được implement trên CÙNG 1 struct
   `Repository` (`internal/adapter/postgres` lẫn `internal/adapter/mysql`),
   nhưng `cmd/server/main.go` cần 1 kiểu interface cục bộ gộp cả 4 +
   `outbox.Store` để giữ 1 biến `repo` duy nhất qua dialect switch (xem §4).
2. **6 migration, 5 bảng** (`coordinator_runs`, `orchestration_tasks`,
   `dispatch_contexts`, `decision_gates`, `messages`) + outbox — nhiều hơn
   hẳn pilot's 2 bảng, và có bảng dùng `BIGSERIAL` (`messages.sequence`,
   không phải UUID) và cột tên trùng từ khoá dành riêng của MySQL (`read`).

Audit trực tiếp xác nhận (đọc `internal/adapter/postgres/repository.go`
đầy đủ, không suy đoán): **không có `gen_random_uuid()` nào được đọc lại
bởi Go code** — mọi INSERT (`Create`, `CreateDispatchContext`, `CreateGate`,
`CreateWithTasks`) đều mint id bằng `uuid.NewString()` trước và luôn liệt
kê `id` trong column list. Kết luận giống `usage-service`: cột MySQL chỉ
cần `CHAR(36)` thường, không cần `DEFAULT (UUID())`.

## 2. `impact()` chạy thật trước khi sửa (bắt buộc theo AGENTS.md)

6 lần chạy (`mode: callgraph`, `direction: upstream`, `summaryOnly: true`),
tất cả **LOW**, không HIGH/CRITICAL nào:

| Symbol | impactedCount | risk |
|---|---|---|
| `New` (`internal/adapter/postgres/repository.go`) | 2 | LOW |
| `run` (`cmd/server/main.go`) | 1 | LOW |
| `OrchestrationTaskRepository` (interface, `ports.go`) | 3 | LOW |
| `DispatchContextRepository` (interface, `ports.go`) | 3 | LOW |
| `GateRepository` (interface, `ports.go`) | 3 | LOW |
| `CoordinatorRunRepository` (interface, `ports.go`) | 3 | LOW |

Không interface nào bị đổi chữ ký — mọi phương thức mới
(`internal/adapter/mysql.Repository`) implement lại đúng nguyên văn các
interface hiện có.

## 3. Migration dialect-safe

```
backend-go/services/orchestration-service/migrations/
├── postgres/   (6 migration × up/down, nội dung y hệt — chỉ `git mv`)
└── mysql/      (mới, 6 migration × up/down)
```

Điểm dịch đáng chú ý nhất (khác pilot vì schema phức tạp hơn):

- `messages.sequence BIGSERIAL` → `BIGINT AUTO_INCREMENT` (không phải UUID,
  khác mọi bảng khác trong service này).
- `messages."read"` — `read` là từ khoá dành riêng của MySQL (dùng trong
  `LOCK TABLES ... READ`) → phải backtick-quote (`` `read` ``) trong cả DDL
  lẫn tên cột ở index; không repository method nào ghi cột này tường minh
  (chỉ dùng default), nên không có SQL nào khác trong
  `internal/adapter/mysql` cần backtick thêm.
- 4 partial index của Postgres (`idx_gates_pending`,
  `idx_dispatch_contexts_user`, `idx_dispatch_contexts_worktree`,
  `idx_coordinator_runs_unreported`, `idx_orchestration_outbox_events_unpublished`
  — 5 thực ra) đều dùng `WHERE <cột> IS NOT NULL` hoặc `WHERE status IN (...)`
  — MySQL không có filtered/partial index, nên MySQL variant dùng composite
  index thường (mất tính chất "nhỏ bất kể lịch sử", không mất tính đúng đắn
  của query).
- `TEXT` → `VARCHAR(255)` cho `handle`/`user_id`/`worktree_id` (đều nằm
  trong 1 index ở trên) — InnoDB từ chối `TEXT`/`BLOB` trong khoá không có
  prefix length, cùng lý do issue-tracking-service's `connections` migration
  (BE-DB-SOL-004 §3) đã gặp.
- `spec`/`deps`/`options JSONB DEFAULT '{}'`/`'[]'`  → `JSON DEFAULT ('{}')`/
  `DEFAULT ('[]')` (MySQL 8.0.13+ cho phép default biểu thức trong ngoặc
  cho cột JSON — không load-bearing vì Go code luôn truyền giá trị tường
  minh ở mọi INSERT, xác nhận bằng đọc `Create`/`CreateWithTasks`).
- 5 bảng có RLS ở bản Postgres (`coordinator_runs`, `orchestration_tasks`,
  `dispatch_contexts`, `decision_gates`, `messages`) — MySQL variant bỏ
  toàn bộ `ENABLE ROW LEVEL SECURITY`/`CREATE POLICY`, có comment giải
  thích (theo BE-DB-SOL-001 §4's phát hiện: RLS này chưa từng thực sự
  enforce trên Postgres — không mất bảo vệ thực tế nào).
- Không có FK cross-schema; mọi FK trong service giữ nguyên
  (`orchestration_tasks.coordinator_run_id` → `coordinator_runs.id`,
  `orchestration_tasks.parent_id` self-referencing, v.v.) — MySQL/InnoDB hỗ
  trợ đầy đủ, giữ nguyên `ON DELETE CASCADE` nơi Postgres có.

## 4. `internal/adapter/mysql/repository.go`

Implement lại đúng cả 4 interface + `common/outbox.Store` trên 1 struct
`Repository` — không đổi chữ ký nào. Các điểm dịch kỹ thuật chính (khác
pilot vì `orchestration-service` dùng nhiều `RETURNING`/`FOR UPDATE`/
`= ANY($1)` hơn hẳn `usage-service`):

- **`RETURNING` → tính giá trị ở Go + SELECT lại trong cùng transaction**:
  mọi `id`/`created_at` được Go mint trước INSERT (`uuid.NewString()` +
  `time.Now().UTC()`) nên không cần đọc lại; các UPDATE cần trả về hàng đầy
  đủ (`UpdateStatusAndPromote`, `Complete`, `Fail`, `ClaimReady`) dùng
  `UPDATE ... WHERE <CAS predicate>` + kiểm tra `RowsAffected()` (0 = không
  match / đã bị race) + `SELECT` lại hàng trong CÙNG transaction — giữ đúng
  ngữ nghĩa CAS của bản Postgres (`RETURNING` vắng mặt ⇔ `pgx.ErrNoRows`
  ⇔ MySQL's `RowsAffected() == 0`).
- **`UpdateStatusAndPromote`'s finalize-run bước cuối** — bản Postgres dùng
  `UPDATE coordinator_runs ... WHERE status='running' RETURNING origin_task_id`
  để vừa lấy `origin_task_id` vừa xác nhận thắng race trong 1 câu lệnh.
  MySQL variant: `SELECT origin_task_id` riêng TRƯỚC (giá trị này không đổi
  nên đọc trước an toàn), sau đó `UPDATE ... WHERE status='running'` +
  kiểm tra `RowsAffected()==1` để quyết định có set `RunFinalized` hay
  không — cùng invariant, khác thứ tự câu lệnh.
- **`id = ANY($1)`** (dùng ở `promoteReadySiblings`'s `UPDATE ... WHERE id
  IN (...)` và `MarkPublished`) — không có tương đương MySQL trực tiếp;
  dùng chung 1 helper `inClausePlaceholders` build `IN (?,?,...)` động
  (cùng kỹ thuật usage-service's `MarkPublished` đã dùng, tái sử dụng ở
  đây thay vì viết lại 2 lần).
- **`ResolveGate`'s `FOR UPDATE OF g`** — cú pháp khoá-riêng-1-bảng-trong-
  join của Postgres. MySQL hỗ trợ cú pháp tương đương (`FOR UPDATE OF
  table_name`) chỉ từ 8.0.20+ — để tránh phụ thuộc phiên bản, adapter dùng
  `FOR UPDATE` trơn (khoá CẢ 2 bảng trong `LEFT JOIN`), bảo thủ hơn bản
  Postgres một chút (khoá rộng hơn), không kém an toàn hơn.
- **`ClaimReady`** — không có `RETURNING`, nhưng vẫn giữ đúng contract "1
  trong N lần gọi đồng thời thắng": bọc trong transaction, `UPDATE ...
  WHERE status='ready'` (row lock InnoDB đảm bảo chỉ 1 giao dịch match tại
  1 thời điểm), kiểm tra `RowsAffected()`, `SELECT` lại nếu thắng, `COMMIT`.
  Test `TestRepository_ClaimReady_ExactlyOneWinsUnderConcurrency` (mirror
  1:1 từ bản Postgres) xác nhận thật, không chỉ suy luận lý thuyết.
- **`nullableString` helper** — thay cho Postgres's `NULLIF($n,'')`/
  `*string`-arg workaround (cần vì pgx's type inference giữa `text` và cột
  `uuid` — xem `postgres/repository.go`'s comment ở
  `CreateDispatchContext`): MySQL's cột `CHAR(36)` không có type mismatch
  này, `database/sql`'s default converter tự dereference `*string` non-nil
  hoặc map `nil` → `NULL` — đơn giản hơn bản Postgres, không cần `NULLIF`
  trong SQL nào cả.

## 5. Wiring `cmd/server/main.go`

`cmd/server/main.go` trước rollout này đọc `cfg.DatabaseDSN` trực tiếp
(README's "Known gaps: `common/secrets` không wire" — đúng thật, xác nhận
bằng đọc code). Rollout này đóng gap đó theo đúng pattern batch-1: thêm
`DatabaseCredentialsFile` vào `internal/config.Config` (giống
`usage-service`), gọi `secrets.DatabaseCredentialsFromFile` +
`dbcapability.DetectDialectFromDSN`.

Khác pilot ở 1 điểm: vì `ports.go` có 4 interface riêng (không phải 1
`usecase.Repository` gộp), `main.go` khai báo 1 interface cục bộ
```go
type repository interface {
    usecase.OrchestrationTaskRepository
    usecase.DispatchContextRepository
    usecase.GateRepository
    usecase.CoordinatorRunRepository
    outbox.Store
}
```
để giữ đúng pattern "1 biến `repo`, gán theo dialect switch, dùng lại y hệt
ở toàn bộ phần wiring usecase bên dưới" — không phải sửa mọi lời gọi
`usecase.NewXxx(repo, ...)` hiện có. `toMySQLDriverDSN` copy nguyên văn từ
`usage-service` (không share qua package chung — cùng tiền lệ
issue-tracking-service/issue-status-sync).

## 6. Test

`internal/adapter/mysql/repository_test.go` — mirror 15/15 test case của
`internal/adapter/postgres/repository_test.go` 1:1 (tên test giữ nguyên),
cộng 2 test bổ sung: `TestRepository_Outbox_EnqueueFetchMarkPublished` và
`TestRepository_MarkPublished_EmptyIDsIsNoop` (không có test outbox trực
tiếp nào ở bản Postgres — outbox chỉ được test gián tiếp qua
`UpdateStatusAndPromote`/`CreateDispatchContext`/`CreateGate`'s
`event.ID != ""` branch — theo tiền lệ TASK-BE-DB-009 thêm test outbox
round-trip riêng).

Không có RLS policy nào cần 1 test tenant-isolation-without-RLS RIÊNG theo
nghĩa TASK-BE-DB-003's pattern (không có bảng nào trong service này từng
có RLS enforce thật + cần chứng minh scoping tường minh thay thế nó theo
cách khác các bảng khác chưa có) — `TestRepository_ListActiveDispatchContextsForUser_ReturnsOnlyCallerNonTerminal`
(mirror từ bản Postgres) đã là bằng chứng tenant-isolation thật (2 tenant,
kiểm tra không rò rỉ), đủ cho scope rollout này; xem §7 nếu phát hiện gap.

## 7. Kết quả thực tế

Xem TASK-BE-DB-015's mục "Kết quả thực tế" — build/test thật, không lặp
lại ở đây.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| 4 interface riêng thay vì 1 `Repository` gộp | Thấp | Giải quyết bằng 1 interface cục bộ trong `main.go`, không đổi `ports.go` |
| `FOR UPDATE OF g` không dùng được (MySQL 8.0.20+ mới có) | Thấp | Dùng `FOR UPDATE` trơn — khoá rộng hơn, không kém an toàn |
| Driver TiDB thật chưa test | Trung bình | Cùng gap đã ghi nhận ở BE-DB-SOL-002 §Rủi ro cho pilot — chưa đóng ở rollout nào tới nay |
| 5 partial index → composite index thường | Thấp | Mất tính chất "nhỏ", không mất tính đúng của query |

## Không thuộc phạm vi solution này

- 11 service còn lại của rollout 15 service — ngoài phạm vi.
- Đóng gap TiDB thật (container `pingcap/tidb`) — ngoài phạm vi batch này,
  giống mọi rollout trước.
