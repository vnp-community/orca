# TASK-V2-018: Thông báo duyệt làm giàu sự kiện không cần actor; lỗi vĩnh viễn không chặn outbox

**From Solution:** (phát hiện khi đọc `docker logs orca-go-request` sau vòng nghiệm thu v2) · **Resolves:** lỗi lặp `REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE ... AUTH_NO_ACTOR`
**Priority:** P1 · **Service/Area:** request-service, auth-service, deploy/dev
**Files:**
- backend-go/proto/orca/auth/v1/auth.proto (+ mã sinh `auth.pb.go`, `auth_grpc.pb.go`)
- backend-go/services/auth-service/internal/usecase/list_active_tenant_admins.go (+ test)
- backend-go/services/auth-service/internal/adapter/grpc/directory_server.go (+ test)
- backend-go/services/auth-service/internal/config/directory.go
- backend-go/services/auth-service/cmd/server/{internal_caller_guards.go,main.go,directory_caller_guard_test.go}
- backend-go/services/request-service/internal/adapter/grpcclient/{admin_directory_resolver.go,team_membership_resolver.go} (+ test)
- backend-go/services/request-service/internal/adapter/eventbus/approval_notification_store.go (+ test)
- backend-go/services/request-service/internal/usecase/enrichment_not_retryable.go (+ test)
- backend-go/services/request-service/internal/config/config.go, cmd/server/{wire_approval.go,readiness_internal_tokens.go} (+ test)
- backend-go/services/request-service/e2e/{harness_test.go,stubs/servers.go}
- deploy/dev/{docker-compose.yml,.env.example,README.md,scripts/ensure-request-env.sh}, docs/guides/request/admin-enable-request-flow.md, README của auth-service và request-service
**Depends on:** TASK-E2E-020 (mẫu `internalcaller` fail-closed và `/readyz` token)
**Status:** `[x]` XONG về mã và test (cập nhật 2026-10-09, chưa commit); **chưa chạy trên stack thật**

---

## Context

`docker logs orca-go-request` lặp mỗi 2 giây:
`approval notification enrichment failed; will retry ... error=REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE: ... list users: rpc error: code = Unauthenticated desc = AUTH_NO_ACTOR`.

Nguyên nhân gốc (có từ commit fe584aeb6, không phải hồi quy gần đây):

1. `ApprovalNotificationStore.FetchUnpublished` làm giàu sự kiện `orca.request.approval.*` trong relay outbox nền. Ngữ cảnh ở đó chỉ có tenant, không có người dùng.
2. Với approver `role:admin`, `AdminDirectoryResolver.ListAdmins` gọi `auth.ListUsers`. RPC này là của admin console: `requireAdminActor` đòi actor là admin (`AUTH_NO_ACTOR` khi không có, `AUTH_NOT_ADMIN` khi actor không phải admin). Hệ quả phụ: cùng đường đó cũng hỏng khi người dùng thường mở approval (kiểm tra đủ điều kiện trong `OpenApproval` gọi `ListAdmins` với actor là người báo cáo).
3. Khi một dòng không làm giàu được, `FetchUnpublished` trả `records[:i]`: mọi dòng sau nó (kể cả sự kiện không liên quan) bị giữ vô hạn. Một lỗi cấu hình/quyền vĩnh viễn làm kẹt cả outbox của request-service, thông báo duyệt không bao giờ phát.

## Thiết kế đã chọn và vì sao

**Hướng ưu tiên: RPC nội bộ chỉ-đọc của auth-service, guard bằng `internalcaller`.**

