# BE-CV-TASK-091-04: Test canary rò rỉ bí mật (ingest → DB, log, outbox, lỗi)

**From Solution:** BE-CV-SOL-091-security-scan-flag-and-ingest
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/security_canary_test.go`, `internal/adapter/*/security_canary_integration_test.go` (mới)
**Depends on:** BE-CV-TASK-091-03, BE-CV-SOL-072-security-tests-service-gateway
**Status:** [x] DONE

## Context
CR-091 2.5: chuỗi canary không được xuất hiện ở bất kỳ đầu ra nào. Mở rộng CR-072 "che secret".

## Việc cần làm
1. Agent giả nhúng canary vào `message`, `fixHint`, `file`, `ruleId`, `stepId`, lỗi.
2. Quét: truy vấn DB hai dialect, log capture (slog handler), outbox payload, thông điệp lỗi gRPC, span attribute.
3. Cả đường lỗi: agent crash, timeout, huỷ.

## Kiểm thử
- `go test ./... && go test -tags=integration` hai dialect.

## Tiêu chí hoàn thành
- [x] Không chuỗi canary ở bất kỳ nơi nào.

## Rủi ro
Bắt log cần handler thử nghiệm đồng nhất với cấu hình thật.
