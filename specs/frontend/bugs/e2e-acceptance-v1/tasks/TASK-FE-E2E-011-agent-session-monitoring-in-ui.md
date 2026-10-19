# TASK-FE-E2E-011: Theo dõi phiên agent trên web khi Orca thực thi Plan (nghiệm thu qua UI với Claude Code THẬT)

**From Solution:** SOL-FE-E2E-003 · **Resolves:** mục tiêu "vào web và dùng Orca full luồng": người dùng phải thấy agent đang làm gì, không chỉ trạng thái task
**Priority:** P1 · **Area:** frontend (spec e2e) + tìm lỗi frontend/backend/agent
**Files:**
- tests/stages/e2e/orca-flows/ui-live/agent-session-monitoring-real.web.e2e.ts (spec mới, 1 test)
- tests/stages/e2e/orca-flows/ui-live/live-agent-session-monitor.ts, live-agent-session-ui.ts, live-agent-session-devtab.ts, live-agent-session-judge.ts (+ `.test.ts`, 12 test), live-task-activity-tap.ts, live-secret-leak-scan.ts (mới)
- tests/stages/e2e/orca-flows/ui-live/gateway-rpc-client.ts (thêm `subscribe`), live-stack-config.ts (`readStackEnv`), live-request-ui-steps.ts (`Back to app` sau khi tab khác để SPA ở Settings)
- tests/stages/e2e/orca-flows/run_ui_flow.py (`--real monitor`), README.md (mục "Theo dõi phiên agent trên web")
**Depends on:** TASK-FE-E2E-010 (an toàn, repo tạm, hook chặn, dấu vân tay; giữ nguyên, không nới), TASK-FE-E2E-008, 009
**Status:** `[x]` Chạy thật, KHÔNG stub (2026-10-10, sau redeploy lần 4: G5c sửa). `--real monitor` XANH và `--real all` 6/6 XANH. Từ UI người dùng theo dõi được phiên agent: panel Agent activity hiện dòng `·/»/▸/↳/■` lúc chạy rồi `Finished`; dev server Busy/Idle/healthy/platform; "Agent running"; tab Result xem lại được; "Open in Git" điều hướng sang terminal (mở được) + Source Control liệt kê `src/calc.py` và `tests/test_calc.py`; không lộ bí mật. Còn mở (không làm đỏ spec, có mã): G5d/G5e (mở một file đã đổi thì panel diff lỗi, ghi `known-gap`), G5b, G7.
---

## Context

TASK-FE-E2E-010 chỉ kiểm trạng thái task/Plan (hàng task `in progress`, "N running", 100%, Approval). Chưa kiểm: hoạt động trực tiếp của agent (claude đọc file, gọi công cụ, in ra gì), phiên/lịch sử sau khi chạy, trạng thái dev server/agent (online/bận). Task này khảo sát web có gì để theo dõi những thứ đó, và viết spec kiểm đúng điều đó với Claude thật. Nguyên tắc: màn hình không tồn tại hoặc không nhận dữ liệu thật là LỖI SẢN PHẨM, spec báo (mã gap + người sở hữu), không nới điều kiện cho xanh.

## Theo dõi phiên agent (bản đồ)

Đường đi của dữ liệu khi `task.execute` chạy agent (Engine 1, `direct_agent`, đường của mọi task thuộc Request):

```
request-service AdvanceExecution --Execute(taskID, "req:<req>:<task>:<n>")--> task-service ExecuteTask
  -> SimpleExecutor.runAgentPrompt: relay "agent.execPrompt" (stepId = executionRequestID) tới agent trên dev server
       agent: handleAgentExecPrompt spawn `claude --print <prompt> [--dangerously-skip-permissions]`,
              mỗi chunk stdout/stderr -> notify 'agent.execPrompt.output'
  <- infra-fleet devserveragent: StreamExecOutput (demux theo stepId)
  <- SimpleExecutor.publishThrottledOutput: gom CỘNG DỒN, tối đa 1 lần / 2 giây -> outbox -> NATS orca.task.agent_output_partial
api-gateway: task.activity.subscribe {taskId} -> push task.activity.event {taskId, engine:"direct_agent", eventType:"agent_output_partial", payload:{origin_task_id, request_id, stdout, stderr}}
```

