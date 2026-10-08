# BE-REQ-SOL-003: Máy trạng thái Request, registry luồng theo loại và use case `TransitionRequest`

> ✅ Đã triển khai (kiểm chứng 2026-10-08). Ghi chú: [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md).

**CR:** [CR-REQ-003](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md)
**Service:** `request-service` (`internal/domain`, `internal/usecase`, `internal/adapter/grpc`) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, use case một việc), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (outbox cùng giao dịch), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject, giao lặp)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `task-service/internal/domain/task.go` (`SetStatus`, `ErrCannotSetInProgress`) và `usecase/report_execution_result.go` (mẫu callback idempotent), `common/outbox/outbox.go`, `common/apperrors/apperrors.go`, README v6 mục 3.3, 3.4 và 8. `request-service` chưa có mã (SOL-001, 002 chưa triển khai). Bảng chuyển, registry, use case trong CR là thiết kế mới, không có tiền lệ cùng quy mô trong repo.

### Correction relative to CR-REQ-003

| # | CR nói | Hệ quả khi đọc mã và các CR khác | Xử lý |
|---|--------|----------------------------------|-------|
| C1 | Retry CAS 3 lần "đọc lại từ bước 1" trong một `InTx` | Với CAS thua, giao dịch REPEATABLE READ (MySQL) vẫn thấy snapshot cũ; retry phải mở **giao dịch mới**. Và `CreateRequest` (CR-REQ-004) gọi `TransitionRequest` trong giao dịch của nó nên không thử lại được ở lớp trong | `Execute` retry chỉ khi chưa có giao dịch trong ctx (`TxRunner.InTransaction(ctx) == false`); khi lồng thì trả `REQUEST_VERSION_CONFLICT` cho lớp ngoài quyết định |
| C2 | "Test kiến trúc (grep CI) từ chối `SET status` ở adapter" | `RequestRepository.Update` (SOL-002) ghi mọi cột, kể cả `status`, nên grep SQL không chặn được | Test dùng `go/parser`: cấm gán `.Status`, `.ReturnedFromStage`, `.ReturnReason` ngoài `transition_request.go` và `*_test.go` (xem 2.F) |
| C3 | Payload `status_changed` | Consumer (CR-REQ-005) cần thứ tự và dedup | Thêm `version` (phiên bản sau chuyển) và `number`; cộng thêm, theo quy ước additive của `common/eventbus.Event` |
| C4 | README v6 mục 3.6 không có RPC đọc luồng | README mục 8 điểm 12 giao `GetRequestFlow` cho CR-REQ-003 | Thêm `GetRequestFlow` (xem 2.G) |
| C5 | Trigger `return_to_backlog` chỉ có `stage`, `reason` | CR-REQ-006 cần `category` (cột `returned_category`) | `TransitionInput.Category` thêm ở SOL-006, ở đây để trường sẵn nhưng chưa ghi cột (cột chưa có) |

## 2. Giải pháp

### A. Cây thư mục (mới, trong `request-service`)

```
internal/domain/request_flow_registry.go    # FlowDefinition, FlowFor, PhasesFor
internal/domain/request_trigger.go          # type Trigger string, hằng
internal/domain/request_transition.go       # NextStatus(flow, size, from, trigger)
internal/domain/request_flow_path.go        # HappyPath(flow, size) []RequestStatus (cho GetRequestFlow)
internal/domain/request_flow_errors.go      # lỗi REQUEST_FLOW_*, REQUEST_TRANSITION_*
internal/usecase/transition_request.go
internal/usecase/transition_request_events.go   # dựng payload và subject
internal/usecase/get_request_flow.go
internal/usecase/status_write_guard_test.go     # test kiến trúc go/parser
```

### B. Registry (`request_flow_registry.go`)

```go
type AnalysisKind string // none | solution | diagnosis | findings | answer
type PlanKind string     // none | plan | task_list | single_task
type PhaseRule string    // never | always | when_size_L
type GateSubject string  // "" | solution | findings | answer | plan | task_list | pre_deploy | phase

type FlowDefinition struct {
    Type                   RequestType
    HumanConfirmRequired   bool
    AnalysisKind           AnalysisKind
    AnalysisGate           GateSubject   // "" nghĩa là không có cổng sau phân tích (hotfix)
    PlanKind               PlanKind
    PhaseRule              PhaseRule
    StartGate              GateSubject   // chiếm awaiting_plan_approval
    ExecutionGates         []GateSubject // phase, pre_deploy: chặn trong executing
    CompletesAfterAnalysis bool
}
func FlowFor(t RequestType) (FlowDefinition, error)  // REQUEST_FLOW_UNKNOWN_TYPE
func (f FlowDefinition) PhasesFor(size RequestSize) bool
```

