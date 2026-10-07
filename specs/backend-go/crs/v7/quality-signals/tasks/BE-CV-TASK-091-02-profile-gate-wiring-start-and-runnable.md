# BE-CV-TASK-091-02: `ProfileGate` cắm vào `StartQualityRun` và `GetQualityProfile`

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/security_scan_policy.go`, test (mới); điểm gọi trong use case của SOL-085
**Depends on:** BE-CV-TASK-091-01, BE-CV-SOL-085-quality-gate-evaluator-and-profiles, BE-CV-SOL-013-authorization-flags-and-audit
**Status:** [x] DONE

## Context
Cờ hiệu lực PQ-24 (fail closed, cache ≤ 5 s). Tắt ⇒ `CODEINTEL_PROFILE_UNKNOWN` + `available[]` đã lọc; `quality_gate_enabled` tắt ưu tiên `CODEINTEL_QUALITY_GATE_DISABLED`.

## Việc cần làm
1. `AllowStart(profile, scope, role)` và `Filter(runnable)`.
2. `scope=worktree` + bảo mật ⇒ cần `quality_profile_write`.
3. Không gọi agent khi bị từ chối.

## Kiểm thử
- Cổng giả: ma trận cờ × scope × vai trò; đếm lời gọi agent = 0 khi từ chối.

## Tiêu chí hoàn thành
- [x] Mã lỗi đúng PQ-01.

## Rủi ro
Điểm cắm cần SOL-085 đồng ý chữ ký.
