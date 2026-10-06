# TASK-REQ-012-03: Domain `request-service`: `plan_shape`, nhãn, `PlanProposal`, `ValidateProposal`

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/plan_shape.go` (mới), `plan_labels.go` (mới), `plan_proposal.go` (mới), `plan_proposal_validation.go` (mới), `plan_shape_test.go`, `plan_proposal_validation_test.go` (mới)
**Depends on:** CR-REQ-003 (registry `FlowFor`, `PhasesFor`, `Size`) đã có ở `request-service`
**Status:** `[ ] TODO`

---

## Context

- `backend-go/services/request-service` chưa tồn tại ngày 2026-10-06 (CR-REQ-001 dựng, CR-REQ-003 thêm registry). Task này chỉ chạy được khi các kiểu `FlowDefinition`, `Size`, `PhasesFor` đã merge; nếu chưa, chặn và báo. Không bịa chữ ký: đọc `internal/domain/flow_registry.go` (tên giả định) trước khi viết.
- Domain thuần Go (stdlib), theo `arch/03`: không import proto, DB, gRPC.
- Cả CR-012, 013, 014 dùng chung một danh sách nhãn: đặt hằng ở `plan_labels.go`, không lặp chuỗi ở nơi khác.
- Phát hiện cycle ở `task-service` dùng `domain.DetectCycle`; ở đây kiểm sớm bằng thuật toán Kahn để trả lỗi rõ cho người xem đề xuất.

## Việc cần làm

1. `plan_shape.go`: kiểu `PlanShape` (`ShapePlanPhases`, `ShapePlanTasks`, `ShapeTaskListShell`, `ShapeSingleTask`, `ShapeNone`); `PlanShapeFor(f FlowDefinition, size Size) PlanShape` từ `f.Plan.Kind` (`plan|task_list|single_task|none`) và `PhasesFor(size)`.
2. `plan_labels.go`: `const (LabelPreDeploy = "gate:pre_deploy"; LabelRegressionTest = "test:regression"; LabelRollback = "rollback"; LabelPerfBaseline = "check:baseline"; LabelPerfAfter = "check:after"; LabelTestsBefore = "check:tests_before"; LabelTestsAfter = "check:tests_after")`; hàm `HasLabel(labels []string, l string) bool`.
3. `plan_proposal.go`: `PlanProposal`, `PhaseProposal`, `TaskProposal` (trường như solution 2.3); `(*PlanProposal) AllTasks() []TaskProposal` cho kiểm tổng; `(*TaskProposal) EffectiveLabels()` thêm `LabelPreDeploy` khi `Irreversible`.
4. `plan_proposal_validation.go`: `ValidateProposal(shape PlanShape, req RequestType, p PlanProposal, lim PlanLimits) error` trả lỗi có mã:
   - `REQUEST_PLAN_PHASES_NOT_ALLOWED`, `REQUEST_PLAN_PHASES_REQUIRED`, `REQUEST_PLAN_MIXED_CHILDREN`, `REQUEST_PLAN_TOO_LARGE`, `REQUEST_PLAN_INVALID_DEPENDENCY`, `REQUEST_PLAN_REGRESSION_TEST_MISSING` (`bug`, `security`), `REQUEST_PLAN_INVALID_TASK_TYPE` (InvalidArgument), `REQUEST_PLAN_INVALID_TITLE` (InvalidArgument; rỗng hoặc quá 200 ký tự).
   - `PlanLimits{MaxPhases, MaxTasksPerContainer, MaxTasksTotal int}` đọc từ config (mặc định 8, 20, 100, chưa kiểm chứng).
   - `ShapeTaskListShell` và `ShapeSingleTask`: không Phase; `ShapeSingleTask` đúng một task.
5. Hàm `detectIndexCycle(n int, edges [][2]int) bool` (Kahn) cho `depends_on_indices` trong từng container và `depends_on_phase_indices` giữa Phase.
6. Lỗi dùng `apperrors` theo quy ước repo (`KindFailedPrecondition`/`KindInvalidArgument` với mã `REQUEST_*`).
7. Cổng rỗng `PlanPreconditions func(Request, PlanProposal) error` được `GeneratePlan` gọi sau `ValidateProposal`; hiện thực để SOL-014 gắn vào, mặc định `nil`.

## Kiểm thử

- `TestPlanShapeFor_AllElevenTypes_ThreeSizes` (table-driven, dùng `FlowFor` thật): đúng bảng của solution 2.2.
- `TestValidateProposal`: mỗi dòng bảng lỗi một ca xanh và một ca đỏ; ca vòng `A→B→A`, tự phụ thuộc, chỉ số âm, chỉ số vượt; vượt `MaxTasksTotal`; `bug` thiếu `test:regression`; `Irreversible` thêm `gate:pre_deploy`.
- `go test ./services/request-service/internal/domain/... -run 'Plan' -v` (từ `/opt/repos/orca/backend-go`).

## Tiêu chí hoàn thành

- [ ] `change_request` size S vẫn cần ít nhất một Phase; `bug` size M có Phase bị `PHASES_NOT_ALLOWED`; `bug` size L thiếu Phase bị `PHASES_REQUIRED`.
- [ ] Mọi mã lỗi ở bảng 2.5 của solution có test.
- [ ] Không import ngoài stdlib và `common/apperrors` trong `domain`.
- [ ] Không file tên `helpers`, `utils`, `common`, `misc`.

## Rủi ro và lưu ý

- Mặc định giới hạn là giả định; để cấu hình qua `config.Config` (CR-REQ-001) thay vì hằng cứng.
- Nhãn là chuỗi tự do trên `Task.Labels`; người sửa tay có thể xoá (xem SOL-014 mục rủi ro).
- Chữ ký `FlowDefinition` có thể đổi khi CR-REQ-003 hoàn thiện: giữ `PlanShapeFor` là điểm duy nhất đọc chúng.
