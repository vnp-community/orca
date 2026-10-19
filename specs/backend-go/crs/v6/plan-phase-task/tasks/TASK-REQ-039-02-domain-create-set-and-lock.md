# TASK-REQ-039-02: Miền `WorktreeBaseRef`, `CreateRequest`, `SetRequestBaseRef` và khoá

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/{worktree_base_ref,base_ref_errors}.go` (mới), `internal/usecase/{set_request_base_ref,create_request,worktree_presence,base_ref_checker}.go` (mới/sửa), `internal/adapter/{postgres,mysql}/worktree_presence.go` (mới), `cmd/server/wire_request_lifecycle.go` (sửa), `proto/orca/request/v1/request.proto` (sửa) + mã sinh, `internal/adapter/grpc/` mapper và handler (sửa)
**Depends on:** TASK-REQ-039-01
**Status:** [x] DONE (2026-10-19)

---

## Context

- `ChangeRequestType` là mẫu cho một ghi có khoá lạc quan kèm `ExecutionGuard`.
- Worktree đã có được nhận ra bằng `request.openspec_changes` (trừ `abandoned`) và `TaskView.WorktreeID` trong cây thực thi.
- `request.proto`: `Request` trống từ trường 34; `CreateRequestRequest` trống từ trường 9.

## Việc cần làm

1. Proto: `Request.base_ref = 34`, `Request.base_ref_locked = 35`, `Request.base_ref_lock_reason = 36` (`"proposal"|"task"|"terminal"|""`), `CreateRequestRequest.base_ref = 9`, RPC `SetRequestBaseRef`. Sinh lại mã bằng quy trình của repo; không sửa tay file sinh.
2. `domain.ParseWorktreeBaseRef` và `Request.WorktreeBaseRefOrDefault` theo SOL-039 mục 2.2; ba lỗi theo mục 2.7.
3. `WorktreePresence` (cổng) với hai cài đặt SQL; `Has(ctx, requestID) (present bool, reason string, err error)`. Request ở trạng thái cuối trả `present=true, reason="terminal"`.
4. `CreateRequest`: nhận `BaseRef`, `ParseWorktreeBaseRef`, `BaseRefChecker.Check` (fail-open khi `Unreachable`, fail-closed khi `NotFound`), lưu; kết quả mang `BaseRefUnverified`.
5. `SetRequestBaseRef`: một transaction: `GetForUpdate` → kiểm quyền → `ParseWorktreeBaseRef` → `WorktreePresence.Has` → `BaseRefChecker.Check` → `SetBaseRef`. Thành công ghi sự kiện outbox `request.base_ref_changed` (không chứa nội dung Request) và một dòng audit theo CR-REQ-035.
6. Mapper proto: `base_ref`, và `base_ref_locked`/`lock_reason` tính khi đọc (một truy vấn `WorktreePresence` cho `Get`; cho `List` dùng truy vấn gộp theo lô để không N+1).
7. Nối trong `wire_request_lifecycle.go`; thiếu cổng thì khởi động lỗi rõ (không tắt lặng lẽ).

## Kiểm thử

- Bảng `ParseWorktreeBaseRef` (≥ 30 ca, mỗi ký tự cấm một ca).
- `TestSetRequestBaseRef_Unlocked_Succeeds`, `_LockedByProposal`, `_LockedByTask`, `_Terminal`, `_VersionConflict`, `_ConcurrentWithWorktreeCreation_NoLostLock`, `_NotFoundRef_RejectedNoWrite`, `_Unreachable_StoredWithWarning`, `_PermissionDenied`.
- `TestCreateRequest_BaseRef_*` cho rỗng, hợp lệ, không hợp lệ, `NotFound`, `Unreachable`.
- Hợp đồng hai dialect cho `WorktreePresence` (có `openspec_changes`, có task có worktree, không có gì, `abandoned`).
- `go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run 'BaseRef|WorktreePresence' -v`.

## Tiêu chí hoàn thành

- [x] Tạo và sửa `base_ref` đúng quy tắc khoá; không có khe giữa kiểm và ghi (test đồng thời).
- [x] Ba mã lỗi trả đúng loại gRPC và `details`.
- [x] `List` không N+1 (test đếm truy vấn hoặc `EXPLAIN`).
- [x] Thiếu cổng khi khởi động thì lỗi, không tắt im lặng.

## Rủi ro và lưu ý

- `Get`/`List` thêm một truy vấn presence: đo trên Request có nhiều task.
- Chạy `gitnexus_impact` cho `CreateRequest.Execute`, `Request` (struct) và mapper.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

---

## Ghi chú triển khai (2026-10-19)

### Tệp đã đổi (đường dẫn dưới `backend-go/`)

- Proto: `proto/orca/request/v1/request.proto` (+ `request.pb.go`, `request_grpc.pb.go` sinh bằng `protoc 33.4` theo đúng lệnh trong brief; tái sinh trước khi sửa cho diff rỗng, nên diff chỉ gồm phần thêm). `Request.base_ref=34`, `base_ref_locked=35`, `base_ref_lock_reason=36`; `CreateRequestRequest.base_ref=9`; `CreateRequestResponse.base_ref_unverified=3`; RPC `SetRequestBaseRef` (`SetRequestBaseRefRequest{request_id, base_ref, expected_version}` → `{request, base_ref_unverified}`).
- Miền: `domain/worktree_base_ref.go` (`ParseWorktreeBaseRef`, `Request.WorktreeBaseRefOrDefault`), `domain/base_ref_errors.go` (3 mã lỗi + `BaseRefLockReason` + `BaseRefError` mang `details`), `domain/request.go`, `domain/outbox_subjects.go` (`orca.request.request.base_ref_changed`), `domain/audit_action.go` (`request.base_ref.change`), `domain/rpc_catalog.go` (`SetRequestBaseRef`: `GroupTriage`, `AgentAllowed=false`), `domain/erasable_columns.go`.
- Use case: `usecase/base_ref_checker.go` (cổng `BaseRefChecker`, chính sách fail-open/fail-closed `verifyBaseRef`), `usecase/worktree_presence.go` (cổng `WorktreePresence` + `TaskAwareWorktreePresence`), `usecase/base_ref_lock_annotator.go`, `usecase/set_request_base_ref.go`, `usecase/create_request.go` (+`BaseRef`, `WithBaseRefChecker`, `BaseRefUnverified`), `usecase/get_request.go`, `usecase/list_requests.go` (`WithBaseRefLock`), `usecase/audit_event_tap.go`.
- Adapter: `adapter/{postgres,mysql}/worktree_presence.go` (phần lưu trong request-service: Request ở trạng thái cuối và `openspec_changes` khác `abandoned`), `adapter/grpc/{server_base_ref,base_ref_status}.go`, `server.go`, `server_wiring.go`, `server_intake.go`, `request_mapper.go`, `flow_gate.go` (RPC mới là `classAdvance`), `adapter/grpcclient/base_ref_checker_unavailable.go`.
- Nối dây: `cmd/server/wire_request_lifecycle.go` (`wireBaseRef`), `cmd/server/store_wiring.go`, `cmd/server/main.go`.
- Hợp đồng/test: `contracttest/base_ref_lock_contract.go` (mới), `postgres|mysql/request_base_ref_integration_test.go`, `domain/worktree_base_ref_test.go`, `usecase/{set_request_base_ref,set_request_base_ref_concurrency,set_request_base_ref_failures,create_request_base_ref,worktree_presence,base_ref_fakes}_test.go`, `adapter/grpc/server_base_ref_test.go`, `cmd/server/wire_base_ref_test.go`, `adapter/grpcclient/base_ref_checker_unavailable_test.go`, `usecase/audit_test.go` (thêm một dòng).

### Lệnh đã chạy và kết quả

- `cd backend-go/services/request-service && go test ./internal/domain/... ./internal/usecase/... ./internal/adapter/grpc/... ./cmd/... ./internal/adapter/grpcclient/... -run 'BaseRef|WorktreePresence' -count=1 -v`: PASS, 121 dòng PASS gồm test con; `ParseWorktreeBaseRef` có 47 ca (13 hợp lệ, 34 không hợp lệ, mỗi ký tự cấm và mỗi quy tắc một ca).
- `go test -race ./...` toàn `services/request-service`: xanh. `go vet ./...` và `go build ./...`: xanh.
- Hợp đồng hai dialect (`-tags=integration`, kèm biến Docker của colima như task 01): `TestPostgres_BaseRefLockContract` (Postgres 16) và `TestMySQL_BaseRefLockContract` (MySQL 8.0) PASS. Gồm `WorktreePresence` (không có gì, `preparing`/`ready`/`archived` là `proposal`, `abandoned` không khoá, `completed`/`cancelled` là `terminal` kể cả khi có change, `request_backlog` không khoá, id lạ/sai định dạng/khác tenant, trang 60 Request), `SetRequestBaseRefEndToEnd` (ghi, sự kiện outbox không chứa tiêu đề/nội dung, khoá do proposal, `abandoned`, trạng thái cuối, sai phiên bản, không tìm thấy) và `ConcurrentWithWorktreeCreationNoLostLock` (40 vòng thật trên DB; cả hai thứ tự đều xảy ra, ví dụ Postgres 7 lần đổi trước và 33 lần worktree trước, MySQL 4 và 36). Kiểm tra đột biến: đổi `LockRequest` thành `Get` thì test đồng thời đỏ ngay vòng 2 (đã hoàn nguyên).
- Test đồng thời ở tầng use case (200 vòng, `-race`) và 8 người ghi cùng phiên bản (đúng 1 thắng).
- Tên test theo task: `TestSetRequestBaseRef_Unlocked_Succeeds`, `_LockedByProposal`, `_LockedByTask`, `_Terminal`, `_VersionConflict`, `_ConcurrentWithWorktreeCreation_NoLostLock`, `_NotFoundRef_RejectedNoWrite`, `_Unreachable_StoredWithWarning`, `_PermissionDenied`; `TestCreateRequest_BaseRef_{Empty,Valid,Invalid,NotFound,Unreachable,...}`.
- Chạy toàn bộ gói tích hợp: xem ghi chú task 01 (hai lỗi có sẵn do agent khác đang sửa `solution_authorization.go`, một lỗi NATS chập chờn).

### Hành vi đã chốt

- `ParseWorktreeBaseRef`: `""` hợp lệ (mặc định); chuỗi chỉ gồm khoảng trắng là `REQUEST_BASE_REF_INVALID` (theo tiêu chí chấp nhận của CR, mục 4; SOL mục 2.2 nói cắt khoảng trắng rồi rỗng là hợp lệ, hai chỗ mâu thuẫn, chọn CR vì người dùng đã gõ gì đó). Thêm quy tắc theo thành phần (`/.x`, `x.lock/`), `@` đơn, UTF-8 sai, ký tự điều khiển C1/DEL, khoảng trắng Unicode. Thông báo lỗi nêu quy tắc, không lặp lại giá trị.
- Lỗi và `details`: `apperrors` không có chỗ cho `details`, nên `domain.BaseRefError` mang `Details`, và `adapter/grpc/base_ref_status.go` gắn `errdetails.ErrorInfo{Reason: <mã>, Domain: "orca.request.v1", Metadata}` (`reason` cho LOCKED; `ref`, `suggestions` tối đa 5 cho NOT_FOUND). `common/apperrors` không bị sửa (ngoài phạm vi).
- `SetRequestBaseRef`: quyền = `ActorKind==user` (agent bị `REQUEST_ACTOR_NOT_ALLOWED`, `PermissionDenied`) cộng nhóm `GroupTriage` ở interceptor (owner/reporter, như `ChangeRequestType`); RPC **không** cho agent (`AgentAllowed=false`, nên `request.rego` không đổi). Nếu task 05 cho MCP gọi thì phải đổi cờ này và `agent_rpcs` trong `policy/orca-authz/request.rego` (ngoài phạm vi của tôi).
- `expected_version == 0` bỏ qua kiểm phiên bản (như `ChangeRequestType`); repository vẫn kiểm đúng phiên bản của hàng đã khoá.
- Fail-open/fail-closed (S4) cài ở `verifyBaseRef`: chỉ `NotFound` từ checker mới từ chối; lỗi hoặc `Unreachable` thì lưu và đặt `BaseRefUnverified`; ctx hết hạn thì trả lỗi của ctx, không lưu.
- Cờ khoá suy ra khi đọc: `GetRequest` một lần `HasMany`, `ListRequests` một lần `HasMany` cho cả trang (Request trạng thái cuối quyết định bằng trạng thái, không tra cứu). Lỗi tra cứu không làm hỏng việc đọc (cờ để trống, ghi cảnh báo); đường ghi quyết định lại dưới khoá hàng. `base_ref_locked` chỉ đáng tin ở `GetRequest`, `ListRequests` và `SetRequestBaseRef`; các RPC khác trả `toProtoRequest` không tra cứu nên để trống.
- Sự kiện `orca.request.request.base_ref_changed` chỉ chứa `request_id, from, to, actor_id, version`; `AuditTap` đổi thành một dòng audit `request.base_ref.change` (metadata `from`, `to`). Test audit kiểm không rò tiêu đề.

### Lệch so với văn bản task và lý do

1. `BaseRefChecker.Check` chạy **trước** transaction (không đứng sau `WorktreePresence.Has` như thứ tự trong task): một lời gọi git-gateway tới 10 giây không được giữ khoá hàng Request. Mọi thứ quyết định ghi có được hay không (phiên bản, trạng thái cuối, presence) vẫn chạy trong transaction sau `LockRequest`. Hệ quả nhỏ: ref không tồn tại được báo trước khi báo Request đã khoá.
2. `WorktreePresence` tách hai nửa vì worktree của task nằm trong DB của task-service, không có SQL nào của request-service thấy được: cài đặt SQL (`postgres|mysql/worktree_presence.go`) chỉ trả `terminal` và `proposal`; `usecase.TaskAwareWorktreePresence` thêm `task` qua `TaskClient.ListTasks` (một lời gọi cho cả trang). Task-service lỗi là lỗi `REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE`, không bao giờ là "chưa có worktree". Lý do trả kiểu `domain.BaseRefLockReason` (string có kiểu) thay cho `string` trần.
3. Chưa có checker thật gọi git-gateway (thuộc task 03 trong cây file của SOL). Nối dây hiện dùng `grpcclient.UnavailableBaseRefChecker` (luôn `Unreachable`, nghĩa là ref được lưu và đánh dấu chưa kiểm chứng) cùng một cảnh báo khi khởi động; task 03 phải thay thế nó. Một ref không rỗng mà không có checker (`WithBaseRefChecker` chưa gọi) bị từ chối `REQUEST_BASE_REF_CHECK_NOT_CONFIGURED`, không bao giờ nhận không kiểm.
4. Lấy `GetForUpdate` từ `RequestLocker.LockRequest` có sẵn (xem task 01).
5. Cột `CreateRequestResponse.base_ref_unverified` và `SetRequestBaseRefResponse.base_ref_unverified` là trường proto thêm (task nói "kết quả mang `BaseRefUnverified`").

### Phát hiện cần task sau xử lý (rủi ro)

- **`PrefetchCreateBase` của git-gateway không phân biệt "ref không tồn tại" với "không với tới"**: mọi lỗi của `FetchAndResolveRef` thành `WORKTREE_PREFETCH_FAILED` (Internal), và lỗi không mang nguyên nhân qua gRPC. Mã `WORKTREE_BASE_REF_NOT_FOUND` kèm `available` chỉ có ở `CreateWorktree`. Giả định trong SOL mục 2.4 (NotFound kèm gợi ý từ PrefetchCreateBase) không đúng với mã hiện tại. Task 03 cần một trong hai: sửa git-gateway để trả `NotFound` có phân loại, hoặc dùng `SearchRefs` (so khớp chính xác trong danh sách ref, lấy 5 gợi ý). `SearchRefs` chỉ thấy ref đã có trong clone cục bộ, nên ref từ xa chưa fetch sẽ bị coi là không có: phải cân nhắc.
- **Khe giữa các dịch vụ không thể đóng bằng transaction của request-service**: worktree đề xuất được tạo trong git-gateway rồi mới ghi `openspec_changes` (best effort, `recordWorkspace`), worktree của task do task-service tạo. Khoá hàng chỉ loại được khe giữa "kiểm presence" và "ghi `base_ref`". Để một lời gọi tạo worktree đã đọc ref cũ không thắng một lần đổi ref, task 03/04 phải: lấy `LockRequest`, đọc `BaseRef` mới nhất và ghi dấu `preparing` (đề xuất) **trước** khi gọi git-gateway; với task, đọc `BaseRef` từ DB lúc dispatch chứ không từ struct cũ. Test hợp đồng `ConcurrentWithWorktreeCreationNoLostLock` đã mô hình đúng thứ tự "khoá, đọc ref, ghi dấu" đó.
- `Get`/`List` thêm một lần `ListTasks` cho Request chưa có dấu khoá trong DB (chưa đo trên Request nhiều task; không lọc theo `task_type` vì lọc nhầm sẽ bỏ sót khoá).

### Chưa kiểm chứng

- `golangci-lint` không chạy được (xem task 01); chỉ `gofmt`, `go vet`.
- Chưa chạy e2e hay thử qua gateway/MCP (task 05, 06), chưa thử với task-service và git-gateway thật.
- Chưa có `EXPLAIN` cho `worktreePresenceQuery` (kiểm "không N+1" bằng bộ đếm lời gọi: một `HasMany` và một `ListTasks` cho trang 40-50 Request; câu SQL là một câu cho cả trang theo thiết kế).
- `go.mod` của request-service vẫn ghi `google.golang.org/genproto/googleapis/rpc` là `// indirect` dù mã thường (không chỉ test) đã import `errdetails`; `go build` không bị ảnh hưởng, `go mod tidy` sẽ đổi dòng đó.
