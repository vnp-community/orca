# IMPLEMENTATION-NOTES: approval (v6)

Mục theo thứ tự: request-service (CR-REQ-009, CR-REQ-010, 2026-10-08), rồi notification-service (TASK-REQ-010-05, 2026-10-07, giữ nguyên).

## request-service (TASK-REQ-009-01..06, 010-01..04, 010-06, 010-07) — 2026-10-08

### Quyết định và lệch so với task
- **Migration**: số thật là `0003_approvals`, `0004_approval_policies` (đợt R1a), đủ cột; thêm duy nhất `0040_approval_sweeper` (Postgres: chỉ mục `approvals_sweep` + policy RLS `relay_scan` chỉ SELECT cho sweeper xuyên tenant; MySQL: chỉ mục). Sweeper đặt tenant của claim lên ctx trước `InTx`, nên mọi ghi đều chạy dưới RLS bình thường.
- **`Approval.Stage` = `Request.Status` lúc mở** (`Request.Stage` không ai điền); khớp golden của notification-service (`awaiting_plan_approval`). `STAGE_MISMATCH` so với giá trị này.
- **Thứ tự khoá Request rồi Approval ở mọi writer** (`RequestLocker`, `SELECT ... FOR UPDATE` trên `requests`). `DecideApproval`, `CancelApproval`, `ExtendApproval` đọc `Approval` không khoá trước (ngoài giao dịch) chỉ để lấy `request_id`. Nhắc không khoá Request.
- **Hết hạn lười** trong `DecideApproval` được commit (không rollback) rồi trả `REQUEST_APPROVAL_EXPIRED`; dùng chung `ExpireApprovals.ExpireLocked` với sweeper (đưa Request về backlog, `approval_expired`, danh mục `other`).
- **Stage trả về khi hết hạn/từ chối** theo `domain.ApprovalReturnStage`; trong `executing` là `phase` chỉ khi flow có Phase, còn lại `task` (đúng `StageForStatus`; task gốc ghi `phase` cho `pre_deploy` trong `executing`, sẽ bị `ReturnRequestToBacklog` từ chối với flow không có Phase).
- **`request_type`**: `RequestTypeApprovalRecorder` thay `NoopApprovalRecorder` cho `ProposeRequestClassification`/`ConfirmRequestType` (mở approval do `system` khi đề xuất; xác nhận trực tiếp thì đóng approval là `approved` không chạy handler). Approve qua `ApprovalService` đi qua handler gọi `ConfirmRequestType` (phụ thuộc vòng được cắt bằng `BindConfirm`). Digest = hash(type, size, urgency).
- **7 chủ thể còn lại** dùng `TransitionSubjectHandler` + cổng `SubjectArtifacts` (Describe/Decided/Closed). Approve: `Decided` rồi `analysis_approved` hoặc `plan_approved` theo `stage` (cổng chạy trong `executing` không đổi trạng thái); Reject: `Decided` rồi `ReturnRequestToBacklog` (`rejected`). **Chưa gắn dịch vụ thật**: `approvalSubjectArtifacts()` (cmd/server/wire_approval.go) trả rỗng, nên mở approval các chủ thể này trả `REQUEST_APPROVAL_SUBJECT_UNAVAILABLE` (fail closed). Việc còn lại cho CR-REQ-007/008/012/013/014: cài `SubjectArtifacts` và đăng ký trong hàm đó (hoặc `registry.Register` handler riêng), chạy `contracttest.RunSubjectHandlerContract`.
- **Không gRPC trong giao dịch**: tra cứu team/admin làm trước giao dịch rồi phát lại (`OpenApproval.prefetch`, `AuthorizeApprovalDecision.Prefetch`); chính sách đổi giữa chừng thì thử lại. Ngoại lệ: khi caller đã giữ giao dịch (phân loại mở approval `request_type`) và chính sách tắt `allow_requester_approve`.
- **Mặc định cho phép tra cứu lỗi**: mở approval khi directory lỗi coi như có người duyệt đủ điều kiện; quyết định luôn kiểm lại theo snapshot (lỗi thật khi cần team thì `DIRECTORY_UNAVAILABLE`).
- **Thông báo**: bước làm giàu ở `adapter/eventbus.ApprovalNotificationStore` bọc `outbox.Store` (không sửa `common/outbox`); producer ghi thêm `reporter_id`, `self_approval_allowed`, `request_number`, `requested_by` vào payload. Lỗi directory dừng lô trước sự kiện đó. Golden: `internal/usecase/testdata/approval_notification/` (bản sao của notification-service) và `internal/domain/testdata/approval_events/` (payload producer).
- **Cấu hình**: `REQUEST_APPROVAL_ENABLED` nay mặc định bật (mọi chủ thể có handler thật); `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` chỉ cho dev. Mới: `REQUEST_APPROVAL_SWEEP_INTERVAL` (60s), `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS` (50), `TENANT_SERVICE_ADDR`, `AUTH_SERVICE_ADDR` (thiếu thì tra cứu fail closed). Cập nhật `config_test.go` và bỏ test "bật approval không handler thì không lên" (không còn đúng; thay bằng test `fillApprovalRegistry`).
- **Domain**: lỗi là `*apperrors.AppError` (`REQUEST_APPROVAL_*`), nên `internal/domain` import `common/apperrors` và `common/secretscan` (comment lưu được che bí mật qua `RedactSecrets`, cắt phần vượt cửa sổ quét 1 MiB vì scanner để nguyên phần đó). `ParsePrincipal` từ chối ID rỗng.
- **Chính sách**: `approvers` lưu mảng chuỗi (`"reporter"`, `"user:id"`, `"team:id"`, `"role:admin"`) khớp proto; `Upsert` tạo khi `id` rỗng, cập nhật bắt buộc `expected_version`.
- Đã sửa nợ R1a: không còn `exec` trần cho repository Approval; `ApprovalGateReader` (Postgres) cũng đi qua `scoped`.

