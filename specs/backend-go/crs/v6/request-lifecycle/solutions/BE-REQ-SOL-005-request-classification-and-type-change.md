# BE-REQ-SOL-005: Phân loại Request bằng AI, xác nhận loại, đổi loại và lịch sử

> ✅ Đã triển khai (kiểm chứng 2026-10-08). AI chạy bất đồng bộ theo run có lease (D3); chưa kiểm chứng với dev server agent thật. Chi tiết: [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md).

**CR:** [CR-REQ-005](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-005-request-classification-and-type-change.md)
**Service:** `request-service` (use case, consumer, adapter `grpcclient`, adapter `eventbus`, migration) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (consumer, dedup, at-least-once), [`services/ai-provider-service.md`](../../../../tdd/services/ai-provider-service.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `task-service/internal/adapter/grpcclient/{aidecompose_relay.go, ai_provider_resolver.go, project_execution_resolver.go, dev_server_reachability.go, tenant_forwarding.go}`, `task-service/internal/usecase/ai_decompose.go` (cách dùng `AICompleter`, `ResolveConnection`), `proto/orca/aiprovider/v1/aiprovider.proto` (`ResolveProviderResponse{account{id, credential_ref, dev_server_id, model_hint, ...}}`), `proto/orca/infrafleet/v1/infrafleet.proto` (`Relay{connection_id, method, params_json}`, `RelayByDevServer{dev_server_id, ...}`), `common/eventbus/eventbus.go` (`Subscribe(ctx, stream, durable, subject, fn)`), `notification-service/internal/usecase/handle_incoming_event.go` (dedup bằng `processed_events`).

### Correction relative to CR-REQ-005

| # | CR nói | Mã thật | Xử lý |
|---|--------|---------|-------|
| C1 | `ResolveProvider` rồi `Relay` `ai.complete` theo `connectionID` | `AICompleter` của `task-service` chỉ gửi `{prompt}`; `ResolveProvider` chỉ lấy `credential_ref` để gắn ngữ cảnh. Quan trọng hơn, `connectionID = projectID` **luôn trượt** (BUG-025): phải rơi về `ListRepos` rồi `RelayByDevServer(devServerID)` khi project chưa có hàng `infra.connections` | Cổng `AIConnectionResolver` trả `{ConnectionID, DevServerID}`; bản cài sao logic `ProjectExecutionResolver` + `DevServerReachability` (không import chéo service). `ResolveProvider` chỉ dùng để lấy `account.id` truyền `accountId` cho `ai.complete` (tuỳ chọn, xem 2.B) |
| C2 | "Tối đa 5 lần AI, đếm bằng số dòng `request_type_history` có `actor_kind='ai'`" | Lần AI thất bại không ghi dòng lịch sử (không có `type`), nên chỉ lần thành công được đếm; thất bại lặp lại không bị giới hạn: vòng lặp tốn chi phí | Thêm cột `requests.classification_attempts` (migration mới, 2.A); mọi lần gọi AI, thành công hay thất bại, tăng một đơn vị trong giao dịch ghi kết quả. Điểm lệch có chủ ý so với CR: cần xác nhận (Q1) |
| C3 | Consumer "dedup bằng `processed_events`" | Nếu đánh dấu `processed` trước rồi AI lỗi thì sự kiện mất; nếu đánh dấu sau thì giao lặp có thể chạy AI hai lần | `MarkProcessed` nằm **trong cùng giao dịch** ghi kết quả (AI gọi ngoài giao dịch, trước đó); trạng thái `classifying` còn đóng vai chặn thứ hai |
| C4 | Consumer có người dùng | Sự kiện outbox chỉ mang `tenant_id` (envelope) và `actor_id` trong payload; `ListRepos` của `project-service` cần user (`PROJECT_NO_USER`) | Consumer dựng ctx với `tenant.WithTenantID(ev.TenantID)` và `tenant.WithUserID(request.ReporterID)` |
| C5 | README v6 mục 3.1 vẽ mũi tên tới `ai-provider-service` | Đã được README mục 8 điểm 1 sửa: relay `ai.complete` của dev server agent | Theo mục 8 |

## 2. Giải pháp

### A. Migration `0004_request_classification_attempts` (hai dialect)

```sql
-- Postgres
ALTER TABLE request.requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0
    CHECK (classification_attempts >= 0);
-- MySQL
ALTER TABLE requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0;
```
Số `0004` chỉ đúng nếu `0003_request_source_hints` đã có; đọc thư mục để chốt. Hệ quả: migration của SOL-006 thành `0005`. Proto: `Request.classification_attempts = 26` (số 24 là `source_hints`, 25 dành cho `returned_category`).

### B. Đề xuất của AI

Cổng (`usecase/ports.go`):

```go
type RequestClassifier interface {
    Classify(ctx context.Context, in ClassificationInput) (domain.ClassificationProposal, error)
}
type ClassificationInput struct{ RequestID, ProjectID, Title, Body string; Hints domain.SourceHints }
type AIConnectionResolver interface {
    ResolveForProject(ctx context.Context, projectID string) (AIConnection, error) // ConnectionID hoặc DevServerID; ErrNoDevServer
}
```

`domain.ClassificationProposal{Type RequestType; Size RequestSize; Urgency Urgency; Confidence float64; Reason string}` và `ParseClassificationProposal([]byte) (ClassificationProposal, error)`: JSON chặt (`DisallowUnknownFields`), `type` thuộc 11 giá trị, `size` `S|M|L`, `urgency` `normal|urgent`, `confidence` trong [0,1], `reason` tối đa 2000 rune. Sai thì `ErrProposalInvalid` (không phải `apperrors` công khai, chỉ nội bộ).

Bản cài `adapter/grpcclient/request_classifier.go`: dựng prompt (`classification_prompt.go`), `AIConnectionResolver` rồi `Relay(connection_id)` hoặc `RelayByDevServer(dev_server_id)` với method `ai.complete`, params `{prompt}` (thêm `accountId` nếu `ResolveProvider` trả `account.id`; bỏ qua lỗi `ResolveProvider`). Timeout 60 giây. Prompt: nội dung Request (cắt 20000 rune) nằm trong khối ranh giới ngẫu nhiên đánh dấu là dữ liệu không phải chỉ dẫn; `issue_type`, `labels` chỉ là gợi ý; yêu cầu đúng một đối tượng JSON. Đầu ra không bao giờ chạy hay đưa vào lệnh. Sai định dạng: thử lại một lần, rồi thất bại.

Kích hoạt: consumer durable `request-classifier` trên stream `REQUEST`, subject `orca.request.request.status_changed`, lọc `to == "classifying"` (và `trigger == "start_classification"` hoặc `reopen`). Cũng gọi bằng `ClassifyRequest`.

`ProposeRequestClassification.Execute(ctx, requestID, eventID string, trigger)`:
1. Đọc Request; `status` khác `classifying`/`awaiting_type_confirmation` thì thoát (giao lặp). Kiểm `classification_attempts < 5` (`REQUEST_CLASSIFICATION_LIMIT` cho `ClassifyRequest`; consumer dừng im lặng và vẫn `proposal_ready` thất bại nếu đang `classifying`).
2. Gọi `RequestClassifier` **ngoài giao dịch**.
3. `InTx`: nếu `eventID != ""` thì `ProcessedEvents.MarkProcessed(ctx, eventID, subject)` (đã xử lý thì thoát); đọc lại Request (CAS); thành công: đặt `type`, `type_source=ai`, `size`, `urgency`, `confidence`, `classification_reason`, `classification_attempts+1`, `Update`; `RequestTypeHistory.Append(from=loại trước hoặc rỗng, to, actor_kind=ai)`; nếu đang `classifying` thì `TransitionRequest(proposal_ready, ExpectedFrom=classifying)`; `ApprovalRecorder.RequestTypeApproval`; outbox `orca.request.request.classified` `{request_id, type, size, urgency, confidence, failed:false}`. Thất bại: `classification_reason` ghi lý do ngắn (nếu chưa có đề xuất cũ), `classification_attempts+1`, `proposal_ready` nếu `classifying` (type rỗng), outbox `classified` `failed=true`.

### C. Xác nhận (`confirm_request_type.go`)

`ConfirmRequestType.Execute(ctx, in)`: chỉ `ActorKind=user`; `status == awaiting_type_confirmation` (`REQUEST_TRANSITION_NOT_ALLOWED` nếu khác); `type` hợp lệ (`REQUEST_TYPE_REQUIRED`); `size` bắt buộc khi `FlowFor(type).PhaseRule == when_size_L` (`REQUEST_SIZE_REQUIRED`); `hotfix` yêu cầu `urgency=urgent` (`REQUEST_HOTFIX_REQUIRES_URGENT`); `hotfix` và `security` yêu cầu `reason` (`REQUEST_REASON_REQUIRED`). `expected_version` lệch thì `REQUEST_VERSION_CONFLICT`. Trong `InTx`: cập nhật `type`, `size`, `urgency`; `type_source=ai` nếu `type` bằng đề xuất AI hiện có (`type_source=='ai'` và cùng `type`), ngược lại `human`; khác đề xuất AI thì `Append(from=đề xuất, to, user, reason)`; `ApprovalRecorder.Approve`; `TransitionRequest(type_confirmed)`; outbox `type_confirmed`. Giao lặp (đã qua `awaiting_type_confirmation` và `type` bằng nhau): thành công, không ghi gì thêm.

### D. Đổi loại (`change_request_type.go`)

Cho phép khi `status` thuộc `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`. Tại `awaiting_type_confirmation` trả `REQUEST_TRANSITION_NOT_ALLOWED` (dùng `ConfirmRequestType`). Bảng đường đổi (`domain/request_type_change_paths.go`): `bug|task|refactor|performance → change_request`; `docs → task|change_request`; `security → hotfix`; `spike|question|hotfix → REQUEST_TYPE_CHANGE_USE_CHILD`; còn lại `REQUEST_TYPE_CHANGE_NOT_ALLOWED`; cùng loại `REQUEST_TYPE_UNCHANGED`. `InTx`: CAS; kiểm `ExecutionGuard.HasActiveExecution` khi `executing` (`REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION`; bản giả trả `false` tới CR-REQ-011); `type=new_type`, `type_source=human`, `size`/`urgency` nếu gửi; `Append(user, reason bắt buộc)`; `ApprovalCanceller.CancelPending(requestID, "type_changed")`; `TransitionRequest(type_change)`; outbox `type_changed` `{from, to, actor_id, reason}`. Giữ phần đã làm: không đụng `solutions`, Plan, Task.

### E. Cổng tích hợp mềm

`ApprovalRecorder{RequestTypeApproval, Approve}` và `ApprovalCanceller{CancelPending}` có bản no-op (`approval_noop.go`), `ExecutionGuard` có bản `false` (`execution_guard_noop.go`), đều thay ở CR-REQ-009/011. Q1 của CR (Approval thật hay không) giữ mở.

### F. Proto và RPC

```proto
rpc ClassifyRequest(ClassifyRequestRequest) returns (ClassifyRequestResponse);
rpc ConfirmRequestType(ConfirmRequestTypeRequest) returns (ConfirmRequestTypeResponse);
rpc ChangeRequestType(ChangeRequestTypeRequest) returns (ChangeRequestTypeResponse);
rpc ListRequestTypeHistory(ListRequestTypeHistoryRequest) returns (ListRequestTypeHistoryResponse);
```
Message theo CR-REQ-005 mục 2.1 (`expected_version` int64). `actor_id` rỗng khi `actor_kind=ai`.

### G. Mã lỗi

`REQUEST_NOT_CLASSIFIABLE`, `REQUEST_CLASSIFICATION_LIMIT`, `REQUEST_TYPE_UNCHANGED`, `REQUEST_TYPE_CHANGE_NOT_ALLOWED`, `REQUEST_TYPE_CHANGE_USE_CHILD`, `REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION` (`FailedPrecondition`); `REQUEST_TYPE_REQUIRED`, `REQUEST_SIZE_REQUIRED`, `REQUEST_HOTFIX_REQUIRES_URGENT` (`InvalidArgument`); `REQUEST_REASON_REQUIRED` đã có từ SOL-003.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| AI thất bại vẫn vào `awaiting_type_confirmation` | Request không kẹt, người dùng chọn tay |
| Cột `classification_attempts` | Chặn vòng lặp AI kể cả khi thất bại; không cần suy từ lịch sử |
| `MarkProcessed` cùng giao dịch ghi kết quả | Không mất sự kiện, không chạy AI hai lần ghi hai kết quả |
| Gọi AI ngoài giao dịch | Không giữ khoá hàng và kết nối DB tới 60 giây |
| Relay `RelayByDevServer` khi không có connection | Mã thật của `task-service` đã gặp BUG-025 |
| Đường đổi loại hạn chế theo bảng nâng cấp | Đúng README v6 mục 3.4 (Q3 của CR) |
| Cổng Approval, Guard no-op | Không phụ thuộc cứng CR-REQ-009, 011 |

## 4. Phụ thuộc và thứ tự

Cần SOL-004 (Request vào `classifying`, `source_hints`), SOL-003 (`TransitionRequest`), SOL-001 (stream `REQUEST`, `processed_events`). Mở khoá SOL-006, CR-REQ-007, 008, 019. Thứ tự task: 01 domain, 02 classifier, 03 consumer và `processed_events`, 04 propose, 05 confirm, 06 change type, 07 proto và gRPC, 08 test tích hợp.

## 5. Kiểm thử

- **Unit:** phân tích và kiểm đầu ra AI (hợp lệ, thiếu trường, ngoài enum, `confidence` ngoài [0,1], JSON thừa trường); bảng đường đổi loại; luật `size`, `hotfix`; dựng prompt (cắt độ dài, ranh giới dữ liệu, giữ nguyên chuỗi chứa "bỏ qua hướng dẫn"); bộ phân loại giả trả giá trị lạ bị loại.
- **Integration, Postgres và MySQL:** Confirm đồng thời (CAS, đúng một thắng); consumer giao lặp (chỉ một đề xuất, `processed_events`); lịch sử đúng thứ tự `at` sau AI, người sửa, đổi loại; lần AI thứ 6 bị chặn kể cả khi các lần trước thất bại; Approval no-op không làm hỏng giao dịch; migration `0004` up/down.
- **Hợp đồng:** `buf breaking`; test tên giá trị `type`, `size`, `urgency` trong proto và domain khớp README mục 3.2.
- **Chưa kiểm chứng:** chạy với dev server agent thật; `ai.complete` trả JSON ổn định; chất lượng phân loại (không có tập dữ liệu).

## 6. Rủi ro và điểm chưa kiểm chứng

- `ai.complete` có trả JSON đúng khuôn và đủ nhanh trong 60 giây chưa kiểm chứng; project không có dev server kết nối thì mọi phân loại rơi vào nhánh thất bại.
- Sao logic `ProjectExecutionResolver` nghĩa là nhân đôi mã (không có thư viện chung giữa service); chấp nhận theo chính sách "mỗi service tự có client".
- Prompt injection chỉ được giảm bằng ranh giới dữ liệu và kiểm enum; đề xuất sai vẫn cần người xác nhận (D5).
- `ExecutionGuard` giả tới CR-REQ-011: đổi loại từ `executing` không chặn được Task đang chạy.
- `confidence` do mô hình tự báo, chưa hiệu chỉnh; UI không dùng để bỏ bước xác nhận.
- Số `0004` có thể đụng PR khác (CR-REQ-007, 009).

## 7. Câu hỏi mở

- **Q1.** Chấp nhận cột `classification_attempts` (lệch CR) hay giữ cách đếm bằng lịch sử và chấp nhận thất bại lặp lại không giới hạn.
- **Q2.** `request_type` là Approval thật hay chỉ hành động xác nhận (CR Q1); cổng no-op giữ cả hai khả năng.
- **Q3.** Có cho đổi loại tự do ngoài bảng nâng cấp, ví dụ `bug` sang `task`.
- **Q4.** Có truyền `accountId` từ `ResolveProvider` vào `ai.complete` hay để agent tự chọn tài khoản.
- Sẽ được CR-REQ-028 mở rộng: nhánh "thiếu thông tin" của phân loại có thể sinh `awaiting_information` thay vì `awaiting_type_confirmation`; không phụ thuộc ở đây.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 2 (D5), 3.2, 3.4, 8
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 1, 4, 7
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`, `ai_provider_resolver.go`, `project_execution_resolver.go`, `dev_server_reachability.go`, `tenant_forwarding.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/ai_decompose.go`
- `/opt/repos/orca/backend-go/proto/orca/aiprovider/v1/aiprovider.proto`, `proto/orca/infrafleet/v1/infrafleet.proto`
- `/opt/repos/orca/backend-go/common/eventbus/eventbus.go`
- `/opt/repos/orca/backend-go/services/notification-service/internal/usecase/handle_incoming_event.go`, `internal/adapter/eventbus/consumer.go`
- `/opt/repos/orca/backend-go/services/request-service/internal/usecase/{propose_request_classification,confirm_request_type,change_request_type}.go`, `internal/domain/request_type_change_paths.go`, `adapter/grpcclient/request_classifier.go` (mới)
