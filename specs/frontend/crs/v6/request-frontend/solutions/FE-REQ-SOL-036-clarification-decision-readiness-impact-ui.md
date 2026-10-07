# FE-REQ-SOL-036: UI Clarification, Decision, Readiness, thẻ rủi ro, RiskAcceptance, lệch kế hoạch, kết quả thực thi

> 🔴 **Not Started.** Rà soát 2026-10-07: chưa có `ClarificationPanel`, `useClarifications`, `RiskSummaryCard`, `ReadinessBadge`, `ExecutionResultPanel`. Viết ngày 2026-10-06 từ CR-REQ-036 và đối chiếu CR-REQ-028, 029, 030, CONTRACT backend; chưa chạy test hay ứng dụng.

**CR:** [CR-REQ-036](../../../../../../docs/crs/v6/request-frontend/CR-REQ-036-clarification-decision-readiness-impact-ui.md), [ADDENDUM-2026-10-06](../../../../../../docs/crs/v6/request-frontend/ADDENDUM-2026-10-06.md)
**Area:** frontend (`frontend/src/shared`, `components/request/{clarification,decision,readiness,impact,execution}/`, `hooks/`)
**Hợp đồng backend:** [CONTRACT-request-ui-api.md](../../../../../backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md) mục 7 (chỗ mở rộng `clarification.*`, `decision.*`, `impact.*`, `readiness.*`); [CR-REQ-028](../../../../../../docs/crs/v6/request-artifact-model/CR-REQ-028-clarification-and-decision-records.md), [CR-REQ-029](../../../../../../docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md), [CR-REQ-030](../../../../../../docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md).
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md); [v4/05-runtime-client](../../../../tdd/v4/05-runtime-client.md)
**Phụ thuộc solution:** [FE-REQ-SOL-018](./FE-REQ-SOL-018-request-frontend-foundation.md), [020](./FE-REQ-SOL-020-solution-review-ui.md), [021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md), [FE-REQ-SOL-032](./FE-REQ-SOL-032-graph-canvas-and-lenses.md) (`RiskBadge`, `riskPresentation`, `GraphMini`, `GraphPanel`).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `frontend/src/renderer/src/lib/screen-submit-shortcut.ts` (`isScreenSubmitShortcut`, `getScreenSubmitModifierLabel`), `components/ShortcutKeyCombo.tsx`, `components/ui/{dialog,sheet,table,textarea,checkbox,progress,tabs,skeleton,badge,button}.tsx`, `components/task/TaskDetail.tsx`, `guides/STYLEGUIDE.md` (mục destructive dòng 296, mục "UI copy must not overclaim" dòng 236), các solution 018 đến 021 và 032, và các tài liệu backend 028, 029, 030 và CONTRACT.

- Chưa có `components/request/` (FE-REQ-SOL-018 đến 021 chưa triển khai). `RequestStatus` ở CONTRACT mục 1 và FE-REQ-SOL-018 có **11** giá trị, không có `awaiting_information`; CR-REQ-028 mục 2.1 nâng lên **12**.
- `components/ui/` không có `radio-group`, `switch`, `alert`: câu hỏi `single_choice` dùng `ToggleGroup type="single"` hoặc `<input type="radio">` có `Label`; cờ xác nhận dùng `checkbox.tsx`. Chọn `ToggleGroup` để theo primitive có sẵn (xem task 036-03).
- `TaskDetail.tsx` không nhận props, đọc `activeTaskId` từ store (ghi trong SOL-021); `ExecutionResultPanel` và `ReadinessBadge` cắm qua store và `usePlanTree`.
- Không có thành phần tải tệp dùng lại được (không có file tên `*dropzone*` hay `*file-upload*` trong `renderer/src`; chỉ `components/admin/fleet/fleet-import-dialog.tsx` dùng `<input type="file">`, chưa đọc kỹ); backend v1 chỉ nhận `file` là **văn bản** tối đa 64 KB (CR-028 2.4 bước 2).
- `RejectReasonDialog` (SOL-020 task 020-04) có `REJECT_REASON_MIN_LENGTH = 10`; tái dùng ngưỡng cho lý do chọn khác đề xuất và chấp nhận rủi ro.

**Correction relative to CR-REQ-036 (CONTRACT và CR backend thắng khi lệch):**