### Kiểm chứng
- `go build ./...`, `go vet ./...`, `go test ./...` (request-service) PASS; `gofmt -l` sạch.
- `go test -tags integration ./internal/... ./cmd/...` PASS với Postgres 16 (vai trò ứng dụng NOSUPERUSER NOBYPASSRLS) và MySQL 8.0 thật: `RunApprovalRepositoryContract` (14), `RunApprovalPolicyContract` (5), `RunApprovalFlowContract` (16, gồm đua cancel/approve 50 lần, expire xuyên tenant, nhắc đúng một lần với hai bộ quét, request_type qua `ConfirmRequestType` thật), cùng `TestRun_ServesApprovalServicesWhenEnabled` (chạy `run()` với approval bật).

### Chưa kiểm chứng
- tenant-service và auth-service thật (client chỉ test với bản giả). Chưa rõ `ListTeamMembers` có phân trang không; `ListUsers` của auth-service tin `tenant_id` trong thân (ta luôn gửi tenant của ctx).
- `buf breaking` (không sửa proto). Chạy hai request-service cùng nhau với notification-service và NATS (payload chỉ so với golden).
- Hành vi dưới tải thật của `FOR UPDATE` trên `requests`; metric đếm hết hạn/nhắc (hiện chỉ log).
- `RunSubjectHandlerContract` cho handler của CR 005/007/008 (chưa tồn tại).

### Câu hỏi mở
- README v6 mục 3.6 cần liệt kê `ExtendApproval` và `ApprovalPolicyAdminService` (người điều phối).
- Kiểm "team tồn tại" khi lưu chính sách (cần `GetTeam`/`ListTeams` từ tenant-service): hiện chính sách trỏ team không tồn tại thì không ai khớp.
- `ListApprovals`/`GetApproval` hiện cho mọi người dùng trong tenant xem; nếu cần theo quyền xem Request thì dùng `request_visibility`.
- Truyền `subject_id` cho `RequestApproval` qua ctx (`WithRequestedSubjectID`) vì chữ ký `SubjectHandler` không đổi; CR sau có thể muốn tham số riêng.


## notification-service (TASK-REQ-010-05, phần notification của TASK-REQ-028-06) — 2026-10-07

