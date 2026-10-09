# TASK-E2E-001: Chốt và commit kênh gateway `clarification.*`

**From Solution:** SOL-E2E-001 · **Resolves:** BUG-E2E-001
**Priority:** P0 · **Service/Area:** api-gateway
**Files:**
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_clarification.go (đã có)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_clarification_test.go (đã có)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request.go
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_test.go
- backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml
- specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md (thêm mục kênh)
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Bốn kênh `clarification.list/get/answer/cancel` đã được thêm và test xanh trong working tree nhưng chưa commit. Frontend `useClarifications.ts` đã gọi sẵn các kênh này.

## Việc cần làm

- [ ] Chạy `gitnexus impact` cho `registerRequestChannels`; ghi blast radius vào PR (G10). — _chưa chạy được: không có công cụ gitnexus trong phiên_
- [x] Thêm mục kênh `clarification.*` vào CONTRACT-request-ui-api.md (tên kênh, tham số, hình dạng đáp ứng camelCase, mã lỗi).
- [x] Xác nhận `clarification.*` vẫn nằm trong `excluded_channels.yaml` với lý do "việc của người".
- [ ] Commit cùng test; không gộp với thay đổi khác. — _chưa commit_

## Tiêu chí hoàn thành

- [x] `go test ./internal/adapter/wscompat/ ./internal/adapter/mcpserver/...` xanh.
- [x] e2e `RQL-14..17` xanh trên stack thật.
- [x] CONTRACT có mục mới.

## Kiểm thử

Đã có: `TestClarificationViewOf_UsesFrontendShape`, `TestClarificationStatusFilter`, `TestRequestChannels_Inventory`.

## Kết quả thực thi

Xong. Mục `2.6 clarification.*` thêm vào CONTRACT-request-ui-api.md (kèm mã lỗi `REQUEST_CLARIFICATION_*`); `clarification.*` còn trong `excluded_channels.yaml`. e2e RQL-14..17 PASS. `gitnexus impact` chưa chạy được (không có công cụ).
