# BE-CV-TASK-082-06: Bộ test hợp đồng repository (hai dialect)

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/repotest/quality_repository_contract.go`, `internal/adapter/postgres/quality_repository_contract_test.go`, `internal/adapter/mysql/quality_repository_contract_test.go` (mới, `-tags=integration`)
**Depends on:** BE-CV-TASK-082-04, 082-05
**Status:** [x] DONE

## Context
Một bộ kịch bản chung cho hai dialect (khuôn `repository_contract_test.go` của SOL-011). Ma trận CI `dialect: [postgres, mysql]`.

## Việc cần làm
1. Kịch bản: `Create` đồng thời (một thắng, một `ErrRunActive`), `Finish` CAS xung đột, `InsertBatch` lặp, phân trang keyset, lọc `in_scope`/severity, `MarkOrphans`, `PurgeExpired` theo lô.
2. Cô lập tenant cho **mọi** phương thức; Postgres kiểm RLS bằng SQL trực tiếp.
3. Kiểm schema qua `information_schema`.

## Kiểm thử
- `go test -tags=integration ./internal/adapter/... -v` mỗi dialect.

## Tiêu chí hoàn thành
- [x] Hai dialect xanh; tên package test không dùng `utils/helpers/common`.

## Rủi ro
Tên package chia sẻ test (`repotest`) cần đồng bộ với SOL-011.
