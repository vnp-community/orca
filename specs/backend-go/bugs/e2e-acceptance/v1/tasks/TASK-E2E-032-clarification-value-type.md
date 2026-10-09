# TASK-E2E-032: Trường `value_type` cho câu hỏi Clarification

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-B
**Priority:** P2 · **Service/Area:** proto, request-service
**Files:**
- backend-go/proto/orca/request/v1/clarification.proto
- backend-go/services/request-service/internal/domain (luật kiểm tra trường đích)
- backend-go/services/request-service/internal/adapter/grpc/clarification_mapper.go
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_clarification.go
**Depends on:** TASK-E2E-001
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`type_fields.time_box_hours` có `kind: text` nhưng chỉ nhận chuỗi số; UI không biết.

## Việc cần làm

- [x] Thêm `string value_type` (`string|number|boolean|list`) vào `ClarificationQuestion` (cộng thêm, G2).
- [x] Suy ra từ luật kiểm tra đang dùng cho `target_path`; không cần đổi luật.
- [x] Gateway: ánh xạ sang `valueType`.
- [x] Test: `time_box_hours` → `number`; `data_exposed` → `boolean`; mặc định `string`.

## Tiêu chí hoàn thành

- [x] `MTX-13` vẫn xanh; frontend nhận `valueType` (TASK-FE-E2E-003).

## Kiểm thử

Unit test ánh xạ; gateway test.

## Kết quả thực thi

Xong. `ClarificationQuestion.value_type` (proto cộng thêm, sinh lại bằng protoc), suy từ luật kiểm tra trường đích (`time_box_hours`→number, `data_exposed`→boolean, `repro_steps`→list, mặc định string); gateway `valueType`; test ở domain, mapper, gateway.