| # | CR-036 ghi | CR backend / CONTRACT | Quyết định |
|---|---|---|---|
| 1 | Kiểu câu hỏi `text|single|multi|file|confirm` | CR-028 proto: `TEXT|SINGLE_CHOICE|MULTI_CHOICE|FILE|BOOLEAN` | Parser nhận cả hai họ, chuẩn hoá về `text|single_choice|multi_choice|file|boolean`; UI hiển thị `boolean` là Có/Không |
| 2 | Gọi `decision.record {...}` rồi `solution.choose`/`approval.approve` | CR-028 2.8 bước 1: `ChooseSolutionOption` **tự gọi** `RecordDecision` cùng transaction; kênh là `decision.list|confirm` (CR-028 mục 9 dòng 227), **không có** `decision.record` | Không khai `decision.record`. Lý do chọn đi cùng `solution.choose` (trường `rationale`, tạm; CONTRACT hiện có `comment?`). Xác nhận lần hai: `decision.confirm` |
| 3 | Xác nhận gõ tên "phân biệt hoa thường" | CR-028 2.8 bước 4: so khớp **NFC, cắt khoảng trắng, không phân biệt hoa thường** (`REQUEST_DECISION_CONFIRMATION_MISMATCH`) | Client so khớp đúng quy tắc server (NFC, trim, không phân biệt hoa thường); server là nguồn chân lý |
| 4 | Phương án rủi ro cao theo `risk ≥ high` hay `breaking_change`... | CR-028: `risk_level normal|high` của **Decision** do `DecisionRisk.Assess` (backend); khác với mức rủi ro đánh giá tác động (CR-030) | Hai khái niệm: `Decision.riskLevel` điều khiển hộp gõ tên; `ImpactAssessment.level` điều khiển `RiskAcceptance`. UI không tự suy `riskLevel` |
| 5 | `Trả về Plan` đưa Request về `planning`; có "Huỷ Phase" | CR-030 2.7: drift mở Approval `subject_type=phase`, `stage=drift_review`; `OnApproved` tiếp tục, `OnRejected` là `ReturnToBacklog(stage=phase, category=rejected)`; **không có RPC huỷ Phase** | "Chấp nhận lệch" = `approval.approve`; "Trả về" = `approval.reject` (kết quả là Request về backlog, ghi rõ trong dialog); bỏ nút "Huỷ Phase" |
| 6 | Người không đủ vai trò thấy dòng "Cần người duyệt có vai trò X" | CR-030 2.6: `REQUEST_RISK_APPROVER_NOT_ALLOWED`; quyền thật theo `team:<risk_approver_team>` | Hiện dòng chung "Cần người duyệt thuộc nhóm được chỉ định" kèm tên team chỉ khi backend trả; không suy vai trò ở client |
| 7 | Duyệt gửi `impact.accept` cho từng phát hiện rồi `approve` | CR-030 mục 8 (bảng tác động CR-009): `DecideApprovalRequest` thêm `viewed_impact_digest`, `accepted_finding_ids[]` | `approval.approve` mang `viewedImpactDigest` (mức Trung bình trở lên) và `acceptedFindingIds` (mức Cao); tên camelCase chưa chốt trong CONTRACT (tạm) |
| 8 | "Bỏ qua cổng" ghi đè, lý do tối thiểu 10 | CR-030 2.6: `risk.override {requestId, gate, reason}`, `reason` **≥ 20** ký tự; quyền `team:<risk_override_team>` hoặc admin | Ngưỡng 20 cho override, 10 cho acceptance |
| 9 | Bản nháp trả lời chỉ ở client | CR-028: `AnswerClarification complete=false` **lưu nháp phía server** | Giữ nháp ở client (không `localStorage`) và **không** gọi `complete=false` tự động; ghi câu hỏi mở 3 |
| 10 | `clarification.answer {id, answers[], version}` | CR-028 proto: `answers[{questionId, valueJson, acceptDefault}]`, `complete`, `expectedVersion` | Dùng tên CR-028 camelCase: `{clarificationId, answers, complete, expectedVersion}` |
| 11 | Có `readiness.*` cho Request và Task | CR-028: `request.readiness` (cấp **Request**, `GetRequestReadiness`); CR-029: `readiness.check|get|list` (cấp **Task**, `TaskReadinessReport`) | Hai kiểu riêng: `RequestReadinessReport` (chỉ dùng ở bước hỏi) và `TaskReadinessReport`; `ReadinessBadge` chỉ cho Task |
| 12 | `ExecutionResultPanel` đọc "hợp đồng kết quả" | CR-029 2.5: bản ghi lưu ở `task_execution_records`; RPC `ListExecutionRecords` là nội bộ; **chưa có kênh WS cho UI** | Cần kênh mới `execution.get {taskId}` (tên đề xuất, câu hỏi mở 1); khi chưa có thì panel hiện `stdout` cũ (đúng với task trước CR-029) |
| 13 | `impact.get|graph|accept|drift` | CR-030 2.8 còn `impact.request`, `impact.compare`, `impact.findings`, `impact.evidence`, `impact.heatmap`, `risk.override` | `SolutionDimensionTable` dùng `impact.compare`; `ImpactFindingList` dùng `impact.findings` và `impact.evidence`; khởi chạy đánh giá dùng `impact.request` |
| 14 | Điểm và "ba lý do chính" "do backend hay frontend suy ra" (câu hỏi 3) | CR-030 `impact.get`: trả mức, điểm, **3 lý do chính**, độ tin cậy, tuổi index, `narrative` | Backend trả sẵn; frontend không suy |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/
  request-types.ts                (sửa, SOL-018) RequestStatus += 'awaiting_information'; kiểu mới ở mục 2.2
  request-artifact-types.ts       (mới) Clarification*, Decision*, ImpactAssessment*, RiskAcceptance, TaskReadinessReport, ExecutionResult
  request-artifact-parsers.ts     (mới) parseClarification, parseDecision, parseImpactSummary, parseTaskReadiness, parseExecutionResult
  request-rpc-methods.ts          (sửa) hằng kênh mục 2.3
  request-flow-registry.ts        (sửa) thêm awaiting_information
