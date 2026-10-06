# BE-CV-TASK-091-06: Kiểm thử tích hợp cờ quét bảo mật (hai dialect)

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/security_flag_integration_test.go` (mới, `-tags=integration`)
**Depends on:** BE-CV-TASK-091-02, 091-03, 091-05, BE-CV-SOL-073-settings-flag-and-rollout
**Status:** [ ] TODO

## Context
Ma trận `dialect: [postgres, mysql]`; `SetSettings` (admin) bật/tắt cờ; cache ≤ 5 s.

## Việc cần làm
1. Cờ tắt/bật/lỗi đọc; bật `quality_security_scan_enabled` khi `quality_gate_enabled` tắt.
2. Ingest finding bảo mật qua đường thật (agent giả) và kiểm sanitize ở DB.
3. Cô lập tenant: tenant A bật không ảnh hưởng tenant B.

## Kiểm thử
- `go test -tags=integration ./... -v` mỗi dialect.

## Tiêu chí hoàn thành
- [ ] Mọi mục SOL §4 có test.

## Rủi ro
Hiệu lực cờ ≤ 5 s: test dùng đồng hồ giả cho cache.
