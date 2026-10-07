# BE-CV-TASK-011-02: Migration Postgres `0002_code_intel_core` (7 bảng, RLS, chính sách bảo trì)

**From Solution:** BE-CV-SOL-011-data-model-and-migrations
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/postgres/0002_code_intel_core.up.sql`, `0002_code_intel_core.down.sql` (mới)
**Depends on:** BE-CV-TASK-010-05, BE-CV-TASK-011-01
**Status:** [x] DONE

---

## Context

Cột/khoá/chỉ mục lấy nguyên văn từ hợp đồng §4.2 T1–T7 (không chép lại ở task này). Quy ước RLS: §4.1 và mẫu `mcp-service/migrations/postgres/0001_init.up.sql` (DO-block `FOREACH`, `FORCE`, `NULLIF`). **Chạy `ls services/code-intel-service/migrations/postgres` trước khi đặt số `0002`.**

## Việc cần làm

1. Tạo bảy bảng trong schema `codeintel`: `tenant_settings`, `repo_bindings`, `graph_snapshots`, `review_states`, `finding_dismissals`, `c4_overrides`, `reindex_jobs` với **đủ cột** của T1–T7 (PQ-24: không `ALTER` sau). `tenant_settings.code_intel_enabled` không DEFAULT.
2. CHECK có tên: `chk_*` cho `index_scope`, `status`/`mode` (`reindex_jobs`), `percent`, `disposition`, `review_states.status`, `payload_bytes <= 16777216`, `index_policy`, `ai_review_level`, `hotspot_window_days BETWEEN 30 AND 365`.
3. UNIQUE/chỉ mục theo T2–T7; ràng buộc `uq_reindex_active_key UNIQUE (active_key)` đặt tên tường minh.
4. RLS: khối `DO $$ … FOREACH` cho bảy bảng: `ENABLE`, `FORCE`, `tenant_isolation` (`USING` + `WITH CHECK`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid`).
5. Chính sách bảo trì (`current_setting('app.maintenance', true) = 'on'`, không `INSERT`): `maint_read` `FOR SELECT` trên `graph_snapshots`, `repo_bindings`, `reindex_jobs`, `review_states`, `outbox_events`, `processed_events`; `maint_delete_expired` `FOR DELETE` trên `graph_snapshots` (`expires_at < now()`); `maint_delete_published` trên `outbox_events` (`published_at IS NOT NULL`); `maint_delete_old` trên `processed_events`. Chú thích mỗi chính sách bằng lý do ngắn (mốc thời gian theo cấu hình nằm ở truy vấn Go, SOL-011 D3).
6. `down`: `DROP POLICY` các chính sách thêm cho `outbox_events`/`processed_events`, rồi `DROP TABLE` ngược thứ tự tạo (không `DROP SCHEMA`, vì `0001` còn dùng).
7. Dùng tiền tố `codeintel.` mọi nơi; không `commit` làm tên cột (đã đổi `head_commit`).

## Kiểm thử

- Integration (`-tags=integration`, Postgres): `TestMigration_0002_UpDownUp` (`golang-migrate` từ `0001`); kiểm từng CHECK bằng `INSERT` sai; UNIQUE `active_key` (NULL lặp được, trùng bị cản); PK/UNIQUE từng bảng.
- RLS (role `NOSUPERUSER NOBYPASSRLS`): với mỗi bảng, `app.tenant_id=A` chèn, `B` không thấy; `app.maintenance='on'` thấy chéo tenant ở bảng có `maint_read`, `INSERT` bị từ chối, `DELETE` chỉ xoá được dòng khớp điều kiện.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... -run Migration`.

## Tiêu chí hoàn thành

- [x] up/down/up sạch; bảy bảng đủ cột T1–T7.
- [x] RLS cô lập tenant; chính sách bảo trì không cho `INSERT`.
- [x] CHECK/UNIQUE từ chối đúng.

## Rủi ro và lưu ý

- `trigger` không cần nháy ở Postgres (kiểm ở 011-01) nhưng nên nháy để khớp bản MySQL.
- Dev dùng superuser nên RLS không hiệu lực; test phải tự tạo role.
