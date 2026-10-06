# TASK-REQ-016-07: Năm route HTTP `/v1/requests`, `/v1/approvals` và cập nhật README gateway

**From Solution:** BE-REQ-SOL-016
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/request_routes.go` (mới), `.../request_routes_test.go` (mới), `.../router.go`, `backend-go/services/api-gateway/README.md`, `backend-go/services/api-gateway/internal/usecase/requestsource.go` (mới)
**Depends on:** TASK-REQ-016-02, TASK-REQ-016-03, TASK-REQ-016-04
**Status:** `[ ] TODO`

---

## Context

- `router.go:55` field `TaskClient`; `:187-188` `if deps.TaskClient != nil { mountTaskRoutes(authed, deps.TaskClient) }`. Mẫu mount có điều kiện.
- `task_routes.go:19` `mountTaskRoutes(r chi.Router, client taskv1.TaskServiceClient)`; lỗi JSON trả `INVALID_ARGUMENT`; `usage_routes.go:182` `writeGRPCError(w, err)`.
- Route: CONTRACT mục 4 (năm route). Body approve: `{version, digest, comment}`.
- Luật nguồn dùng chung với WS (SOL-016 mục 2.7, Q4): hàm thuần để `httpgateway` không import `wscompat`.

## Việc cần làm

1. `internal/usecase/requestsource.go`: chuyển lõi `resolveRequestSource` (không phụ thuộc `ctx` kiểu `ToolOrigin`) thành `ResolveSource(in SourceInput, origin *OriginInput) (Resolved, error)`; `wscompat` và `httpgateway` cùng gọi. Route HTTP không có `ToolOrigin` nên `origin=nil`.
2. `request_routes.go`: `mountRequestRoutes(r chi.Router, req requestv1.RequestServiceClient, appr requestv1.ApprovalServiceClient)` với `POST /v1/requests`, `GET /v1/requests`, `GET /v1/requests/{id}`, `GET /v1/approvals/pending`, `POST /v1/approvals/{id}/approve`, `POST /v1/approvals/{id}/reject`. Danh tính lấy từ middleware (như `task_routes.go`), không từ body. Đổi view dùng lại cấu trúc view (đặt view ở gói dùng chung giữa `wscompat` và `httpgateway`, ví dụ `internal/usecase/requestview.go` (mới), thay vì sao chép).
3. `router.go`: thêm `RequestClient`, `ApprovalClient` vào `Deps`; mount sau `mountTaskRoutes`, có điều kiện `!= nil`.
4. `README.md` của api-gateway: thêm `request-service` (biến `REQUEST_SERVICE_ADDR`), danh sách kênh mới, năm route, cảnh báo "không OPA trước định tuyến: `request-service` tự kiểm quyền".
5. `cmd/server/main.go`: truyền hai client vào `httpgateway` deps.

## Kiểm thử

- `request_routes_test.go` (mẫu `task_routes_test.go:143` `taskTestRouter`): 5 route thành công; JSON hỏng thì 400 `INVALID_ARGUMENT`; chưa xác thực thì 401; lỗi gRPC thành mã HTTP qua `writeGRPCError`; `POST /v1/requests` với `source.provider=manual` hoặc `mcp` thì từ chối; body có `tenantId` giả bị bỏ qua.
- Test hồi quy: `go test ./internal/adapter/httpgateway/...`.

## Tiêu chí hoàn thành

- [ ] Năm route chạy, có test.
- [ ] `tenant_id` không đọc từ body.
- [ ] README gateway cập nhật; không còn mô tả "request-service không tồn tại".

## Rủi ro và lưu ý

- Hai bản view (WS và HTTP) dễ lệch; dùng chung một gói là bắt buộc.
- Route phê duyệt bằng HTTP vẫn cần `digest`; client script cũ theo CR (chỉ `version`) sẽ nhận `INVALID_ARGUMENT`; ghi vào README.
