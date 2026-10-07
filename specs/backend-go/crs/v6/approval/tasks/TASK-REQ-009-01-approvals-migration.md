# TASK-REQ-009-01: Migration `approvals` (Postgres và MySQL)

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục C
**Priority:** P0
**Service/Area:** `request-service` / migrations
**File:** `backend-go/services/request-service/migrations/postgres/NNNN_approvals.{up,down}.sql` (mới), `backend-go/services/request-service/migrations/mysql/NNNN_approvals.{up,down}.sql` (mới)
**Depends on:** CR-REQ-001 và CR-REQ-002 đã merge (có thư mục migrations và bảng `requests`)
**Status:** `[x] DONE`

## Context

- `backend-go/services/request-service` không tồn tại tại thời điểm soạn (đã `ls`); số migration phải xác định lúc làm. Bước 1 bên dưới là bắt buộc.
- Mẫu cột sinh MySQL thay partial index: `task-service/migrations/mysql/0012_task_sources.up.sql` (`project_key`). Mẫu RLS: migration `0012` Postgres của `task-service` và `0002_request_core` (CR-REQ-002).
- DDL đầy đủ nằm ở solution mục C; task này chỉ biến nó thành file.

## Việc cần làm

1. `ls backend-go/services/request-service/migrations/postgres backend-go/services/request-service/migrations/mysql`; lấy số lớn nhất rồi cộng 1 (cả hai dialect phải cùng số). Ghi số vào mô tả PR.
2. Postgres up: tạo `request.approvals` với đủ cột (gồm `subject_digest`, `self_approval_allowed`, `idempotency_key`, `reminded_at`, `created_at`, `updated_at`), CHECK `subject_type` (8 giá trị) và `status` (5 giá trị), FK `request_id -> request.requests(id)`.
3. Postgres: chỉ mục duy nhất một dòng `pending` cho mỗi chủ thể `(tenant_id, subject_type, subject_id) WHERE status='pending'`; duy nhất `(tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL`; ba chỉ mục đọc `(tenant_id, request_id, created_at)`, `(tenant_id, status, due_at)`, `(tenant_id, status, created_at)`.
4. Postgres: `ENABLE` và `FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` giống bảng `requests`.
5. MySQL up: kiểu tương đương (`CHAR(36)`, `VARCHAR(20/40/64/128)`, `TIMESTAMP(6)`, `TINYINT(1)`, `CHAR(64)` cho digest); cột sinh `pending_key VARCHAR(120) GENERATED ALWAYS AS (IF(status='pending', CONCAT(subject_type,':',subject_id), NULL)) STORED` kèm `UNIQUE KEY (tenant_id, pending_key)`; `UNIQUE KEY (tenant_id, idempotency_key)` (NULL không va chạm).
6. Cả hai down: `DROP TABLE approvals`.
7. Cập nhật danh sách bảng trong README của `request-service` (nếu CR-REQ-001 đã tạo) hoặc ghi vào mô tả PR.

## Kiểm thử

- Test migration của service (khuôn `usage-service` repository_test): up, down, up trên Postgres 14+ và MySQL 8.0.1+.
- Test SQL tối thiểu: chèn `subject_type='bogus'` thất bại ở cả hai DB; chèn hai dòng `pending` cùng chủ thể thất bại; một dòng `pending` và một `approved` cùng chủ thể thành công.
- Lệnh: `cd backend-go/services/request-service && go test ./internal/adapter/... -run Migration` (tên test cụ thể do khung CR-REQ-001 quy định, chưa kiểm chứng).

## Tiêu chí hoàn thành

- [x] Hai dialect cùng số migration, up/down/up chạy sạch.
- [x] CHECK từ chối `subject_type` và `status` lạ ở cả hai DB.
- [x] Chỉ mục `pending` duy nhất hoạt động ở cả hai DB (kể cả sau khi một dòng chuyển sang `approved`).
- [x] RLS bật ở Postgres; truy vấn không đặt `app.tenant_id` không thấy dòng nào.

## Rủi ro và lưu ý

- `subject_id` tối đa 64 ký tự: đủ cho UUID (36). Không thu hẹp.
- Khoá MySQL: `pending_key` dài 20+1+64=85 < 120, nhưng khoá duy nhất cộng `tenant_id` dưới 3072 byte với `utf8mb4`: kiểm bằng cách chạy thật.
- Nếu CR-REQ-009 mở rộng `subject_type` ở CR bổ sung sau, đó là migration expand-only riêng.
