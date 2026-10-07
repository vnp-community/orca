# TASK-REQ-010-01: Migration `approval_policies` và `approval_approvers`

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục C
**Priority:** P1
**Service/Area:** `request-service` / migrations
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_approval_policies.{up,down}.sql` (mới)
**Depends on:** TASK-REQ-009-01 (bảng `approvals`)
**Status:** `[x] DONE`

## Context

- `request-service` chưa có thư mục migration lúc soạn; số `NNNN` phải đọc lại `migrations/postgres` và `migrations/mysql` rồi cộng 1 (sau `NNNN_approvals`). Hai dialect cùng số.
- Mẫu cột sinh và NULL trong khoá: `task-service/migrations/mysql/0012_task_sources.up.sql`, `0014_task_sources_site.up.sql` (xử lý NULL khác nhau giữa hai dialect).
- Khoá chính MySQL không nhận NULL, nên `principal_id` của `reporter` là chuỗi rỗng.

## Việc cần làm

1. Đọc số migration tiếp theo (bước bắt buộc), ghi vào PR.
2. `approval_policies`: `id`, `tenant_id`, `project_id NULL`, `subject_type` (cùng CHECK 8 giá trị với `approvals`), `request_type NULL` (CHECK 11 loại), `size NULL CHECK IN ('S','M','L')`, `urgency NULL CHECK IN ('normal','urgent')`, `approvers` (`JSONB` / `JSON`) `NOT NULL`, `allow_requester_approve`, `due_after_seconds INT NULL`, `priority INT NOT NULL DEFAULT 0`, `enabled`, `version`, `created_by`, `created_at`, `updated_at`. Chỉ mục `(tenant_id, subject_type, enabled)`.
3. `approval_approvers`: khoá chính `(approval_id, principal_kind, principal_id)`, CHECK `principal_kind IN ('user','team','role','reporter')`, FK `approval_id -> approvals(id)`, chỉ mục `(tenant_id, principal_kind, principal_id)`.
4. Postgres: RLS `tenant_isolation` cho cả hai bảng. MySQL: lọc ở repository.
5. Down: `DROP TABLE approval_approvers; DROP TABLE approval_policies;` (thứ tự do FK).
6. Cột `approvals.self_approval_allowed` đã có từ task 009-01; không thêm lại.

## Kiểm thử

- Up/down/up hai dialect; chèn `principal_kind='bogus'` thất bại; chèn trùng khoá chính thất bại; xoá `approvals` còn `approval_approvers` bị FK chặn (hoặc quyết định `ON DELETE CASCADE` và ghi lại lý do trong PR; Approval không bị xoá theo thiết kế nên chọn chặn).
- Lệnh: bộ test migration của service (cùng cách task 009-01).

## Tiêu chí hoàn thành

- [x] Hai dialect cùng số, up/down/up sạch.
- [x] CHECK từ chối giá trị lạ ở cả hai DB.
- [x] Truy vấn `WHERE project_id IS NULL OR project_id=?` cùng kết quả ở hai DB trên dữ liệu mẫu.

## Rủi ro và lưu ý

- Cột JSON `approvers` chỉ để lưu và hiển thị; không truy vấn bằng toán tử JSON (khác nhau giữa hai DB). Việc lọc người duyệt dùng `approval_approvers`.