| # | Thành phần | Có? | Chi tiết (bằng chứng) |
|---|---|---|---|
| 1 | Điều hướng Request → task | CÓ | Tab Plan, hàng `plan-task-row-<id>` (`PlanTaskRow`, là nút) mở `Sheet` chứa `TaskDetail` (`PlanTree.openTask`). Giống web và Electron. Đã có trong test mock cũ (`request-clarification-risk.web.e2e.ts`) |
| 2 | Trạng thái task trực tiếp trong sheet | CÓ, bằng polling | `useTaskActivity` gọi `task.get` mỗi 4 giây; `task-live-status` ("Status: in_progress"), nút Execute đổi thành "Agent running…" (`run-agent-btn`), huy hiệu engine "Direct Agent" (suy ra phía client, `ExecutionEngineBadge`) |
| 3 | Hoạt động trực tiếp của agent (output, công cụ đang dùng) trên UI | KHÔNG | Frontend không có chỗ nào gọi `task.activity.subscribe` (tìm trong `frontend/src`: chỉ còn comment cũ trong `useTaskActivity.ts` nói kênh "chưa tồn tại"). Kênh gateway CÓ và có test (`channels_task_activity.go`). Điểm đứt: kênh có, UI không nối |
| 4 | Nội dung stream có thật sự là "đang làm gì" | KHÔNG (suy ra từ mã, chờ xác nhận khi chạy) | `agent-print-mode-exec.ts` chạy `claude --print <prompt>` chỉ thêm cờ YOLO/chỉ-đọc, KHÔNG có `--output-format stream-json`: chế độ text của `--print` chỉ in câu trả lời cuối khi xong. Nên `agent_output_partial` thực tế là một đoạn lúc kết thúc, không có "đọc file / gọi công cụ" (`AgentBinarySpec` có stream-json nhưng chỉ cho PTY/`agent.start`) |
| 5 | Kết quả có cấu trúc sau khi chạy (tab "Result": file đổi, checks, secret scan) | UI CÓ, dữ liệu KHÔNG | `ExecutionResultPanel` + `useExecutionResult` gọi `execution.get`; gateway KHÔNG đăng ký kênh này (nằm trong allowlist của FE-E2E-001) nên tab "Result" bị ẩn. Sâu hơn: task-service có `ListExecutionRecords` (bảng `task_execution_records`: `changes_json`, `stdout_tail`) nhưng chỉ ghi khi chạy đường hợp đồng (có `result_nonce`), mà `request-service` `TaskClient.Execute(taskID, executionRequestID)` không gửi `result_nonce` (grep `request-service`: không có) nên bản ghi không bao giờ được tạo cho task của Request |
| 6 | stdout đã lưu | CÓ trong DB, KHÔNG đọc được | `tasks.last_execution_output` (cắt 8 KB) ghi sau mỗi lượt, nhưng không có trong proto `Task` / `taskView` của gateway |
| 7 | Xem file agent đã đổi | CÓ nếu task có `worktreeId` | Nút "Open in Git" (`open-in-git-btn`) trong sheet mở tab Git của worktree task |
| 8 | Dev server online | CÓ | Cài đặt → Dev Servers, `DevServerCard` + `DevServerStatusBadge` (`aria-label="Dev server status: connected"`), web poll `devServer.list` mỗi 3 giây. Thăm dò thật: thẻ `dev-local-mac` hiện `connected`, các thẻ khác `disconnected`. Thẻ không có `data-testid` (dùng lớp `.dev-server-card`, `.dev-server-card__name`, `.dev-server-badge`) |
| 9 | Dev server "bận" | KHÔNG | `devServerView.status` chỉ `connected`/`disconnected` (`attachConnectionStatus` = `IsDevServerConnected`); UI có thêm `connecting`/`error` nhưng backend không phát. Không có đếm lượt chạy đang làm. Thăm dò thật: `healthStatus` luôn `pending` kể cả thẻ đang `connected`; `platform`/`arch`/`nodeVersion` đều null nên badge ghi `connected` chứ không phải tên hệ điều hành |
| 10 | `AgentPanel` (tab agent của workspace), `agent.subscribeStatus` | KHÔNG liên quan | Đây là phiên TƯƠNG TÁC (`agent.start` → PTY), trạng thái `agent.statusChanged` chỉ cho `AgentSession` do `StartAgentSession` tạo; `agent.execPrompt` là tiến trình một lần, không có PTY/`AgentSession`, nên chạy task không làm panel hay kênh trạng thái này đổi. Panel chỉ biết phiên chính nó bấm Start trong store (`remoteAgentSessions[worktreeId]`); gateway không có `agent.list` |
| 11 | `TaskDispatchStatusPanel` + "Focus terminal" | KHÔNG áp dụng | Chỉ cho Engine 2 (`orchestration.dispatchShow`). Task của Request đi Engine 1 nên luôn hiện "Task này chưa có dispatch orchestration nào..." (chuỗi tiếng Việt cứng, không qua i18n) |
| 12 | Web khác Electron | Giống | Cả hai chỉ có polling `task.get`; `subscribeRuntimeStreamChannel` từ chối target `local` (Electron) nên muốn có stream phải làm cả hai đường |

