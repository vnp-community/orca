# BE-CV-TASK-083-02: Migration `0005_coverage_reports` hai dialect

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/{postgres,mysql}/0005_coverage_reports.{up,down}.sql` (mới)
**Depends on:** BE-CV-TASK-082-02 (số migration kế trước; đọc `ls` trước khi đặt số)
**Status:** [x] DONE

## Context
C-DM §4.2 T13. UNIQUE `(tenant_id, repo_binding_id, head_commit, dirty, tree_hash, scope_key, source)` (~1 949 byte MySQL utf8mb4, tính tay).

## Việc cần làm
1. Tạo bảng đúng cột T13; CHECK `payload_bytes ≤ 1048576`, `source`, `language`, `mode` (Postgres).
2. Chỉ mục `(tenant_id, repo_binding_id, created_at)`; RLS `FORCE` + `NULLIF` + policy bảo trì (Postgres).
3. `down` xoá bảng.

## Kiểm thử
- up/down/up hai dialect; `information_schema`; RLS bằng role không superuser.

## Tiêu chí hoàn thành
- [x] UNIQUE hoạt động với `dirty` bool cả hai dialect.

## Rủi ro
Độ dài khoá MySQL chưa chạy thật.