frontend/src/renderer/src/
  hooks/useClarifications.ts  useDecisions.ts  useImpactAssessment.ts  useTaskReadiness.ts  useExecutionResult.ts   (mới)
  components/request/clarification/
    ClarificationPanel.tsx  ClarificationQuestionList.tsx  ClarificationQuestionField.tsx
    ClarificationDeadlineNote.tsx  clarification-answer-validation.ts (thuần)
  components/request/decision/
    SolutionDecisionBar.tsx (sửa, SOL-020)  DecisionRationaleField.tsx  HighRiskDecisionConfirmDialog.tsx
    DecisionHistoryList.tsx  decision-rules.ts (thuần)
  components/request/impact/
    RiskSummaryCard.tsx  SolutionDimensionTable.tsx  ImpactFindingList.tsx  ImpactEvidenceSheet.tsx
    RiskAcceptanceChecklist.tsx  RiskOverrideMenu.tsx  risk-approval-rules.ts (thuần)
  components/request/readiness/
    ReadinessBadge.tsx  ReadinessReportSheet.tsx  PlanDriftBanner.tsx  PlanDriftReviewSheet.tsx
    readiness-action-rules.ts (thuần)
  components/request/execution/
    ExecutionResultPanel.tsx  ExecutionChecksTable.tsx  execution-result-comparison.ts (thuần)
  components/request/plan/PlanTaskRow.tsx (sửa, SOL-021) ReadinessBadge, chip lệch
  components/request/RequestStageTimeline.tsx, RequestDetailHeader.tsx (sửa, SOL-019)
  components/task/TaskDetail.tsx (sửa) tab "Kết quả", khoá Chạy theo sẵn sàng
  i18n/locales/{en,es,ja,ko,zh}.json (sửa)  i18n/request-artifact-locale-coverage.test.ts (mới)
```

### 2.2 Kiểu (chuẩn hoá từ CR-028, 029, 030; camelCase qua gateway)

```ts
type ClarificationQuestionKind = 'text'|'single_choice'|'multi_choice'|'file'|'boolean'
type ClarificationSource = 'readiness'|'solution_open_question'|'plan_assumption'|'task_blocked'|'unknown'
type ClarificationStatus = 'open'|'answered'|'expired'|'cancelled'
type ClarificationQuestion = { id: string; seq: number; questionKey: string; kind: ClarificationQuestionKind
  prompt: string; reason?: string; options?: { value: string; label: string }[]
  suggestedDefault?: unknown; required: boolean; answer?: unknown }
type Clarification = { id: string; displayId: string; requestId: string; source: ClarificationSource
  sourceRef?: string; status: ClarificationStatus; resumeStatus?: string; round: number
  questions: ClarificationQuestion[]; dueAt?: string; assigneeIds?: string[]; version: number }
