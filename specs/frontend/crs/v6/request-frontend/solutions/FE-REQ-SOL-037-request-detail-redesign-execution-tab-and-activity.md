# FE-REQ-SOL-037: Chi tiết Request: nhãn Requests ở sidebar, thang tiến độ theo luồng, Plan duyệt một lần, tab Execution, tab History đầy đủ

> **🚧 Đã triển khai (7 DONE, 2 PARTIAL trên 9 task, 2026-10-16).** 01 đến 04, 06 đến 08 DONE; 05 PARTIAL (thiếu dòng "Verify passed" của task OpenSpec, cần dữ liệu Verify từ backend); 09 PARTIAL (e2e thật `continuous` PASS với engine native; chưa chạy run OpenSpec sống; ảnh chụp chưa đưa vào PR; bản dịch es/ja/ko/zh cần người bản ngữ rà). Xem từng task để biết chi tiết.

**CR:** [CR-REQ-037](../../../../../../docs/crs/v6/request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
**Area:** frontend (`components/sidebar/`, `components/request/{plan,execution,activity}/`, `components/task/TaskDetail.tsx`, `shared/`, `hooks/`, `i18n/`)
**Hợp đồng backend:** [BE-REQ-SOL-038](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) (`request.flow`, `request.activity`, `request.executionSummary`, `Request.engine`, `Request.executionMode`); [CONTRACT-request-ui-api.md](../../../../../backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md).
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md); [v4/05-runtime-client](../../../../tdd/v4/05-runtime-client.md)
**Phụ thuộc solution:** [FE-REQ-SOL-018](./FE-REQ-SOL-018-request-frontend-foundation.md), [019](./FE-REQ-SOL-019-request-list-detail-classification-ui.md), [021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md), [036](./FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-16: `components/sidebar/{SidebarNav,SidebarRequestNavButton}.tsx`; `components/request/{RequestDetailPane,RequestStageTimeline,RequestHistoryTab,RequestBacklogBanner,request-stage-timeline-model,request-action-rules,request-status-presentation}.ts(x)`; `components/request/plan/{RequestPlanTab,PlanApprovalBar,PhaseApprovalBar,PlanTaskRow,PhaseNode,plan-view-state,plan-approval-model}.ts(x)`; `components/request/execution/ExecutionResultPanel.tsx`; `components/task/TaskDetail.tsx` (dòng ~389) và `components/task/agent-activity/TaskAgentActivityPanel.tsx`; `shared/{request-flow-registry,request-types,request-rpc-methods}.ts`; `hooks/useRequestEvents.ts`; `assets/main.css` (chỉ có `--status-success` trong nhóm trạng thái).

- `SidebarRequestNavButton` là nút vuông 32 px chỉ có icon `Inbox`; các mục khác trong `SidebarNav` là hàng có nhãn (`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-[13px]`), pill đếm kiểu `rounded-full bg-primary px-1.5 py-px text-[10px] font-semibold`.
- `buildStageTimeline` luôn thêm bước `execution` (`stepOrder.push('execution')`); `STATUS_TO_CURRENT_STEP` không có trạng thái nào trỏ tới `phase`; `request_backlog`/`cancelled` hiện bước hiện tại là `current` hoặc `skipped`.
- `REQUEST_FLOW_REGISTRY` là bảng tĩnh và **lệch** `domain.FlowFor` ở backend (CR-037 mục 2.2): `change_request` có `preDeployApproval` (backend không), `security` (`findings`, Phase khi L; backend: `diagnosis`, không Phase, `StartGate=pre_deploy`), `performance` (`findings`, `task_list`; backend: `diagnosis`, `plan`), `docs` (không cổng; backend: cổng `task_list`), `ops_request` (`task_list`; backend: `plan` và `pre_deploy` trong `executing`), `bug` (`task_list`; backend: `plan`).
- Kênh `request.flow` đã có ở gateway nhưng `REQUEST_RPC_METHODS` không có hằng cho nó; hook gọi nó chưa tồn tại.
- `RequestPlanTab` hiển thị `PlanApprovalBar`, `PhaseApprovalBar` (qua `PlanTree.renderPhaseActions`), `PlanGateChips`; `PlanTaskRow` chỉ có `TaskStatusBadge`, nhãn "Blocked", `ReadinessBadge`, chip "Drifted", `ExecutionEngineBadge`.
- `RequestHistoryTab` chỉ nhận `RequestTypeHistoryEntry[]` (`request.typeHistory`).
- `TaskDetail.tsx` dòng ~389 hiện "← Blocked by:" cho mọi phụ thuộc. Tab Result của một task thuộc Request trên #1242 hiện chữ đỏ "The result is not in the expected format" dù `ExecutionResultPanel` có `AgentOutputView` trung tính; **chưa rõ** bản đang chạy cũ hơn mã nguồn hay `TaskDetail` không truyền `requestId`.
- `main.css` không có token "chờ người"; quyết định không thêm (F9).
- `tests/e2e/` hiện chỉ có `workflow-web`; thư mục `request-web` mà SOL-036 nhắc không tồn tại ở nhánh này (kiểm lại trước khi viết e2e).

### Correction relative to CR-REQ-037

1. CR ghi `usePlanTree` cho cây thực thi; hook này tải theo `request` và dựng cây ở client. Tab Execution dùng lại nó cho danh sách Phase/task và chỉ thêm `useExecutionSummary` cho phần backend tính (lần thử, nguyên nhân, `needs_you`).
2. CR nói "tab Execution hiện khi `planKind != none`"; với `single_task` (hotfix) không có tab Plan nên **Execution là tab duy nhất** chứa thẻ duyệt `pre_deploy`; hook tab (`getVisibleDetailTabs`) phải trả `{analysis, plan, execution}` thay cho `{analysis, plan}`.
3. Nhãn bước trong thang tiến độ lấy khoá `auto.components.request.StageTimeline.step.<id>` hiện có; thêm khoá cho `handover`, `taskList`, các trạng thái `waiting`, `stopped` (task 03), không đổi khoá đã có.
4. `ExecutionResultPanel` tái dùng ở hàng task nhưng nó đã đọc `execution.get` qua `useExecutionResult`; không tự gọi thêm ở hàng task, chỉ mở khi người dùng bấm "Log" hoặc "Result".

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `frontend/src/renderer/src/`)

