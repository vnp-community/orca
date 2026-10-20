# FE-REQ-SOL-043: "Approve and run all steps", công tắc chế độ theo dự án và nhãn chế độ trên tab Execution

> **🟡 Đã triển khai 5/6 task (01 đến 05 DONE), task 06 PARTIAL (2026-10-20).** i18n 5 locale xong; hai spec giao diện sống đã viết và kiểm kiểu nhưng chưa chạy trên stack. Soạn 2026-10-20 từ đọc mã.

**CR:** [CR-REQ-043](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-043-approve-plan-and-run-all-steps.md)
**Area:** frontend / request
**Backend:** [BE-REQ-SOL-043](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md) (tham số `executionMode` của `approval.approve`, cờ `approval.runAll`, ba mã lỗi)
**Phụ thuộc solution:** [FE-REQ-SOL-021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) (thanh duyệt Plan), [FE-REQ-SOL-037](./FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) (tab Execution, `PlanApprovalCard`), [FE-REQ-SOL-040](./FE-REQ-SOL-040-request-base-branch-picker.md) (mẫu cờ tính năng `requestFlowFeatures`)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (đường dẫn dưới `frontend/src/renderer/src/` trừ khi ghi khác):

- `components/request/plan/RequestPlanTab.tsx`: `continuous = request.executionMode === 'continuous'` (`:148`); `continuous && planApproval?.status === 'pending'` thì dựng `PlanApprovalCard` (`:181-189`, `stepCount = flowView?.hasPhases ? tree.phases.length : 0`), ngược lại `PlanApprovalBar` (`:191-195`); `PlanTree` với `PhaseApprovalBar` cho chế độ từng bước (`:238-244`). Props: `request`, `onChanged`, `flow: RequestFlowView | null`, `onOpenExecution`.
- `plan/PlanApprovalBar.tsx` (132 dòng): `approveLabel`, `rejectLabel`; nút "Approve plan"/"Approve task list" qua `PlanDecisionButtons` (`:96-113`); "Regenerate plan" (`:55-70`); ẩn Approve khi `risk.approverNotAllowed` (`hideApprove`) và khoá khi `approveBlockedReason`.
- `plan/PlanDecisionButtons.tsx`: nút Approve gọi `decision.approve(approval, undefined, approveExtras)` (`:58`); khoá khi `approving || rejecting || isDenied || coolingDown`.
- `plan/PlanApprovalCard.tsx`: nút chính "Approve and start execution" (`:94`), nhãn "Request changes"; có danh sách lý do dừng (`needsInfo`, `failsTwice`, `drift` khi `stepCount > 0`, `preDeploy` khi `flow.startGate === 'pre_deploy'` hoặc `executionGates` có `pre_deploy`).
- `hooks/usePlanDecision.ts`: `approve(approval, comment, risk)` gọi `callRequestRpc(APPROVAL_APPROVE, {id, approvalId, expectedVersion, expectedDigest, comment, viewedImpactDigest?, acceptedFindingIds?})` (`:85-111`); khoá bận theo `approve:<id>` bằng `useRef` nên nhấp đúp bị chặn đồng bộ (`:44-83`); `PlanDecisionOutcome = {ok, error?}`.
- `plan/plan-decision-error-message.ts`: đã dịch `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` (`:23`); mã lạ rơi xuống `requestErrorMessageForError`.
- `execution/ExecutionSummaryStrip.tsx`: nhãn trạng thái (Running, Needs you, Stopped, Done), tiến độ, "Stop run" (`data-testid="execution-stop-run"`); không có Pause (F12 của CR-REQ-037). `execution/ExecutionNeedsYou.tsx`: thẻ "Approve step: X" (`:72`).
- `shared/request-execution-types.ts`: `ExecutionSummary.executionMode?: 'step_by_step' | 'continuous'` (`:107`); `RequestFlowView.hasPhases` cho cỡ được hỏi và `source: 'backend' | 'fallback'` (`:16-31`). `shared/request-types.ts:144` `OrcaRequest.executionMode`. Lưu ý: `executionMode` của Request chưa ghim là chế độ *sẽ dùng* (`request_engine_resolver.go:47`).
- Cờ tính năng: `store/slices/request.ts` `requestFlowFeatures: ReadonlySet<string>` (`:56`), `supportsBaseRef(state)` (`:78`), hằng `REQUEST_BASE_REF_FEATURE` ở `shared/request-base-ref.ts:16`; `hooks/useRequestFlowSupport.ts` ghi `features` từ `request.flowStatus`.
- `components/project/ProjectSettings.tsx` (255 dòng): tab General xếp `ProjectDevServerFilterSection`, `ProjectDevServerSection`, `ProjectMobileEmulatorAgentSection`, `ProjectJiraMappingSection` (`:169-175`), rồi vùng nguy hiểm cho chủ dự án. Mẫu một mục có lưu: `ProjectJiraMappingSection.tsx` (167 dòng, `translate('auto.components.project...')`, `toast`, `describeError`). Vai trò admin: `useAppStore.getState().currentUser?.role === 'admin'` (`store/slices/mcp-slice.ts:194`).
- `frontend/src` không gọi `request.engineSettings`/`engineSettingsSet`; `REQUEST_RPC_METHODS` (`shared/request-rpc-methods.ts`) không có hai tên đó. Backend: kênh trả `{projectId, engine, openspecMinVersion, solutionAgent, executionMode, version, preflight?}`; ghi cần **cả bộ** (`engine` rỗng thành `native`; chỉ `executionMode` rỗng là giữ nguyên) và chỉ admin.
- `components/ui` không có `alert-dialog`; có `dialog.tsx`, `tooltip.tsx`. `components/confirmation-dialog.tsx` có `useConfirmationDialog` (dùng ở `ProjectSettings.tsx:58`).
- `request-rate-limit.ts` đã xếp `APPROVAL_APPROVE` vào ghi và ghi lặp được (idempotent).
- i18n: khoá dạng `auto.components.request.<Component>.<key>`, 5 locale `i18n/locales/{en,es,ja,ko,zh}.json`; lint chạy `verify:localization-catalog` và `verify:localization-coverage` (đỏ sẵn trên HEAD, xem ghi chú FE-REQ-TASK-040-05).