- Thêm RPC `AuthService.ListActiveTenantAdmins(ListActiveTenantAdminsRequest{}) -> {user_ids[], truncated}` (proto chỉ cộng thêm; `buf breaking` xanh).
- Tenant lấy **chỉ** từ metadata gRPC đã qua `TenantExtractionInterceptor`; request không có trường tenant nên người gọi không thể hỏi tenant khác. Chỉ trả id (không email/tên). Use case lọc lại `TenantID == ctx tenant` (phòng repo lỗi), chỉ giữ `role=admin` và `is_active`.
- Người gọi được xác thực bằng token riêng `AUTH_DIRECTORY_CALLER_TOKEN` qua `internalcaller.Guard`, **token rỗng từ chối mọi lời gọi** (fail closed, đúng quy tắc TASK-E2E-020). Token riêng (không dùng chung OAuth/MCP) để người giữ bí mật khác không đọc được thư mục.
- Token được đọc tách khỏi `OAuthConfig` vì `LoadOAuth` trả cấu hình rỗng khi OAuth tắt, mà request-service cần RPC này bất kể OAuth.
- Cài đặt dạng wrapper `WithDirectory` (cùng mẫu `WithMcp`) để không đổi chữ ký `authgrpc.New` có 40 tham số.
- request-service dùng RPC này **cho mọi** lời gọi `ListAdmins` (relay, mở approval, thông báo clarification), không chỉ khi thiếu actor: một đường duy nhất, không phụ thuộc quyền của người đang gọi. Không nới quyền `ListUsers` (vẫn đòi admin actor).
- Không có token ở request-service: không dial, `admins = UnavailableDirectory` (fail closed, log ERROR), và `/readyz` đỏ khi `REQUEST_APPROVAL_ENABLED` bật và `AUTH_SERVICE_ADDR` có giá trị. Interceptor gắn token nằm riêng trên kết nối thư mục này (các kết nối auth khác của request-service, audit và rollout, không đổi).

**Phương án khác đã cân nhắc, không chọn:** dùng danh tính actor lưu trong sự kiện để gọi `ListUsers`. Bị loại: (a) người tạo sự kiện có thể không phải admin, nên vẫn `AUTH_NOT_ADMIN`; (b) mượn danh tính người dùng cho việc nền là mở rộng quyền ngầm; (c) actor `system` không tồn tại trong auth. Phương án làm cho `ListUsers` chạy được khi thiếu actor bị loại vì nới quyền.

**Đầu hàng chặn (`ApprovalNotificationStore`)** — giữ ngữ nghĩa thứ tự cho lỗi tạm thời, chỉ đổi cho lỗi không thể thử lại:

| Loại lỗi | Hành vi |
|---|---|
| Tạm thời (auth/tenant-service sập, quá hạn, lỗi thường) | như cũ: dừng lô tại dòng đó, các dòng sau chờ, relay thử lại lô ở lần poll kế (giữ thứ tự toàn cục) |
| Không thể thử lại (`PermissionDenied`, `Unauthenticated`, `Unimplemented`, `InvalidArgument`, `NotFound` từ thư mục; địa chỉ/token chưa cấu hình) | dòng đó ở lại outbox (không xuất bản, không mất), các sự kiện sau **của cùng approval** cũng chờ (requested trước decided), các approval khác và sự kiện không liên quan đi tiếp; log ERROR chống lặp (1 lần/phút/dòng). Khi cấu hình được sửa, dòng được giao ở lần poll kế |
| Payload hỏng | như cũ: xuất bản nguyên bản |

Không xuất bản "trần" (không `user_ids`) vì notification-service bỏ sự kiện không có `user_ids`, sẽ mất thông báo vĩnh viễn. Phân loại bằng dấu `usecase.MarkNotRetryable` đặt ở adapter gRPC (`directoryUnavailable`), usecase/store chỉ đọc qua `IsNotRetryable`, nên mã lỗi `REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE` cho API không đổi.

## Việc cần làm

