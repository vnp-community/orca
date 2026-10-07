# BE-CV-TASK-083-05: Repository MySQL `coverage_reports`

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/mysql/coverage_repository.go` (mới)
**Depends on:** BE-CV-TASK-083-02, 083-03
**Status:** [x] DONE

## Context
MySQL không RLS; `ON DUPLICATE KEY UPDATE`; `dirty` là `TINYINT(1)`.

## Việc cần làm
1. Cài cùng cổng với 083-04; `Prune` dùng `DELETE … ORDER BY created_at LIMIT n`.
2. `WHERE tenant_id = ?` mọi câu.

## Kiểm thử
- Bộ hợp đồng 083-06 trên MySQL; test AST thiếu `tenant_id`.

## Tiêu chí hoàn thành
- [x] Kết quả trùng Postgres trên kịch bản chung.

## Rủi ro
`ON DUPLICATE KEY` chưa chạy; TiDB chưa kiểm.
