# BE-CV-TASK-085-01: Migration `0004_quality_gate` (quality_profiles, quality_waivers, quality_trend_points) hai dialect

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/{postgres,mysql}/0004_quality_gate.{up,down}.sql` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations (`0002`), BE-CV-SOL-082 (`0003`)
**Status:** [ ] TODO

## Context
Cột và khoá lấy nguyên từ hợp đồng §4.2 T10, T11, T12 (không chép lại ở đây). Số migration có thể dịch: chạy `ls migrations/postgres` trước. Mẫu RLS: `mcp-service/migrations/postgres/0001_init.up.sql`. `tenant_settings` đã đủ cột ở `0002` (PQ-24): **không** ALTER.

## Việc cần làm
1. Postgres: schema `codeintel`; 3 bảng; `ENABLE`+`FORCE ROW LEVEL SECURITY`; policy `tenant_isolation` (`USING`+`WITH CHECK`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid`); CHECK `mode IN ('inform','block')`, `verdict IN ('pass','warn','fail','unknown')`, `subject_kind IN (...)`; `definition` JSONB.
2. UNIQUE `(tenant_id, scope_key, name)`; `quality_waivers.active_key` UNIQUE (NULL lặp được); trend UNIQUE `(tenant_id, repo_binding_id, head_commit, turn_key, profile_ref, source)` (L1 của SOL-085-waivers, O-10); chỉ mục `(tenant_id, repo_binding_id, created_at)`; chỉ mục waiver `(tenant_id, repo_id, expires_at)`.
3. MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `TINYINT(1)`; không schema/RLS; CHECK cần MySQL ≥ 8.0.16 (ghi vào README service).
4. `down`: xoá 3 bảng (MySQL) / các bảng trong schema (Postgres, không `DROP SCHEMA` vì chung với bảng khác).

## Kiểm thử
- `-tags=integration` từng dialect: up → down → up; `information_schema` kiểm cột/kiểu/NULL.
- Postgres, role `NOSUPERUSER NOBYPASSRLS`: đặt `app.tenant_id=A`, chèn, đổi `B` không thấy; không đặt thì không thấy và không lỗi cast.
- UNIQUE: chèn trùng `active_key` bị từ chối (23505/1062); hai hàng `active_key IS NULL` được.

## Tiêu chí hoàn thành
- [ ] up/down/up sạch hai dialect; [ ] RLS cách ly tenant bằng SQL trực tiếp; [ ] khoá UNIQUE đúng T10–T12.

## Rủi ro
- Dev compose dùng superuser nên RLS vô hiệu ở dev (hợp đồng §4.1); test phải tự tạo role. Chưa kiểm chứng `golang-migrate` MySQL với database `codeintel`.
