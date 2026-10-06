# BE-CV-TASK-037-07: Repository `finding_dismissals` (Postgres + MySQL) và use case `DismissFinding`

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/dismiss_finding.go`, `dismiss_finding_test.go`; `internal/usecase/ports.go` (sửa: `FindingDismissalRepository`); `internal/adapter/postgres/finding_dismissal_repository.go`, `internal/adapter/mysql/finding_dismissal_repository.go` và `*_integration_test.go` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations (bảng `finding_dismissals` trong `0002_code_intel_core`); BE-CV-SOL-013 (audit, quyền); BE-CV-TASK-037-02
**Status:** [ ] TODO

---

## Context

Bảng đã do 011 tạo (hợp đồng §4.2 T5; **không** thêm migration ở đây). Nếu `BE-CV-SOL-011-repositories-and-maintenance` đã tạo repository cùng tên, task này chỉ bổ sung phương thức còn thiếu. Mẫu giao dịch tenant: `mcp-service/internal/adapter/postgres/tenant_tx.go` (`withTenantTx`, `set_config('app.tenant_id', $1, true)`). MySQL không có RLS ⇒ `WHERE tenant_id = ?` mọi câu.

## Việc cần làm

1. Port: `Upsert(ctx, d Dismissal) error`, `Delete(ctx, tenant, repoID, key string) error`, `ListByKeys(ctx, tenant, repoID string, keys []string) (map[string]Dismissal, error)` (≤ 200 khoá), `CountByRepo`. Mọi hàm nhận `tenantID` tường minh.
2. Postgres: `INSERT INTO codeintel.finding_dismissals (id, tenant_id, repo_id, repo_binding_id, finding_key, disposition, reason, note, dismissed_by, at) VALUES (…) ON CONFLICT (tenant_id, repo_id, finding_key) DO UPDATE SET disposition=EXCLUDED.disposition, reason=…, note=…, dismissed_by=…, at=…, repo_binding_id=…` (giữ `id` cũ); `DELETE … WHERE tenant_id=$1 AND repo_id=$2 AND finding_key=$3`; `ListByKeys` dùng `= ANY($3)`. Chạy trong `withTenantTx`.
3. MySQL: `INSERT … ON DUPLICATE KEY UPDATE …` cùng cột; `ListByKeys` dùng `IN (?,…)` sinh placeholder theo số khoá; `at` dùng `CURRENT_TIMESTAMP(6)` của DB.
4. `dismiss_finding.go`: cờ ⇒ quyền `review_write` ⇒ `selector`→`repo_id`+`binding`; validate (`ValidKey`, `reason`/`note` ≤ 500, `disposition ∈ ignored|resolved`, mặc định `ignored`); `DISMISS`⇒upsert, `RESTORE`⇒delete (idempotent); audit qua `auditclient` (`finding_key`, `disposition`, `action`; không mã nguồn); trả `{finding_key, dismissed, disposition}`. Không xoá cache snapshot (dismiss không nằm ở đó).
5. Không đề cập gate; ghi comment: "Dismiss không miễn cổng chất lượng (PQ-05)".

## Kiểm thử

- Unit use case với fake repo/audit: idempotent hai lần; `RESTORE` không dòng; quyền sai ⇒ `CODEINTEL_NOT_AUTHORIZED` và không chạm repo; `finding_key` sai ⇒ `CODEINTEL_INVALID_PARAMS` (`field:"findingKey"`); `reason` 501 ký tự ⇒ lỗi; audit được gọi một lần.
- Integration `-tags=integration` ma trận `dialect: [postgres, mysql]`: upsert tạo/cập nhật; unique; **hai goroutine dismiss cùng khoá** ⇒ một dòng, không lỗi; restore; `ListByKeys` 200 khoá; **cô lập tenant**: A và B cùng `repo_id`/`finding_key` ⇒ dòng riêng; B không xoá được dòng của A; Postgres bằng role `NOSUPERUSER NOBYPASSRLS` (tenant không đặt ⇒ không thấy dòng nào).
- Quét AST/regex test: mọi hằng SQL trong hai tệp repository chứa `tenant_id`.

## Tiêu chí hoàn thành

- [ ] Hai dialect xanh; `id` giữ nguyên khi upsert lần hai.
- [ ] Mọi câu SQL có `tenant_id`; test cô lập tenant xanh.
- [ ] Audit có; không PII ngoài `user id`.

## Rủi ro và lưu ý

- Lệch tên schema/database `codeintel` giữa dialect (PG schema, MySQL database không tiền tố): dùng biến cấu hình/`Repository` chứ không hardcode `codeintel.` ở MySQL.
- Chưa xác định xung đột với repository của 011; đối chiếu trước khi viết.
