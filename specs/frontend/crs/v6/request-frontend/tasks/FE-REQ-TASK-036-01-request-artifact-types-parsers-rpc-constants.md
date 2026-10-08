# FE-REQ-TASK-036-01: Kiểu và parser Clarification, Decision, Impact, Readiness, ExecutionResult; hằng kênh; `awaiting_information`

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.2, 2.3
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/request-artifact-types.ts`, `request-artifact-parsers.ts` (mới); `frontend/src/shared/request-types.ts`, `request-flow-registry.ts`, `request-rpc-methods.ts`, `request-errors.ts` (sửa, tạo ở FE-REQ-TASK-018-01/018-02); test cùng tên
**Depends on:** FE-REQ-TASK-018-01 (kiểu, registry, parser nền), 018-02 (`classifyRequestRpcError`)
**Status:** [x] DONE (verified 2026-10-07: vitest shared/request-artifact-parsers (14), request-errors, request-flow-registry pass; oxlint+tsc clean)

## Context

- `RequestStatus` hiện có **11** giá trị ở CONTRACT mục 1 và FE-REQ-SOL-018; CR-REQ-028 mục 2.1 nâng lên **12** (`awaiting_information`). Backend: trigger `information_required` vào trạng thái này từ `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`; `information_provided` ra tới `resume_status` (`analyzing`, `planning`, `executing`); `return_to_backlog`, `type_change`, `cancel` cũng nhận nguồn này.
- `REQUEST_FLOW_REGISTRY` (018-01) chép bảng README 3.4; `awaiting_information` không thuộc luồng nào riêng: là trạng thái chen ngang (CR-028 quyết định 1), nên không thêm khoá vào mỗi loại mà thêm hằng `REQUEST_INTERRUPT_STATUSES = ['awaiting_information']` và hàm `isInterruptStatus`.
- Hình dạng Clarification (CR-028 proto, đã đọc): `Clarification {id, display_id, request_id, source, source_ref, status, resume_status, round, questions[], due_at, version}`; `ClarificationQuestion {id, seq, question_key, kind: TEXT|SINGLE_CHOICE|MULTI_CHOICE|FILE|BOOLEAN, prompt, reason, options_json, suggested_default_json, required, target_path, answer_json}`; `AnswerItem {question_id, value_json, accept_default}`. Qua gateway: camelCase (CONTRACT C-quy tắc); `options_json`, `suggested_default_json`, `answer_json` có thể là chuỗi JSON hoặc đối tượng (chưa chốt): parser nhận cả hai.
- Decision (CR-028 2.8): `status open|chosen|effective|superseded`, `risk_level normal|high`, `subject_digest`, `rationale`, `chosen_option_id`, `recommended_option_id`. Không có kênh `decision.record`; chỉ `decision.list|confirm`.
- Impact (CR-030 2.8): `impact.get` trả mức, điểm, ba lý do chính, độ tin cậy, tuổi index, `narrative`; `impact.compare` ma trận Option × 9 chiều; `impact.findings`; `impact.accept`; `impact.drift`. TaskReadiness (CR-029 2.4): `outcome ready|needs_info|spec_defect|env_defect`, `tier`, `findings[{code,tier,path?,message}]`. ExecutionResult (CR-029 2.5/2.6): `parse_status ok|missing|invalid`, `status`, `files_changed`, `checks_run`, verdict, `failure_class`.
- Mẫu: `request-wire-parsers.ts` chịu enum lạ (`'unknown'`); `request-errors.ts` bảng `kind`.

## Việc cần làm

1. `request-types.ts`: thêm `'awaiting_information'` vào `RequestStatus` (thứ 12); cập nhật mọi bảng `Record<RequestStatus, …>` đang có trong repo (`request-status-presentation.ts`, `REQUEST_STATUS_ORDER`): `rg -n "Record<RequestStatus" frontend/src` để tìm; thiếu khoá là lỗi biên dịch (chủ ý).
2. `request-flow-registry.ts`: `REQUEST_INTERRUPT_STATUSES`, `isInterruptStatus(s)`; `REQUEST_STATUS_ORDER` thêm vị trí cho `awaiting_information` (đặt sau `executing`, ghi chú rằng thứ tự thật là bước đang chờ, nên `RequestStageTimeline` phải dùng `resumeStatus` của Clarification để định vị).
3. `request-artifact-types.ts`: khai các kiểu theo SOL-036 mục 2.2 (`Clarification`, `ClarificationQuestion`, `ClarificationQuestionKind`, `ClarificationSource`, `ClarificationStatus`, `Decision`, `DecisionStatus`, `ImpactSummary`, `ImpactFinding {id, dimension, level: GraphRisk, title, evidenceRef?, nodeIds?}`, `ImpactComparison {optionId, dimensions: Record<string, {level, score: number|null, note?: string}>}`, `RiskAcceptance {id, assessmentId, findingId, rationale, acceptedBy, createdAt}`, `ImpactDrift`, `TaskReadinessReport`, `TaskReadinessOutcome`, `ExecutionResult`, `ExecutionFailureClass`, `AnswerPayload {clarificationId, answers: {questionId: string; valueJson: string; acceptDefault: boolean}[], complete: boolean, expectedVersion: number}`). Dùng `GraphRisk` từ `graph-types.ts` (032-03) cho mọi mức rủi ro.
4. `request-artifact-parsers.ts`: `parseClarification` (kind: nhận hai họ tên `single|single_choice`, `multi|multi_choice`, `confirm|boolean` và chuẩn hoá; `source` lạ thành `'unknown'`), `parseDecision`, `parseImpactSummary` (mức rủi ro thiếu → `'unknown'`; `status` lạ → `'unknown'`), `parseImpactFinding`, `parseImpactComparison`, `parseTaskReadinessReport`, `parseExecutionResult`. Không ném lỗi; thiếu trường bắt buộc `id` thì trả `null` và nơi gọi bỏ mục. `options_json`/`suggested_default_json`/`answer_json`: nếu chuỗi, `JSON.parse` trong `try/catch` (lỗi giữ nguyên chuỗi).
5. `request-rpc-methods.ts`: thêm `CLARIFICATION_LIST|GET|ANSWER|CANCEL`, `REQUEST_READINESS`, `DECISION_LIST|CONFIRM`, `IMPACT_REQUEST|COMPARE|FINDINGS|EVIDENCE|HEATMAP|ACCEPT|DRIFT` (đã có `IMPACT_GRAPH|GET` từ 032-03), `RISK_OVERRIDE`, `READINESS_CHECK|GET|LIST`, `EXECUTION_GET` (kênh đề xuất, chú thích "(tạm, chưa có trong CONTRACT)"). Khoá giá trị là chuỗi kênh (`'clarification.answer'`...). Nhóm `GROUP_PREFIXES = ['clarification', 'decision', 'impact', 'risk', 'readiness', 'execution']` để mở rộng.
6. `request-errors.ts`: thêm ánh xạ mã `REQUEST_CLARIFICATION_*` (`NOT_FOUND` → `not_found`; `NOT_OPEN`, `ALREADY_ANSWERED`, `STATE_NOT_ALLOWED` → `invalid_state`; `EXPIRED` → `expired`; `INCOMPLETE`, `INVALID_ANSWER` → `validation`; `NOT_ASSIGNEE` → `forbidden`; `VERSION_CONFLICT` → `conflict`), `REQUEST_DECISION_*` (`RATIONALE_REQUIRED`, `CONFIRMATION_MISMATCH` → `validation`; `NOT_EFFECTIVE` → `invalid_state`; `SELF_CHOICE_FORBIDDEN`, `AGENT_FORBIDDEN` → `forbidden`), `REQUEST_RISK_*` (`ASSESSMENT_PENDING` → `pending` mới; `ACCEPTANCE_REQUIRED` → `validation`; `ASSESSMENT_STALE` → `conflict`; `APPROVER_NOT_ALLOWED` → `forbidden`; `OVERRIDE_REASON_REQUIRED` → `validation`), `REQUEST_IMPACT_NO_CONNECTION` → `no_dev_server` mới, `REQUEST_READINESS_WAIVE_FORBIDDEN` → `forbidden`. Thêm hai `kind` mới `pending` và `no_dev_server` vào kiểu `RequestRpcError.kind` (kiểm mọi `switch` bằng `pnpm run lint:switch-exhaustiveness`).
7. Hằng sự kiện `ARTIFACT_EVENT_TYPES` (chuỗi theo CONTRACT mục 7, chưa chốt): `clarification.requested|answered|expired|cancelled`, `decision.recorded|confirmed`, `impact.assessed`, `impact.drift_detected`, `risk.accepted`, `readiness.reported`, `execution.verified`.

## Bảng tham chiếu nhanh

| Nhóm | Kênh (hằng) | Tham số chính |
|---|---|---|
| Clarification | `clarification.list`, `.get`, `.answer`, `.cancel` | `{requestId}`, `{clarificationId}`, `{clarificationId, answers[], complete, expectedVersion}` |
| Request readiness | `request.readiness` | `{id}` |
| Decision | `decision.list`, `decision.confirm` | `{requestId}`, `{decisionId, confirmationText, expectedVersion}` |
| Impact | `impact.request\|compare\|findings\|evidence\|heatmap\|accept\|drift` | theo CR-030 2.8 |
| Risk | `risk.override` | `{requestId, gate, reason}` |
| Task readiness | `readiness.check\|get\|list` | `{taskId}`, `{phaseId}` |
| Execution | `execution.get` (đề xuất) | `{taskId, latestOnly?}` |

- Khoá i18n: không. Phím tắt: không. Trạng thái UI: không (task thuần kiểu, parser).
- Quy tắc parser: enum lạ thành `'unknown'`, thiếu `id` thì bỏ mục, JSON hỏng giữ chuỗi thô.

## Trình tự làm gợi ý

1. Viết test parser trước (enum lạ, JSON hỏng, hai họ tên `kind`).
2. Thêm `awaiting_information` vào `RequestStatus`, chạy typecheck để tìm mọi `Record<RequestStatus, …>` thiếu khoá.
3. Viết `request-artifact-types.ts`, `request-artifact-parsers.ts`.
4. Thêm hằng kênh và ánh xạ mã lỗi mới; chạy `pnpm run lint:switch-exhaustiveness`.
5. Phối hợp với người làm FE-REQ-TASK-018-01/018-02 nếu chưa merge (gộp một PR).

## Kiểm thử

- `request-artifact-parsers.test.ts`: mỗi parser với (a) bản đủ trường, (b) enum lạ, (c) thiếu trường, (d) JSON hỏng trong `options_json`; `kind` hai họ tên cho cùng kết quả; `risk` thiếu thành `unknown`, không `low`; đầu vào `null`/số trả `null`/mặc định không ném.
- `request-errors.test.ts` (mở rộng): mỗi mã mới ra đúng `kind`; mã lạ `REQUEST_FOO_BAR` → `unknown`.
- `request-flow-registry.test.ts` (mở rộng): `isInterruptStatus('awaiting_information')` true; 12 trạng thái.
- Kiểm biên dịch: chạy typecheck của repo (`pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json`, chưa kiểm chứng như SOL-018) và `pnpm run lint:switch-exhaustiveness` để bắt `switch`/`Record` thiếu `awaiting_information`.
- Chạy: `pnpm --filter orca-frontend test src/shared/request-artifact-parsers src/shared/request-errors src/shared/request-flow-registry`.

## Tiêu chí hoàn thành

- [ ] `RequestStatus` có 12 giá trị; mọi `Record<RequestStatus, …>` biên dịch được.
- [ ] Parser chịu enum lạ và JSON hỏng, không ném lỗi.
- [ ] Hằng kênh nằm ở `request-rpc-methods.ts`, kênh chưa có trong CONTRACT được đánh dấu "(tạm)".
- [ ] Mã lỗi `REQUEST_CLARIFICATION_*`, `REQUEST_DECISION_*`, `REQUEST_RISK_*` ra đúng `kind`.
- [ ] Không file `shared/` nào import từ `renderer/`.

## Rủi ro và lưu ý

- Thay đổi `RequestStatus` chạm FE-REQ-SOL-018, 019 (badge, bộ lọc): phối hợp với người làm các task đó, không sửa chúng ngoài phạm vi dòng thêm khoá; nếu 018-01 chưa merge, gộp thay đổi này vào cùng PR.
- Tên trường backend qua gateway (`displayId`, `resumeStatus`, `suggestedDefault`) là suy ra từ proto camelCase; đối chiếu CONTRACT khi nhóm kênh được thêm.
- `options_json` là chuỗi hay đối tượng chưa rõ; parser nhận cả hai là có chủ đích.
- Hai `kind` lỗi mới (`pending`, `no_dev_server`) cần cập nhật `FE-REQ-TASK-018-02` hoặc bảng `kind` ở SOL-018: ghi chú trong PR.

## Ghi chú triển khai (2026-10-07)

`awaiting_information` đã có sẵn trong `RequestStatus`/`REQUEST_STATUS_ORDER` (đợt trước). Thêm `request-artifact-types.ts`, `request-artifact-parsers.ts`, `isInterruptStatus`, hằng kênh clarification/decision/impact/readiness/execution, ánh xạ mã lỗi (`REQUEST_CLARIFICATION_*`, `REQUEST_DECISION_*`, `REQUEST_RISK_*`...) và 3 `kind` mới `expired|pending|no_dev_server`. `Approval.stage` thêm (additive) cho `drift_review`. Chưa chạy `pnpm run lint:switch-exhaustiveness`.