Mười một giá trị đúng bảng CR-REQ-003 mục 2.1 (khai báo bằng `map[RequestType]FlowDefinition` khởi tạo một lần, trả bản sao để người gọi không sửa được slice `ExecutionGates`). `diagnosis` dùng subject `solution` (README không có `diagnosis`; CR-REQ-008 phân biệt bằng `Solution.kind`). `request_type` không nằm trong registry (luôn là cổng đầu).

### C. Trigger và bảng chuyển

Hằng trigger: `start_classification`, `proposal_ready`, `type_confirmed`, `analysis_ready`, `analysis_approved`, `analysis_rejected`, `analysis_revision`, `plan_ready`, `plan_approved`, `plan_rejected`, `plan_revision`, `execution_finished`, `return_to_backlog`, `type_change`, `reopen`, `cancel` (16 trigger). `NextStatus(flow, size, from, trigger) (RequestStatus, error)` cài đúng bảng CR mục 2.2; cặp ngoài bảng trả `REQUEST_TRANSITION_NOT_ALLOWED`. Điểm cần chú ý khi cài:

| Trigger | Quy tắc đích |
|---------|--------------|
| `type_confirmed` | `analyzing` nếu `AnalysisKind != none`, ngược lại `planning` |
| `analysis_ready` | `awaiting_analysis_approval` nếu `AnalysisGate != ""`; ngược lại `awaiting_plan_approval` nếu `PlanKind == single_task`; ngược lại `planning` |
| `analysis_approved` | `completed` nếu `CompletesAfterAnalysis`, ngược lại `planning` |
| `analysis_rejected` | `request_backlog` (kèm `returned_from_stage = analysis`) |
| `plan_rejected` | `request_backlog` (`returned_from_stage = plan`) |
| `return_to_backlog` | từ 7 trạng thái đang xử lý; `stage` do người gọi cấp |
| `cancel` | mọi trạng thái trừ `completed`, `cancelled` |
| `completed`, `cancelled` | không nhận trigger nào |

Cổng `phase` và `pre_deploy` của `ops_request` chặn trong `executing`, không đổi `status`; `pre_deploy` của `hotfix`/`security` chiếm `awaiting_plan_approval` và `plan_approved` của hai loại này do Approval `pre_deploy` kích hoạt (Q1).

### D. Use case `TransitionRequest`

```go
type TransitionInput struct {
    RequestID    string
    Trigger      domain.Trigger
    ExpectedFrom *domain.RequestStatus // nil nghĩa là không kiểm giao lặp
    ActorID      string
    ActorKind    domain.ActorKind      // ai | user | system
    Stage        domain.ReturnStage    // chỉ return_to_backlog, analysis_rejected, plan_rejected
    Reason       string
}
type TransitionResult struct { Request domain.Request; Applied bool }

func (uc *TransitionRequest) Execute(ctx context.Context, in TransitionInput) (TransitionResult, error)
```

Bước (trong `once(ctx, in)`): `tenant.RequireTenantID`; `repo.Get`; nếu `ExpectedFrom != nil` và lệch `status`: tính đích của (`*ExpectedFrom`, `Trigger`), nếu `status == đích` trả `Applied=false` thành công, ngược lại `REQUEST_STATE_STALE`; `domain.NextStatus`; kiểm `type` khi `type_confirmed` (`REQUEST_TYPE_NOT_SET`) và `Reason` không rỗng khi vào `request_backlog` hoặc `cancel` (`REQUEST_REASON_REQUIRED`); đặt `Status`, `ReturnedFromStage`, `ReturnReason` (xoá khi rời `request_backlog`); `repo.Update(ctx, r, r.Version)` (CAS); `InsertOutboxEvent` `orca.request.request.status_changed`, thêm `orca.request.request.completed` khi `to = completed`.

`Execute` = vòng retry bọc `once` trong `TxRunner.InTx` tối đa 3 lần **chỉ khi** `!tx.InTransaction(ctx)` (C1). Cổng mới `TxRunner.InTransaction(ctx) bool` do TASK-REQ-003-03 thêm vào port và hai adapter.

Payload: `{request_id, project_id, number, from, to, trigger, type, actor_id, actor_kind, stage, reason, version, at}`.

### E. Mã lỗi (`request_flow_errors.go`)

`REQUEST_TRANSITION_NOT_ALLOWED` (FailedPrecondition, kèm `from`, `trigger`), `REQUEST_STATE_STALE` (FailedPrecondition), `REQUEST_TYPE_NOT_SET` (FailedPrecondition), `REQUEST_REASON_REQUIRED` (InvalidArgument), `REQUEST_FLOW_UNKNOWN_TYPE` (InvalidArgument); `REQUEST_VERSION_CONFLICT` đã có ở SOL-002.

### F. Chốt chặn ghi `status`

`internal/usecase/status_write_guard_test.go` parse mọi file `.go` không phải `_test.go` trong `internal/usecase` bằng `go/parser`, tìm `*ast.AssignStmt` có vế trái là selector `.Status`, `.ReturnedFromStage`, `.ReturnReason` (sau này `.ReturnedCategory`), và fail nếu file khác `transition_request.go`. `domain.NewRequest` được phép đặt `Status = new` vì nằm trong package `domain`, không phải `usecase`.

