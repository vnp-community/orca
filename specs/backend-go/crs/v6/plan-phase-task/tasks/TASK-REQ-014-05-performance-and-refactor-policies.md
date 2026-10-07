# TASK-REQ-014-05: Chính sách `performance` (baseline, đo lại) và `refactor` (test cũ vẫn xanh)

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/type_policy_performance.go` (mới), `internal/domain/type_policy_refactor.go` (mới), `internal/domain/perf_metrics.go` (mới), `internal/domain/type_policy_performance_test.go`, `type_policy_refactor_test.go` (mới)
**Depends on:** TASK-REQ-014-03, TASK-REQ-014-02 (schema metrics)
**Status:** `[x] DONE`

---

## Context

- Nhãn: `check:baseline`, `check:after` (performance); `check:tests_before`, `check:tests_after` (refactor) từ `plan_labels.go` (SOL-012).
- Số đo nằm ở `request_checks` (không lấy từ output task). `CompletionChecks` nhận `[]RequestCheck` đã sắp theo thời gian; dùng `Latest` để lấy bản có hiệu lực.
- `tests_modified` do agent khai; backend không xác minh vì không thêm lệnh git (README v6 mục 6). Phải ghi rõ ở `Summary`.
- `performance` đo baseline ở bước Chẩn đoán (CR-REQ-008 chỉ chừa `measurements[]`); việc ghi `perf_baseline` qua RPC task 02.
- Thiếu dữ liệu là `missing` (chờ, chưa trả backlog); số đo không đạt là `failed`.

## Việc cần làm

1. `perf_metrics.go`: kiểu `PerfBaseline`/`PerfAfter` giải mã từ `metrics` JSON; hàm thuần `ImprovementPercent(baseline, value float64, dir Direction) (float64, error)`: `change = (value - baseline) / baseline * 100`; `lower_is_better` đảo dấu; `baseline == 0` trả lỗi `ErrBaselineZero`.
2. `type_policy_performance.go`:
   - `PlanPreconditions`: cần `perf_baseline` `passed` (lấy qua cổng `RequestCheckReader` truyền vào policy khi dựng) hoặc `REQUEST_PERF_BASELINE_MISSING`; Plan có task `check:baseline` đầu và `check:after` cuối, nếu thiếu `REQUEST_PLAN_PERF_CHECK_TASKS_MISSING`.
   - `CompletionChecks`: so từng metric của `perf_baseline` với `perf_after`; thiếu `perf_after` hoặc thiếu metric là `missing`; `improvement% >= target_change_percent` là `passed`, ngược lại `failed` với `Summary = "Hiệu năng chưa đạt: <name> <x>% < <target>%"`; `baseline = 0` là `failed` "baseline bằng 0".
   - `PreExecutionGate`, `OnCompleted`: rỗng.
3. `type_policy_refactor.go`:
   - `PlanPreconditions`: task `check:tests_before` đầu và `check:tests_after` cuối, nếu thiếu `REQUEST_PLAN_TEST_CHECK_TASKS_MISSING`.
   - `CompletionChecks`: cần `tests_before` và `tests_after`; đạt khi `tests_after.failed == 0`, `tests_after.total >= tests_before.total`, `tests_modified == false`; thiếu bản ghi là `missing`; vi phạm bất kỳ điều kiện là `failed`, `Summary` nêu điều kiện vi phạm và dòng "tests_modified do agent khai, chưa được xác minh".
4. Đăng ký hai policy vào `NewPolicyRegistry`.
5. Không đọc `LastExecutionOutput` của task để suy số đo.

## Kiểm thử

- `TestImprovementPercent_BothDirections`, `_BaselineZero`.
- `TestPerformancePolicy_Completion`: table-driven (đạt, không đạt một metric, thiếu `perf_after`, thiếu một metric, baseline 0, hai metric một đạt một không).
- `TestPerformancePolicy_PlanPreconditions_MissingBaseline`, `_MissingCheckTasks`.
- `TestRefactorPolicy_Completion`: `failed>0`, `total` giảm, `tests_modified=true`, đạt cả ba, thiếu `tests_before`.
- `TestRefactorPolicy_PlanPreconditions_MissingCheckTasks`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'Performance|Refactor|ImprovementPercent' -v`.

## Tiêu chí hoàn thành

- [x] `performance`: thiếu `perf_baseline` thì không `GeneratePlan`; `perf_after` đạt thì hoàn tất; một metric chưa đạt thì Request vào backlog với lý do nêu tên metric; baseline 0 không chia cho 0.
- [x] `refactor`: `tests_after.failed > 0`, `total` giảm hoặc `tests_modified=true` đều không hoàn tất; đạt cả ba thì hoàn tất.
- [x] Hai policy là hàm thuần, test không cần DB.

## Rủi ro và lưu ý

- Ngưỡng `target_change_percent` do AI/Chẩn đoán đề xuất có thể phi thực tế; người duyệt Plan chịu trách nhiệm.
- Agent khai sai thì backend không biết; ghi rõ ở tóm tắt cho người xem.
- Số thực dùng `float64`; so sánh ngưỡng với dung sai tránh lỗi làm tròn (ví dụ `>= target - 1e-9`).
