# BE-CV-TASK-086-14: RPC `RefreshCiRun` và `CiComparisonService`

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/refresh_ci_run.go`, `backend-go/services/code-intel-service/internal/usecase/ci_comparison_service.go` (mới)
**Depends on:** BE-CV-TASK-086-13, 086-08, BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Status:** [ ] TODO

## Context
C-DM §3.2: `RefreshCiRun` (action `quality_read`, kênh `codeIntel.quality.ci`); `GetQualityGate` trả `comparison[]` (SOL-085 gọi `ForHead`).

## Việc cần làm
1. Handler theo chuỗi bảo vệ C-DM §3; lỗi `CODEINTEL_*: msg | {json}` (PQ-02).
2. `CiComparisonService.ForHead(tenant, binding, head)` đọc run cục bộ + CI, gọi `CompareLocalAndCI`.
3. Ánh xạ proto↔domain; response ≤ 2 MiB.

## Kiểm thử
- Handler: cờ tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED`; phiên thiết bị ⇒ `NOT_AUTHORIZED`; tenant khác ⇒ `NOT_FOUND`.

## Tiêu chí hoàn thành
- [ ] `TestChannelInventory` phía gateway (SOL-040) khớp kênh `quality.ci`.

## Rủi ro
Phụ thuộc SOL-085/040 cho đăng ký kênh.
