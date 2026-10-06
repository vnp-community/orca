# BE-CV-TASK-011-10: Repository MySQL: `graph_snapshots` (tỉa, hạn mức) và `reindex_jobs` (`active_key`, đếm hạn mức)

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/mysql/{graph_snapshot,reindex_job}_repository.go` (+ `_integration_test.go`) (mới)
**Depends on:** BE-CV-TASK-011-03, 011-04, 011-06
**Status:** [ ] TODO

---

## Context

Tương đương 011-08 trên MySQL. Khác biệt cú pháp: `LIMIT` không cho dùng trong subquery của `IN`; `DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? SECOND)`; không `RETURNING`; cột `` `trigger` `` phải có nháy ngược trong mọi câu `INSERT/SELECT` của `reindex_jobs`.

## Việc cần làm

1. `Put`: `INSERT … ON DUPLICATE KEY UPDATE` (cập nhật `schema_version`, `etag`, `total_count`, `payload`, `payload_bytes`, `truncated`, `tool_versions`, `created_at = CURRENT_TIMESTAMP(6)`, `expires_at = DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? SECOND)`); tỉa 3 commit bằng bảng dẫn xuất: chọn các `head_commit` cần giữ (`ORDER BY MAX(created_at) DESC LIMIT 3`) trong `SELECT` riêng rồi `DELETE … WHERE view=? AND head_commit NOT IN (?,?,?)` với danh sách đã đọc; hạn mức binding: đọc tổng, xoá từng dòng cũ nhất cho đến khi dưới hạn (không xoá bản mới nhất mỗi view). Tất cả trong một `*sql.Tx`.
2. `Get`, `DeleteByBinding`, `TenantBytes` như 011-08.
3. `reindex_job.Create`: `INSERT` job; `Number == 1062` mà `Message` chứa `uq_reindex_active_key` → `CODEINTEL_REINDEX_IN_PROGRESS`; khác → `CODEINTEL_ALREADY_EXISTS`; ghi outbox cùng tx.
4. `Get`, `ListByBinding`, `UpdateProgress` (`percent` NULL khi nil), `Finish` (`active_key = NULL`), `CountActiveByDevServer` (join `repo_bindings`), `CountActiveByTenant`, `LastSucceededFinishedAt`.
5. Phương thức `Finish` kiểm `CanTransition` ở use case; adapter kiểm thêm `WHERE status IN ('queued','running')` để không ghi đè trạng thái cuối.

## Kiểm thử

- Integration MySQL: cùng kịch bản 011-08 (qua `contracttest`); riêng: phân biệt lỗi `active_key` với PK qua `Message`; `ON DUPLICATE KEY` không đổi `id` snapshot; tỉa với danh sách `NOT IN` rỗng (chưa đủ 3 commit) không xoá gì.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/mysql/... -run 'Snapshot|Reindex'`.

## Tiêu chí hoàn thành

- [ ] Hành vi bằng bản Postgres qua bộ hợp đồng chung.
- [ ] `trigger` có nháy ngược mọi nơi.

## Rủi ro và lưu ý

- `NOT IN` với danh sách rỗng trả sai; luôn nhánh riêng khi không có gì để giữ/xoá.
- Chuỗi `Message` của MySQL có thể đổi theo phiên bản: dùng `strings.Contains` với tên khoá, chưa kiểm chứng trên TiDB.
