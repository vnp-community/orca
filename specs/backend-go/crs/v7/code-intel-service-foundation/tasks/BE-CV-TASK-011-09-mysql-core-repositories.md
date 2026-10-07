# BE-CV-TASK-011-09: Repository MySQL: settings, binding, review state, dismissal, C4, processed events

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/mysql/{tenant_settings,repo_binding,review_state,finding_dismissal,c4_override,processed_event}_repository.go` (+ `_integration_test.go`) (mới)
**Depends on:** BE-CV-TASK-011-03, 011-04, 011-06
**Status:** [x] DONE

---

## Context

MySQL không RLS: **mọi** truy vấn có `tenant_id = ?` và lấy tenant bằng `tenant.RequireTenantID` (test AST của 010-06 kiểm). Không `RETURNING`: ghi rồi `SELECT` lại trong cùng `*sql.Tx`. Vi phạm duy nhất `*mysql.MySQLError` `Number == 1062` (như `task-service/.../mysql/task_sources.go:15`). Cột `` `trigger` `` có nháy (không thuộc các bảng của task này nhưng quy tắc áp dụng cho 011-10).

## Việc cần làm

1. Cùng phương thức/hành vi như 011-07, khác cú pháp: `INSERT … ON DUPLICATE KEY UPDATE version = version + 1, updated_at = CURRENT_TIMESTAMP(6), …` rồi `SELECT` theo khoá duy nhất để lấy `id`/`version` hiện hành.
2. `GetOrCreate` settings: `INSERT IGNORE` hoặc `ON DUPLICATE KEY UPDATE tenant_id = tenant_id`; sau đó `SELECT`.
3. CAS: `UPDATE … SET … version = version + 1 … WHERE id=? AND tenant_id=? AND version=?`; `RowsAffected()==0` → `SELECT` phân biệt `NOT_FOUND`/`VERSION_CONFLICT` (an toàn vì `version` luôn đổi).
4. `MarkProcessed`: `INSERT IGNORE` và kiểm `RowsAffected()==1`.
5. JSON: truyền `[]byte`/`json.RawMessage`; đọc ra `[]byte`; chuỗi UTF-8 hỏng đã bị domain chặn.
6. `ListByProject` v.v. luôn `LIMIT ?`.
7. Ghi outbox qua `insertOutboxEvents` (010-06) cùng `*sql.Tx`.

## Kiểm thử

- Integration MySQL (`common/testutil.StartMySQL`): cùng kịch bản 011-07 (qua bộ `contracttest`, 011-11); CAS 20 goroutine; 1062 mapping.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/mysql/... -run 'TenantSettings|RepoBinding|ReviewState|Dismissal|C4|Processed'`.

## Tiêu chí hoàn thành

- [x] Hành vi trùng khớp bản Postgres qua bộ hợp đồng chung.
- [x] Mọi truy vấn có `tenant_id`; test AST xanh.

## Rủi ro và lưu ý

- `ON DUPLICATE KEY UPDATE` kích hoạt trên **bất kỳ** khoá duy nhất; mỗi bảng chỉ có UNIQUE nghiệp vụ + PK (id sinh mới nên không trùng) — kiểm.
- `RowsAffected` của `ON DUPLICATE KEY` trả 2 khi cập nhật: không dùng để suy luận "đã tồn tại".
