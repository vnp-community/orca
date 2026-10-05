# CR-REQ-003 — Máy trạng thái Request và registry luồng theo loại

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-003 |
| **Tên** | Một máy trạng thái dùng chung cho 11 loại, một registry mô tả loại nào bỏ qua bước nào, một use case duy nhất ghi `status` |
| **Loại** | Feature (lõi miền) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-002 |
| **Mở khoá** | CR-REQ-004, 005, 006, 007, 009, 012, 013, 014 |
| **Tác động** | `backend-go/services/request-service/internal/{domain,usecase}` (mới). Không có RPC công khai mới, trừ đề nghị Q2 |

---

## 1. Bối cảnh và vấn đề

README v6 mục 3.3 liệt kê 11 trạng thái, mục 3.4 liệt kê luồng của 11 loại, nhưng chưa có quy tắc thi hành. Nếu mỗi CR (tạo, phân loại, duyệt, thực thi, trả backlog) tự ghi `requests.status` thì bảng chuyển lệch nhau theo thời gian. Task hiện có tiền lệ tốt: `domain.SetStatus` chặn `in_progress` ngoài `ExecuteTask` (`ErrCannotSetInProgress` trong `task-service/internal/domain/task.go`). Request cần một chốt chặn tương tự, nhưng theo bảng chuyển đầy đủ.

CR này sở hữu: bảng chuyển, registry luồng và use case `TransitionRequest`. Không sở hữu: cách AI phân loại (CR-REQ-005), nội dung Approval (CR-REQ-009), sinh Solution/Plan (CR-REQ-007, 012).

## 2. Giải pháp đề xuất

### 2.1 Registry luồng (`internal/domain/request_flow_registry.go`, mới)

Mã Go tĩnh, một giá trị `FlowDefinition` cho mỗi loại, tra bằng `FlowFor(RequestType) (FlowDefinition, error)`. Loại lạ trả `REQUEST_FLOW_UNKNOWN_TYPE`.

```
FlowDefinition{
  Type, HumanConfirmRequired bool,        // hotfix, security: người bắt buộc xác nhận
  Analysis  {Kind: solution|diagnosis|findings|answer|none, GateSubject: solution|findings|answer|"" },
  Plan      {Kind: plan|task_list|single_task|none},
  PhaseRule: never | always | when_size_L,
  StartGate: subject Approval chiếm awaiting_plan_approval (plan|task_list|pre_deploy|""),
  ExecutionGates: [phase | pre_deploy],   // cổng trong lúc executing, không đổi status
  CompletesAfterAnalysis bool,            // spike, question
}
```

Giá trị theo README v6 mục 3.4 (`diagnosis` dùng subject `solution` vì README không có subject `diagnosis`):

| Loại | Analysis (gate) | Plan | PhaseRule | StartGate | ExecutionGates | HumanConfirm | CompletesAfterAnalysis |
|------|-----------------|------|-----------|-----------|----------------|--------------|------------------------|
| `change_request` | solution (`solution`) | plan | always | `plan` | `phase` | | |
| `bug` | diagnosis (`solution`) | plan | when_size_L | `plan` | | | |
| `hotfix` | diagnosis (không cổng) | single_task | never | `pre_deploy` | | ✔ | |
| `task` | none | task_list | never | `task_list` | | | |
| `spike` | findings (`findings`) | none | never | | | | ✔ |
| `question` | answer (`answer`) | none | never | | | | ✔ |
| `refactor` | solution (`solution`) | plan | when_size_L | `plan` | | | |
| `security` | diagnosis (`solution`) | plan | never | `pre_deploy` | | ✔ | |
| `performance` | diagnosis (`solution`) | plan | never | `plan` | | | |
| `docs` | none | task_list | never | `task_list` | | | |
| `ops_request` | none | plan | never | `plan` | `pre_deploy` | | |

`request_type` luôn là cổng đầu (chiếm `awaiting_type_confirmation`) và không nằm trong registry. `PhasesFor(size)`: `always` thì đúng, `when_size_L` thì đúng khi `size = L`, ngược lại sai. Size thiếu coi là không phải L.

### 2.2 Trigger và bảng chuyển (`request_trigger.go`, `request_transition.go`, mới)