### Correction relative to the request brief

1. Brief nói thêm nút phụ vào cả `PlanApprovalBar` và `PlanApprovalCard`. `PlanApprovalCard` chỉ dựng khi Request đã `continuous` và đã là hành động "duyệt và chạy hết"; thêm nút phụ ở đó là thừa. Thay vào đó nút chính của thẻ gửi `executionMode: 'continuous'` tường minh (cam kết của thẻ không phụ thuộc cài đặt dự án đổi giữa chừng). Xem D6 ở CR.
2. Brief gọi "engine settings pane" trong Project Settings: **không có** pane nào như vậy; phải tạo mục mới trong tab General.
3. Giao diện không phân biệt được ghim và "sẽ dùng" (trường `executionMode` chung); nút phụ có thể gặp `REQUEST_EXECUTION_MODE_ALREADY_PINNED` ở Request mở lại. Xử lý bằng thông điệp và ẩn nút (task 03), không thêm trường backend.
4. Cờ tính năng bắt buộc: gateway cũ bỏ qua `executionMode` nên nếu thiếu cờ thì người dùng bấm "chạy hết" mà Request vẫn chạy từng bước.

## 2. Giải pháp

### 2.1 Cây file

```
shared/request-run-all.ts                                                           (mới) APPROVAL_RUN_ALL_FEATURE, RUN_ALL_EXECUTION_MODE, parseApproveResult, ENGINE_SETTINGS parse
shared/request-rpc-methods.ts                                                       (sửa) ENGINE_SETTINGS, ENGINE_SETTINGS_SET
renderer/src/runtime/request-rate-limit.ts                                          (sửa) ENGINE_SETTINGS_SET vào WRITE_METHODS
renderer/src/store/slices/request.ts                                                (sửa) supportsApproveRunAll
renderer/src/hooks/usePlanDecision.ts                                               (sửa) approve(..., options), runAllApplied
renderer/src/components/request/plan/plan-run-all-model.ts                          (mới) canOfferRunAll, dialogCopyModel
renderer/src/components/request/plan/RunAllStepsConfirmDialog.tsx                   (mới)
renderer/src/components/request/plan/PlanRunAllButton.tsx                            (mới) nút phụ + hộp thoại + trạng thái
renderer/src/components/request/plan/PlanApprovalBar.tsx, PlanDecisionButtons.tsx, PlanApprovalCard.tsx, RequestPlanTab.tsx  (sửa)
renderer/src/components/request/plan/plan-decision-error-message.ts                 (sửa) ba mã mới
renderer/src/components/request/execution/ExecutionRunModeLabel.tsx, execution-run-mode-model.ts  (mới) + ExecutionSummaryStrip.tsx (sửa)
renderer/src/hooks/useProjectEngineSettings.ts                                      (mới)
renderer/src/components/project/ProjectRequestExecutionSection.tsx                  (mới) + ProjectSettings.tsx (sửa)
renderer/src/i18n/locales/{en,es,ja,ko,zh}.json                                     (sửa)
tests/stages/e2e/orca-flows/ui-live/approve-run-all.web.e2e.ts, live-run-all-steps.ts   (mới); continuous-execute-real.web.e2e.ts (sửa)
```

