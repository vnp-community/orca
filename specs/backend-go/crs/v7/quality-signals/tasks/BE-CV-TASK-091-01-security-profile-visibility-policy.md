# BE-CV-TASK-091-01: Chính sách hiển thị profile bảo mật (hàm thuần)

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/security_profile_visibility.go`, test (mới)
**Depends on:** BE-CV-SOL-010
**Status:** [x] DONE

## Context
PQ-01(4); SOL mục 2.B, C3, C4. Lọc theo `kind∈{security,dependency}` hoặc tiền tố tên, và theo suite.

## Việc cần làm
1. `IsSecurityProfile`, `SuiteContainsSecurity`, `FilterRunnable(list, suites, enabled)`.
2. Không phụ thuộc tên cuối của CR-081.

## Kiểm thử
- Bảng: kind, tên, suite lách cờ, cờ bật/tắt.

## Tiêu chí hoàn thành
- [x] Chỉ stdlib.

## Rủi ro
Suite mặc định của agent chưa chốt.
