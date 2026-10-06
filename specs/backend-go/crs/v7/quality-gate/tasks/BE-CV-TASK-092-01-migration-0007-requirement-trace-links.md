# BE-CV-TASK-092-01: Migration `0007_requirement_trace_links` hai dialect

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `migrations/{postgres,mysql}/0007_requirement_trace_links.{up,down}.sql` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations
**Status:** [ ] TODO

## Việc cần làm
1. Bảng theo hợp đồng §4.2 T15; UNIQUE `(tenant_id, repo_id, scope_key, requirement_key, link_kind, evidence_kind, evidence_ref)`; CHECK `link_kind IN ('evidence_confirm','evidence_reject','worktree_task')`.
2. Postgres RLS `FORCE` + `tenant_isolation`; MySQL không RLS. Chạy `ls migrations/postgres` trước (số có thể dịch).
3. Chỉ mục `(tenant_id, repo_id, scope_key)`. `down`.

## Kiểm thử / Tiêu chí hoàn thành
- [ ] up/down/up hai dialect; [ ] RLS cách ly (role `NOBYPASSRLS`); [ ] unique chặn trùng; [ ] không FK.

## Rủi ro
- Khoá unique dài trên MySQL: kiểm giới hạn độ dài khoá InnoDB (`utf8mb4`, 3072 byte); nếu vượt, rút `evidence_ref` bằng băm (đã có quy tắc ở SOL-092 §2.5). Chưa kiểm chứng.
