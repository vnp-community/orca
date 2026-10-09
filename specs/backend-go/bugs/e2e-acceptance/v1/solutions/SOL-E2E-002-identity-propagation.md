# SOL-E2E-002: Truyền danh tính (tenant, user, actor) một lần, ở một chỗ, cho mọi bước đồng bộ và bất đồng bộ

**Resolves:** BUG-E2E-003, BUG-E2E-006, BUG-E2E-008 · **Nguyên nhân gốc:** RC-B
**Service:** `common/grpcmw`, `common/tenant`, `project-service`, `workflow-service`, `task-service`, `request-service`, `infra-fleet-service`, `api-gateway`
**Tasks:** TASK-E2E-006 đến 012

## Vấn đề

Mỗi adapter gọi gRPC sang service khác phải tự nhớ gắn metadata tenant/user. Có hai mẫu thất bại:
1. **Quên gắn** (`IsReachable`, `DevServerRelay`): service đích trả `NO_TENANT` / `NO_USER`.
2. **Mất ngữ cảnh khi chuyển sang bất đồng bộ** (`context.Background()` chỉ cộng tenant): danh tính biến mất giữa chừng; khôi phục sau khởi động lại cũng không có.

Thêm vào đó gateway không đưa danh tính vào miền (task thiếu `CreatorId`), nên miền không có người sở hữu.

## Thiết kế

### 1. Một interceptor máy khách dùng chung

Thêm vào `common/grpcmw`: `ClientIdentityInterceptor()` (unary) và `ClientIdentityStreamInterceptor()`. Với mỗi lời gọi đi, đọc từ `ctx` cái mà interceptor máy chủ (`TenantExtractionInterceptor`) đã đặt vào: `tenant.TenantID`, `tenant.UserID`, `tenant.Role`, `tenant.ActorType`, rồi gắn đúng các khoá metadata mà phía nhận đang đọc (`MetadataTenantID`, `MetadataUserID`, …).

Quy tắc: **không ghi đè** metadata đã có sẵn trong `ctx` ra ngoài (adapter vẫn có thể cố ý đặt danh tính khác, ví dụ `asUser`); chỉ điền chỗ trống. Không có tenant trong `ctx` thì **không tự bịa**, để phía nhận từ chối như cũ (fail-closed).

Cung cấp `grpcmw.ClientDialOptions()` trả `[]grpc.DialOption` đã có interceptor (và `otelgrpc`), và quy định mọi `grpc.NewClient` đi qua nó. Một script lint (`backend-go/ci/check-grpc-dials.sh`) liệt kê chỗ còn dial thủ công.

Các hàm `withTenantMetadata` hiện có trong từng service được giữ ở bước đầu (không đổi hành vi), rồi xoá dần sau khi interceptor đã áp dụng và test pass, để không tạo thay đổi lớn một lần.

### 2. Tác vụ nền giữ danh tính, tách huỷ

Thay mẫu `tenant.WithTenantID(context.Background(), id)` bằng `context.WithoutCancel(ctx)` (Go 1.21+, repo dùng 1.25/1.26): giữ **mọi giá trị** (tenant, user, actor, trace) nhưng bỏ tín hiệu huỷ của RPC đã trả. Gói thành `tenant.Detach(ctx)` để có một tên và một chỗ ghi chú lý do. Áp dụng cho `workflow-service` `Execute` và các điểm khác tìm thấy bằng `grep` (task execution, automation, outbox consumer).

### 3. Khôi phục workflow giữ người chạy

Lưu `started_by` trên `workflow.executions` (migration Postgres và MySQL). `RecoverExecutions` dựng context với người đó. Execution cũ không có cột: chạy tiếp với tenant-only (hành vi hiện tại), có log cảnh báo.

### 4. Miền có chủ sở hữu thật

`task-service` `CreateTask` tự suy ra người tạo từ danh tính gRPC (G3), **bỏ qua** `creator_id` do client gửi, trừ khi lời gọi mang token nội bộ (ví dụ AIApply, tạo từ nguồn). Gateway vẫn truyền `CreatorId` (đã sửa) để đường nội bộ tương thích.

### 5. Vá dữ liệu

Migration idempotent: với mỗi task thiếu grant `owner` của `owner_id`/người tạo, thêm grant. Chạy cho cả hai dialect; có chế độ đếm trước (dry-run) trong runbook.

## Tương thích

- Interceptor chỉ **thêm** metadata còn thiếu: không đổi hành vi nơi đã đúng. Rủi ro lớn nhất là service nhận bắt đầu thấy `user` ở chỗ trước đây không có và đổi hành vi theo (ví dụ kiểm quyền thành viên). Giảm thiểu: bật theo từng service, chạy `orca-flows` sau mỗi bước.
- Không đổi proto.
- `CreateTask` bỏ qua `creator_id` từ client công khai: client chính thống đều lấy từ gateway, không ảnh hưởng.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Interceptor rò danh tính sang đích không nên nhận | chỉ dial service nội bộ cùng tenant; danh sách dial được rà bởi script lint |
| Trùng khoá metadata gây giá trị kép | chỉ thêm khi chưa có; test bảng |
| `WithoutCancel` giữ giá trị lâu hơn dự kiến (rò bộ nhớ) | context chỉ mang vài chuỗi nhỏ; không giữ request body |
| Migration grant chạy lâu trên bảng lớn | phân lô theo `id`, có thể chạy lại |

## Tiêu chí hoàn thành

- Không còn `grpc.NewClient` ngoài helper (script lint xanh), trừ danh sách ngoại lệ có lý do.
- Unit test chứng minh một lời gọi đi mang tenant, user, actor; không ghi đè metadata có sẵn; không bịa tenant.
- Khởi động lại `workflow-service` giữa một execution có bước `project:` vẫn hoàn tất (test tích hợp).
- Task tạo trước migration có cây con không rỗng sau migration.
- e2e `DEV-04..06`, `WFL-12`, `WFL-13`, `TSK-07` xanh; thêm một e2e khởi động lại giữa chừng.
