# BE-CV-TASK-091-03: Hậu xử lý finding bảo mật khi nạp

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/security_finding_rules.go`, `backend-go/services/code-intel-service/internal/usecase/security_finding_sanitizer.go`, test (mới)
**Depends on:** BE-CV-TASK-082-08
**Status:** [ ] TODO

## Context
SOL mục 2.C, C6; PQ-26 regex; H8.

## Việc cần làm
1. Tiền tố `ruleId` cho phép; ngoài ra bỏ + đếm.
2. `SEC-SECRET/*`: thay `message`/`fix_hint` bằng bảng cố định theo loại; `col=end_col=0`.
3. Đăng ký như `FindingPostProcessor` trong `IngestQualityRun`.

## Kiểm thử
- Bảng loại bí mật; loại lạ ⇒ `unknown`; `GOVULN`/`OSV` giữ message đã che.

## Tiêu chí hoàn thành
- [ ] Không còn văn bản gốc agent cho `SEC-SECRET` sau bước này.

## Rủi ro
Bảng loại bí mật phải khớp parser agent.