```
components/sidebar/SidebarRequestNavButton.tsx                       hàng có nhãn (sửa)
shared/request-flow-registry.ts                                      sửa bảng dự phòng cho khớp backend
shared/request-types.ts                                              RequestFlowView, ExecutionSummary, RequestActivityEntry, OrcaRequest.engine/executionMode (sửa)
shared/request-wire-parsers.ts                                       parser ba kiểu mới (sửa)
shared/request-rpc-methods.ts                                        FLOW, ACTIVITY, EXECUTION_SUMMARY (sửa)
hooks/{useRequestFlow,useExecutionSummary,useRequestActivity}.ts     (mới)
components/request/request-stage-timeline-model.ts                   dựng thang từ RequestFlowView (viết lại)
components/request/RequestStageTimeline.tsx                          hiển thị chặng, trạng thái chờ/dừng (sửa)
components/request/request-action-rules.ts                           getVisibleDetailTabs theo luồng (sửa)
components/request/RequestDetailPane.tsx                             tab Execution, dải việc tiếp theo (sửa)
components/request/NextActionBar.tsx + next-action-rules.ts          (mới)
components/request/plan/{RequestPlanTab,PlanApprovalBar,PlanOutline}.tsx   khối duyệt một lần, đề cương chỉ đọc (sửa/mới)
components/request/execution/{ExecutionTab,ExecutionSummaryStrip,ExecutionNeedsYou,ExecutionStepList,ExecutionTaskRow,ExecutionFailureCard,HandoverRow}.tsx   (mới)
components/request/execution/execution-task-state.ts                 suy trạng thái hiển thị của task (mới)
components/request/activity/{RequestActivityTab,ActivityList,ActivityRow,activity-grouping}.ts(x)   (mới)
components/request/RequestHistoryTab.tsx                             giữ làm dự phòng khi runtime cũ (sửa)
components/task/TaskDetail.tsx                                       nhãn phụ thuộc, tab Result (sửa)
i18n/locales/{en,es,ja,ko,zh}.json, i18n/request-locale-coverage.test.ts   khoá mới (sửa)
```

