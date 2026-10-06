# BE-CV-TASK-083-09: RPC `GetCoverage` và bảo trì coverage

**From Solution:** BE-CV-SOL-083
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_coverage.go`, `internal/adapter/grpc/` (handler), thêm vào job bảo trì của SOL-011
**Depends on:** BE-CV-TASK-083-07, 083-08, 083-01, BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Status:** [ ] TODO

## Context
C-DM §3 chuỗi bảo vệ (guard, cờ `quality_gate_enabled`, OPA `quality_read`, selector, quyền trước cache); phản hồi ≤ 2 MiB. Kênh `codeIntel.quality.coverage` ở SOL-040.

## Việc cần làm
1. Thứ tự: measured theo `run_id`/`head` (khớp `dirty`) → estimated → `null` + `reason`.
2. Cắt `files` nếu vượt 2 MiB, `truncated=true`.
3. Đăng ký `Prune` (30 ngày, cap 20/binding) vào bảo trì.

## Kiểm thử
- Bảng ca ba nhánh; tenant A/B; cờ tắt → `CODEINTEL_QUALITY_GATE_DISABLED`.

## Tiêu chí hoàn thành
- [ ] `proto.Size` ≤ 2 MiB.

## Rủi ro
Phụ thuộc file proto của SOL-085.
