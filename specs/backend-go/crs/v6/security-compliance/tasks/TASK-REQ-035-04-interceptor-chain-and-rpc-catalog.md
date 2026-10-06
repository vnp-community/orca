# TASK-REQ-035-04: Chuỗi interceptor (`Guard`, `ActorType`, `RequestAccess`) và danh mục RPC → nhóm hành động

**From Solution:** BE-REQ-SOL-035 (mục B, C)
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/{rpc_catalog.go,request_access.go}` (mới), `.../internal/usecase/authorize_request_action.go` (mới), `.../internal/adapter/grpc/{interceptors.go,actor_type.go,request_access.go,stream_interceptors.go}` (mới), `.../internal/adapter/grpcclient/project_role_resolver.go` (mới), `.../cmd/server/main.go` (sửa), `.../internal/config/config.go` (sửa: `GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN`) và `_test.go` tương ứng
**Depends on:** TASK-REQ-035-02 (`tenant.ActorType`), TASK-REQ-035-03 (`RequestPolicy`), BE-REQ-SOL-001, 002 (`RequestRepository`), TASK-REQ-025-02 (`flow_gate`, cùng danh mục), BE-REQ-SOL-009 (`ApprovalRepository` để định vị theo `approval_id`)
**Status:** `[ ] TODO`

---

## Context

- `internalcaller.Guard(expectedToken, fullMethods...)` (`common/internalcaller/internalcaller.go`): lọc theo **tên đầy đủ** phương thức, token rỗng ⇒ từ chối hết, so `subtle.ConstantTimeCompare`; có `StreamGuard` cho stream. Một token mỗi thể hiện ⇒ cần hai thể hiện (solution C2).
- `grpcmw.ChainUnary(logger)` đã gồm recovery → `TenantExtractionInterceptor` → logging. Mọi thứ của task này đi **sau** nó bằng `grpc.ChainUnaryInterceptor(...)` thứ hai (gRPC-Go ghép nhiều tuỳ chọn theo thứ tự khai báo). `TenantExtractionInterceptor` **tin metadata** vô điều kiện (comment trong `grpcmw.go`), nên `Guard` là lớp bảo vệ duy nhất cho tới khi có mTLS.
- `TASK-REQ-025-02` định nghĩa `flowMethodClass` (lớp: đọc, thoát an toàn, đi tiếp, nội bộ) và test quét `RequestService_ServiceDesc`/`ApprovalService_ServiceDesc`. Danh mục RPC của task này có một chiều khác (nhóm quyền) nhưng **cùng tập khoá**: test đối chiếu hai bảng.
- `project-service` `ListMembers` (proto dòng 17) và mẫu tra vai trò ở `project-service/internal/usecase/authorization.go:109` (không có dòng thành viên ⇒ `callerProjectRole = ""`). Proto thật: đọc `proto/orca/project/v1/project.proto` để chọn message/field (`project_id`, danh sách thành viên có `user_id`, `role`).
- Không phân biệt "không tồn tại" và "thuộc tenant khác" khi định vị Request (CR 2.2 bước 3): luôn `NOT_FOUND` `REQUEST_NOT_FOUND`.

## Việc cần làm

1. `rpc_catalog.go` (domain): kiểu `Group`, `Locator`, `Entry{Group Group; Locator Locator; RateClass string; AgentAllowed bool}`, biến `Catalog map[string]Entry` điền theo bảng ở solution mục B với tên đầy đủ `/orca.request.v1.RequestService/<Rpc>`, `/orca.request.v1.ApprovalService/<Rpc>`, `/orca.request.v1.AiBudgetAdminService/<Rpc>`. `RateClass` ∈ `read|write|ai|webhook|none` theo CR 2.7 (`GenerateSolution`, `GeneratePlan`, `ClassifyRequest`, `StartPhase` là `ai`; nhóm `create`, `triage`, `lifecycle`, `decide` là `write`; `read` là `read`).
2. `func ValidateCatalog(descs ...*grpc.ServiceDesc) error`: mọi phương thức trong `descs` (unary và stream) phải có `Entry`; mọi khoá của `Catalog` phải tồn tại trong `descs`; trả danh sách thiếu và thừa. `main.go` gọi lúc khởi động và `log.Fatal` nếu lỗi (tiêu chí chấp nhận 3 của CR).
3. `func PublicMethods() []string` và `func InternalMethods() []string` suy ra từ `Catalog` (`Group == internal` thuộc danh sách nội bộ). `Guard` nhận các danh sách này, không gõ tay.
4. `config.go`: `GatewayInternalToken` (`GATEWAY_INTERNAL_TOKEN`), `ServiceInternalToken` (`SERVICE_INTERNAL_TOKEN`). Rỗng: dịch vụ vẫn khởi động nhưng `Guard` từ chối hết (log cảnh báo to ở khởi động); **không** bật chế độ bỏ qua cho dev (dev đặt token trong compose).
5. `actor_type.go`: `ActorTypeInterceptor()` đọc `metadata` `grpcmw.MetadataActorType`, `ctx = tenant.WithActorType(ctx, v)` (chuẩn hoá bởi `WithActorType`: lạ/thiếu ⇒ `user`); hàm tương đương cho stream (`streamIdentity`) bọc `grpc.ServerStream` để đổi `Context()` (struct `wrappedStream{grpc.ServerStream; ctx}`) và trích **cả** tenant/user/role/IP (vì `TenantExtractionInterceptor` không phủ stream).
6. `domain/request_access.go`: `type AccessInput{Method string; Entry Entry; ProjectRole string; GlobalRole string; IsReporter bool; ActorType string}` và `type AccessDecision{Allowed bool; Reason string}`; hàm thuần `RPCName(fullMethod string) string` (phần sau dấu `/` cuối) để đưa vào Rego `input.rpc`.
7. `usecase/authorize_request_action.go`: `AuthorizeRequestAction{requests RequestRepository; approvals ApprovalLocator; roles ProjectRoleResolver; policy RequestPolicy; audit AuditRecorder}` với `Authorize(ctx, method string, req any) error`: (a) `Entry` từ `Catalog`; (b) `internal` ⇒ chỉ qua nếu `Guard` đã cho (ở đây luôn `nil`); (c) `authenticated`: cần `tenant.RequireTenantID` và `tenant.UserID`; (d) `admin`: `tenant.Role(ctx)=="admin"` và `ActorType != agent`; (e) các nhóm còn lại: định vị theo `Locator` (`req` hỗ trợ getter `GetRequestId()`, `GetApprovalId()`, `GetProjectId()` qua interface; không có getter ⇒ `LocNone`); `LocRequestID` ⇒ `requests.Get` theo tenant (không thấy ⇒ `NOT_FOUND`), lấy `project_id`, `reporter_id`; `LocApprovalID` ⇒ `ApprovalLocator.RequestIDOf(approvalID)` rồi như trên; `LocProjectID` ⇒ `project_id` từ thân; (f) `roles.RoleOf(ctx, tenant, project, user)` (rỗng nếu không là thành viên); (g) `policy.Decision(...)`; `false` ⇒ `ErrForbidden` (`REQUEST_FORBIDDEN`, `PermissionDenied`) và `audit.Record(request.access.denied, outcome=denied)` (metadata chỉ `rpc`, `request_id`, `actor_type`); lỗi policy ⇒ `Internal` (không cho qua).
8. `project_role_resolver.go`: `ProjectRoleResolver{client projectv1.ProjectServiceClient; cache}`: `RoleOf(ctx, tenantID, projectID, userID) (string, error)` gọi `ListMembers` (chuyển tiếp metadata tenant theo mẫu `tenant_forwarding.go` của `task-service`), tìm `userID`; cache theo `(tenant, project, user)` TTL 30 giây (`sync.Map` + thời điểm; dọn khi truy cập); `ProjectsOf(ctx, user)` cho lọc danh sách (dùng `ListProjects`; đọc proto lúc làm). Lỗi gọi `project-service` ⇒ `Unavailable` (không suy là "không có quyền" im lặng, nhưng cũng không cho qua).
9. `interceptors.go`: `RequestAccessInterceptor(a *AuthorizeRequestAction) grpc.UnaryServerInterceptor` gọi `Authorize`; `RPC` thuộc `ListRequests`/`ListBacklog` không lọc theo dự án: qua cho nhóm `read` và đặt vào ctx cờ `FilterToMemberProjects` để usecase (CR-REQ-015) chỉ trả dự án mà người gọi là thành viên (admin xem hết). Chuỗi đầy đủ ở `main.go` theo solution mục B; đọc `TASK-REQ-025-02` để `flowGate` đứng trước `RequestAccessInterceptor`.
10. `stream_interceptors.go`: `StreamGuard` cho phương thức stream (hiện chỉ `ExportTenantRequests`, nhóm `admin`), `streamIdentity`, `streamAccess` (`Authorize` với `req = nil` cho nhóm `admin`).

## Kiểm thử

- `rpc_catalog_test.go`: `TestValidateCatalog_AllMethodsClassified` (từ `RequestService_ServiceDesc`, `ApprovalService_ServiceDesc`, `AiBudgetAdminService_ServiceDesc`); `TestValidateCatalog_DetectsMissing` (thêm RPC giả vào desc ⇒ lỗi); `TestCatalogMatchesFlowGate` (cùng tập khoá với `flowMethodClass` của 025-02); `TestCatalogMatchesRego` (đọc `group_roles` từ `request.rego` bằng `opa`/regexp và đối chiếu mọi `Group` trừ `decide|authenticated|admin|internal`).
- `authorize_request_action_test.go` (fake repo, fake policy): đủ `admin`, `owner`, `member`, `reporter`, người lạ, `agent` cho **mỗi RPC** trong bảng (sinh test từ `Catalog`: với từng RPC chọn nhóm rồi kiểm theo ma trận `want[group][role]`); `x-orca-role` rỗng ⇒ không admin; `Request` của tenant khác ⇒ `NOT_FOUND` (cùng mã với không tồn tại); policy lỗi ⇒ `Internal`; chỉ khi `Allowed` thì handler được gọi.
- `actor_type_test.go`: metadata `agent` ⇒ ctx `agent`; thiếu ⇒ `user`; `root` ⇒ `user`.
- `interceptors_test.go` (`bufconn`): không có `x-orca-internal-token` ⇒ `PermissionDenied` `INTERNAL_CALLER_REQUIRED`; token gateway gọi RPC nội bộ ⇒ bị từ chối (token service mới được); token cấu hình rỗng ⇒ từ chối tất cả; `agent` gọi `Approve`, `StartPhase`, `SetRequestFlowSettings` ⇒ `PermissionDenied`.
- `project_role_resolver_test.go`: cache 30 giây (đồng hồ giả): thành viên bị gỡ vẫn còn trong cache tới khi hết hạn (ghi vào test để nhắc rủi ro).
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... ./services/request-service/internal/adapter/grpcclient/...`.