- [x] Proto: `ListActiveTenantAdmins` + 2 message; `buf generate` (protoc-gen-go v1.36.11, protoc-gen-go-grpc 1.6.1); chỉ `auth.pb.go`/`auth_grpc.pb.go` đổi; các file khác chỉ lệch header được trả về nguyên trạng.
- [x] auth-service: use case `ListActiveTenantAdmins`, wrapper gRPC, guard `directoryCallerGuard`, config `DirectoryCallerToken`, nối trong `main.go`.
- [x] request-service: resolver mới, `MarkNotRetryable`/`IsNotRetryable`, phân loại trong `directoryUnavailable`, `UnavailableDirectory` đánh dấu không thể thử lại, `ApprovalNotificationStore` theo bảng trên, config `AUTH_DIRECTORY_CALLER_TOKEN`, `dialApprovalDirectories`, `/readyz`.
- [x] Stub e2e `stubs.Auth` cài RPC mới (đòi token và tenant trong metadata như bản thật), harness truyền token.
- [x] Compose (auth-service và request-service cùng biến), `.env.example`, `ensure-request-env.sh` (sinh `AUTH_DIRECTORY_CALLER_TOKEN` nếu thiếu), README deploy, hướng dẫn bật luồng Request, README hai service.
- [ ] Chạy trên stack thật (redeploy do người điều phối) và xác nhận log hết lỗi lặp, outbox được xả.
- [ ] Commit.

## Tiêu chí hoàn thành

- [x] Sự kiện `orca.request.approval.requested` với approver `role:admin` được làm giàu `user_ids` từ ngữ cảnh chỉ có tenant (test `TestDialApprovalDirectories_AdminsResolveFromBackgroundContext`, qua server gRPC thật có guard + interceptor tenant).
- [x] Token rỗng/sai bị từ chối; tenant khác không đọc được (auth-service `TestDirectoryCallerGuard_*`, usecase `TestListActiveTenantAdmins_NeedsNoActorAndReturnsOnlyActiveAdminsOfTheContextTenant` và `..._DropsRowsOfAnotherTenantEvenIfTheRepositoryReturnsThem`, resolver chặn tham số tenant lệch ctx).
- [x] Lỗi vĩnh viễn không chặn các sự kiện khác; lỗi tạm thời vẫn giữ thứ tự (test store).
- [ ] Trên stack thật: `docker logs orca-go-request` không còn dòng `AUTH_NO_ACTOR`; thông báo duyệt tới được notification-service.

## Kiểm thử

Chạy với `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`:

- `cd backend-go/services/auth-service && go vet ./... && go test ./...` — PASS.
- `cd backend-go/services/request-service && go vet ./... && go test ./...` — PASS.
- `go test -tags integration ./cmd/... ./internal/adapter/eventbus/...` (request-service) — PASS (một lần chạy đầu của `eventbus` FAIL không tái hiện được ở lần chạy sau, đã chạy lại xanh; chưa tìm ra nguyên nhân, nghi tài nguyên container).
- `go test -tags e2e ./e2e/...` (request-service, binary thật + Postgres + NATS + stub) — PASS sau khi cập nhật stub/harness. Trước khi cập nhật, harness FAIL vì `/readyz` đỏ (đúng thiết kế mới: approval bật + `AUTH_SERVICE_ADDR` mà thiếu token).
- `cd backend-go/proto && buf lint` — 307 cảnh báo, không tăng; `buf breaking --against '../../.git#branch=main,subdir=backend-go/proto'` — xanh.
- `go build ./...` ở api-gateway, mcp-service; `backend-go/ci/check-grpc-dials.sh` — ok.

## Kết quả thực thi

Xong về mã và test; **chưa chạy trên stack thật** (cấm chạy compose trong đợt này).