type DecisionStatus = 'open'|'chosen'|'effective'|'superseded'
type Decision = { id: string; displayId: string; subjectKind: string; subjectId: string; subjectDigest: string
  chosenOptionId?: string; recommendedOptionId?: string; rationale: string
  riskLevel: 'normal'|'high'; status: DecisionStatus; chooserId?: string; confirmedBy?: string; version: number }
type ImpactLevel = 'low'|'medium'|'high'|'critical'|'unknown'   // = GraphRisk
type ImpactSummary = { assessmentId: string; digest: string; level: ImpactLevel; score: number|null
  topReasons: string[]; confidence: 'low'|'medium'|'high'|null; indexAgeCommits?: number
  assessedAt: string|null; tool: string|null; stale: boolean; mode: 'shadow'|'enforce'
  status: 'collecting'|'ready'|'partial'|'failed'|'unknown'; hardRules: string[]; narrative?: string }
type TaskReadinessOutcome = 'ready'|'needs_info'|'spec_defect'|'env_defect'
type TaskReadinessReport = { taskId: string; outcome: TaskReadinessOutcome|'unknown'; tier?: string
  findings: { code: string; tier: string; path?: string; message: string }[]
  specDigest?: string; durationMs?: number; checkedAt?: string }
type ExecutionFailureClass = 'retryable'|'needs_info'|'spec_defect'|'env_defect'|'agent_defect'|'unknown'
type ExecutionResult = { taskId: string; attempt: number; parseStatus: 'ok'|'missing'|'invalid'
  status?: 'done'|'blocked'|'failed'|'needs_info'; summary?: string; filesChanged: string[]
  checksRun: { id: string; exit: number }[]; verdict?: { status: 'passed'|'failed'; findings: { code: string; message: string }[] }
  failureClass?: ExecutionFailureClass; stdoutTail?: string; outputs?: Record<string, unknown> }