### Quyết định
- Binding và rule approval đã có sẵn trong mã nhưng không có test. Tách sang tệp theo tính năng: `internal/adapter/eventbus/request_subjects.go` (`requestBindings`, nối vào `Subjects` bằng `withRequestBindings`) và `internal/domain/request_notification_rules.go` (`requestSubjectRules`, gộp vào `subjectRules` qua `init`). CR sau chỉ thêm hàng vào hai tệp này, không sửa `consumer.go` hay `notification_event.go` (tránh xung đột giữa 010-05 và 028-06).
- Thêm `subjectRule.SameOriginDeepLink` cho bốn rule request: `deep_link` của payload không phải đường dẫn trong ứng dụng (`/x`, không phải `//host`, `/\host`, `https:`) bị bỏ, dùng `DeepLink` mặc định `/?section=requests`. Rule cũ không đổi hành vi.
- `approval.decided` thêm `DeepLink` mặc định (audit nêu thiếu).
- `Type` clarification dùng dạng chấm (`request.clarification_requested`, `request.clarification_expired`) theo CR-REQ-028 và khớp approval, không phải dạng gạch dưới của task 028-06.
- Không có `Locked`: title do producer đặt thắng, các chuỗi trong rule chỉ là dự phòng. Hệ quả: producer chịu trách nhiệm không đưa `comment`, nội dung câu hỏi/trả lời vào `body`.

### Hợp đồng payload (producer: request-service outbox, stream `REQUEST`, subject `orca.request.*`)
Chung: `user_ids` (bắt buộc không rỗng, thiếu thì bị bỏ qua có log, không thử lại), `title`, `body` (ngắn), `deep_link` (đường dẫn trong ứng dụng). Trường khác được bỏ qua. Nhắc: `reason:"reminder"` (clarification thêm `reminder:true`), dùng cùng subject `requested` và event id mới.

| Subject | Durable | Kênh | Severity | Type | Trường thêm (không dùng để dịch) |
|---|---|---|---|---|---|
| `orca.request.approval.requested` | `notification-service-request-approval-requested` | ws + push | warning | `request.approval_requested` | `approval_id, request_id, subject_type, subject_id, stage, requested_by, due_at` |
| `orca.request.approval.decided` | không (ephemeral) | ws + push | info | `request.approval_decided` | `approval_id, request_id, decision, decided_by` |
| `orca.request.clarification.requested` | `notification-service-request-clarification-requested` | ws + push | warning | `request.clarification_requested` | `clarification_id, request_id, display_id, source, due_at` |
| `orca.request.clarification.expired` | `notification-service-request-clarification-expired` | ws | warning | `request.clarification_expired` | `clarification_id, request_id, display_id, source, due_at` |

Golden: `backend-go/services/notification-service/internal/domain/testdata/request_notification/*.json` (sao chép, không import chéo). Golden của TASK-REQ-010-04 chưa tồn tại; request-service nên sinh payload khớp các tệp này.

### Lệch giữa producer hiện có và hợp đồng (request-service; đã xử lý ở mục request-service phía trên, 2026-10-08)
- `request-service/internal/usecase/publish_approval_notifications.go`: `deep_link` dùng chuỗi giữ chỗ `request=req_id` (phải là `request_id` thật); `body` decided ghép `payload["decision"].(string)` không kiểm kiểu; chưa chắc đã được nối vào bộ phát outbox. Cần 010-04 xử lý.
- Producer chưa điền `user_ids` ở event gốc `open_approval.go` (đúng thiết kế: bước làm giàu phải chạy trước publish); chưa kiểm chứng đầu-cuối.

### Kiểm chứng
- `go test ./...` PASS trong `notification-service`; `go vet`, `gofmt -l` sạch.
- `go test -tags integration ./internal/adapter/eventbus/...` với NATS container thật: `TestRequestSubjects_DurableEventsPublishedWhileConsumerDownAreDelivered` PASS (3 subject Durable giao sau khi consumer khởi động, người nhận khác bị lọc, không giao thừa). Lần chạy cả package có một lần `TestIdleStopped_...` hết thời gian chờ reaper của testcontainers (hạ tầng); chạy lại riêng PASS.
- Tên stream `REQUEST` khớp `request-service/cmd/server/main.go:89`. Chưa chạy hai service cùng nhau.

### Chưa làm / mở
- Triển khai lại `notification-service` sau khi `request-service` phát hành (danh sách subject cố định trong mã).
- Durable dùng một con trỏ chung giữa replica: chỉ replica nhận được sự kiện broadcast WS tới người nối vào nó (hạn chế đã có ở các binding Durable khác).
- TASK-REQ-028-06 phần `request-service` (consumer resume, expire, remind, recipients, wiring, metric): chưa làm.
