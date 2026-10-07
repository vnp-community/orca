# BE-CV-TASK-089-01: Migration `0006_agent_turns` hai dialect

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `migrations/{postgres,mysql}/0006_agent_turns.{up,down}.sql` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations
**Status:** [x] DONE

## Context
Cột/khoá: hợp đồng §4.2 T14 (không chép lại). Chạy `ls migrations/postgres` trước (số có thể dịch).

## Việc cần làm
1. Postgres: bảng + RLS `FORCE` + `tenant_isolation`; CHECK `source IN ('renderer','hook','both')`, `model_source IN ('agent_session','transcript','unknown')`; JSONB `commands_summary/claims/verification`.
2. UNIQUE `(tenant_id, repo_binding_id, client_turn_id)`; chỉ mục `(tenant_id, repo_binding_id, ended_at DESC)`, `(tenant_id, repo_binding_id, end_head_commit)`, `(expires_at)`.
3. MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`; chỉ mục DESC cần MySQL ≥ 8.0.
4. `down`.

## Kiểm thử
- up/down/up hai dialect; RLS với role `NOBYPASSRLS`; UNIQUE chặn trùng.

## Tiêu chí hoàn thành
- [x] cột đúng T14; [ ] RLS cách ly; [ ] không FK.

## Rủi ro
- JSON ≤ 8 KiB kiểm ở ứng dụng (MySQL không CHECK tiện).