```

Mọi enum lạ thành `'unknown'` (đúng nguyên tắc parser của SOL-018). `ImpactSummary.status`, `hardRules`, `ExecutionResult.verdict` là suy luận từ CR-030 2.8 và CR-029 2.6; hình dạng JSON thật **chưa chốt** (câu hỏi mở 2).

### 2.3 Kênh WS (tạm; đối chiếu CONTRACT khi nhóm kênh được thêm)

Hằng ở `request-rpc-methods.ts`; mọi tham số camelCase, không gửi `tenantId`/`userId`; lỗi qua `classifyRequestRpcError` (mã `REQUEST_<NHÓM>_*`).

| Nhóm | Kênh | Tham số | Dùng ở |
|---|---|---|---|
| `clarification.*` | `list {requestId, status?}`, `get {clarificationId}`, `answer {clarificationId, answers[{questionId, valueJson, acceptDefault}], complete, expectedVersion}`, `cancel {clarificationId, expectedVersion}` | CR-028 mục 9 | 2.4 |
| `request.readiness` | `{id}` | `GetRequestReadiness` (cấp Request) | 2.4 (hiển thị mục thiếu) |
| `decision.*` | `list {requestId}`, `confirm {decisionId, confirmationText, expectedVersion}` | CR-028 | 2.5 |
| `solution.choose` | thêm `rationale?` (tạm) | CONTRACT 2.2 hiện `comment?` | 2.5 |
| `impact.*` | `get`, `request {subjectType, subjectId}`, `compare {solutionId}`, `findings {assessmentId, dimension?, minLevel?}`, `evidence {findingId}`, `heatmap`, `accept {assessmentId, findingId, rationale, assessmentDigest}`, `drift {phaseId}` | CR-030 2.8 | 2.6, 2.8 |
| `risk.override` | `{requestId, gate, reason}` | CR-030 2.6 | 2.6 |
| `readiness.*` | `check {taskId}`, `get {taskId}`, `list {phaseId}` | CR-029 2.4 | 2.7 |
| `execution.get` | `{taskId, latestOnly?}` | **chưa có**, đề xuất | 2.9 |
| `approval.approve` | thêm `viewedImpactDigest?`, `acceptedFindingIds?` | CR-030 mục 8 | 2.6 |

Sự kiện (CONTRACT mục 7: thêm giá trị `RequestEventType` additive; chuỗi chính xác chưa chốt): `clarification.requested|answered|expired|cancelled`, `decision.recorded|confirmed`, `impact.assessed`, `impact.drift_detected`, `risk.accepted`, `readiness.reported`, `execution.verified`. Mỗi sự kiện chỉ kích hoạt `refetch` của hook tương ứng; `request.status_changed` kèm `trigger=information_required|information_provided` làm mới Request. Sự kiện không mang nội dung câu hỏi hay câu trả lời.

### 2.4 Clarification (task 036-03)

- `ClarificationPanel` hiện khi `request.status === 'awaiting_information'` và `useClarifications` có Clarification `open` (mỗi Request tối đa một, CR-028 quyết định 3). `RequestStageTimeline` hiện nhãn "Chờ bổ sung thông tin" ở bước `resumeStatus`.
- Mỗi câu: `prompt`, dòng phụ `reason`, chip nguồn (`source`), giá trị mặc định đề xuất điền sẵn, nhãn "Đề xuất" và **không tự gửi**; `required` có dấu; người không thuộc `assigneeIds` (và không admin) thấy chỉ đọc kèm "Đang chờ người được chỉ định trả lời" (lỗi `REQUEST_CLARIFICATION_NOT_ASSIGNEE` là nguồn chân lý).
- Gửi một lần cho cả danh sách: `answer {complete: true, answers}`; khoá tới khi mọi câu `required` có giá trị hoặc `acceptDefault`. Phản hồi `stillMissing=true` (CR-028: `still_missing`) hiện danh sách mục còn thiếu và mở Clarification vòng kế (`round + 1`); vượt `REQUEST_CLARIFICATION_MAX_ROUNDS` Request về backlog `missing_info` (hiển thị ở màn backlog, SOL-023).
- `file`: chỉ văn bản tối đa 64 KB (`{filename, mime, size, text}`); chọn tệp bằng `<input type="file">` đọc `File.text()`; quá 64 KB hoặc nhị phân thì lỗi cạnh trường; không có kho tải lên.
- Nháp ở state hook (mất khi tải lại, cảnh báo khi rời) vì câu trả lời có thể nhạy cảm; không `localStorage`.
- Phím tắt: `isScreenSubmitShortcut(event)` gửi khi tiêu điểm trong ô văn bản (`metaKey` Mac, `ctrlKey` nơi khác), chip `ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']}`.
- Lỗi: `REQUEST_CLARIFICATION_VERSION_CONFLICT` tải lại và giữ nháp; `..._EXPIRED`/`..._NOT_OPEN` làm mới và chỉ đọc; `..._INVALID_ANSWER` lỗi cạnh trường (`aria-invalid`); `..._ALREADY_ANSWERED` làm mới.

### 2.5 Decision (task 036-04)

Mở rộng `SolutionDecisionBar` (SOL-020), thứ tự gọi: `solution.choose {…, rationale?}` (ghi Decision) → nếu `Decision.riskLevel === 'high'` thì `HighRiskDecisionConfirmDialog` → `decision.confirm` → `approval.approve`. `decision-rules.ts` thuần: `requiresRationale(chosen, recommended)`, `matchesConfirmation(input, title)` (NFC, trim, không phân biệt hoa thường), `canApprove(decision, approval, digest)`.

| Tình huống | UI |
|---|---|
| Chọn đúng đề xuất | Lý do tuỳ chọn |
| Chọn khác đề xuất | `DecisionRationaleField` bắt buộc, tối thiểu 10 ký tự sau trim (server chỉ đòi không rỗng, `REQUEST_DECISION_RATIONALE_REQUIRED`) |
| `Decision.riskLevel==='high'` | Hộp gõ tên: liệt kê `Decision.options[i].risk.reasons`, ô gõ lại tiêu đề, `Enter` xác nhận khi khớp; nút mặc định (không `destructive`) |
| `Decision.status==='chosen'` (chưa `effective`) | Banner "Chờ xác nhận lần hai"; nút Duyệt khoá (`REQUEST_DECISION_NOT_EFFECTIVE`) |
| `subjectDigest` đổi | Banner "Nội dung vừa thay đổi", khoá Duyệt, tải lại |
| `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN` | Dòng "Người báo cáo không được tự chọn", nút ẩn |
| Chọn lại khi Approval `pending` | Cho phép; `DecisionHistoryList` từ `decision.list` |

Duyệt Plan khi Solution chưa có Decision `effective`: nút Duyệt Plan khoá kèm tooltip (SOL-021 nhận cờ từ `useDecisions`).