**Blast radius đã rà tay (không có gitnexus/codegraph trong phiên):**
- `usecase.AdminDirectoryResolver.ListAdmins`: gọi ở `expand_approval_recipients.go`, `open_approval.go` (`liveLookup.Admins`), `clarification_notifications.go`; cổng không đổi, chỉ đổi adapter. Cài đặt khác: `contracttest/approval_flow_contract.go` (rig giả, không ảnh hưởng).
- `directoryUnavailable`: chỉ hai resolver trong `grpcclient`. Mã lỗi, `Kind` và thông điệp không đổi; chỉ thêm dấu không-thể-thử-lại trong chuỗi `Err`.
- `UnavailableDirectory`: trả bản sao `AppError` thay vì con trỏ dùng chung; không nơi nào so sánh con trỏ (đã grep), chỉ `errors.As` theo mã.
- `ApprovalNotificationStore`: chỉ `cmd/server/main.go` (bọc outbox khi `ApprovalEnabled`). Thay đổi hành vi duy nhất ngoài lỗi vĩnh viễn: giá trị trả về là bản sao các record thay vì cắt lát cùng mảng.
- `internalTokensConfigured`: chỉ `registerInternalTokenReadiness` (main + test); thêm điều kiện sau hai điều kiện cũ.
- `AuthServiceServer`: mọi nơi dùng đều nhúng `Unimplemented...` hoặc interface, build api-gateway/mcp-service/auth-service/request-service xanh. `stubs.Auth.ListUsers` bị bỏ (không ai gọi nữa).
- auth-service: không đổi repo/SQL, không migration. `internalCallerGuards` giữ nguyên chữ ký; guard mới là phần tử nối thêm vào chuỗi.

**Cần có khi redeploy (người điều phối):**
1. `bash deploy/dev/scripts/ensure-request-env.sh` để sinh `AUTH_DIRECTORY_CALLER_TOKEN` trong `deploy/dev/.env` (hiện chưa có; không in giá trị). Nếu bỏ qua: auth-service từ chối RPC (log WARN), request-service `/readyz` đỏ và container unhealthy sau khoảng 50 giây vì compose đã có healthcheck `/readyz` (TASK-E2E-020) khi `REQUEST_APPROVAL_ENABLED=true`.
2. Build và dựng lại **cả hai** `auth-service` và `request-service` (proto thêm RPC; auth-service phải có RPC trước khi request-service gọi): `redeploy-local.sh auth-service request-service`.
3. Sau khi tạo lại container nhớ restart các service Go giữ IP cũ (script lo).

**Chưa kiểm chứng / rủi ro còn lại:**
- Chưa chạy trên stack thật: chưa xác nhận dòng log lỗi biến mất, outbox cũ (các dòng `approval.*` tồn đọng từ trước) được xả, và notification-service nhận thông báo.
- Các dòng tồn đọng sẽ được làm giàu và phát khi relay chạy lại; thông báo cũ có thể tới muộn (hoặc nhân đôi nếu trước đó đã phát một phần: notification-service khử trùng theo id sự kiện, id không đổi).
- Nếu lô 100 dòng (`outbox.DefaultConfig.BatchSize`) chỉ gồm toàn dòng lỗi vĩnh viễn thì dòng sau lô vẫn bị che cho tới khi sửa cấu hình; log ERROR là tín hiệu. Chưa có metric đếm dòng bị giữ (nên thêm cùng `orca_request_outbox_pending` nếu muốn cảnh báo).
- Thứ tự toàn cục chỉ còn được bảo toàn cho lỗi tạm thời; với lỗi vĩnh viễn thứ tự giữ theo từng approval, không theo toàn outbox (chủ ý, nêu ở trên).
- `tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh` (ngoài phạm vi được phép sửa) chỉ sinh ba token (không có `AUTH_DIRECTORY_CALLER_TOKEN`) cho CI, trong khi bật `REQUEST_APPROVAL_ENABLED true`: cần thêm `AUTH_DIRECTORY_CALLER_TOKEN` vào danh sách khoá của nó, nếu không `/readyz` của request-service trong CI sẽ đỏ.
- Không chạy test tích hợp Postgres/MySQL của repo vì không đổi repo nào.
- Lỗi có sẵn, không do thay đổi này: `LoadOAuth` trả cấu hình rỗng khi OAuth tắt nên `AUTH_MCP_PRINCIPAL_CALLER_TOKEN` không được đọc trong trường hợp đó (không ảnh hưởng RPC mới vì đã đọc tách).