### 2.2 Dữ liệu, cờ tính năng và kết quả duyệt

- `APPROVAL_RUN_ALL_FEATURE = 'approval.runAll'`; `supportsApproveRunAll(state)` đọc `requestFlowFeatures`. Gateway cũ: tập rỗng, nút ẩn.
- `parseApproveResult(raw)`: `{requestStatus?: string, executionMode?: 'continuous' | 'step_by_step'}`; thiếu hoặc sai kiểu → `undefined`, không ném lỗi.
- `usePlanDecision.approve(approval, comment?, risk?, options?: {executionMode?: 'continuous'})`: thêm `executionMode` vào args khi có. Khoá bận vẫn `approve:<id>` (một khoá cho cả hai nút, nên không thể bấm hai nút gần nhau). Kết quả `PlanDecisionOutcome` thêm `runAllApplied?: boolean` = `true` khi đã yêu cầu và phản hồi `executionMode === 'continuous'`, `false` khi đã yêu cầu mà phản hồi khác (gateway lạ nuốt tham số).

### 2.3 Nút phụ và hộp thoại

`canOfferRunAll({request, approval, flow, supported, approverHidden, alreadyPinnedHint})` thuần:

- `approval?.status === 'pending'` và `approval.subjectType` (kể cả `rawSubjectType`) là `plan` hoặc `task_list`;
- `flow?.source === 'backend' && flow.hasPhases` (bảng `fallback` đã lệch với backend, CR-REQ-038 mục 1.3; không dựa vào nó để mời một hành động có hệ quả);
- `request.executionMode !== 'continuous'`;
- `supported` (cờ) và `!approverHidden` (`risk.approverNotAllowed`) và `!alreadyPinnedHint`.

`PlanRunAllButton` nằm cạnh nút Approve trong `PlanApprovalBar`: `variant="outline"`, nhãn "Approve and run all steps", cùng điều kiện khoá với nút chính (`locked`, `approveBlockedReason`, `coolingDown`). Bấm mở `RunAllStepsConfirmDialog` (`components/ui/dialog`): tiêu đề "Run all {{n}} steps without asking again?", thân (hai đoạn ngắn):

> Orca starts each step as soon as the previous one finishes, using your access. It stops only when the agent needs information from you, a task keeps failing (the request then goes back to the backlog), a change drifts from the plan and needs a review{{preDeploy}}. You can stop the run at any time.

