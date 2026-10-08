# TASK-REQ-034-01: Migration `ai_governance` và repository (ledger, ngân sách, bộ đếm, chính sách bước, quyết định cổng)

**From Solution:** BE-REQ-SOL-034 (mục B)
**Priority:** P0 (ngân sách và `egress` là P0 trước khi bật cờ cho tenant thật)
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_ai_governance.{up,down}.sql` (mới), `.../internal/usecase/ports.go` (sửa), `.../internal/adapter/{postgres,mysql}/{ai_ledger,ai_budget,ai_step_policy,ai_gate_decision}.go` (mới), `.../internal/adapter/{postgres,mysql}/ai_governance_integration_test.go` (mới)
**Depends on:** BE-REQ-SOL-001, 002; TASK-REQ-025-01 (bảng `tenant_settings`)
**Status:** [ ] TODO

---

## Context

- `request-service` chưa tồn tại; mọi file "(mới)". Số `NNNN`: bắt buộc `ls backend-go/services/request-service/migrations/postgres backend-go/services/request-service/migrations/mysql` lúc làm; hai dialect cùng số. CR 001, 002, 004, 006, 007, 009, 010, 035 cùng thêm migration.
- Phụ thuộc `tenant_settings` (TASK-REQ-025-01: `tenant_id PK`, `request_flow_enabled`); nếu migration đó chưa merge thì task này **chờ**, không tự tạo bảng. CR-REQ-035 cũng `ALTER` bảng này (retention): hai migration `ADD COLUMN` khác tên cột nên không xung đột nhưng thứ tự theo ngày merge.
- Khác CR: khoá chính bộ đếm là `(tenant_id, budget_id, window_key)` (solution C1); thêm `ai_gate_decisions` (C2); cột `ai_gate_mode` ở `tenant_settings`.
- Mẫu RLS Postgres: `mcp-service/migrations/postgres/0001_init.up.sql` + `internal/adapter/postgres/tenant_tx.go`; mẫu cú pháp hai dialect cho `ON CONFLICT`/`ON DUPLICATE KEY`: các repository của `task-service` (`adapter/postgres`, `adapter/mysql`). MySQL không có RLS: `tenant_id` ở mọi `WHERE`.
- `usage-service/migrations/postgres/0001_init.up.sql` chỉ để tham khảo cấu trúc RLS; không sửa service đó.

## Việc cần làm

1. Chốt `NNNN`. Viết `NNNN_ai_governance.up.sql` Postgres đúng SQL ở solution mục B: `ai_usage_ledger` (hai chỉ mục), `ai_budgets` (UNIQUE `(tenant_id, scope_kind, scope_value, period)`; `scope_value` chuỗi rỗng cho `tenant`, không NULL), `ai_budget_counters` (PK `(tenant_id, budget_id, window_key)`, FK `ON DELETE CASCADE` tới `ai_budgets`), `ai_step_policies` (`request_type`, `size` lưu chuỗi rỗng thay NULL; UNIQUE `(tenant_id, step, request_type, size)`), `ai_trace_blobs` (FK tới ledger, `CHECK` 256 KB), `ai_gate_decisions`; `ALTER TABLE request.tenant_settings ADD COLUMN ai_egress_mode, ai_trace_level, ai_gate_mode`.
2. Với mọi bảng mới Postgres: `ENABLE` và `FORCE ROW LEVEL SECURITY`; policy `tenant_isolation` có `USING` và `WITH CHECK` dùng `NULLIF(current_setting('app.tenant_id', true), '')::uuid`.
3. `down.sql`: `ALTER TABLE ... DROP COLUMN` ba cột; `DROP TABLE` ngược thứ tự phụ thuộc: `ai_gate_decisions`, `ai_trace_blobs`, `ai_budget_counters`, `ai_step_policies`, `ai_budgets`, `ai_usage_ledger`.
4. MySQL tương ứng: `CHAR(36)`, `JSON`, `DECIMAL(12,6)`, `TIMESTAMP(6)`, `BOOLEAN` → `TINYINT(1)`; `CHECK` cần 8.0.16 trở lên; không có RLS.
5. Cổng trong `ports.go`:

```go
type AILedgerRepository interface {
    Insert(ctx context.Context, e domain.LedgerEntry) error
    Finish(ctx context.Context, id string, upd domain.LedgerFinish) error      // tokens, status, finished_at, model_used
    SummaryByRequest(ctx context.Context, requestID string) (domain.LedgerSummary, error)
    PurgeExpiredBlobs(ctx context.Context, now time.Time, limit int) (int, error)
    InsertBlob(ctx context.Context, b domain.TraceBlob) error
}
type AIBudgetRepository interface {
    ListEnabled(ctx context.Context, f domain.BudgetFilter) ([]domain.AIBudget, error) // khớp (project, request_type, step)
    List(ctx context.Context) ([]domain.AIBudget, error)
    Upsert(ctx context.Context, b domain.AIBudget, expectedVersion int64) (domain.AIBudget, error)
    Delete(ctx context.Context, id string) error
    EnsureCounter(ctx context.Context, budgetID, windowKey string, windowStart time.Time) error // INSERT ... DO NOTHING
    TryAdd(ctx context.Context, budgetID, windowKey string, d domain.UsageDelta, lim domain.Limits) (ok bool, after domain.Counter, err error)
    MarkWarned(ctx context.Context, budgetID, windowKey string, at time.Time) (first bool, err error)
    Adjust(ctx context.Context, budgetID, windowKey string, d domain.UsageDelta) error // Settle, có thể âm
}
type AIStepPolicyRepository interface { List(ctx) ([]domain.StepPolicy, error); Upsert(ctx, domain.StepPolicy, expectedVersion int64) (domain.StepPolicy, error) }
type AIGateDecisionRepository interface { Insert(ctx context.Context, d domain.GateDecisionRecord) error }
```

6. `TryAdd` Postgres: câu `UPDATE ... SET used_tokens = used_tokens + $1, ... WHERE ... AND ($7::bigint IS NULL OR used_tokens + $1 <= $7) ...` đúng như solution mục D; trả `ok = RowsAffected()==1`, đọc lại bộ đếm bằng `RETURNING` (Postgres) hoặc `SELECT` ngay trong cùng giao dịch (MySQL). `lim` là `Limits{Tokens *int64; Calls *int32; AgentSeconds *int32; CostUSD *decimal}` (con trỏ nil = không giới hạn; dùng `sql.NullInt64`/`NullFloat64`).
7. `MarkWarned`: `UPDATE ... SET warned_at = $now WHERE tenant_id=? AND budget_id=? AND window_key=? AND warned_at IS NULL`, `first = RowsAffected()==1`.
8. Tiền tệ: dùng `decimal` ở domain (kiểu chuỗi hoặc `shopspring/decimal` nếu repo đã dùng; kiểm `go.mod` của các service, không thêm phụ thuộc mới nếu chưa có: khi đó dùng số nguyên micro-USD `int64` ở domain và chia 1e6 khi hiển thị, cột vẫn `NUMERIC(12,6)`).
9. Mọi hàm gọi `tenant.RequireTenantID(ctx)`; truy vấn có `tenant_id`; ngoại lệ không có.

## Kiểm thử

- `ai_governance_integration_test.go` (`-tags=integration`, cả hai dialect):
  - `TestTryAdd_ConcurrentOnlyOneWins`: giới hạn 100 token, hai goroutine mỗi bên 70 (hai `InTx` riêng): đúng một `ok`, tổng `used_tokens=70`.
  - `TestTryAdd_NullLimitsUnlimited`; `TestTryAdd_OneDimensionFails_NoPartialAdd` (calls vượt, tokens không bị cộng nhờ điều kiện `WHERE` gộp).
  - `TestEnsureCounter_Idempotent`; `TestMarkWarned_OnlyFirst`.
  - `TestBudgetUpsert_VersionConflict`; `TestBudgetUnique_ScopeTuple`.
  - `TestStepPolicy_EmptyStringScopeUnique`: hai dòng cùng `(tenant, step, '', '')` bị từ chối.
  - `TestTenantIsolation_AllTables`: tenant B không đọc/sửa bảng nào của tenant A; Postgres không đặt `app.tenant_id` thì 0 dòng.
  - `TestMigration_UpDownUp`; `TestTenantSettings_NewColumnsDefault` (`external_allowed`, `digest_only`, `off`).
  - `TestCheckConstraints`: `step='x'`, `period='week'`, `ai_egress_mode='all'` đều lỗi.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/... && go test -tags=integration ./services/request-service/internal/adapter/...` (cần Docker; chưa chạy).