| Trigger | Từ | Đến | Điều kiện |
|---------|----|----|-----------|
| `start_classification` | `new` | `classifying` | |
| `proposal_ready` | `classifying` | `awaiting_type_confirmation` | cả khi AI lỗi (CR-REQ-005), lúc đó `type` rỗng |
| `type_confirmed` | `awaiting_type_confirmation` | `analyzing` nếu `Analysis.Kind != none`, ngược lại `planning` | `type` đã đặt |
| `analysis_ready` | `analyzing` | `awaiting_analysis_approval` nếu có `GateSubject`; `awaiting_plan_approval` nếu `Plan.Kind = single_task`; ngược lại `planning` | |
| `analysis_approved` | `awaiting_analysis_approval` | `completed` nếu `CompletesAfterAnalysis`, ngược lại `planning` | |
| `analysis_rejected` | `awaiting_analysis_approval` | `request_backlog`, `returned_from_stage=analysis` | cần `reason` |
| `analysis_revision` | `awaiting_analysis_approval` | `analyzing` | người duyệt yêu cầu sinh lại (CR-REQ-007) |
| `plan_ready` | `planning` | `awaiting_plan_approval` | |
| `plan_approved` | `awaiting_plan_approval` | `executing` | |
| `plan_rejected` | `awaiting_plan_approval` | `request_backlog`, `returned_from_stage=plan` | cần `reason` |
| `plan_revision` | `awaiting_plan_approval` | `planning` | |
| `execution_finished` | `executing` | `completed` | CR-REQ-013 quyết định điều kiện |
| `return_to_backlog` | `classifying`, `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing` | `request_backlog` | `stage` và `reason` (CR-REQ-006) |
| `type_change` | `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing` | `awaiting_type_confirmation` | CR-REQ-005 ghi lịch sử |
| `reopen` | `request_backlog` | `classifying` | CR-REQ-006 |
| `cancel` | mọi trạng thái không phải `completed`, `cancelled` | `cancelled` | CR-REQ-006 |

Mọi cặp không có trong bảng bị từ chối. `completed` và `cancelled` là trạng thái cuối, không có chuyển đi. Khi rời `request_backlog` (chỉ `reopen`, `cancel`), `returned_from_stage` đặt NULL và `return_reason` về rỗng, giữ bản ghi lý do trong sự kiện `request.returned` đã phát.

Cổng `phase` (mỗi phase) và `pre_deploy` của `ops_request` là `Approval` chặn thực thi trong lúc `executing`; Request không đổi trạng thái. `pre_deploy` của `hotfix` và `security` chiếm `awaiting_plan_approval`, nên `plan_approved` của hai loại này được kích hoạt bởi Approval `pre_deploy`. Cần xác nhận cách đọc này (Q1).

### 2.3 Use case `TransitionRequest` (`internal/usecase/transition_request.go`, mới)

```
TransitionInput{ RequestID, Trigger, ExpectedFrom *RequestStatus,
                 ActorID, ActorKind(ai|user|system), Stage, Reason }
TransitionResult{ Request, Applied bool }
```

Các bước, trong một `TxRunner.InTx`:
1. `tenant.RequireTenantID`; đọc Request (`REQUEST_NOT_FOUND` nếu không có).
2. Nếu `ExpectedFrom != nil` và `status != *ExpectedFrom`: nếu `status` bằng đích của (`*ExpectedFrom`, `Trigger`) thì trả `Applied=false` thành công (giao lặp); ngược lại `REQUEST_STATE_STALE`.
3. `domain.NextStatus(flow, status, trigger, size)`; không có đường thì `REQUEST_TRANSITION_NOT_ALLOWED` (kèm `from`, `trigger`).
4. Kiểm tiền điều kiện: `type` đặt khi `type_confirmed` (`REQUEST_TYPE_NOT_SET`); `reason` không rỗng khi vào `request_backlog` (`REQUEST_REASON_REQUIRED`).
5. `Update` với CAS `version` (CR-REQ-002); thua CAS thì thử lại tối đa 3 lần đọc lại từ bước 1, sau đó `REQUEST_VERSION_CONFLICT`.
6. `InsertOutboxEvent` cùng transaction: `orca.request.request.status_changed` payload `{request_id, project_id, from, to, trigger, type, actor_id, actor_kind, stage, reason, at}`. Thêm `orca.request.request.completed` khi `to = completed`.

Chỉ file này và `cancel`/`reopen` của CR-REQ-006 (qua cùng use case) được ghi `status`. Test kiến trúc (grep trong CI) từ chối `SET status` ở adapter ngoài hàm của use case này (Q3).

### 2.4 Mã lỗi

| Mã | Kind | Khi |
|----|------|-----|
| `REQUEST_TRANSITION_NOT_ALLOWED` | FailedPrecondition | cặp (trạng thái, trigger) không có trong bảng |
| `REQUEST_STATE_STALE` | FailedPrecondition | `ExpectedFrom` lệch trạng thái hiện tại và không phải giao lặp |
| `REQUEST_TYPE_NOT_SET` | FailedPrecondition | `type_confirmed` khi chưa có loại |
| `REQUEST_REASON_REQUIRED` | InvalidArgument | vào `request_backlog` hoặc `cancel` thiếu lý do |
| `REQUEST_FLOW_UNKNOWN_TYPE` | InvalidArgument | loại ngoài registry |
| `REQUEST_VERSION_CONFLICT` | FailedPrecondition | hết lần thử CAS (CR-REQ-002) |

### 2.5 Hai dialect