Nút "Run all steps" (primary) và "Not now" (outline). `n` = `tree.phases.length` (cùng nguồn `stepCount` của thẻ); `{{preDeploy}}` = ", or it reaches the pre-deploy approval" khi `flow.startGate === 'pre_deploy'` hoặc `executionGates` có `pre_deploy`. Văn bản khác với bản gợi ý của người yêu cầu một vế ("when the agent needs information from you"), vì Clarification thật sự dừng run (CR-REQ-038). Xác nhận gọi `decision.approve(approval, undefined, approveExtras, {executionMode: 'continuous'})`; nút xác nhận có trạng thái bận (`Loader2`); Esc và "Not now" đóng, không gửi gì. Khi `runAllApplied === false`: toast cảnh báo "This server didn't run the plan on its own. Approve each step as usual.".

`PlanApprovalCard`: nút chính truyền `approveExecutionMode="continuous"` xuống `PlanDecisionButtons` (prop mới), không thêm nút.

### 2.4 Lỗi

`plan-decision-error-message.ts`: `REQUEST_EXECUTION_MODE_ALREADY_PINNED` → "This request already runs step by step because it was started that way before. Approve it normally." và đặt cờ cục bộ ẩn nút phụ; `REQUEST_EXECUTION_MODE_NOT_APPLICABLE` và `REQUEST_EXECUTION_MODE_INVALID` → thông điệp chung "Couldn't start the run this way. Approve it normally." và `console.warn` mã (đây là lỗi lập trình, không nên xảy ra). `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` giữ nguyên.

### 2.5 Nhãn chế độ trên tab Execution

`execution-run-mode-model.ts`: `runModeLabelOf(summary.executionMode, flow?.hasPhases)` → `'continuous' | 'step_by_step' | null`: `continuous` → hiện; `step_by_step` chỉ hiện khi luồng có Phase; rỗng hoặc không có Phase → `null`. `ExecutionRunModeLabel` là một chip `Badge`/văn bản nhỏ cạnh nhãn trạng thái trong `ExecutionSummaryStrip` ("Runs all steps on its own" / "Asks before each step"), kèm tooltip "Steps run with the access of the person who approved the plan.". Không thêm màu mới (F9 của CR-REQ-037). Không nút mới.

### 2.6 Công tắc chế độ theo dự án

- `useProjectEngineSettings(projectId)`: `callRequestRpc(ENGINE_SETTINGS, {projectId})` đọc; `save(executionMode)` gọi `ENGINE_SETTINGS_SET` với `{projectId, engine, openspecMinVersion, solutionAgent, executionMode, expectedVersion}` lấy từ lần đọc gần nhất. Trạng thái: `loading | ready | forbidden | unsupported | error`; `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT` (`domain/engine_errors.go:19`) → đọc lại rồi báo "Someone else changed these settings. The latest values were loaded."; gateway cũ không có kênh → `unsupported` (mục ẩn).
- `ProjectRequestExecutionSection`: nhóm hai lựa chọn "Ask before each step" / "Run all steps after one approval" (radio hoặc `Select` của `components/ui`), mô tả hiệu lực "Applies to requests that start running from now on. Requests already running keep their mode."; admin (`currentUser.role === 'admin'`) sửa được; người khác thấy giá trị ở dạng chỉ đọc kèm "Only an administrator can change this."; ghi chú khi bật: "Orca will start each step with the access of the person who approves the plan."; thành công → `toast.success`. Đặt trong tab General sau `ProjectJiraMappingSection`; không động vào các mục khác (`ProjectSettings.tsx` là file 255 dòng, chỉ thêm một dòng nhúng).
- Lưu ý đã nêu ở CR (R5): lưu ghi cả engine hiện hành vào dòng cài đặt dự án; mục hiển thị một dòng nhỏ nói điều này khi `version === 0` (chưa có dòng, đang dùng mặc định triển khai).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Nút phụ chỉ khi `flow.source === 'backend'` và `hasPhases` | Bảng dự phòng đã lệch; hành động có hệ quả không dựa vào nó |
| F2 | Cờ `approval.runAll` bắt buộc, kèm đối chiếu `executionMode` trả về | Gateway cũ nuốt tham số lạ |
| F3 | `PlanApprovalCard` gửi `continuous` tường minh, không thêm nút phụ | Cam kết của thẻ không phụ thuộc cài đặt đổi giữa chừng |
| F4 | Hộp thoại xác nhận bắt buộc, văn bản nêu "using your access" | Hệ quả bảo mật: người duyệt là người thực thi |
| F5 | Một khoá bận `approve:<id>` cho cả hai nút | Không thể duyệt hai lần hoặc hai chế độ |
| F6 | Không thêm bất kỳ điều khiển nào cho Request đang chạy từng bước | Ngoài phạm vi (CR mục 2.10) |
| F7 | Nhãn chế độ dùng thành phần có sẵn, không màu mới | F9 của CR-REQ-037 |
| F8 | Công tắc dự án đọc rồi ghi cả bộ cài đặt | Backend coi `engine` rỗng là `native` |
| F9 | Công tắc chỉ sửa được bởi admin; người khác chỉ đọc | Backend chặn; hiển thị đúng thay vì để lỗi |

