# BE-REQ-SOL-041: Chỉnh phân tích bằng prompt: quyền approver, `revision_feedback`, activity và kiểm chứng đường lỗi

> **📝 Chưa triển khai (0/4 task).** Soạn 2026-10-19 từ đọc mã.

**CR:** [CR-REQ-041](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md)
**Service:** `request-service` · `proto/orca/request/v1` · `api-gateway` (`wscompat`, MCP)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, chỉ mục), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (quyền, không lộ nội dung)
**Phụ thuộc solution:** [BE-REQ-SOL-007](./BE-REQ-SOL-007-solution-generation-options-and-selection.md), [BE-REQ-SOL-008](./BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md), [BE-REQ-SOL-038](../../plan-phase-task/solutions/BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) (nhật ký hoạt động). Frontend: [FE-REQ-SOL-041](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-19:

- `usecase/generate_solution.go`: `checkAnalysisState` cho phép `analyzing`, hoặc `awaiting_analysis_approval` khi có `feedback`. Trong `start`, transaction huỷ Approval đang chờ (`CancelPending(..., "revision_requested")`), `SupersedeDecisions`, `SupersedeOpen`, rồi `transition` với `TriggerAnalysisRevision`. `maxFeedbackRunes = 2000`.
- Quyền: `prepare` gọi `uc.auth.AuthorizeGenerate` (khi không phải lời gọi hệ thống). `ReporterOrAdminAuthorizer` cho reporter và `admin|lead|owner`. `ApproverAwareAuthorizer.AuthorizeChoose` đã cho approver chọn phương án bằng cách liệt kê Approval `pending` kiểu `solution` rồi `Decider.CanDecide`; `AuthorizeGenerate` thì không.
- `Feedback` đi vào `AnalysisRun.Feedback` (cột `analysis_runs.feedback`, `0030_solution_analysis`) và vào prompt qua `BuildSolutionPrompt` (khối `<feedback>` bọc `fenceUntrusted`). Đường chỉ-đọc (`RunAgentReadonlyAnalysis`) và đường `ai.complete` dùng chung bộ dựng này, nên chẩn đoán, Findings, Answer cũng nhận phản hồi.
- `priorArtifacts` (`run_solution_generation.go:117`) đưa các bản `superseded` và `rejected` vào prompt, nhưng **không điền `PriorArtifact.Feedback`**, nên prompt chỉ có phản hồi mới nhất, không có các phản hồi trước.
- `Solution` proto dừng ở trường 17; không có trường phản hồi. `AnalysisRun` proto không có `feedback`.
- Nhật ký hoạt động là đọc `UNION ALL` ở `adapter/{postgres,mysql}/request_activity.go`; `analysis.generated` đọc từ `request.solutions`. Chưa có kind cho yêu cầu chỉnh.
- Cổng duyệt Analysis theo registry: `solution` (change_request, refactor, bug, security, performance), `findings` (spike), `answer` (question). `hotfix` không có cổng.

### Correction relative to CR-REQ-041

1. CR coi danh sách phiên bản là mới; **frontend đã có** `SolutionVersionSwitcher` (CR-REQ-020-02) và nút Regenerate (không phản hồi). Phần backend của danh sách chỉ cần `revision_feedback` và xác nhận `ListSolutions` trả cả bản `superseded` (đã lọc được bằng `status`).
2. CR nói `AuthorizeRevise` lọc Approval `solution`; cổng của `spike` là `findings`, của `question` là `answer`: lọc theo tập ba kiểu của cổng Analysis, không chỉ `solution`.
3. CR chưa nêu `priorArtifacts` thiếu phản hồi cũ: thêm vào (task 02) để agent thấy cả chuỗi yêu cầu chỉnh.

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/services/request-service/`)

```
internal/usecase/solution_authorization.go            (sửa) AuthorizeRevise, interface SolutionActorAuthorizer
internal/usecase/generate_solution.go                 (sửa) prepare chọn Revise/Generate
internal/usecase/run_solution_generation.go           (sửa) priorArtifacts mang Feedback
internal/usecase/solution_prompt.go                   (sửa) in phản hồi của từng bản trước
internal/usecase/list_solutions.go                    (sửa) điền revision_feedback
internal/domain/solution.go                           (sửa) RevisionFeedback
internal/domain/request_activity.go                   (sửa) ActivityAnalysisRevisionRequested
internal/adapter/{postgres,mysql}/request_activity.go (sửa) nhánh UNION mới
internal/adapter/{postgres,mysql}/solution_repository.go (sửa) nạp feedback của run theo lô
internal/adapter/grpc/{server_solution.go,solution_mapper.go} (sửa)
migrations/{postgres,mysql}/00NN_analysis_runs_revision_index.{up,down}.sql (mới, nếu EXPLAIN cho thấy cần)
internal/adapter/contracttest/                         (sửa) activity, solution contract, quyền
proto/orca/request/v1/solution.proto                   (sửa) Solution.revision_feedback = 18
services/api-gateway/internal/adapter/wscompat/, mcpserver/tools/ (sửa) revisionFeedback
```

### 2.2 Quyền chỉnh

`SolutionActorAuthorizer` thêm `AuthorizeRevise(ctx, req) error`. `ReporterOrAdminAuthorizer.AuthorizeRevise` = `AuthorizeGenerate`. `ApproverAwareAuthorizer.AuthorizeRevise`: qua nếu `Base.AuthorizeRevise` qua; nếu không, liệt kê Approval `pending` của Request có `subject_type ∈ {solution, findings, answer}`, và qua khi `Decider.CanDecide(ctx, req, ap) == nil` cho bất kỳ Approval nào; ngược lại trả `ErrSolutionForbidden`. `prepare` chọn `AuthorizeRevise` khi `req.Status == awaiting_analysis_approval && in.Feedback != ""`, còn lại `AuthorizeGenerate`. Lời gọi hệ thống (`PrepareForSystem`, `ExecuteForSystem`) không đổi. Kiểm quyền chạy trong `prepare`, trước `InTx`, nên trước khi Approval bị huỷ.

### 2.3 `revision_feedback`

`Solution.revision_feedback` (chuỗi, rỗng cho bản không sinh từ phản hồi) lấy từ `analysis_runs.feedback` của `generation_run_id`. `ListSolutions` nạp feedback theo lô (`WHERE id IN (...)` một truy vấn) để tránh N+1. Không thêm cột vào `solutions`. Nội dung chỉ trả cho người có quyền đọc Request (đã được `flow_gate`/quyền đọc bảo đảm).

### 2.4 Phản hồi cũ trong prompt

`priorArtifacts` điền `PriorArtifact.Feedback` từ run của từng bản; `BuildSolutionPrompt` đã có chỗ in `reviewer comment` (`solution_prompt.go:56`), dùng `fenceUntrusted(truncateRunes(..., promptArtifactRunes))`. Giữ giới hạn kích thước hiện có.

### 2.5 Activity

Kind mới `analysis.revision_requested` (`ActivityAnalysisRevisionRequested`): nhánh `UNION` đọc `request.analysis_runs` có `feedback IS NOT NULL AND feedback <> ''`, thời điểm `started_at`, tác nhân `user` với `actor_id = actor_id` của run, `subject_type='solution'`, `summary` cố định "Requested changes to the analysis", **không chọn cột `feedback`** (C9). Mức `severity=info`. Thêm vào danh sách kind hợp lệ cho bộ lọc và vào `activity_presentation` của backend nếu có.

### 2.6 Đường lỗi và đồng thời

- Lượt chỉnh thất bại: Request vẫn ở `analyzing`; `Regenerate` (không phản hồi) và chỉnh lại (có phản hồi, từ `analyzing` đã được phép? `checkAnalysisState` chỉ cho `awaiting_analysis_approval` khi có feedback, còn `analyzing` thì chạy lại thường) phải khả thi; bản cũ ở `superseded` được đưa vào `priorArtifacts` của lượt kế. Test chứng minh Request không kẹt.
- Hai người chỉnh cùng lúc: người sau thấy `REQUEST_SOLUTION_WRONG_STATE` (Request đã `analyzing`), giao diện làm mới.
- Idempotency: cùng `idempotencyKey` không sinh lượt thứ hai (đã có trong `StartRun`).
- `MaxAgentRunsPerProject` và lease một lượt mỗi Request giữ nguyên.

### 2.7 Gateway, MCP, CONTRACT

`solution.list` trả `revisionFeedback`; `solution.generate` đã nhận `feedback` (tài liệu hoá rõ quyền approver); MCP `solution_generate` và công cụ đọc Solution thêm trường; `CONTRACT-request-ui-api.md` mục mới.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| R1 | `AuthorizeRevise` tách riêng, chỉ áp cho đường chỉnh | Approver không được khởi động phân tích đầu |
| R2 | Kiểm quyền trước khi huỷ Approval | Sau khi huỷ, người đó không còn là approver |
| R3 | Lọc ba kiểu Approval của cổng Analysis | spike và question dùng `findings`, `answer` |
| R4 | `revision_feedback` lấy từ run, không thêm cột | Một nguồn sự thật; nạp theo lô |
| R5 | Activity không bao giờ chọn cột `feedback` | C9 |
| R6 | Điền phản hồi cũ vào prompt | Agent cần cả chuỗi yêu cầu để không lặp lỗi cũ |

## 4. Phụ thuộc và thứ tự

01 quyền → 02 trường và prompt → 03 activity → 04 đường lỗi, gateway, CONTRACT, e2e. 02 và 03 song song sau 01.

## 5. Kiểm thử

- Quyền: bảng (reporter, admin, lead, owner, approver theo user/team/role, người chỉ đọc, người lạ) × (`analyzing` không phản hồi, `awaiting_analysis_approval` có phản hồi) cho cả ba kiểu cổng.
- Thứ tự: test khẳng định `AuthorizeRevise` chạy trước `CancelPending`.
- Hợp đồng hai dialect: `revision_feedback` theo lô; nhánh activity; chỉ mục (EXPLAIN trên Request có ≥ 50 lượt).
- Prompt: golden cho prompt có 3 vòng chỉnh (kiểm phản hồi cũ và mới đều có, đều nằm trong khối bọc).
- Không lộ: test log/span/khung sự kiện không chứa nội dung phản hồi.
- E2E: `suite_analysis_revision.py` trên fixture tài chính cho bốn kiểu tài liệu.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Lượt chỉnh thất bại sau khi bản cũ bị `superseded`** (CR R2): không khôi phục được bản cũ; chấp nhận vì người dùng chỉnh lại hoặc Regenerate. Cần kiểm trạng thái sau thất bại bằng test thật.
- **Chế độ knowledge hub/OpenSpec** (CR R4): `openspec_solution_prompt.go` không thấy xử lý `Feedback`; chỉ ảnh hưởng nếu đường đó sinh Solution. Task 04 xác nhận đường nào thực sự chạy khi `solutionAgent=true` và, nếu thiếu, thêm phản hồi vào đó.
- **Kích thước prompt** tăng theo số vòng: `priorArtifacts` mỗi bản bị cắt 8000 rune, nhưng số bản không giới hạn (quyết định E2); cần tổng giới hạn rune cho khối `prior_artifacts` (đề xuất 40000, giữ bản mới nhất khi cắt) để prompt không vượt trần.
- Chưa chạy thật.

## 7. Câu hỏi mở

Còn một: có cần hoàn tác về bản cũ (CR-REQ-041 mục 7). Đã chốt: approver được chỉnh; phản hồi đọc được bởi mọi người đọc được Request; không giới hạn số lần chỉnh.

## 8. Tham chiếu

- `backend-go/services/request-service/internal/usecase/{generate_solution,solution_authorization,run_solution_generation,solution_prompt,run_agent_readonly_analysis,recover_interrupted_analysis_runs}.go`
- `backend-go/services/request-service/internal/adapter/postgres/request_activity.go`, `migrations/postgres/0030_solution_analysis.up.sql`
- `backend-go/proto/orca/request/v1/solution.proto`
- [CR-REQ-041](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md), kế hoạch tổng: [`IMPLEMENTATION-PLAN-CR-REQ-039-042`](../../../../../IMPLEMENTATION-PLAN-CR-REQ-039-042.md)
