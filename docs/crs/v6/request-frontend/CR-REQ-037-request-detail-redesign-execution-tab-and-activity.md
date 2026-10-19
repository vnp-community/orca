# CR-REQ-037 — Thiết kế lại chi tiết Request: nhãn Requests ở sidebar, thang tiến độ theo luồng, Plan duyệt một lần, tab Execution và tab History đầy đủ

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-037 |
| **Tên** | Nút Requests có nhãn; thang tiến độ sinh từ luồng của backend; tab Plan chỉ đọc kèm một khối duyệt; tab Execution mới theo dõi agent và việc cần người; tab History thành nhật ký hoạt động; sửa các nhãn gây hiểu nhầm |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-16 |
| **Trạng thái** | 🚧 Đã triển khai 2026-10-16 (7 DONE, 2 PARTIAL trên 9 task; quyết định F8 đến F12 đã chốt; e2e thật qua giao diện PASS với engine native; còn thiếu "Verify passed" và run OpenSpec sống; còn 2 câu hỏi mở ở mục 7) |
| **Phụ thuộc** | CR-REQ-018 đến 021, 036; backend [CR-REQ-038](../plan-phase-task/CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md) (`request.activity`, `request.executionSummary`, `Request.engine`, `Request.execution_mode`, chế độ `continuous`) |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/sidebar/SidebarRequestNavButton.tsx`; `components/request/{RequestDetailPane,RequestStageTimeline,RequestHistoryTab}.tsx`, `request-stage-timeline-model.ts`, `request-action-rules.ts`; `components/request/plan/*`; `components/request/execution/*` (mở rộng) và `components/request/activity/*` (mới); `components/task/TaskDetail.tsx`; `shared/{request-flow-registry,request-types,request-rpc-methods}.ts`; `hooks/`; `i18n/locales/*.json` |

> Nguồn: khảo sát request #1242 (`change_request`, 7 Phase, 28 task) trên UI chạy thật ngày 2026-10-16 và đọc code. Mockup tương tác đã trình bày trong phiên làm việc; mô tả đầy đủ ở mục 2 là bản chuẩn.

---

## 1. Bối cảnh và vấn đề

Quan sát trên #1242 (đang ở `request_backlog`, trả về từ bước Thực thi vì task #TG-1037 lỗi 2 lần):

1. **Nút Requests ở sidebar chỉ có icon** (`Inbox`, 32 px), trong khi Tasks, Automations, Orca Mobile là hàng có nhãn. Người dùng không biết nó là gì.
2. **Thang tiến độ sai.** `request-stage-timeline-model.ts` có `STATUS_TO_CURRENT_STEP` không trỏ trạng thái nào tới bước `phase`, nên khi Request vào `executing` bước Phases tự thành "done" còn Execution là "current" suốt quá trình; Request đã về backlog vẫn hiện Phases "done" và Execution "current". Hàm cũng **luôn thêm bước Execution** (`stepOrder.push('execution')`), kể cả `question` và `spike` vốn không có plan.
3. **Tab Plan lẫn việc thực thi.** Tab Plan vừa là đề cương vừa là nơi duyệt từng Phase ("Approve phase", "Start phase", chip cổng), xem trạng thái từng task (badge `TaskStatusBadge`), mở `TaskDetail` để "Execute with Agent" và xem hoạt động agent. Người dùng phải duyệt và bấm từng bước (7 lần với #1242) thay vì duyệt một lần.
4. **Task lỗi không rõ làm gì tiếp.** #TG-1037 lỗi 2 lần nhưng hàng task hiện "Open" (`PlanTaskRow` chỉ có `TaskStatusBadge` và nhãn "Blocked"; `TaskStatus` không có `failed`). Lý do lỗi chỉ nằm trong một câu dài ở `RequestBacklogBanner` ("lỗi sau 2 lần: execution failed: retryable TIMED_OUT"), trong khi log cuối của agent là `unrecognized_model`. Tab Result của task hiện chữ đỏ "The result is not in the expected format" dù task của Request vốn không trả khối kết quả có cấu trúc (mã nguồn có `AgentOutputView` trung tính cho trường hợp này; cần xác nhận bản đang chạy có dùng không, xem mục 7). `TaskDetail.tsx:389` hiện "← Blocked by:" cho 3 task đã xong.
5. **History không có thông tin.** `RequestHistoryTab` chỉ hiển thị `request.typeHistory`: một dòng "Unknown → Change request" trên #1242.
6. **Bảng luồng tĩnh ở frontend đã lệch backend** (`REQUEST_FLOW_REGISTRY`, mục 2.2), nên tab, bước và cổng hiển thị sai cho nhiều loại.
7. **Không có thông tin `engine` trên Request**, nên không biết khi nào hiện Verify, archive và bàn giao (OpenSpec) so với native.

## 2. Giải pháp đề xuất

### 2.1 Sidebar: nút Requests thành hàng có nhãn

Sửa `SidebarRequestNavButton.tsx` dùng cùng kiểu hàng như `SidebarNav.tsx` (Automations, Orca Mobile): `flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] font-medium tracking-tight`, trạng thái active `bg-worktree-sidebar-accent text-worktree-sidebar-accent-foreground`, icon `Inbox` `size-4` đổi `strokeWidth` khi active, nhãn "Requests" lấy từ khoá có sẵn `auto.components.request.SidebarRequestNavButton.label`.

- Số chờ duyệt thành pill bên phải hàng (cùng kiểu pill "Agents" đang dùng: `rounded-full bg-primary px-1.5 py-px text-[10px] font-semibold text-primary-foreground`, đã chốt dùng màu `primary` thay cho `destructive` hiện tại), không còn đè lên góc icon. Giữ giới hạn `99+`, `aria-label` có số, `data-testid="sidebar-requests"`, `aria-pressed` đổi sang `aria-current="page"` như các hàng khác.
- Vẫn ẩn khi `requestFlowSupport !== 'supported'`.
- Cập nhật `SidebarRequestNavButton.test.tsx` và `SidebarNav.test.tsx`.

### 2.2 Luồng lấy từ backend, không từ bảng tĩnh

Thêm hằng `REQUEST_RPC_METHODS.FLOW = 'request.flow'` (kênh đã có ở gateway, frontend chưa dùng) và hook `useRequestFlow(type, size)` trả `RequestFlowView` (`analysisKind`, `analysisGate`, `planKind`, `hasPhases`, `startGate`, `executionGates`, `completesAfterAnalysis`, `openSpecProfile`). `Request` thêm `engine` và `executionMode` (CR-REQ-038). Mọi quyết định hiển thị đọc từ đây; **không** rẽ nhánh theo `type === '...'`.

`REQUEST_FLOW_REGISTRY` chỉ còn là **dự phòng** khi runtime chưa có `request.flow`, và phải được sửa cho khớp backend (`domain.FlowFor`). Các chỗ đang lệch (đối chiếu 2026-10-16):

| Loại | Bảng tĩnh frontend | Backend (`FlowFor`) |
|---|---|---|
| `change_request` | có `preDeployApproval` | không có `pre_deploy`; cổng: `plan` (bắt đầu), `phase` (trong `executing`, tuỳ `execution_mode`) |
| `security` | `findings`, Phase khi L, `analysisApproval` + plan + pre_deploy | `diagnosis` (cổng `solution`), Plan không Phase; `StartGate = pre_deploy` (thay cho duyệt Plan) |
| `performance` | `findings`, `task_list` | `diagnosis` (cổng `solution`), Plan (`plan`), không Phase |
| `docs` | không có cổng | có cổng `task_list` |
| `ops_request` | `task_list`, chỉ duyệt plan | Plan (`plan`), cổng `pre_deploy` trong `executing` |
| `bug` | `task_list` | Plan (`plan`), Phase khi L |
| `hotfix` | cổng pre-deploy như một cổng thường | `StartGate = pre_deploy` chiếm `awaiting_plan_approval` (trước khi chạy) |

Thêm test hợp đồng: một fixture JSON (sinh từ `FlowFor` bằng script ở backend) và test frontend so khớp bảng dự phòng với fixture để không lệch lại.

**Engine không suy ra từ loại.** Theo cấu hình hiện tại `change_request` và `refactor` chạy OpenSpec; các loại còn lại chạy native. Nhưng backend có profile `Light` cho `bug`, `security`, `performance` và cài đặt theo dự án, nên frontend đọc `request.engine`, không đoán.

### 2.3 Thang tiến độ (thay `buildStageTimeline`)

Các chặng sinh từ `RequestFlowView` và `engine`:

```
Classification → [Analysis] → [Plan | Task list] → [Execution] → [Handover]
```

| Chặng | Có khi |
|---|---|
| Analysis | `analysisKind != none` |
| Plan hoặc Task list | `planKind` là `plan` hoặc `task_list` (nhãn theo `planKind`) |
| Execution | `planKind != none` (kể cả `single_task`) |
| Handover | `engine == openspec` |

- `question` và `spike`: Classification → Analysis, kết thúc ở Analysis (sửa lỗi luôn thêm Execution).
- `hotfix` (`single_task`): không có chặng Plan; cổng `pre_deploy` chiếm `awaiting_plan_approval` nên chặng **Execution** hiện "chờ duyệt".
- Bước "Phases" **bỏ khỏi thang**; Phase nằm trong đề cương Plan và tiến độ Execution.

Ánh xạ trạng thái → chặng và trạng thái chặng:

| Trạng thái Request | Chặng hiện tại | Trạng thái chặng |
|---|---|---|
| `submitted`, `classifying`, `awaiting_type_confirmation` | Classification | chạy (`classifying`), chờ người (`awaiting_type_confirmation`) |
| `analyzing` | Analysis | chạy |
| `awaiting_analysis_approval` | Analysis | chờ người |
| `planning` | Plan | chạy |
| `awaiting_plan_approval` | Plan (hoặc Execution khi `single_task`) | chờ người |
| `executing` | Execution | chạy; chờ người nếu `needs_you` không rỗng |
| `completed` | tất cả xong; Handover "đang xử lý" khi `handover.state=archiving` (xem 7) | xong |
| `awaiting_information` | chặng của `resumeStatus` (đã có) | chờ người |
| `request_backlog`, `cancelled` | chặng của `returnedFromStage` (`phase` và `task` gộp vào Execution) | **dừng** (icon và chữ "stopped", không còn "current") |

Hiển thị mỗi chặng: icon + chữ (không chỉ màu); dòng phụ ngắn ("step 3/7 · 9/28 task", "chờ duyệt", "step 7/7 · 27/28 · lỗi"). Văn bản cho trình đọc màn hình (`sr-only`): done, running, waiting for you, stopped, pending. Test `request-stage-timeline-model.test.ts` viết lại theo bảng trên cho cả 11 loại.

### 2.4 Các tab theo luồng

`getVisibleDetailTabs` đọc `RequestFlowView`: **Overview**, **Analysis** (nếu có phân tích), **Plan** hoặc **Task list** (nếu `planKind` là `plan`/`task_list`), **Execution** (nếu `planKind != none`), **History**, **Related**. `single_task` (hotfix) không có tab Plan. `question` và `spike` không có Plan lẫn Execution; cuối tab Analysis có "Create follow-up request" theo `CHILD_REQUEST_RULES` hiện có.

### 2.5 Tab Plan: đề cương chỉ đọc và một lần duyệt

Áp khi `executionMode = continuous` (xem 2.9 cho `step_by_step`).

- **Khối duyệt** (chỉ khi có Approval `pending` của `startGate`), theo `startGate`:
  - `plan` / `task_list`: "Approve once and the agent runs all N steps/tasks", kèm danh sách "Dừng và hỏi anh chỉ khi: cần thông tin; task lỗi 2 lần (Request về backlog); có cổng pre-deploy (nếu luồng có)". Nút chính "Approve and start execution", phụ "Request changes" và "Regenerate". Giữ `ApprovalRiskGateSection` (chấp nhận rủi ro, gồm mọi Phase) và quy tắc "chưa có Decision thì khoá Duyệt" của CR-REQ-036.
  - `pre_deploy` làm cổng bắt đầu: với `security` (có tab Plan) thẻ nằm ở tab Plan, nội dung "Review before the fix runs". Với `hotfix` (`single_task`, không có tab Plan) thẻ nằm trong "Needs you" của tab Execution (đã chốt, F8).
  - Khi `executionMode = continuous`, thẻ duyệt `plan` liệt kê phát hiện rủi ro của **mọi Phase** (không chỉ Plan) và cần chấp nhận trước khi nút Approve bật (CR-REQ-038, B8); đây là điểm duy nhất người duyệt thấy rủi ro theo Phase, nên mỗi phát hiện ghi rõ Phase nào.
- **Đề cương**: nếu `hasPhases` thì danh sách bước (tên, số task, ước tính giờ, mở rộng xem tên task); không thì danh sách task phẳng theo thứ tự. Không còn badge trạng thái task, nút Run, "Approve phase", "Start phase", `PlanGateChips` ở chế độ này.
- **Sau khi duyệt**: Plan chỉ đọc, từng bước có dấu tích, và dải "Plan approved by `<tên>`, `<giờ>`. Follow in Execution" (nút chuyển tab). Giữ `PlanDriftSection` và `ReadinessBadge` (đã có CR-REQ-036) vì là thông tin của kế hoạch.
- `usePlanDecision` giữ nguyên cho Approve, Reject, Regenerate; bỏ `startPhase` khỏi đường chính (còn dùng ở `step_by_step`).

### 2.6 Tab Execution (mới)

Hiện khi `planKind != none`. Trước `executing`: "Not started. Waiting for plan approval" kèm liên kết sang Plan. Dữ liệu: `GetExecutionSummary` (poll 5 s khi `executing`, và làm mới theo `request.status_changed`, `task.*`) cộng cây Plan hiện có (`usePlanTree`).

Bố cục từ trên xuống:

1. **Dải tóm tắt**: thanh tiến độ, "27/28 tasks · step 7/7", thời gian đã chạy, trạng thái (Running / Needs you / Stopped / Done). Chỉ có nút "Stop run" (dùng huỷ Request hiện có, có hộp xác nhận); không có "Pause after this task" trong v6 (CR-REQ-038, B9).
2. **Needs you** (chỉ khi `needs_you[]` không rỗng): mỗi mục một thẻ có đường tới hành động: câu hỏi của agent → "Answer" (mở `ClarificationPanel`); cổng `pre_deploy` → "Approve / Reject" (dùng `PlanDecisionButtons`, `ApprovalRiskGateSection`); drift → "Review" (`PlanDriftReviewSheet`); task lỗi làm Request về backlog → "Reopen and rerun" / "View last log" / "Cancel request". Mục có hạn (`due_at`) hiện "Còn N ngày".
3. **Bước và task**: bước đã xong thu gọn một dòng (số task, thời lượng); bước hiện tại mở sẵn; bước sau chỉ đếm. Nếu không có Phase thì danh sách task phẳng.
4. **Hàng task** (`ExecutionTaskRow`), trạng thái hiển thị suy ra từ `task.status` và `summary`:

| Hiển thị | Điều kiện | Thông tin kèm theo | Hành động |
|---|---|---|---|
| Queued | chưa chạy, phụ thuộc đã xong hoặc không có | – | – |
| Waiting | phụ thuộc chưa xong | "waits for #TG-N" | mở task chặn |
| Running | có trong `running[]` | thời gian đã chạy, lần thử n/max, **nhật ký agent trực tiếp** (mở sẵn, ~5 dòng cuối) | "Open full log" |
| Done | `done` | thời lượng; với OpenSpec: "Verify passed (exit 0)" | – |
| Failed | có trong `failed[]` | "Failed n of max attempts · `<cause>` · `<giờ>`", dòng lỗi cuối của agent | Rerun, Log, Attempts |
| Blocked | `blocked` | lý do/đối tượng chặn | theo lý do |

   Dùng `Failed` thay vì "Open" cho task lỗi; trạng thái này suy ra ở frontend từ `failed[]`, không thêm `failed` vào `TaskStatus`.
5. **Chân tab theo engine**: OpenSpec → hàng **Handover** ("after the last task: Orca archives the change on the proposal branch, then lists the task worktree, proposal branch and knowledge proposal"; sau khi xong hiển thị liên kết tới từng đầu ra; `handover.state` và số lần archive). Native → "Done when the last task is done", không có Verify riêng, không có Handover.
6. **Trạng thái dừng** (`request_backlog`): thẻ lỗi ở đầu tab (thay `RequestBacklogBanner` dài một câu): "Stopped: `<task>` failed `n/max` times, request returned to backlog", nguyên nhân có cấu trúc, "Everything before it is done and kept", nút "Reopen and rerun `<task>`", "View last log", "Cancel request".

Tái dùng: `TaskAgentActivityPanel` (`components/task/agent-activity/`, phần "Agent activity" của `TaskDetail`), `ExecutionResultPanel`, `PlanDecisionButtons`. Thêm component: `ExecutionSummaryStrip`, `ExecutionNeedsYou`, `ExecutionStepList`, `ExecutionTaskRow`, `ExecutionFailureCard`, `HandoverRow`; hook `useExecutionSummary`.

### 2.7 Thanh "việc cần làm tiếp theo" ở đầu chi tiết Request

Một dải duy nhất dưới thang tiến độ, hiện ở mọi tab, chọn **một** mục ưu tiên cao nhất từ `needs_you[]` và trạng thái: (1) task lỗi, (2) chờ người duyệt hoặc trả lời, (3) đang chạy, (4) đang chờ (phụ thuộc, Phase chưa tới lượt), (5) hoàn tất. Mỗi dải nêu mã task hoặc đối tượng và có nút tới đúng tab hoặc thực hiện ngay. Khi không có việc nào: "Nothing needs you right now". Tab Execution có chấm trạng thái (và `aria-label`) khi có việc cần người.

### 2.8 Tab History thành nhật ký hoạt động

Đổi nội dung `RequestHistoryTab` (giữ tên tab "History") sang dữ liệu `request.activity`:

- Nhóm theo ngày, mới nhất trước, "Load more" bằng con trỏ.
- Bộ lọc: All, Needs attention (`severity != info`), Approvals, Execution, Changes, By me.
- Mỗi dòng: icon theo `kind`, tiêu đề ngắn, thời điểm tương đối (`formatRequestRelativeTime`), người làm (AI, người dùng, hệ thống, agent), chi tiết một dòng, liên kết tới đối tượng (task, approval, log, tab Analysis/Plan/Execution). Dòng lỗi (`severity=error`) nổi bật bằng icon và chữ, không chỉ màu.
- Nội dung chỉ là văn bản; không hiển thị `body` của Request hay log thô trong dòng (mở bằng `Sheet` có kiểm quyền).
- Trạng thái rỗng "No activity yet", tải, lỗi (nút Retry), "unsupported" khi runtime chưa có kênh (giữ danh sách đổi loại cũ làm dự phòng).
- Làm mới theo `request.status_changed` và sự kiện thực thi; không poll riêng khi tab không mở.

### 2.9 Chế độ `step_by_step` và tương thích ngược

Khi `executionMode = step_by_step` (mặc định của backend), giữ hành vi CR-REQ-021: tab Plan còn "Approve phase", "Start phase", chip cổng. Tab Execution vẫn có và chỉ **theo dõi**; các cổng Phase hiện trong "Needs you" ("Phase 4 is waiting for your approval") với nút tới hành động. Runtime cũ không có `request.executionSummary`/`request.activity` thì tab Execution ẩn, History giữ dữ liệu đổi loại; không lỗi đỏ.

### 2.10 Sửa nhãn gây hiểu nhầm

- `TaskDetail.tsx` mục phụ thuộc: "← Blocked by:" → "Depends on" kèm đếm trạng thái ("3 of 3 done") và biểu tượng xong cho từng mục; chỉ gọi "Blocked by" khi còn mục chưa xong.
- Tab Result của task thuộc Request: không hiện cảnh báo đỏ "not in the expected format" khi task không thuộc hợp đồng có cấu trúc; hiện `AgentOutputView` trung tính (đã có trong mã nguồn). Xác nhận vì sao bản đang chạy hiện cảnh báo đỏ (mục 7).
- `RequestBacklogBanner`: giữ cho các lý do khác; với lỗi task thì hiển thị `ExecutionFailureCard` (2.6).

### 2.11 i18n, giao diện, SSH

- Khoá `auto.components.request.*` cho mọi chuỗi mới, đủ `en, es, ja, ko, zh`, kèm test phủ khoá theo mẫu `request-locale-coverage.test.ts`.
- Chỉ dùng token và primitive có sẵn (`guides/STYLEGUIDE.md`, `main.css`). **Chưa có token "chờ người"**: `main.css` chỉ có `--status-success` cùng các token `destructive`, `primary`, `muted-foreground` và `risk-*`; không tự thêm màu mới (AGENTS.md). **Đã chốt (F9): không thêm token mới.** "Chờ người" và "chạy" phân biệt bằng icon và chữ; "chờ người" dùng icon bàn tay hoặc dấu hỏi kèm chữ "Waiting for you", "chạy" dùng icon quay kèm chữ "Running", cùng tông `primary`; "lỗi" dùng `destructive`, "xong" dùng `--status-success`, "dừng" và "đang chờ" dùng `muted-foreground`.
- Mọi lời gọi qua `callRuntimeRpc`/`callRequestRpc`, không giả định thực thi cục bộ (SSH, WSL); chịu độ trễ 50 đến 200 ms. Phím tắt đa nền tảng nếu có (không thêm phím tắt cho Duyệt).
- Không thêm `max-lines` disable; tách file theo domain (không đặt tên `helpers`, `utils`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Luồng, tab, chặng đọc từ `request.flow` và `Request.engine` | Bảng tĩnh đã lệch backend; quy ước dự án là không rẽ nhánh trực tiếp theo loại |
| F2 | Bỏ chặng "Phases" khỏi thang; Phase thuộc Plan (kế hoạch) và Execution (tiến độ) | Phase không có trạng thái Request riêng nên không bao giờ "current" |
| F3 | Plan chỉ đọc sau duyệt; mọi việc chạy nằm ở Execution | Tách "kế hoạch" khỏi "thực thi"; người dùng một nơi để theo dõi |
| F4 | `Failed` là trạng thái hiển thị suy ra, không thêm vào `TaskStatus` | Backend không có `failed` ở task; tránh lệch kiểu thêm (README v6 mục 8 điểm 11) |
| F5 | Một dải "việc cần làm tiếp theo" duy nhất | Người dùng quay lại sau nhiều giờ cần biết việc đầu tiên |
| F6 | Giữ `step_by_step` đầy đủ | Backend giữ mặc định cũ (CR-REQ-038 B1) |
| F7 | Không có giao diện chỉnh `execution_mode` trong CR này, kể cả trên thẻ duyệt Plan | Cài đặt theo dự án (CR-REQ-038, B10) |
| F8 | Thẻ duyệt `pre_deploy` bắt đầu: `security` ở tab Plan, `hotfix` trong "Needs you" của tab Execution | Hotfix không có tab Plan (chốt 2026-10-16) |
| F9 | Không thêm token màu mới cho "chờ người"; phân biệt bằng icon và chữ | AGENTS.md cấm tự thêm giá trị màu; `main.css` chỉ có `--status-success` (chốt 2026-10-16) |
| F10 | Pill số chờ duyệt ở sidebar dùng màu `primary` như "Agents" | Nhất quán các hàng điều hướng (chốt 2026-10-16) |
| F11 | Tab History chỉ có sự kiện cấp Request và task; log agent mở riêng | CR-REQ-038, B12 |
| F12 | Không có nút Pause; chỉ "Stop run" | CR-REQ-038, B9 |

## 4. Tiêu chí chấp nhận

- [ ] Sidebar hiện hàng "Requests" có icon, nhãn, trạng thái active và pill số chờ duyệt (`99+` khi vượt); ẩn khi runtime không hỗ trợ.
- [ ] Thang tiến độ đúng cho cả 11 loại theo bảng 2.3 (kể cả `question`/`spike` kết thúc ở Analysis, `hotfix` không có Plan, chỉ OpenSpec có Handover); Request `request_backlog` hiển thị chặng bị dừng, không hiện "Phases done, Execution current".
- [ ] Tab hiển thị theo `RequestFlowView`; `single_task` không có tab Plan; `question`, `spike` không có Plan/Execution.
- [ ] `hotfix` chờ `pre_deploy`: thẻ duyệt hiện trong "Needs you" của tab Execution; `security`: thẻ hiện ở tab Plan.
- [ ] `continuous`: thẻ duyệt Plan liệt kê phát hiện rủi ro của mọi Phase, ghi rõ Phase; chưa chấp nhận hết thì Approve khoá.
- [ ] Tab Execution không có nút "Pause"; có "Stop run" kèm hộp xác nhận.
- [ ] `continuous`: tab Plan có đúng một khối duyệt, không có nút Approve/Start phase; duyệt xong Plan chuyển chỉ đọc kèm liên kết sang Execution.
- [ ] Tab Execution hiện tóm tắt, "Needs you", bước/task, nhật ký agent của task đang chạy, và với task lỗi: số lần thử, nguyên nhân, hành động Rerun/Log.
- [ ] #1242 (dữ liệu thật): task #TG-1037 hiện `Failed 2 of 2`, không còn "Open"; thẻ dừng nêu nguyên nhân có cấu trúc; có nút "Reopen and rerun".
- [ ] Task OpenSpec có Verify và hàng Handover; task native không có.
- [ ] Dải "việc cần làm tiếp theo" chọn đúng mục theo thứ tự ưu tiên 2.7 và dẫn tới đúng tab.
- [ ] History hiển thị ít nhất các `kind` ở CR-REQ-038 bảng 2.2, lọc theo nhóm, phân trang, không hiển thị `body`; runtime cũ giữ danh sách đổi loại.
- [ ] `step_by_step`: hành vi CR-REQ-021 giữ nguyên; Phase chờ duyệt xuất hiện trong "Needs you".
- [ ] Phần phụ thuộc trong `TaskDetail` không còn "Blocked by" khi mọi phụ thuộc đã xong; tab Result không hiện cảnh báo đỏ cho kết quả không có cấu trúc của task thuộc Request.
- [ ] Bảng dự phòng `REQUEST_FLOW_REGISTRY` khớp fixture từ backend (test).
- [ ] Không có màu hex mới; mọi chuỗi có 5 locale; trạng thái luôn có icon và chữ.

## 5. Kiểm thử

- Unit: `buildStageTimeline` (11 loại × trạng thái chính × `engine`); chọn mục ưu tiên của dải 2.7; suy ra trạng thái hàng task 2.6; nhóm và lọc nhật ký; so khớp bảng dự phòng với fixture backend.
- Component: `SidebarRequestNavButton`, `RequestStageTimeline`, khối duyệt Plan (các `startGate`), `ExecutionNeedsYou`, `ExecutionTaskRow` (6 trạng thái), `ExecutionFailureCard`, `HandoverRow`, `ActivityList`; `RequestDetailPane` với runtime cũ (không kênh mới).
- Hook: `useRequestFlow`, `useExecutionSummary`, `useRequestActivity` (huỷ khi unmount, làm mới theo sự kiện, cooldown).
- E2E (cần CR-REQ-038): Request `change_request` duyệt một lần chạy hết nhiều Phase; task lỗi 2 lần về backlog rồi "Reopen and rerun"; câu hỏi của agent hiện trong "Needs you"; `task` native không có Handover. Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Phụ thuộc cứng vào CR-REQ-038; khi backend chưa có, chỉ làm được 2.1, 2.3 (một phần), 2.10 và sửa bảng dự phòng.
- Gộp nhiều thông tin vào một dải "việc cần làm tiếp theo" có thể che mục thứ hai; "Needs you" ở tab Execution liệt kê đủ.
- `request.activity` có thể chậm với Request dài; UI cần phân trang và trạng thái tải rõ.
- Quy tắc ưu tiên (task lỗi trước việc của người) chưa được người dùng thật kiểm chứng.
- "Đang chạy" và "chờ người" cùng tông `primary` (không thêm token, F9); khác biệt chỉ ở icon và chữ nên cần kiểm trực quan, đặc biệt ở chế độ tối và trình đọc màn hình.
- Ước tính giờ trong đề cương lấy từ `estimatedHours` của task; có thể thiếu hoặc là ước tính của AI (đánh nhãn "estimated").
- Chưa chạy ứng dụng sau thiết kế; mọi thứ ở trên là kế hoạch.

## 7. Quyết định đã chốt và câu hỏi còn mở

**Đã chốt (người yêu cầu, 2026-10-16)** theo đề xuất trong bản nháp: F8 (vị trí thẻ duyệt `pre_deploy`), F9 (không token màu mới), F10 (pill `primary`), F11 (History chỉ cấp Request và task), F12 (không Pause), cùng B8 đến B12 ở CR-REQ-038. Cách hiểu F12 là suy luận từ câu "đồng ý với đề xuất"; cần xác nhận khi duyệt.

**Còn mở:**

1. Bước archive và bàn giao OpenSpec diễn ra ở `executing` hay `completed`; thang tiến độ đặt Handover "đang xử lý" theo trạng thái nào (CR-REQ-038 mục 7, câu 2)? Bảng ánh xạ ở 2.3 tạm dùng `completed` theo sơ đồ luồng của người yêu cầu.
2. Vì sao #1242 hiện cảnh báo đỏ ở tab Result dù mã nguồn có `AgentOutputView`: bản đang chạy cũ hơn mã nguồn, hay `TaskDetail` không truyền `requestId`? Cần kiểm trước khi sửa 2.10.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-018 | Thêm `REQUEST_RPC_METHODS.FLOW`, `ACTIVITY`, `EXECUTION_SUMMARY`; kiểu `RequestFlowView`, `ExecutionSummary`, `RequestActivityEntry`; `OrcaRequest` thêm `engine`, `executionMode`; sửa bảng dự phòng `REQUEST_FLOW_REGISTRY` |
| CR-REQ-019 | `RequestStageTimeline` và `buildStageTimeline` theo 2.3; `RequestDetailPane` thêm tab Execution, dải 2.7; `getVisibleDetailTabs` theo luồng; `RequestHistoryTab` theo 2.8 |
| CR-REQ-021 | Tab Plan theo 2.5 khi `continuous`; `PhaseApprovalBar`, `PlanGateChips` chỉ còn ở `step_by_step`; nút Run không còn ở Plan |
| CR-REQ-023 | Hàng Execute backlog hiển thị lần thử và nguyên nhân từ `ExecutionSummary` (đề xuất) |
| CR-REQ-036 | `ExecutionResultPanel` tái dùng trong `ExecutionTaskRow`; `PlanDriftBanner` vẫn ở Plan, `drift` thêm vào "Needs you" |
| Sidebar hiện có | `SidebarRequestNavButton` đổi sang hàng có nhãn (2.1) |

## 9. Tham chiếu

- [CR-REQ-038](../plan-phase-task/CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md); [CR-REQ-019](./CR-REQ-019-request-list-detail-classification-ui.md), [CR-REQ-021](./CR-REQ-021-plan-phase-tree-and-approval-ui.md), [CR-REQ-036](./CR-REQ-036-clarification-decision-readiness-impact-ui.md)
- `frontend/src/renderer/src/components/sidebar/{SidebarNav,SidebarRequestNavButton}.tsx`
- `frontend/src/renderer/src/components/request/{RequestDetailPane,RequestStageTimeline,RequestHistoryTab,RequestBacklogBanner,request-stage-timeline-model,request-action-rules}.ts(x)`, `plan/{RequestPlanTab,PlanApprovalBar,PhaseApprovalBar,PlanTaskRow,plan-view-state}.ts(x)`, `execution/ExecutionResultPanel.tsx`
- `frontend/src/renderer/src/components/task/TaskDetail.tsx` (phụ thuộc, tab Result)
- `frontend/src/shared/{request-flow-registry,request-types,request-rpc-methods}.ts`
- `backend-go/services/request-service/internal/domain/request_flow_registry.go` (`FlowFor`), `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_flow.go`
- `guides/STYLEGUIDE.md`, `frontend/src/renderer/src/assets/main.css`
- Mới: `components/request/execution/{ExecutionSummaryStrip,ExecutionNeedsYou,ExecutionStepList,ExecutionTaskRow,ExecutionFailureCard,HandoverRow}.tsx`, `components/request/activity/*`, `hooks/{useRequestFlow,useExecutionSummary,useRequestActivity}.ts`