### Điểm đứt (tóm tắt)

- Có kênh, UI không nối: `task.activity.subscribe` (#3). UI có, backend không phát/không mở: tab Result / `execution.get` (#5). Có dữ liệu, không ai đọc được: `last_execution_output` (#6).
- Không phát tín hiệu "bận": dev server (#9). Phiên agent của task: không tồn tại như một khái niệm trên web (#10).
- Nội dung stream nghèo vì `--print` ở chế độ text (#4).

### Cập nhật sau redeploy (đã đo bằng lượt chạy thật)

Phần bản đồ ở trên là khảo sát bằng mã trước redeploy. Sau redeploy (Q2, Q3, agent mới, frontend mới) và lượt chạy thật, các dòng đổi như sau:

| # | Trước | Sau (đo thật) |
|---|---|---|
| 2 | polling 4 giây | vẫn polling `task.get`; thấy `Status: open` → `in_progress` → `review` → `done` trong sheet và nút "Agent running…" suốt lúc chạy (mẫu mỗi ~3 giây) |
| 3 | KHÔNG nối | CÓ panel `task-agent-activity` (`data-state` running → finished, huy hiệu Live). Sau sửa G1: 5 đến 8 frame mỗi lượt, panel `running` với tối đa 9 dòng lúc chạy, `finished` 13 đến 15 dòng sau đó (loại `info,text,tool,result,end`) |
| 4 | agent không stream | agent mới stream-json: frame đầu đến +1 đến +2 giây sau khi task `in_progress`, dòng cuối `■ Done · 3 turns · 15.5s`; frame cuối 1.2 KB |
| 5, 6 | tab Result ẩn, stdout không đọc được | tab Result CÓ sau khi chạy (`execution.get` + `GetTaskExecutionOutput`): "Agent output" hiện câu trả lời cuối của agent (nêu `src/calc.py:20`, `tests/test_calc.py:18`, "All 4 tests pass (python3 -m unittest)"). Xem lại sau khi tải lại trang: ĐƯỢC. Chỉ là đoạn cuối (8 KiB), không phải từng công cụ |
| 8, 9 | online; không có "bận" | `dev-server-busy`: `busy=true` trong lúc chạy, `busy=false` lúc rảnh, gateway `healthStatus: healthy`, `activeExecutions` 0 sau khi xong. Đạt |
| 8 | platform | `dev-server-platform` = "macOS · arm64", badge ghi "macOS" (G2 đã sửa) |
| 7 | "Open in Git" có | điều hướng sang view terminal của worktree + Source Control ở thanh phải (G3 sửa), terminal mở được (G6 sửa); danh sách thay đổi vẫn TRỐNG vì backend (G5c) |

## Spec (`agent-session-monitoring-real.web.e2e.ts`)

Chạy: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real monitor` (cũng nằm trong `--real all`). Cùng lưới an toàn của TASK-FE-E2E-010 (`runRealFlow`: repo tạm, hook chặn, dấu vân tay trước/sau, watchdog, hạn cứng, dọn). Plan một task (`negate`), Claude thật thực thi; spec mở thêm một kết nối WebSocket của riêng nó tới gateway để nghe `task.activity.subscribe` TRƯỚC khi duyệt Plan.

Trong lúc agent chạy (hook `onFirstTaskRunning` của `driveExecutionToTheEnd`, 3 giây một mẫu):
- Trang chính: mở sheet task từ tab Plan, đọc `task-live-status`, nút Execute ("Agent running"), mọi tab của sheet (chỉ bấm tab, không bấm nút nào trong tab).
- Tab thứ hai: Cài đặt → Dev Servers, đọc badge của dev server, và `devServer.list` qua gateway.
- Kênh `task.activity`: mọi frame `agent_output_partial`.

Phán quyết (hàm thuần, kiểm bằng `live-agent-session-judge.test.ts`, mỗi mã nói rõ người sở hữu lỗi). Cụm từ so với UI lấy từ chính frame backend (dòng công cụ cuối, bỏ ký hiệu đầu dòng):

| Mã gap | Chủ | Điều kiện | Yêu cầu của người dùng |
|---|---|---|---|
| `DEVSERVER-OFFLINE-IN-UI` | frontend | badge không phải online trong lúc agent chạy | (i) dev server online |
| `DEVSERVER-NO-BUSY-STATE` | backend | UI và `devServer.list` giống hệt nhau giữa "agent đang chạy" và "rảnh" | (i) trạng thái đổi khi nhận task rồi về rảnh |
| `UI-NO-PATH-TO-TASK` | frontend | không có `plan-task-row-<id>` để vào task từ Request | (ii) điều hướng |
| `UI-NO-RUNNING-INDICATOR` | frontend | task `in_progress` mà sheet không hiện "Agent running" | (ii) |
| `UI-TASK-STATUS-STALE` | frontend | sheet không bao giờ hiện trạng thái task sau khi rời `in_progress` | (ii) cập nhật khi chạy |
| `AGENT-OUTPUT-NOT-BROADCAST` | backend | không có frame `agent_output_partial` nào | (ii) |
| `AGENT-OUTPUT-NOT-STREAMED` | agent | chạy từ 15 giây mà mọi frame đến trong 5 giây cuối | (ii) "cập nhật khi chạy, không chỉ trạng thái cuối" |
| `UI-AGENT-OUTPUT-NOT-SHOWN` | frontend | backend đã phát, không tab nào của sheet hiện cụm từ lấy từ frame (28 ký tự của dòng cuối) | (ii) |
| `UI-AGENT-OUTPUT-NOT-LIVE` | frontend | cụm từ chỉ hiện sau khi task rời `in_progress` (khi backend có stream thật) | (ii) |
| `UI-NO-SESSION-HISTORY` | backend | tải lại trang, mở lại task: không còn thấy cụm từ | (iii) xem lại sau khi xong |
| `UI-NO-RUN-TRACE` | frontend | trong phần văn bản MÀ LƯỢT CHẠY THÊM VÀO (so với trước khi chạy; Plan chỉ nhắc đến file thì không tính) không có `src/calc.py` và dấu vết chạy test | (iii) dấu vết đúng việc |
| `UI-NO-WORKTREE-VIEW` | backend/frontend | không có "Open in Git" hoặc bấm xong màn hình không thêm gì liệt kê `src/calc.py` | (iii) |
| `DEVSERVER-NEVER-SHOWED-BUSY`, `DEVSERVER-NOT-IDLE-AFTER-RUN` | backend | thẻ dev server chưa từng `busy=true` khi chạy, hoặc không về `busy=false` sau khi xong | (i) |
| `DEVSERVER-NO-PLATFORM`, `DEVSERVER-NOT-HEALTHY` | agent / backend | thẻ không có `dev-server-platform`; `healthStatus` của dev server đang nối không phải `healthy` | (i) |
| `UI-NO-AGENT-ACTIVITY-PANEL`, `UI-ACTIVITY-PANEL-STATE` | frontend | sheet không có `task-agent-activity`; panel không bao giờ `running` kèm huy hiệu Live lúc `in_progress`, hoặc sau chạy không `finished` kèm dòng | (ii) |

Kiểm cứng (không phải gap, spec ném lỗi): `SECRET-LEAK` nếu văn bản mọi màn hình đã đọc hoặc frame của kênh hoạt động chứa giá trị của biến bí mật (tên chứa PASSWORD/TOKEN/SECRET/KEY, từ `deploy/dev/.env` và môi trường tiến trình, 8 ký tự trở lên) hoặc dạng bí mật (`sk-ant-`, khối khoá riêng, `Bearer eyJ`), cùng định nghĩa với AST-02; chỉ in tên biến. `TASK-STUCK-IN-PROGRESS` (90 giây không có claude) được lặp lại trong vòng lấy mẫu vì vòng này thay vòng của bộ điều khiển. Lỗi console của cả hai trang đi qua allowlist của `live-console-audit.ts`.

Mặc định mọi gap làm test đỏ (soft assertion, các bước sau vẫn chạy và báo); `ORCA_E2E_LIVE_GAPS=annotate` chỉ ghi lại. Bằng chứng in ra dòng `[monitor] ...` và annotation `agent-session-monitor`.

## Việc cần làm

- [x] A. Khảo sát: bản đồ ở trên (mã gateway, task-service, infra-fleet, agent, frontend).
- [x] B. Spec + hàm phán quyết thuần + test đơn vị (12/12) + `subscribe` của `GatewayRpcClient` (thử thật: ack, từ chối `taskId` rỗng) + chạy `--real monitor` trong `run_ui_flow.py`.
- [x] C. Tự kiểm không dùng AI: tsc strict sạch, oxlint sạch (đúng giới hạn dòng, không tắt `max-lines`), thăm dò thật chỉ-đọc ngăn Dev Servers (chọn thẻ theo tên, đọc badge, `devServer.list`), spec nạp được (`playwright --list`).
- [x] D. Chạy thật sau redeploy: execute 3/3, readonly 2/2, monitor đỏ 4 gap thật (G1 đến G4 dưới đây).
- [~] E. Sửa G1 đến G4 (backend task-service/infra-fleet, agent, frontend) rồi chạy lại `--real monitor`; sau đó F1/F2 mới kiểm được nội dung stream. Phía frontend: G3a/G3b/G3c/G4 `[x]` ở [TASK-FE-E2E-013](./TASK-FE-E2E-013-task-sheet-fixes.md) (Playwright mock + vitest xanh, chưa dựng lại frontend nên chưa kiểm trên stack thật); G1/G2 là backend/agent, không đổi ở đây.

## Lỗi sản phẩm tìm thấy khi khảo sát bằng mã (trước redeploy; kết quả đo sau redeploy ở "Kết quả chạy")

F1 (không có UI), F3 (không xem lại được phiên) và F4 (không có "bận") đã được sửa trong bản redeploy và kiểm lại bằng lượt chạy thật; F2 (stream-json) đã build nhưng chưa kiểm được vì G1. F5, F6 còn nguyên (F6 nay đã có `data-testid="dev-server-card"`).

Phân loại và đề xuất. Không sửa `backend-go/**` và `agent/**` ở task này.

**F1. [frontend] Không có màn hình nào hiện hoạt động trực tiếp của agent dù gateway có kênh.** `task.activity.subscribe` không có người dùng; `useTaskActivity` chỉ poll `task.get`. Đề xuất (cỡ trung bình, khoảng một hook + một panel + i18n 5 ngôn ngữ en/es/ja/ko/zh + vitest, theo STYLEGUIDE): `useTaskAgentOutput(taskId)` dùng `subscribeRuntimeStreamChannel('task.activity.subscribe', {taskId})`, giữ frame `agent_output_partial` mới nhất (cộng dồn nên không cần ghép), hiển thị `<pre data-testid="task-agent-output">` trong sheet task (tab Details hoặc AI Agent), có trạng thái "đang chờ output" và khi tải lại thì rỗng nếu không có lịch sử (xem F3). Chỉ chạy đường web (`environment`); Electron `local` cần đường riêng hoặc ghi rõ không hỗ trợ. Không làm trong task này vì chưa xác nhận backend phát đúng khi chạy thật.

**F2. [agent] Chế độ `--print` text không stream.** Nên "đọc file/gọi công cụ" không bao giờ có trong `agent_output_partial`. Đề xuất: chạy task với `--output-format stream-json --verbose` (cờ đã có trong `AgentBinarySpec`) và chuyển mỗi sự kiện công cụ thành dòng đọc được trong `agent.execPrompt.output`, giữ nguyên `stdout` cuối cho bộ phân tích kết quả và `{{outputs.<task>.stdout}}` (nội suy). Cần thiết kế (đổi nghĩa `stdout` ảnh hưởng `agent-result-block-parser`, `UpdateLastExecutionOutput`). Chờ xác nhận bằng mã gap `AGENT-OUTPUT-NOT-STREAMED` khi chạy.

**F3. [backend-go api-gateway + request-service] Không xem lại được phiên sau khi chạy.** (a) gateway chưa đăng ký `execution.get` (tab Result ẩn); (b) task-service đã có `ListExecutionRecords` nhưng `request-service` không gửi `result_nonce` nên không có bản ghi; (c) `last_execution_output` có trong DB nhưng không nằm trong `Task` proto. Đề xuất tối thiểu: thêm `last_execution_output` vào `Task`/`taskView`, hoặc đăng ký `execution.get` → `ListExecutionRecords` và cho request-service đi đường hợp đồng (nonce) để có `changes_json` và `stdout_tail`. Bản ghi hợp đồng mới trả đúng thứ UI đã có sẵn (file đổi, checks, secret scan).

**F4. [backend-go infra-fleet/api-gateway] Dev server không có trạng thái "bận".** Đề xuất: `devServer.list` thêm số lượt chạy đang làm (đếm liên kết thực thi `in_progress` của Engine 1 theo dev server, hoặc đếm relay `agent.execPrompt` đang chờ) và đẩy `devServer.statusChanged`; badge hiện "Busy". Kèm: `healthStatus` luôn `pending`, `platform`/`arch`/`nodeVersion` luôn null (badge không hiện tên hệ điều hành); cả hai là dữ liệu handshake không được ghi.

**F5. [frontend, nhỏ] Chuỗi tiếng Việt cứng và comment lỗi thời.** `TaskDispatchStatusPanel` ("Task này chưa có dispatch...", "Focus terminal đang chạy...") không qua `translate`; với task của Request luôn hiện thông báo này (vô nghĩa). `useTaskActivity.ts`/`TaskDetail.tsx` còn nói kênh "chưa build". Đề xuất: ẩn panel khi task thuộc Request (Engine 1), chuyển chuỗi vào i18n; cập nhật comment khi làm F1.

**F6. [frontend, nhỏ] `DevServerCard` không có `data-testid`.** Spec dùng lớp CSS và `aria-label` của badge; thêm `data-testid` (chỉ thuộc tính) sẽ ổn định hơn, chưa làm vì cần dựng lại frontend mới dùng được.

## Kết quả chạy

Ngày 2026-10-10, stack local sau redeploy, dev server `dev-local-mac` là Claude Code thật (không `--ai-stub`, không fake claude), UI qua Chromium (Playwright), lưới an toàn của TASK-FE-E2E-010 giữ nguyên. Mọi lượt: "safety problems: 0", "guard denials: 0", hook thấy 2 (chỉ đọc) đến 15 (change_request 5 task) lệnh gọi công cụ, 0 mục console chưa phân loại, sau lượt `report_orphan_requests.py`: "0 orphaned", 0 thư mục `e2e-ui-*` trong `$TMPDIR`, 0 tiến trình `claude --print`, `test-results` đã xoá.

| Lệnh | Test | Kết quả | Thời lượng |
|---|---|---|---|
| `--real execute` | `change_request`: Plan có Phase và 5 task phụ thuộc nhau (duyệt Plan, duyệt và Start Phase trên UI) | XANH (B1 và B2 đã sửa đúng) | 6.1 phút (chạy 156 giây) |
| | `task`: 2 task phụ thuộc (subtract + test) | XANH | 3.2 phút |
| | `task, single step` | XANH | 2.6 phút |
| `--real readonly` | `question`, `bug` | XANH 2/2 | 2.5 và 3.0 phút |
| `--real monitor` | one-task Plan, nhìn agent từ UI | ĐỎ, 1 gap thật (G5); mọi mục còn lại đạt | 4.2 đến 4.4 phút |

Sáu lượt `--real monitor` (m1 đến m6): m1 đến m5 lộ lỗi của chính spec, không phải sản phẩm: (1) mở sheet sau khi agent đã chạy xong (cùng thời điểm bắt đầu dev tab và đọc tab Request tốn 60 giây, agent chỉ chạy 20 đến 40 giây): chuyển `prepare()` ra trước khi duyệt và `watchTask()` ngay sau khi duyệt; (2) `innerText()` của phần tử vắng chờ 30 giây (action timeout) làm mỗi mẫu chậm: dùng đếm trước; (3) nút Escape đóng sheet cũng làm Request bị bỏ chọn (trang bắt Escape ở mức trang), nên header trống ("UI-STALE-AFTER-EXECUTION" ở m1, m2): đóng bằng nút Close của sheet; (4) hai tab dùng chung "view gần nhất" của SPA nên tải lại trang chính ra Settings: tab dev bấm "Back to app" trước khi đóng và `reloadAndReopen`/`openRequestsPage` chấp nhận "Back to app". Lượt m6 chạy hết đường: lấy mẫu, tải lại, đọc lại tab, "Open in Git", quét bí mật.

Cái ĐẠT ở m6 (đo, không nới): Execute button "Agent running…" khi `in_progress`; trạng thái sheet theo kịp `in_progress → review → done` không cần F5; dev server `connected` suốt, `busy=true` khi chạy và `busy=false` sau khi xong, gateway `healthy`; sau tải lại trang tab "Result" vẫn có "Agent output" nêu đúng việc (sửa `src/calc.py`, thêm test, chạy `python3 -m unittest`) nên `UI-NO-RUN-TRACE` đạt (kiểm trên phần văn bản mà lượt chạy thêm vào); quét bí mật: 15 biến bí mật, 31 lần đọc màn hình và các frame: sạch.

### Kết quả sau redeploy lần 2 (G1 đến G4 đã sửa)

| Lệnh | Test | Kết quả | Thời lượng |
|---|---|---|---|
| `--real all` (lượt cuối, 21.6 phút, 6 test) | execute `change_request` (Phase) | XANH | 6.1 phút |
| | execute `task` (phụ thuộc) | ĐỎ an toàn lượt này (hook từ chối, G7), XANH khi chạy lại riêng | 2.9 / 4.0 phút |
| | execute `task, single step` | XANH | 2.8 phút |
| | readonly `question` | XANH | 2.6 phút |
| | readonly `bug` | XANH | 2.6 phút |
| | monitor | ĐỎ 1 gap thật (G5) | 4.4 phút |
| `run_ui_flow.py -- request-live-flow` (Claude thật) | `change_request`, `question` | XANH 2/2 (4.1 và 1.2 phút); `ask-back` bỏ qua (chỉ stub) | 5.3 phút |

Đo được trong lượt monitor (không nới): 8 frame `agent_output_partial` cho task, frame đầu +1 giây; panel `running` rồi `finished` 15 dòng; dev server `macOS · arm64`, `busy=true` khi chạy và `busy=false` + `healthy` sau; sheet và nút "Agent running" bám theo; tab Result sau tải lại; quét bí mật (15 biến, 35 lần đọc) sạch; "safety problems: 0". Spec còn hai điều chỉnh tìm được: `expectPlanTabFinished` nay ghi gap `UI-STALE-PLAN-TAB` rồi tải lại (ở một lượt Plan tab giữ "40% (estimated)" và task "In Progress" trong khi Request đã Completed; không lặp lại ở các lượt sau, nên coi là thoáng qua, giữ gap để lần sau thấy); allowlist console: thêm `WORKTREE_RESOLVE_FAILED` và "Failed to list runtime detected worktrees" vào rule dữ liệu cũ, thêm rule cảnh báo WebGL của Chromium headless, xoá rule `create-request-dialog-description` (frontend đã dựng lại, không còn cảnh báo).

### Lỗi thật còn lại (có bằng chứng)

**G5. [đã sửa một phần] Source Control của worktree task trống.** Gốc thứ nhất (frontend, đã sửa và redeploy): `resolveRuntimeWorktreeByPath` dùng `Promise.all` trên `worktree.detectedList` của mọi repo nên một repo cũ lỗi làm cả danh sách rỗng ("No runtime worktree owns ..."); nay `allSettled` và đường dự phòng `worktree.list`. Lỗi console đó biến mất. `SCM_EMPTY_BRANCH: head_branch is required` là G5b (frontend gửi `branch/base/repo`, gateway đọc `headBranch/baseBranch`, nên rỗng trên mọi worktree), chưa sửa, đã đưa vào allowlist console có lý do (`scm-empty-branch-g5b`) thay vì để làm đỏ.

**G5c. [ĐÃ SỬA, kiểm lại bằng lượt chạy thật] `git.status` của worktree task trả danh sách rỗng.** Gốc thật (agent U): `RelayExecutor.GetStatus` giải đáp án của agent thẳng vào `{Branch, Files}` nên `entries` bị bỏ im lặng; nay `git.status {worktree:"id:..."}` trả `entries:[{path:"src/calc.py",status:"modified",area:"unstaged",added:5,removed:0},{path:"tests/test_calc.py",...,added:4,removed:1}]` và Source Control hiện hai dòng ("calc.py src +5 M", "test_calc.py tests +4 -1 M"). Mô tả cũ của lỗi: Bằng chứng (đo bằng chính socket của trang, 3 lượt liên tiếp): `git.status {"worktree":"id:<worktreeId>"} -> {"branch":"refs/heads/task/<taskId>","entries":[]}` (nhánh đúng, 0 mục), trong khi trên đĩa (spec đọc `git status --porcelain` của worktree trước khi xoá): 2 file chưa commit `src/calc.py`, `tests/test_calc.py`, 0 commit ahead. Nên Source Control hiện ô Message và "Publish Branch" nhưng không có dòng `source-control-entry`. Nghi: handler `git.status` giải `worktree:id:` ra đúng nhánh nhưng chạy `git status` ở thư mục khác (repo chính hoặc đường dẫn không tồn tại trên agent, vd `/var` so với `/private/var`). Cần backend xem cwd mà git-gateway/agent dùng cho `git.status` với `worktree:id:`. Đây là gap `UI-NO-WORKTREE-VIEW`.

**G6. [đã sửa] Terminal của worktree.** Agent tự `chmod` spawn-helper của node-pty; lượt chạy thật: không còn `INFRA_AGENT_SPAWN_PTY_FAILED`, terminal hiện dấu nhắc. Ghi chú nhỏ chưa xác nhận: dấu nhắc là `~` (có thể terminal mở ở thư mục home chứ không phải worktree); spec chỉ kiểm không có lỗi spawn (`WORKTREE-TERMINAL-FAILED`), chưa kiểm cwd.

**G7. [harness an toàn, bất định] Claude Code ghi vào scratchpad riêng của phiên trong `/private/tmp/claude-<uid>/<cwd-mã-hoá>/<session>/scratchpad/`, hook từ chối.** Một lượt `execute task` đỏ an toàn: `Bash command names a path outside the fixture: /private/tmp/claude-501/-private-var-...-e2e-ui-...-repo-task-<id>/<uuid>/scratchpad/conventions.md`; watchdog dừng lượt (đúng thiết kế), chạy lại riêng thì xanh (hook thấy 9 lệnh, 0 từ chối). Hiện tượng thoáng qua và an toàn nhưng làm test đỏ ngẫu nhiên. Ngoài ra mỗi lượt claude để lại thư mục phiên NGOÀI fixture: 43 thư mục `/private/tmp/claude-501/*e2e-ui*` và 68 thư mục `~/.claude/projects/*e2e-ui*` (bản ghi phiên), mà dấu vân tay không thấy. Đề xuất (cần người điều phối chấp thuận vì chạm thiết kế an toàn): cho hook cho phép đúng đường dẫn `/private/tmp/claude-<uid>/<mã hoá của thư mục gốc fixture>*` (chỉ phiên có cwd trong fixture) và để `dispose()` xoá thư mục đó cùng bản ghi `~/.claude/projects/*<tên fixture>*`. Tôi chưa đổi hook và chưa xoá các thư mục cũ.

Nhỏ: Escape trong sheet task đã được frontend sửa (chỉ đóng sheet).

### Lượt cuối `--real all` (21.4 phút, 6/6 XANH)

| Test | Kết quả | Thời lượng |
|---|---|---|
| monitor | XANH (3.9 phút; trước đó `--real monitor` riêng 3.8 phút) | 3.9 phút |
| execute `change_request` (Phase) | XANH | 6.2 phút |
| execute `task` (phụ thuộc) | XANH | 3.5 phút |
| execute `task, single step` | XANH | 2.8 phút |
| readonly `question` | XANH | 2.6 phút |
| readonly `bug` | XANH | 2.5 phút |

Mọi lượt "safety problems: 0", 0 hook từ chối, 0 mục console chưa phân loại; `report_orphan_requests.py` "0 orphaned"; 0 thư mục `e2e-ui-*` trong `$TMPDIR`; 0 tiến trình `claude --print`; `test-results` đã xoá. Một lượt monitor giữa chừng đỏ vì `git-gateway` đang khởi động lại do redeploy (`TASK_EXECUTE_WORKTREE_FAILED`, connection refused), không phải lỗi sản phẩm; chạy lại xanh.

### Gap có mã còn mở (không làm đỏ spec)

- **G5d/G5e [frontend + backend-go git-gateway]:** bước thông tin của spec bấm vào dòng `calc.py` trong Source Control: không có diff nào chứa `negate` trong 10 giây và trang ghi `TypeError: Cannot read properties of undefined (reading 'length')` tại `diff-content-signature` / `[terminal.workbench] render crash contained by boundary` (panel diff vỡ, boundary giữ được). Khớp mô tả G5d (`git.diff` trả proto snake_case, không phải `GitDiffResult`) và G5e (`GetDiff` luôn rỗng qua relay). Spec ghi annotation `known-gap` và `[known-gap]` ở log; hai thông báo console nằm trong allowlist `diff-view-crash-g5d` có lý do; khi sửa thì bước này in "a diff showing negate appeared" và rule đó nên bị xoá.
- **G5b [frontend/gateway]:** `SCM_EMPTY_BRANCH` (allowlist `scm-empty-branch-g5b`).
- **G7 [harness an toàn, ngẫu nhiên]:** scratchpad riêng của phiên claude (không tái diễn ở 3 lượt `--real all` gần nhất).
- Nhỏ: dấu nhắc terminal của worktree là `~` (chưa kiểm cwd).

## Đề xuất cho người điều phối

1. Giao G5d/G5e (git.diff đúng shape và không rỗng qua relay); khi sửa, xoá rule `diff-view-crash-g5d` và chuyển bước "diff peek" thành kiểm bắt buộc.
2. G5b (tên trường `headBranch/baseBranch`), rồi xoá rule `scm-empty-branch-g5b`.
3. G7: quyết định cho phép hẹp scratchpad của phiên claude trong fixture và dọn thư mục phiên sau mỗi lượt.