## Tiêu chí hoàn thành

- [ ] Gọi RPC công khai không có token gateway đúng bị từ chối; token cấu hình rỗng từ chối tất cả.
- [ ] Khởi động thất bại nếu có RPC chưa có trong `Catalog`.
- [ ] Mỗi RPC có test phân quyền đủ `admin|owner|member|reporter|stranger|agent`.
- [ ] `agent` gọi `Approve`, `StartPhase`, `SetRequestFlowSettings` nhận `PermissionDenied`.
- [ ] "Không có" và "tenant khác" đều trả `REQUEST_NOT_FOUND`.

## Ví dụ tham khảo

Một đoạn `Catalog` (ví dụ, tên RPC thật đọc từ proto lúc làm):

```go
var Catalog = map[string]Entry{
    "/orca.request.v1.RequestService/GetRequest":       {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
    "/orca.request.v1.RequestService/CreateRequest":    {Group: GroupCreate, Locator: LocProjectID, RateClass: "write", AgentAllowed: true},
    "/orca.request.v1.RequestService/ConfirmRequestType": {Group: GroupTriage, Locator: LocRequestID, RateClass: "write"},
    "/orca.request.v1.RequestService/StartPhase":       {Group: GroupExecute, Locator: LocRequestID, RateClass: "ai"},
    "/orca.request.v1.ApprovalService/Approve":         {Group: GroupDecide, Locator: LocApprovalID, RateClass: "write"},
    "/orca.request.v1.RequestService/ReportTaskOutcome": {Group: GroupInternal, RateClass: "none"},
}
```

Ma trận kỳ vọng dùng chung với task 03: bảng nhóm × vai trò nằm trong `testdata/access_matrix.json` để cả test Go và `request_test.rego` đọc cùng một nguồn.

## Rủi ro và lưu ý

- Nếu mTLS và NetworkPolicy chưa bật, `Guard` là lớp bảo vệ duy nhất; token chung có thể lộ (CR mục 6).
- Cache vai trò 30 giây: thành viên vừa bị gỡ còn làm được vài thao tác (cần xác nhận chấp nhận).
- Gọi `project-service` trong đường nóng của mỗi RPC thêm độ trễ; cache giảm nhưng lần đầu vẫn tốn một RPC (chưa đo).
- `ListMembers` có thể trả danh sách lớn với dự án đông người; nếu có RPC tra thành viên đơn lẻ (kiểm proto) thì dùng nó.
- Nhóm của `RecordRequestCheck`, `RequestApproval` là suy luận; xác nhận khi proto CR-014, 009 đã chốt.
