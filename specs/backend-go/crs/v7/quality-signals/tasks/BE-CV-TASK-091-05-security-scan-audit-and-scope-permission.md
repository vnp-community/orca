# BE-CV-TASK-091-05: Audit lần chạy quét, quyền `scope=worktree`, lỗi môi trường

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/security_scan_policy.go` (mở rộng), cấu hình danh sách `reason` cho phép của lỗi agent (SOL-023)
**Depends on:** BE-CV-TASK-091-02, BE-CV-SOL-013-authorization-flags-and-audit, BE-CV-SOL-023-infra-fleet-codeintel-transport
**Status:** [x] DONE

## Context
SOL mục 2.D-E, C5. `auditclient.Append` thiếu `actor_type/target_type` và nuốt lỗi: ghi vào `details`.

## Việc cần làm
1. Audit `quality.security_scan.started` (profile, scope, người); lỗi audit không làm hỏng run nhưng có metric.
2. Cho phép `reason` `network_policy`, `go_modules_unavailable` đi qua bộ lọc dữ liệu lỗi.
3. Kiểm `quality_profile_write` cho `worktree`.

## Kiểm thử
- Cổng giả: audit đúng nội dung; lỗi ENV_NOT_READY tới client với `missingTools[]`.

## Tiêu chí hoàn thành
- [x] Mỗi lần chạy profile bảo mật có đúng một audit.

## Rủi ro
Audit tạm nằm ở `details` tới khi SOL-013 sửa.