### 2.6 Rủi ro và RiskAcceptance (task 036-05, 036-06)

- `RiskSummaryCard` (trong `SolutionOptionCard`): `RiskBadge` + điểm + `topReasons` (ba) + "Đánh giá lúc, dựa trên", `stale` thì "Index cũ, chưa đánh giá được đầy đủ"; không đánh giá thì "Chưa đánh giá" (không Thấp); `status==='collecting'` hiện skeleton và nhãn giai đoạn; `mode==='shadow'` thêm nhãn "Tham khảo" (CR-030 2.9: không chặn).
- `SolutionDimensionTable`: từ `impact.compare` (ma trận Option × 9 chiều) cộng hàng Effort và Quay lui của SOL-020; cột đề xuất đánh dấu, **không chọn sẵn**; ô thiếu dữ liệu hiện "Chưa đánh giá".
- `ImpactFindingList`: `impact.findings`, bằng chứng `impact.evidence` mở trong `Sheet`, văn bản thuần; nút "Xem trên đồ thị" mở `GraphPanel` (SOL-032) đúng lens với `meta.findingIds`.
- `risk-approval-rules.ts#getApprovalRequirements(summary, policyMode)`: `low` không đòi gì; `medium` đòi đã mở phần tác động (gửi `viewedImpactDigest = summary.digest`); `high` đòi `RiskAcceptance` cho mọi phát hiện từ Cao trở lên (mỗi cái `impact.accept {rationale ≥ 10}`), gửi `acceptedFindingIds`; `critical` như `high` cộng "Chờ người duyệt thứ hai" (Approval `requiredApprovals=2`, trường **chưa có trong CONTRACT**, đề xuất ở CR-030 mục 8). Chỉ `mode==='enforce'` mới chặn; `shadow` chỉ hiển thị.
- `RiskAcceptance` gắn `assessmentDigest`: digest đổi (`REQUEST_RISK_ASSESSMENT_STALE`) thì xác nhận cũ mất hiệu lực và UI dựng lại danh sách.
- "Duyệt nhanh" ở hộp duyệt (SOL-022) bị bỏ cho Approval có mức từ Trung bình: cung cấp cờ `requiresImpactReview` cho `ApprovalRow`; việc sửa SOL-022 là việc của SOL-022 (ghi trong mục 7).
- Override: `RiskOverrideMenu` (menu thừa, không phải nút chính) gọi `risk.override`, lý do ≥ 20 ký tự, ghi kiểm toán phía backend.

### 2.7 Readiness và lệch kế hoạch (task 036-07)

- `ReadinessBadge` trên `PlanTaskRow` và đầu `TaskDetail`: bốn kết quả, icon `CircleCheck`, `MessageCircleQuestion`, `FileWarning`, `ServerCrash`; chưa kiểm tra thì không badge; chỉ task làm việc (không `plan`, `phase`).
- `ReadinessReportSheet`: nhóm `findings` theo `tier` (structure, semantic, environment); mỗi mục mã ổn định + `message` + `path`; **chỉ tên** biến môi trường; hành động theo `readiness-action-rules.ts`: `ready` → "Chạy" (nút hiện có); `needs_info` → "Trả lời câu hỏi" (mở `ClarificationPanel`); `spec_defect` → "Sinh lại task" (đưa về bước Plan; backend là `ReturnToBacklog(stage=plan)`, nói rõ trong nhãn); `env_defect` → "Kết nối dev server" và dòng "Không tính vào số lần thử".
- Nút "Kiểm tra sẵn sàng" gọi `readiness.check {taskId}`; chạy thật vẫn tự kiểm lại phía backend (CR-029 cổng `ReadinessGate`). Runtime không có `readiness.*` thì giữ hành vi SOL-021 (không khoá).
- `PlanDriftBanner` khi `impact.drift_detected` hoặc Approval `stage==='drift_review'`: nút chính "Xem và duyệt lại" mở `PlanDriftReviewSheet` (bảng dự kiến so với thực tế từ `impact.drift`, `GraphPanel` lens `execution`). Hành động theo mục Correction 5: "Chấp nhận lệch và tiếp tục" (`approval.approve`, lý do bắt buộc) và "Trả về" (`approval.reject`; kết quả backlog). Không phím tắt.

### 2.8 Kết quả thực thi (task 036-08)

`ExecutionResultPanel` (tab "Kết quả" trong `TaskDetail`) từ `useExecutionResult(taskId)`:

- Khối đầu `status`, `summary`; `filesChanged` với nhãn trong/ngoài `scope` (`SCOPE_VIOLATION` từ `verdict.findings`, ngoài `scope` nổi bật bằng icon và chữ); bảng Check hai cột "Agent báo" (`checksRun`) và "Orca chạy lại" (`verdict`), `CHECK_MISMATCH` đánh dấu "Không khớp"; quét bí mật "Đạt/Không đạt/Chưa chạy" (CR-029 2.6 bước 4; công cụ do CR-REQ-035); `failureClass` kèm câu định tuyến; `needs_info` có nút "Chuyển thành câu hỏi".
- `parseStatus !== 'ok'`: "Kết quả không đúng định dạng" kèm `stdoutTail` văn bản thuần; task cũ không có bản ghi: hiện `stdout` như hiện nay, không lỗi.
- Khi chưa có kênh `execution.get` (`unsupported`): ẩn tab "Kết quả", giữ hành vi cũ.

### 2.9 Trạng thái rỗng, tải, lỗi, quyền

Chưa có Clarification mở: không render. Đang sinh đánh giá: skeleton + nhãn giai đoạn, hoãn 200 ms (độ trễ SSH). Không đánh giá (`REQUEST_IMPACT_ENABLED` tắt, `unsupported`): "Chưa đánh giá", Duyệt như SOL-020. `REQUEST_IMPACT_NO_CONNECTION`/không dev server: "Chưa có dev server kết nối" + nút kết nối. `forbidden`: ẩn nút ghi. `conflict`/`invalid_state`: tải lại, giữ lý do đã gõ. `unsupported`: ẩn khối, không lỗi đỏ.

### 2.10 i18n và token

Tiền tố `auto.components.request.{clarification,decision,readiness,impact,execution}.`; `RiskLevel.*` dùng chung `auto.components.graph.RiskLevel.*` (SOL-032). Đủ 5 locale `en, es, ja, ko, zh`, test phủ khoá. Màu: chỉ token (`--risk-*` từ SOL-032, `--status-success`, `--destructive`).

## 3. Quyết định thiết kế

- Hỏi lại tại bước đang chờ, gửi một lần; nháp chỉ ở client.
- Quy tắc server là nguồn chân lý (so khớp tên, ngưỡng lý do, quyền); client chỉ chặn sớm để giảm vòng khứ hồi, nên mọi quy tắc nằm ở hàm thuần `*-rules.ts` dễ đối chiếu.
- Không có `decision.record`: ghi Decision đi cùng `solution.choose`.
- UI không sửa điểm rủi ro; chỉ chấp nhận (có lý do), ghi đè (≥ 20 ký tự, kiểm toán).
- Không dùng `variant=destructive` cho xác nhận lần hai (STYLEGUIDE dòng 296).
- "Orca chạy lại" tách "Agent báo", đúng nguyên tắc không tin lời agent (CR-029 2.6).
- Không bao giờ hiện "an toàn", "đã phân tích xong" khi chưa có kết quả (STYLEGUIDE dòng 236).
- Mọi nhóm kênh mới ẩn êm khi `unsupported`; hành vi SOL-020, 021 giữ nguyên.

## 4. Phụ thuộc và thứ tự

Cần SOL-018 (RPC client, bus, `RequestStatus`), SOL-020 (`SolutionDecisionBar`, `RejectReasonDialog`), SOL-021 (`PlanTaskRow`, `TaskDetail` khoá Chạy), SOL-032 (`RiskBadge`, `GraphMini`, `GraphPanel`). Backend 028, 029, 030 chưa có solution: làm theo hợp đồng mock ở mức hook, đối chiếu khi chúng ra.

```
036-01 (kiểu, parser, hằng kênh) ─▶ 036-02 (hooks) ─┬─▶ 036-03 (Clarification)
                                                   ├─▶ 036-04 (Decision)
                                                   ├─▶ 036-05 (thẻ rủi ro, bảng chiều, phát hiện) ─▶ 036-06 (RiskAcceptance, cổng duyệt)
                                                   ├─▶ 036-07 (Readiness, lệch kế hoạch)
                                                   └─▶ 036-08 (Kết quả thực thi, i18n, e2e)
```

036-03, 036-04, 036-05, 036-07 song song sau 036-02. 036-04 cần SOL-020; 036-05 cần 032-01 và 032-05; 036-06 cần 036-04 và 036-05.

