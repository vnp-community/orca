# BE-CV-TASK-012-06: Use case và handler gRPC `BindRepo`, `ListRepoBindings`

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/{bind_repo.go,list_repo_bindings.go}`, `internal/adapter/grpc/binding_server.go` (+ `_test.go`) (mới)
**Depends on:** BE-CV-TASK-012-02, 012-05; `GetIndexStatus` usecase (012-11) cho phần `status`
**Status:** [ ] TODO

---

## Context

Hợp đồng §3.1: `BindRepo` (`read`; trả `{binding, status}`), `ListRepoBindings` (`read`; `project_id`, `limit ≤ 200`, không `selector`, không kênh WS). Pipeline kiểm cờ/quyền/hạn mức do SOL-013 bọc ngoài; handler chỉ gọi use case.

## Việc cần làm

1. `BindRepo.Execute(ctx, sel)`: `ResolveTarget` → binding → thăm dò trạng thái (dùng `GetIndexStatus` use case của 012-11 với `refresh=false`; trước khi 012-11 xong dùng cổng `IndexStatusProber` giả) → trả. Audit `codeintel.bind` khi **tạo mới** binding (SOL-013 cung cấp `AuditRecorder`; cổng tiêm, nil an toàn).
2. `ListRepoBindings.Execute(ctx, projectID, limit)`: kiểm quyền do pipeline; `limit` mặc định 100, tối đa 200, ngoài khoảng `CODEINTEL_INVALID_PARAMS`; chỉ trả binding đã có + `last_status` đã lưu (không thăm dò dev server).
3. Handler: ánh xạ proto ↔ domain; **không** đưa `workspace_root`/`dev_server_id` vào log; mapping lỗi qua hàm chung của SOL-013 (`toStatus`).
4. Cập nhật bảng "thật / chưa làm" trong README service.

## Kiểm thử

- Unit: handler với use case giả; `limit` ngoài khoảng; `BindRepo` lần hai (idempotent) không audit lại; `ListRepoBindings` không gọi dev server (fake đếm).
- `go test ./services/code-intel-service/internal/...  -run 'BindRepo|ListRepoBindings'`

## Tiêu chí hoàn thành

- [ ] Hai RPC hoạt động với fake; không gọi agent từ `ListRepoBindings`.
- [ ] Không rò đường dẫn tuyệt đối vào log.

## Rủi ro và lưu ý

- Gateway phải bỏ `workspaceRoot`/`devServerId` khỏi view UI (SOL-040).
