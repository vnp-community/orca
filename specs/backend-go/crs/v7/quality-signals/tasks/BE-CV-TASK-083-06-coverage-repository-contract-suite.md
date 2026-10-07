# BE-CV-TASK-083-06: Bộ test hợp đồng repository coverage (hai dialect)

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/repotest/coverage_repository_contract.go`, `postgres|mysql/coverage_repository_contract_test.go` (mới, `-tags=integration`)
**Depends on:** BE-CV-TASK-083-04, 083-05
**Status:** [x] DONE

## Context
Khuôn như BE-CV-TASK-082-06; ma trận `dialect: [postgres, mysql]`.

## Việc cần làm
1. Kịch bản: upsert idempotent, khoá `dirty`/`tree_hash`, TTL, cap 20, `LatestByHead`.
2. Cô lập tenant mọi phương thức; RLS bằng SQL trực tiếp (Postgres).
3. Kiểm schema `information_schema`.

## Kiểm thử
- `go test -tags=integration ./internal/adapter/... -v`.

## Tiêu chí hoàn thành
- [x] Hai dialect xanh.

## Rủi ro
Đồng bộ package test dùng chung với SOL-011/082.