## Tiêu chí hoàn thành

- [ ] Up/down/up sạch Postgres 14+ và MySQL 8.0.16+; hai thư mục cùng `NNNN`.
- [ ] Câu so sánh-và-cộng từ chối vượt hạn, đúng một bên thắng khi đua, cả hai dialect.
- [ ] RLS `FORCE` bật cho bảng mới; MySQL có `tenant_id` ở mọi `WHERE`.
- [ ] `ai_trace_blobs` không bao giờ nhận dòng khi `ai_trace_level=digest_only` (kiểm ở task 04; ở đây chỉ cấu trúc).
- [ ] Không file `helpers`, `utils`, `common`; không `max-lines` disable.

## Rủi ro và lưu ý

- `ADD COLUMN` MySQL không có `IF NOT EXISTS`: migration chỉ chạy một lần; test up/down/up phải xoá cột ở `down`.
- Giao dịch `Reserve` giữ khoá hàng bộ đếm đến hết giao dịch: giữ ngắn (chỉ các `UPDATE`), không gọi Relay trong giao dịch.
- Dùng `NUMERIC` làm tiền: không so sánh bằng `float64` ở Go.
- Thêm tên các bảng mới vào test quét SQL và test cách ly của BE-REQ-SOL-035 task 05 (danh sách bảng).
