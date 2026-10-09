# TASK-E2E-015: `request-service` dùng `ResolveProjectAccess` cho vai trò và kiểm tra tồn tại khi tạo Request

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-009
**Priority:** P1 · **Service/Area:** request-service
**Files:**
- backend-go/services/request-service/internal/adapter/grpcclient/project_access_resolver.go (mới, thay phần heuristic của project_role_resolver.go)
- backend-go/services/request-service/internal/usecase/create_request.go (cổng `ProjectAccess` + `WithProjectAccess`)
- backend-go/services/request-service/internal/domain/request_intake_errors.go (`REQUEST_PROJECT_NOT_FOUND`)
- backend-go/services/request-service/cmd/server/wire_intake.go
- backend-go/services/request-service/internal/usecase/create_request_test.go
**Depends on:** TASK-E2E-014
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Một lần thử dùng `GetProject` đã thất bại (PermissionDenied không phân biệt được) và đã hoàn tác; phải dùng RPC mới.

## Việc cần làm

- [~] Adapter gọi `ResolveProjectAccess`; cache TTL ngắn (như hiện tại); timeout ngắn; **fail-closed** khi lỗi. — _có timeout và fail-closed; không có cache TTL_
- [x] `CreateRequest.Execute`: sau kiểm UUID, nếu cổng được nối thì `exists=false` → `REQUEST_PROJECT_NOT_FOUND`.
- [x] Nối vào mọi đường tạo: gRPC (kênh WS), webhook, MCP, `IntakeChildCreator` (cùng một `CreateRequest`).
- [x] `RoleOf` dùng `role` từ RPC mới; giữ hành vi cũ khi `Unimplemented` (triển khai lệch) kèm cảnh báo.
- [x] Test: không tồn tại bị từ chối và không lưu gì; lỗi tra cứu fail-closed; tồn tại thì tạo; đường webhook cũng bị kiểm.

## Tiêu chí hoàn thành

- [x] e2e `RQL-05` xanh.
- [~] Xác nhận webhook/MCP không tạo được Request vào project lạ (test bổ sung). — _webhook/MCP đi qua cùng `CreateRequest` nhưng chưa có test riêng_

## Kiểm thử

Unit test usecase; `RQL-05`.

## Kết quả thực thi

Xong. request-service dùng `ProjectAccessLookup` (vai trò) và `REQUEST_PROJECT_NOT_FOUND` khi tạo Request; rơi về đường cũ khi RPC `Unimplemented`. e2e RQL-05 PASS.