### G. `GetRequestFlow`

```proto
rpc GetRequestFlow(GetRequestFlowRequest) returns (GetRequestFlowResponse);
message GetRequestFlowRequest  { string type = 1; string size = 2; }  // size rỗng coi như không phải L
message GetRequestFlowResponse {
  string type = 1; bool human_confirm_required = 2;
  string analysis_kind = 3; string analysis_gate = 4; string plan_kind = 5;
  bool has_phases = 6; string start_gate = 7; repeated string execution_gates = 8;
  bool completes_after_analysis = 9;
  repeated string status_path = 10;   // đường chuẩn từ new tới completed của loại và size này
}
```

`status_path` do `HappyPath(flow, size)` mô phỏng chuỗi trigger chuẩn qua `NextStatus`, nên frontend (CR-REQ-019, 021) không sao chép registry. RPC không cần Request, chỉ cần tenant (để đi qua interceptor chung).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Registry là mã Go, trả bản sao | Đổi luồng cần review và test; không lệch cấu hình giữa môi trường |
| Một use case ghi `status`, có test `go/parser` | Một nơi giữ bất biến; grep SQL không bắt được `Update` tổng quát |
| `ExpectedFrom` tuỳ chọn | Consumer outbox giao lặp: lần hai trả thành công |
| Retry CAS chỉ ở lớp ngoài cùng | Snapshot cũ làm retry vô ích trong giao dịch lồng; lớp ngoài quyết định |
| `HappyPath` suy ra từ bảng chuyển | Không có hai nguồn sự thật cho frontend |
| `diagnosis` gate dùng subject `solution` | README không có subject `diagnosis` |

## 4. Phụ thuộc và thứ tự

Cần SOL-002 (`RequestRepository.Update` CAS, `OutboxWriter`, `TxRunner`). Mở khoá SOL-004, 005, 006 và CR-REQ-007, 009, 012, 013, 014. Thứ tự task: 01 registry, 02 bảng chuyển, 03 use case, 04 test tích hợp hai dialect, 05 `GetRequestFlow`, 06 chốt chặn và test hợp đồng README. 01 và 02 có thể song song sau khi domain của SOL-002 xong.

## 5. Kiểm thử

- **Unit (không DB):** `FlowFor` 11 giá trị đúng bảng và loại lạ lỗi; `PhasesFor(S|M|L)` cho `always`, `when_size_L`, `never`, size rỗng; bảng đầy đủ 11 trạng thái nhân 16 trigger (ma trận kỳ vọng viết tay); chuỗi hợp lệ từ `new` tới `completed` cho từng loại; `HappyPath`; `completed`, `cancelled` không nhận trigger.
- **Integration, Postgres và MySQL:** giao lặp cùng `ExpectedFrom` + `Trigger` (lần hai `Applied=false`, không thêm outbox); 10 lệnh đồng thời trên một Request (số outbox bằng số chuyển áp dụng); ép lỗi ghi outbox thì `status` không đổi; rời `request_backlog` xoá `returned_from_stage` và `return_reason`; CHECK của SOL-002 giữ ở mọi chuyển.
- **Hợp đồng:** test đọc README v6 mục 3.2, 3.3 (hoặc hằng sao chép từ README) kiểm 11 loại, 11 trạng thái trong code khớp; test `go/parser` ở 2.F.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Cách đọc cổng `phase`/`pre_deploy` (Q1) là suy luận chưa được người yêu cầu xác nhận.
- CR-REQ-013 chưa định nghĩa điều kiện `execution_finished` theo loại.
- Registry tĩnh: thêm loại thứ 12 phải sửa mã, `CHECK` migration, proto.
- Test `go/parser` chỉ bắt gán trực tiếp; gán qua hàm trung gian ở package `usecase` vẫn lọt (chấp nhận, bổ sung bằng review).

## 7. Câu hỏi mở

- **Q1.** Xác nhận: `pre_deploy` của `hotfix`/`security` chiếm `awaiting_plan_approval`; `phase` và `pre_deploy` của `ops_request` chặn trong `executing`.
- **Q2.** `GetRequestFlow` đã thêm theo README mục 8 điểm 12; cần CR-REQ-016 cấp kênh WS cho nó.
- **Q3.** Quyền ghi ở mức Request chưa có `Grant` (README mục 8, "điểm chưa ai chốt"): `TransitionRequest` không kiểm quyền người dùng, chỉ bất biến trạng thái; quyền do lớp gRPC/gateway (CR-REQ-010) quyết định.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.7, 8
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 2, 3
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/report_execution_result.go`
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/apperrors/apperrors.go`, `common/eventbus/eventbus.go` (`Event`)
- `/opt/repos/orca/backend-go/services/request-service/internal/domain/request_flow_registry.go`, `request_transition.go`, `internal/usecase/transition_request.go` (mới)