## 4. Phụ thuộc và thứ tự

Cần backend TASK-REQ-043-02 và 043-04 (tham số, cờ, kết quả). Thứ tự: 01 → 02 → 03; 04 độc lập sau 01; 05 độc lập sau 01 (chỉ cần hằng RPC); 06 sau tất cả.

## 5. Kiểm thử

- Parser và hook: `parseApproveResult` (thiếu/sai kiểu); `usePlanDecision.approve` thêm `executionMode`, khoá bận, `runAllApplied`; `supportsApproveRunAll`.
- Thuần: `canOfferRunAll` (từng điều kiện), `runModeLabelOf`.
- Component: `PlanApprovalBar` (nút phụ hiện/ẩn đúng, khoá khi bị chặn, hộp thoại: nội dung, số bước, `preDeploy`, Huỷ không gọi, xác nhận gọi đúng args, nhấp đúp một lần), `PlanApprovalCard` (gửi `continuous`), thông điệp lỗi ba mã, `ExecutionSummaryStrip` (ba trạng thái nhãn), `ProjectRequestExecutionSection` (admin/không admin/loading/lỗi/xung đột/unsupported, gửi đủ bộ giá trị).
- Test hiện có (`PlanApprovalCard.test.tsx`, `plan-approval-bars.test.tsx`, `RequestPlanTab.test.tsx`, `execution-components.test.tsx`) vẫn xanh.
- E2E trực tiếp (`ui-live`, `run_ui_flow.py --ai-stub -- approve-run-all`) trên fixture LedgerLite; ca chạy hết tới xong ở loại B (`--real continuous`, cập nhật spec hiện có).

## 6. Rủi ro và điểm chưa kiểm chứng

- `PlanApprovalBar` hiện 132 dòng; nút phụ nằm ở `PlanRunAllButton` riêng để không vượt `max-lines`. Chạy `gitnexus_impact` cho `PlanApprovalBar`, `PlanDecisionButtons`, `usePlanDecision`, `ExecutionSummaryStrip`, `ProjectSettings` trước khi sửa.
- `RequestFlowView` có thể là `null` khi `request.flow` chưa về: nút ẩn (an toàn), không nhấp nháy.
- Công tắc dự án ghi đè cả bộ cài đặt: hai tab của hai admin cùng lúc được khoá lạc quan chặn, nhưng cài đặt đổi bằng kênh WS ở ngoài trong lúc mở mục sẽ gây xung đột (đã xử lý bằng tải lại).
- `ProjectSettings.tsx` còn nhiều chuỗi cứng (không `translate`) và một `TODO` có sẵn; mục mới phải dùng `translate` và không sửa phần cũ.
- Bản dịch es/ja/ko/zh cần người bản ngữ rà.
- Chưa dựng giao diện thử; mô tả dựa trên đọc mã.

## 7. Câu hỏi mở

Như CR-REQ-043 mục 7. Mặc định tạm: không có lựa chọn trong Approval Inbox; không cho chủ dự án (không phải admin) đổi công tắc; Request đã ghim `step_by_step` không có cách chuyển.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-043](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-043-approve-plan-and-run-all-steps.md); [BE-REQ-SOL-043](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md); [CR-REQ-037](../../../../../../docs/crs/v6/request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