### 2.2 Dữ liệu và hook

- `useRequestFlow(type, size)` gọi `request.flow`, cache theo `(type,size)`, `unsupported` thì trả bảng dự phòng đã sửa. Trả `RequestFlowView` (`analysisKind`, `analysisGate`, `planKind`, `hasPhases`, `startGate`, `executionGates`, `completesAfterAnalysis`, `openSpecProfile`).
- `useExecutionSummary(requestId, enabled)` gọi `request.executionSummary`; poll 5 giây khi `status=executing` và tab hiển thị; làm mới theo `request.status_changed`, `task.*` qua `useRequestSubscription`; tắt khi tab ẩn (`document.visibilityState`).
- `useRequestActivity(requestId, filter)` phân trang bằng `pageToken`, làm mới theo `request.status_changed` khi tab mở; không poll khi tab đóng.
- `unsupported` (runtime cũ) không phải lỗi: tab Execution ẩn, History dùng dữ liệu đổi loại.

### 2.3 Thang tiến độ

`buildStageTimeline({flow, engine, status, returnedFromStage, resumeStatus, summary})` trả `Step[]` với `state` ∈ `done|running|waiting|stopped|pending` và `sub?`. Quy tắc ở CR-037 mục 2.3 (bảng ánh xạ trạng thái). `question`, `spike`: kết thúc ở Analysis; `hotfix`: Execution "waiting" khi `awaiting_plan_approval`; Handover chỉ khi `engine=openspec`. `RequestStageTimeline` hiển thị icon + chữ + dòng phụ; văn bản `sr-only` cho từng trạng thái.

### 2.4 Plan, Execution, History

- **Plan:** khi `executionMode=continuous` hiện một thẻ duyệt theo `startGate` và đề cương chỉ đọc (`PlanOutline`); `step_by_step` giữ giao diện CR-021. Thẻ liệt kê phát hiện rủi ro của mọi Phase (ghi rõ Phase) và khoá Approve tới khi chấp nhận hết (dùng `ApprovalRiskGateSection` hiện có).
- **Execution:** `ExecutionTab` ghép `ExecutionSummaryStrip`, `ExecutionNeedsYou`, `ExecutionStepList`/danh sách task, `HandoverRow`/chân native, `ExecutionFailureCard` khi `request_backlog`. `ExecutionTaskRow` hiển thị 6 trạng thái (`queued|waiting|running|done|failed|blocked`) từ `execution-task-state.ts`; task đang chạy mở sẵn `TaskAgentActivityPanel` ở chế độ thu gọn. Chỉ có "Stop run" (huỷ Request, có xác nhận).
- **Dải việc tiếp theo:** `NextActionBar` chọn một mục theo thứ tự ưu tiên (task lỗi, chờ người, đang chạy, đang chờ, hoàn tất) từ `next-action-rules.ts`.
- **History:** `RequestActivityTab` với bộ lọc All, Needs attention, Approvals, Execution, Changes, By me; nhóm theo ngày; liên kết tới tab/đối tượng.

### 2.5 Trạng thái, i18n, token

Mỗi màn có tải, rỗng, lỗi (Retry), `unsupported`, quyền. Chuỗi mới ở `auto.components.request.*` (`StageTimeline.step.handover`, `ExecutionTab.*`, `ActivityTab.*`, `NextAction.*`, `PlanApprovalCard.*`), đủ 5 locale. Chỉ token có sẵn; trạng thái "chờ người" dùng icon + chữ, tông `primary` (F9).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Luồng đọc từ `request.flow`, bảng tĩnh chỉ là dự phòng có test khớp fixture | Bảng tĩnh đã lệch backend |
| Bỏ chặng "Phases" khỏi thang | Phase không có trạng thái Request riêng nên không bao giờ "current" |
| Plan chỉ đọc sau khi duyệt (continuous) | Tách kế hoạch khỏi thực thi |
| `failed` là trạng thái hiển thị suy ra, không thêm vào `TaskStatus` | Backend không có `failed` ở task; tránh lệch kiểu |
| Một dải việc tiếp theo cộng khối "Needs you" đầy đủ | Người dùng quay lại sau nhiều giờ cần việc đầu tiên, và cần danh sách đủ |
| Không thêm token màu mới | AGENTS.md cấm tự thêm giá trị màu |
| Giữ `step_by_step` đầy đủ | Backend giữ mặc định cũ |

