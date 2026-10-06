# BE-CV-TASK-082-02: Migration `0003_quality_runs_and_findings` hai dialect

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/postgres/0003_quality_runs_and_findings.{up,down}.sql`, `backend-go/services/code-intel-service/migrations/mysql/0003_quality_runs_and_findings.{up,down}.sql` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations (`0001`, `0002`)
**Status:** [ ] TODO

## Context
Cột chính xác: C-DM §4.2 T8 (`quality_runs`, gồm cột CI) và T9 (`quality_findings`). Chạy `ls migrations/postgres` trước để chốt số. Mẫu RLS: `mcp-service` (`FORCE`, `NULLIF`), không theo `scm-integration-service` (không `FORCE`).

## Việc cần làm
1. Postgres: schema `codeintel`, hai bảng, CHECK (`scope`, `status`, `source`, `severity`, `category`), UNIQUE `active_key`, UNIQUE `(tenant_id, repo_binding_id, agent_run_id)`, `(tenant_id, run_id, ordinal)`, các chỉ mục T8/T9, `ENABLE`+`FORCE` RLS, `tenant_isolation`, policy bảo trì hẹp.
2. MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, cột `col`/`end_col`; không chỉ mục `file`.
3. `down` đảo ngược; không FK.

## Kiểm thử
- Integration: up/down/up mỗi dialect; kiểm `information_schema`; CHECK từ chối giá trị sai (Postgres); RLS cô lập tenant bằng SQL trực tiếp.

## Tiêu chí hoàn thành
- [ ] Hai dialect sạch; `active_key` NULL nhiều dòng được phép.

## Rủi ro
Kích thước hàng MySQL utf8mb4 (tính tay, chưa chạy).