Không có SQL riêng ở CR này: dùng repository của CR-REQ-002 (CAS bằng `version`, `InsertOutboxEvent`). Kiểm thử chạy trên cả hai DB vì hành vi CAS và transaction phải giống nhau.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Registry là mã Go, không phải bảng | Đổi luồng cần review và test; không lệch cấu hình giữa môi trường |
| Một use case ghi `status` | Một nơi giữ bất biến, một sự kiện `status_changed` đáng tin |
| `ExpectedFrom` tuỳ chọn | Consumer outbox giao lặp: cùng lệnh lần hai trả thành công, không lỗi |
| `diagnosis` gate dùng subject `solution` | README v6 không có subject `diagnosis`; CR-REQ-008 phân biệt bằng `Solution.kind` |
| `analysis_rejected` về backlog, thêm `analysis_revision` | Theo nghiên cứu (từ chối thì về backlog) mà vẫn cho sinh lại không mất Request |
| `phase` gate không đổi `status` | README v6 không có trạng thái riêng cho phase; thêm trạng thái làm vỡ hợp đồng 3.3 |
| Size thiếu coi là không phải L | Tránh bắt Phase khi chưa có đề xuất; CR-REQ-005 bắt buộc size với `bug`, `refactor` |

## 4. Tiêu chí chấp nhận

- [ ] `FlowFor` trả đúng 11 định nghĩa khớp bảng 2.1; loại lạ trả `REQUEST_FLOW_UNKNOWN_TYPE`.
- [ ] Với mỗi loại, chuỗi trigger hợp lệ đi từ `new` tới `completed` đúng thứ tự trạng thái của README mục 3.4 (test từng loại).
- [ ] `hotfix` không đi qua `awaiting_analysis_approval` hay `planning`; `spike` và `question` đi `analyzing`, `awaiting_analysis_approval`, `completed`.
- [ ] `PhasesFor(S|M|L)` đúng cho `always`, `when_size_L`, `never`.
- [ ] Mọi cặp (trạng thái, trigger) ngoài bảng 2.2 bị từ chối (test bảng đầy đủ 11 trạng thái nhân mọi trigger).
- [ ] `completed` và `cancelled` không nhận trigger nào.
- [ ] `TransitionRequest` giao lặp cùng `ExpectedFrom` và `Trigger` hai lần: lần hai `Applied=false`, không thêm dòng outbox.
- [ ] 10 lệnh đồng thời trên cùng Request: đúng một thắng nếu cùng đích khác nhau, số dòng outbox bằng số chuyển đã áp dụng.
- [ ] Chuyển trạng thái và dòng outbox cùng commit: ép lỗi ghi outbox thì `status` không đổi.
- [ ] Rời `request_backlog` xoá `returned_from_stage` và `return_reason`; ràng buộc CHECK của CR-REQ-002 không bị vi phạm ở mọi chuyển.

## 5. Kiểm thử

- **Unit:** bảng chuyển đầy đủ (table-driven, mọi loại, mọi trạng thái, mọi trigger); `PhasesFor`; sinh payload sự kiện. Không cần DB.
- **Integration, Postgres và MySQL:** `TransitionRequest` đủ các tiêu chí đồng thời, giao lặp và rollback ở mục 4.
- **Hợp đồng:** test đọc README v6 (hoặc hằng sao chép từ README) kiểm tên 11 trạng thái và 11 loại trong code khớp, để lệch README bị bắt ở CI.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Cách đọc cổng `phase`/`pre_deploy` (mục 2.2) là suy luận từ nghiên cứu, chưa được người yêu cầu xác nhận.
- CR-REQ-013 chưa định nghĩa điều kiện `execution_finished` cho từng loại (ví dụ `refactor` đóng khi test cũ xanh, `performance` đóng sau đo lại); CR này chỉ cung cấp trigger.
- Registry tĩnh khiến thêm loại thứ 12 phải sửa mã, migration `CHECK` và proto; chấp nhận được ở v6.

## 7. Câu hỏi mở

- **Q1.** Xác nhận: `pre_deploy` của `hotfix`/`security` chiếm `awaiting_plan_approval`; `phase` và `pre_deploy` của `ops_request` chặn trong `executing`.
- **Q2.** Frontend (CR-REQ-019, 021) cần biết các bước của loại đã chọn. Đề nghị thêm RPC `GetRequestFlow(type, size)` trả `FlowDefinition` cho README mục 3.6; hoặc frontend sao chép registry (dễ lệch). Cần chốt.
- **Q3.** Có cần kiểm tra tự động (grep CI) cấm ghi `status` ngoài use case không, hay chỉ dựa review.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.7
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 2, 3
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go` (`ErrCannotSetInProgress`)
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/report_execution_result.go` (mẫu callback idempotent)
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/apperrors/apperrors.go`
- `/opt/repos/orca/backend-go/services/request-service/internal/domain/request_flow_registry.go`, `request_transition.go`, `internal/usecase/transition_request.go` (mới)