## 5. Kiểm thử

Vitest (`pnpm --filter orca-frontend test <đường dẫn>`). Unit: `request-artifact-parsers.test.ts` (enum lạ, tên kiểu câu hỏi hai họ), `clarification-answer-validation.test.ts` (5 kiểu, bắt buộc, `acceptDefault`, 64 KB), `decision-rules.test.ts` (khác đề xuất, 10 ký tự, NFC, hoa thường), `risk-approval-rules.test.ts` (bảng 4 mức × `shadow|enforce`), `readiness-action-rules.test.ts` (4 kết quả), `execution-result-comparison.test.ts`. Hook: huỷ khi unmount, `refetch` theo sự kiện, `conflict`, `unsupported`. Component: `ClarificationPanel`, `HighRiskDecisionConfirmDialog` (focus, Enter, Esc), `RiskAcceptanceChecklist`, `SolutionDimensionTable`, `ReadinessReportSheet`, `PlanDriftBanner`, `ExecutionResultPanel` (sai schema, task cũ). Phím tắt: `metaKey` giả lập Mac và `ctrlKey` Linux/Windows. `request-artifact-locale-coverage.test.ts`. E2E (cần backend 028, 029, 030) `tests/e2e/request-clarification-risk.spec.ts` (mới). Tất cả **chưa chạy**.

## 6. Rủi ro và điểm chưa kiểm chứng

- Ba CR backend chưa có solution: tên kênh, trường (`assessmentDigest`, `Failure.class`, `rationale` ở `solution.choose`, `viewedImpactDigest`) có thể đổi.
- Nhiều lớp xác nhận (lý do, gõ tên, chấp nhận từng phát hiện, người duyệt thứ hai) có thể gây mệt mỏi bấm cho qua; chưa có nghiên cứu người dùng.
- Dữ liệu tác động có thể đánh giá thấp tác động xuyên gRPC, WS, outbox (CR-030 mục 6): thẻ Thấp có thể gây tự tin sai, nên luôn kèm công cụ và thời điểm.
- Tải tệp ở Clarification chỉ văn bản 64 KB; người dùng mong đợi tệp nhị phân sẽ thất vọng.
- Quyền "người duyệt thứ hai", vai trò kiến trúc phụ thuộc mô hình quyền chưa có (README v6 mục 8 số 10).
- Quét bí mật trong diff chưa chọn công cụ (CR-REQ-035): panel có thể luôn "Chưa chạy".
- `execution.get` chưa tồn tại; nếu backend không thêm, tab "Kết quả" bị ẩn.

## 7. Câu hỏi mở

1. Kênh WS nào cho `ExecutionResult` (đề xuất `execution.get {taskId}`)? CR-029 mới có RPC nội bộ `ListExecutionRecords`.
2. Hình dạng JSON của `impact.get`, `impact.compare`, `impact.drift`, `readiness.get`; tên chính xác `viewedImpactDigest`, `acceptedFindingIds` trong `approval.approve`.
3. Có nên dùng `AnswerClarification complete=false` (nháp phía server) cho nháp dài? Hiện chọn không, vì dữ liệu nhạy cảm.
4. `solution.choose` nhận `rationale` hay dùng `comment` (CONTRACT 2.2)? Nếu gộp, `useSolutionDecision` đổi một chỗ.
5. SOL-022 cần sửa gì để bỏ "Duyệt nhanh" cho Approval mức từ Trung bình và hiện `RiskBadge` ở `ApprovalRow`; ai sở hữu (SOL-022)?
6. Người duyệt thứ hai chọn thế nào (`team:<id>` tự gán hay theo chính sách)?
7. `readiness.check` chạy đồng bộ hay trả `runId` rồi sự kiện `readiness.reported`?

## 8. Tham chiếu

`/opt/repos/orca/docs/crs/v6/request-frontend/CR-REQ-036-clarification-decision-readiness-impact-ui.md`, `/opt/repos/orca/docs/crs/v6/request-artifact-model/CR-REQ-028-clarification-and-decision-records.md`, `/opt/repos/orca/docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md`, `/opt/repos/orca/docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md`, `/opt/repos/orca/specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`, `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDetail.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/docs/research/receive-request/artifact-formats-ontology-and-execution-readiness.md`, `/opt/repos/orca/docs/research/receive-request/impact-assessment-and-risk-scoring.md`.
