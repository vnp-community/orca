# BE-CV-TASK-086-16: Kiểm thử tích hợp nhập CI (hai dialect)

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/refresh_ci_run_integration_test.go` (mới, `-tags=integration`)
**Depends on:** BE-CV-TASK-086-14, 086-15
**Status:** [ ] TODO

## Context
Ma trận `dialect: [postgres, mysql]`; scm giả (gRPC) theo kịch bản mục 4 của SOL.

## Việc cần làm
1. Kịch bản: PR có check đỏ + annotation; cache bất biến; `limited`; SHA lệch; pending → hoàn tất (một event); không PR; provider unsupported.
2. Cô lập tenant: tenant B không thấy run CI tenant A.
3. Canary token trong annotation không lọt DB/log/event.

## Kiểm thử
- `go test -tags=integration ./internal/... -v` mỗi dialect.

## Tiêu chí hoàn thành
- [ ] Mọi mục SOL §4 có test.

## Rủi ro
Thời gian TTL: dùng đồng hồ DB chỉnh được trong test.