## 4. Phụ thuộc và thứ tự

Backend: BE-REQ-SOL-038 task 02 (cài đặt), 04 (`engine`, `executionMode`), 05 (activity), 06 (summary), 07 (kênh). Khi backend chưa có, làm được 037-01 (sidebar), 037-02 (kiểu, parser, hook, bảng dự phòng), 037-03 (thang tiến độ với dữ liệu tĩnh), 037-08 (nhãn). Thứ tự: 01 và 02 song song; 03 sau 02; 04 và 05 sau 02 và 03; 06 sau 05; 07 sau 02; 08 độc lập; 09 cuối.

## 5. Kiểm thử

- **Unit:** `buildStageTimeline` (11 loại × trạng thái chính × engine); `getVisibleDetailTabs`; `next-action-rules` (thứ tự ưu tiên); `execution-task-state` (6 trạng thái); nhóm ngày và lọc nhật ký; test khớp bảng dự phòng với fixture backend.
- **Component:** `SidebarRequestNavButton`, `RequestStageTimeline`, thẻ duyệt Plan (các `startGate`), `ExecutionNeedsYou`, `ExecutionTaskRow`, `ExecutionFailureCard`, `HandoverRow`, `RequestActivityTab`; `RequestDetailPane` với runtime cũ.
- **Hook:** huỷ khi unmount, làm mới theo sự kiện, cooldown, tắt khi tab ẩn.
- **E2E (cần BE-SOL-038):** `change_request` duyệt một lần chạy hết; task lỗi hai lần về backlog rồi "Reopen and rerun"; câu hỏi agent trong "Needs you"; `task` native không có Handover. Chưa chạy.
- Lệnh dự kiến: `pnpm --filter orca-frontend test -- <đường dẫn>`; typecheck và lint theo gốc repo (chưa kiểm chứng script chạy được, xem README frontend mục 3 điểm 10).

## 6. Rủi ro và điểm chưa kiểm chứng

- Phụ thuộc cứng vào backend cho Execution và History; không có backend chỉ có phần sidebar, thang tiến độ, nhãn.
- "Đang chạy" và "chờ người" cùng tông `primary`: khác biệt chỉ ở icon và chữ, cần kiểm trực quan và chế độ tối.
- Poll 5 giây của `useExecutionSummary` với nhiều Request mở cùng lúc chưa đo; chỉ poll Request đang chọn.
- Quy tắc ưu tiên của dải việc tiếp theo chưa qua người dùng thật.
- Bản dịch es, ja, ko, zh cần người bản ngữ rà.
- Chưa chạy typecheck, lint, e2e sau thiết kế.

## 7. Câu hỏi mở

- Archive và bàn giao OpenSpec ở `executing` hay `completed` (quyết định trạng thái "đang xử lý" của chặng Handover; tạm dùng `completed`).
- Vì sao tab Result của task trong #1242 hiện cảnh báo đỏ dù có `AgentOutputView` (xem task 08, bước điều tra).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md`; `CR-REQ-019`, `021`, `036`
- `frontend/src/renderer/src/components/{sidebar,request,task}/` các file nêu ở mục 1
- `backend-go/services/request-service/internal/domain/request_flow_registry.go` (`FlowFor`), `backend-go/proto/orca/request/v1/request.proto`
- [FE-REQ-SOL-021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md), [FE-REQ-SOL-036](./FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md); `guides/STYLEGUIDE.md`
