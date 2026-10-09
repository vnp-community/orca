# SOL-E2E-003: Một nguồn sự thật cho "project có tồn tại và tôi có vai trò gì", cùng quy ước phân loại lỗi

**Resolves:** BUG-E2E-007, BUG-E2E-009, OBS-A · **Nguyên nhân gốc:** RC-C
**Service:** `project-service`, `request-service`, `task-service`, `proto/orca/project/v1`
**Tasks:** TASK-E2E-013 đến 017
**Quyết định đã chốt (2026-10-09):** đồng ý thêm RPC nội bộ `ResolveProjectAccess` vào `ProjectService`.

## Vấn đề

`request-service` hỏi `project-service` bằng `ListMembers` để suy ra vai trò, và coi mọi từ chối (kể cả "project không tồn tại") là "không phải thành viên". Admin bỏ qua kiểm tra vai trò nên không có bước nào kiểm tra **tồn tại**. `GetProject` cũng không phân biệt được hai trường hợp (trả `PermissionDenied` cho cả hai, hoặc `Internal` khi tra cứu thành viên lỗi). Thử dùng `GetProject` làm kiểm tra tồn tại đã thất bại (xem báo cáo e2e, D9).

Cùng họ lỗi: xoá task không tồn tại trả `Internal` vì repo trả lỗi thô.

## Thiết kế

### 1. RPC nội bộ `ResolveProjectAccess`

Thêm vào `ProjectService` (proto, **cộng thêm**):

```proto
// Internal: guarded by internalcaller. One call answers existence and role,
// so callers stop inferring "missing" from "refused".
rpc ResolveProjectAccess(ResolveProjectAccessRequest) returns (ResolveProjectAccessResponse);
message ResolveProjectAccessRequest  { string project_id = 1; string user_id = 2; }
message ResolveProjectAccessResponse { bool exists = 1; string role = 2; /* owner | member | "" */ }
```

- Tenant lấy từ metadata (G3); project của tenant khác coi như không tồn tại.
- `exists=false` **không phải lỗi**; lỗi thật (DB, timeout) vẫn là lỗi.
- Không lộ tồn tại cho người thường: RPC chỉ gọi được bởi service nội bộ có token, không qua gateway.

### 2. `request-service` dùng nó ở một chỗ

- Thay `ProjectRoleResolver.RoleOf` (heuristic `ListMembers`) bằng adapter gọi `ResolveProjectAccess`; cache TTL ngắn như hiện tại.
- `CreateRequest` thêm cổng tồn tại (port `ProjectAccess`, nối bằng `WithProjectAccess` theo mẫu `With...` đã có). Lỗi mới `REQUEST_PROJECT_NOT_FOUND` (`NotFound`). **Fail-closed** khi `project-service` không trả lời.
- Áp dụng cho **mọi đường tạo**: kênh WS, webhook, MCP, tạo con (`IntakeChildCreator`), không chỉ RPC công khai.

### 3. Quy ước lỗi chung cho "không có"

- Quy tắc: repository trả sentinel của miền (`domain.ErrXxxNotFound`) bọc bằng `%w`; usecase ánh xạ `errors.Is` sang `KindNotFound`. Không bao giờ bọc lỗi repo thành `Internal` mà chưa phân loại.
- `project-service.GetProject` với id lạ trả `NotFound` (khi người gọi là admin hoặc thành viên của tenant); người không có quyền với project có thật vẫn nhận `PermissionDenied`.
- Một test hợp đồng bảng (cross-service) khẳng định `unknown id → NotFound` cho `task`, `project`, `workflow`, `request`.

### 4. Dữ liệu mồ côi

Một truy vấn/ script báo cáo Request có `project_id` không còn trong `project.projects` của tenant; huỷ có lý do `orphan_project` qua đường admin; metric `orca_request_orphan_total`.

## Tương thích

Proto chỉ thêm RPC. `ListMembers` vẫn dùng ở nơi khác. Client cũ không bị ảnh hưởng. Trong lúc `project-service` chưa có RPC mới (triển khai lệch), adapter phát hiện `Unimplemented` và **giữ hành vi cũ** kèm log cảnh báo, để không chặn tạo Request.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Thêm một lời gọi vào đường tạo Request (độ trễ, phụ thuộc) | cache TTL 30 giây; timeout 2 giây; fail-closed có thông báo rõ |
| Webhook từ nguồn tin cậy bị chặn vì project đã xoá | trả `NotFound` có mã, nguồn thấy lỗi rõ ràng; không thử lại vô hạn |
| Đổi mã lỗi `GetProject` ảnh hưởng frontend | frontend đã ánh xạ theo `code`; thêm trường hợp trong [SOL-FE-E2E-003](../../../../../frontend/bugs/e2e-acceptance-v1/solutions/SOL-FE-E2E-003-error-taxonomy-and-rate-limit.md) |

## Tiêu chí hoàn thành

- e2e `RQL-05` xanh (đang là FAIL duy nhất).
- `project.get` id lạ trả `NotFound`.
- Test hợp đồng bảng xanh cho 4 service.
- Có báo cáo Request mồ côi và quy trình xử lý cho dữ liệu cũ.
