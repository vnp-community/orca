# BE-CV-TASK-011-03: Migration MySQL `0002_code_intel_core` (7 bảng)

**From Solution:** BE-CV-SOL-011-data-model-and-migrations
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/mysql/0002_code_intel_core.up.sql`, `0002_code_intel_core.down.sql` (mới)
**Depends on:** BE-CV-TASK-010-05, BE-CV-TASK-011-01
**Status:** [x] DONE

---

## Context

Cùng cột như bản Postgres (hợp đồng §4.1 bảng kiểu: `uuid`=`CHAR(36)`, `ts`=`TIMESTAMP(6)`, `json`=`JSON`, `bool`=`TINYINT(1)`, `text`=`TEXT`/`MEDIUMTEXT`). MySQL: không schema (database `codeintel`), không RLS; mọi truy vấn phải lọc `tenant_id` ở ứng dụng. Mẫu: `task-service/migrations/mysql/0005_outbox.up.sql`.

## Việc cần làm

1. Bảy bảng đủ cột T1–T7, `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`. `JSON` không có DEFAULT: ứng dụng luôn chèn giá trị (`'{}'`, `'[]'`).
2. `` `trigger` `` đặt dấu nháy ngược trong DDL (và mọi câu SQL của adapter); áp kết luận 011-01 cho `view`, `at`.
3. Timestamp: `TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)` cho `created_at`; `updated_at`/`expires_at` do ứng dụng đặt bằng đồng hồ DB; cột `NULL` ghi rõ `NULL` (MySQL cũ tự gán DEFAULT cho `TIMESTAMP` đầu tiên không NULL; kiểm `explicit_defaults_for_timestamp` ở 011-01).
4. UNIQUE: `uq_repo_bindings_scope (tenant_id, project_id, scope_key)`, `uq_graph_snapshots_key (tenant_id, repo_binding_id, view, head_commit, params_hash)`, `uq_review_states_key`, `uq_finding_dismissals_key`, `uq_c4_overrides_key`, `uq_reindex_active_key (active_key)`; chỉ mục theo T2–T7.
5. CHECK (8.0.16+) tên `chk_*` như bản Postgres; ghi chú đầu file: bản cũ bỏ qua CHECK, domain là hàng rào chính.
6. `c4_overrides.document` `MEDIUMTEXT`; `review_states.notes` `JSON` (≤ 256 KiB) — kiểm `max_allowed_packet` mặc định (chưa kiểm chứng) đủ cho 256 KiB.
7. `down`: `DROP TABLE IF EXISTS` ngược thứ tự tạo; không đụng `outbox_events`/`processed_events` của `0001`.

## Kiểm thử

- Integration (`-tags=integration`, MySQL testcontainers): `TestMigration_0002_UpDownUp`; CHECK (nếu phiên bản hỗ trợ), UNIQUE (`active_key` nhiều `NULL`, trùng giá trị bị cản với `1062`), độ dài khoá (tạo được); chèn JSON hợp lệ và UTF-8 hỏng (bị từ chối).
- Hợp đồng schema: so `information_schema.columns` với bản Postgres về tên cột và tính NULL (một bảng mong đợi dùng chung ở 011-05).
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/mysql/... -run Migration`.

## Tiêu chí hoàn thành

- [x] up/down/up sạch; tên cột và tính NULL khớp bản Postgres.
- [x] `active_key` NULL lặp được; trùng bị `1062`.
- [x] Mọi câu SQL tham chiếu `trigger` có nháy ngược.

## Rủi ro và lưu ý

- Không có đường chạy MySQL ở dev compose; chỉ CI. Ghi chú vào README service.
- `max_allowed_packet` và `innodb_large_prefix` mặc định chưa kiểm chứng.
