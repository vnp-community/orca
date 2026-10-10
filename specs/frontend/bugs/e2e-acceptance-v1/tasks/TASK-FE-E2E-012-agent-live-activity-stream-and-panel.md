# TASK-FE-E2E-012: Luồng hoạt động trực tiếp của agent (agent) và panel "Agent activity" trên web (frontend)

**From Solution:** SOL-FE-E2E-003 · **Resolves:** TASK-FE-E2E-011 F1 (UI không nối kênh `task.activity.subscribe`), F2 (agent chạy `claude --print` dạng text nên không có "đọc file / gọi công cụ"), F5, F6
**Priority:** P1 · **Area:** agent + frontend + stub e2e
**Files:**
- agent/src/relay/agent-claude-stream-activity.ts (mới: bộ phân tích luồng `stream-json` thành dòng hoạt động + chọn `stdout` cuối), agent-claude-tool-activity-text.ts (mới: mô tả công cụ/kết quả an toàn), agent-exec-prompt-output-capture.ts (mới: bộ đệm + che bí mật + phát `agent.execPrompt.output`)
- agent/src/relay/agent-print-mode-exec.ts (chỉ `handleAgentExecPrompt`), agent-readonly-tool-policy.ts (`ClaudeFlagSupport.streamJson`)
- tests/stages/e2e/orca-flows/fake_claude_cli/{claude, test_fake_claude_cli.py, request_service_contract.overlay.go.txt}, tests/stages/e2e/orca-flows/suite_ai_stub_login.py (AST-02)
- frontend/src/renderer/src/hooks/{task-agent-output-store.ts, useTaskAgentOutput.ts}, components/task/agent-activity/{agent-activity-lines.ts, TaskAgentActivityPanel.tsx}, components/task/{TaskDetail.tsx, TaskDispatchStatusPanel.tsx}, components/dev-server/DevServerCard.tsx, runtime/runtime-rpc-client.ts, hooks/useTaskActivity.ts (chỉ comment), i18n/locales/{en,es,ja,ko,zh}.json
- tests/stages/e2e/legacy/request-web/{request-agent-live-activity.web.e2e.ts (mới), support/mock-request-ws.ts}
**Depends on:** TASK-FE-E2E-010 (an toàn, repo tạm, hook chặn, dấu vân tay: giữ nguyên), TASK-FE-E2E-011 (khảo sát)
**Status:** `[x]` XONG (2026-10-10): kiểm trên stack thật với Claude Code thật (`--real monitor`): panel Agent activity hiện 13–15 dòng thật (`· Session started`, `» ...`, `▸ Read/Edit/Bash`, `■ Done`) lúc chạy rồi `Finished`. Hạn chế: Electron `local` chưa hỗ trợ; backend không giữ lịch sử từng dòng.

---

## Context

Người dùng muốn "vào web và dùng Orca full luồng": khi Orca thực thi Plan, phải thấy agent đang đọc file nào, gọi công cụ gì, in ra gì. Dữ liệu đã đi được từ agent tới gateway (`agent.execPrompt.output` → infra-fleet → task-service gom cộng dồn tối đa 1 lần / 2 giây → NATS → gateway `task.activity.subscribe`) nhưng: (F1) không màn hình nào gọi kênh; (F2) `claude --print` ở chế độ text chỉ in câu trả lời cuối nên luồng không có sự kiện công cụ.

## Định dạng sự kiện claude thật đã quan sát (claude 2.1.266)

`claude --print <prompt> --output-format stream-json --verbose --permission-mode plan --tools Read,Glob,Grep` (stdin đóng như agent, một dòng JSON mỗi sự kiện). Chạy thật 2 lượt ngắn trong thư mục tạm có một file:

| Thứ tự | `type` / `subtype` | Ghi chú |
|---|---|---|
| 1 | `system` / `hook_started`, `hook_response` | hook của người dùng (SessionStart); bỏ qua |
| 2 | `system` / `init` | `model`, `cwd`, `tools[]`, `permissionMode`, `session_id`...; chỉ dùng `model` |
| 3 | `assistant` | `message.content[]`: `{type:"text",text}` hoặc `{type:"tool_use",id,name,input}` (một khối mỗi sự kiện) hoặc `{type:"thinking",thinking:"",signature}` (không hiện) |
| 4 | `rate_limit_event`, `system`/`task_summary`, `system`/`post_turn_summary` | bỏ qua |
| 5 | `user` | `message.content[]`: `{type:"tool_result",tool_use_id,content:<chuỗi>,is_error?}` (+ `tool_use_result` chứa NỘI DUNG FILE đầy đủ, không bao giờ đọc) |
| 6 | `result` / `success` | `is_error`, `num_turns`, `duration_ms`, `result` = đúng văn bản mà chế độ text từng in (không có xuống dòng cuối), `permission_denials[]`... |

Bằng chứng công cụ bị chặn: yêu cầu ghi file trong chế độ plan + `--tools Read,Glob,Grep` cho `tool_result` `is_error:true` ("No such tool available: Write ..."), tức là hạn chế chỉ-đọc vẫn có hiệu lực với định dạng mới. `--output-format stream-json` bắt buộc `--verbose` (xác nhận trong `claude --help` và stub refuse giống thật).

## F2: agent

### Thiết kế

- **Khi nào dùng**: `handleAgentExecPrompt` (cả ghi lẫn chỉ-đọc) thăm dò `claude --help` (cache theo PATH 10 phút, đã có cho chỉ-đọc, nay dùng cho cả hai chế độ). `ClaudeFlagSupport.streamJson` = help có `--output-format`, `stream-json` và `--verbose`. Không có (claude cũ) hoặc thăm dò hỏng → chạy `claude --print <prompt>` text y hệt trước. Không bao giờ làm hỏng thực thi.
- **Đối số**: `--print <prompt> --output-format stream-json --verbose` rồi mới đến cờ cũ (`--dangerously-skip-permissions` khi ghi + trust full, hoặc `--permission-mode plan --tools Read,Glob,Grep` khi chỉ-đọc). Không thêm cờ nới quyền; `trustPreset=full` vẫn bị bỏ qua ở chỉ-đọc. Hook `PreToolUse` của repo (`.claude/settings.json`) không phụ thuộc định dạng đầu ra; `--include-hook-events` KHÔNG dùng.
- **`stdout` trả cho người gọi giữ nghĩa cũ**: = `result` của sự kiện `result` cuối, thêm `\n` cuối như chế độ text (bộ phân tích khối kết quả, nội suy `{{outputs...}}`, tài liệu JSON Solution đọc y hệt). Lỗi (`is_error`, ví dụ "Not logged in · Please run /login") cũng nằm trong `result` nên request-service vẫn phân loại được. Thiếu `result` (hết lượt, bị kill do timeout): dùng văn bản trợ lý đã gom; thiếu cả hai: `stdout` rỗng và `stderr` thêm dòng `claude ended without an answer (<subtype>)`. Luồng không giống stream-json (claude bỏ qua cờ): trả đúng byte thô như trước. `truncated.stdout` tính trên văn bản câu trả lời, không phải trên luồng JSON.
- **Dòng hoạt động** (qua `agent.execPrompt.output` `stream:"stdout"`, cùng cơ chế chunk cũ; mỗi lần dữ liệu tới gửi một lô dòng). Mỗi dòng mở đầu bằng một ký hiệu để giao diện tô kiểu:

| Ký hiệu | Nghĩa | Ví dụ |
|---|---|---|
| `·` | thông tin phiên | `· Session started (claude-opus-5[1m])` |
| `»` | văn bản trợ lý (tối đa 6 dòng x 240 ký tự mỗi khối, thêm `» …` nếu cắt); dòng không phải JSON cũng hiện dạng này | `» I'll read the file.` |
| `▸` | gọi công cụ | `▸ Read src/calc.py`, `▸ Edit src/calc.py`, `▸ Bash: python3 -m unittest`, `▸ Grep: def add in src`, `▸ Glob **/*.py in src` |
| `↳` | kết quả công cụ | `↳ ok · 3 lines` (Read/Grep/Glob: chỉ đếm), `↳ ok · OK` (Bash: dòng cuối của output), `↳ error: Exit code 1 · FAILED (failures=1)` |
| `■` | trạng thái cuối | `■ Done · 2 turns · 5.9s`, `■ Failed: Not logged in · Please run /login` |

- **An toàn nội dung**: chỉ đọc một tập trường ngắn cố định (`file_path`, `pattern`, `path`, `command`, `url`, `query`, `description`); không bao giờ đọc `content`/`old_string`/`new_string` của Write/Edit, `tool_use_result`, hay nội dung kết quả của Read/Grep/Glob. URL bỏ query và fragment. Đường dẫn bỏ tiền tố thư mục chạy (cả bí danh realpath, `/private/var` trên macOS, và dạng Windows). Che bí mật BẰNG `createRedactor` sẵn có của agent (token `sk-`, `gh*_`, `AKIA`, JWT, khối khoá riêng, mật khẩu trong URI, giá trị biến môi trường có tên TOKEN/SECRET/PASSWORD/API_KEY/... lấy từ môi trường agent + env của lượt chạy) TRƯỚC khi rút gọn, để không còn mảnh bí mật bị cắt dở. Chunk stderr cũng được che. Dòng dài rút gọn (chi tiết công cụ 160, văn bản 240, dòng thô 240 ký tự); dòng JSON > 8 MiB bị bỏ qua và ghi một dòng `· skipped an oversized agent event`; tổng dòng hoạt động tối đa 96 KiB mỗi lượt (task-service gửi lại TOÀN BỘ output cộng dồn mỗi lần nên phải nhỏ), vượt thì ghi một dòng `· activity limit reached...` và vẫn cho dòng `■` cuối đi qua.
- UTF-8 bị cắt giữa chunk xử lý bằng `StringDecoder`; không phụ thuộc TTY (chạy y hệt dưới `--stdio` qua SSH, `stdio: ['ignore','pipe','pipe']` như cũ).
- **Không đổi**: `handleAgentExecPromptStream` (`agent.execPromptStream`, khung `stream.chunk` cho bộ thực thi phức tạp/Engine 2) vẫn chạy `claude --print` text; không đổi giao thức RPC (không thêm feature flag).

### Việc cần làm (F2)

- [x] (a) Phát dòng hoạt động đọc được cho `agent.execPrompt.output` (Read/Edit/Bash/Grep/Glob/văn bản trợ lý, kết quả, trạng thái cuối).
- [x] (b) `stdout` cuối = nội dung `result` (+ `\n`), kiểm bằng test đơn vị, bộ phân tích khối kết quả, và lượt chạy thật (bên dưới).
- [x] (c) Che bí mật, cắt độ dài, không dán nội dung file (test riêng cho từng ý).
- [x] (d) Tool policy chỉ đọc giữ nguyên (`--permission-mode plan --tools Read,Glob,Grep`, kiểm đúng thứ tự cờ); định dạng bất ngờ / claude cũ / thăm dò hỏng hạ xuống text thô.
- [x] (e) Stub `fake_claude_cli/claude` nhận `--output-format stream-json --verbose` (từ chối thiếu `--verbose` như bản thật), phát `system init` + `assistant` + `result`, KHÔNG bịa sự kiện công cụ (stub không đọc file nào); `--help` quảng cáo cờ mới. AST-02 siết cho đúng hình thức mới: `argv[2:]` phải bằng đúng `--output-format stream-json --verbose --permission-mode plan --tools Read,Glob,Grep` và `outputFormat/verbose` ghi trong `calls.jsonl`. `request_service_contract.overlay.go.txt` gọi stub theo hình thức mới và lấy `result` (`verify-against-request-service.sh` xanh).
- [x] (f) vitest: bộ phân tích (tool_use/tool_result, text, lỗi, dòng không phải JSON, dòng rất dài, UTF-8 cắt giữa chunk, CRLF, dòng cuối không có `\n`, giới hạn tổng, che trước khi cắt), giữ nguyên `stdout`, che bí mật, cờ `streamJson`, thứ tự cờ ở chỉ-đọc/ghi, hạ cấp.
- [ ] Chạy cả luồng thật trên dev server với agent mới (cần build lại + khởi động lại agent: việc của người điều phối).

### Kiểm

| Lệnh | Kết quả |
|---|---|
| `cd agent && pnpm exec vitest run src/relay/agent-claude-stream-activity.test.ts src/relay/agent-print-mode-exec.test.ts src/relay/agent-readonly-tool-policy.test.ts src/relay/agent-compat-matrix.test.ts src/relay/agent-exec-prompt-options.test.ts src/relay/__tests__/agent-config.test.ts src/relay/__tests__/ai-complete-handler.test.ts` | 7 file, 113 test xanh (parser 20, print-mode 34 gồm 12 ca stream-json) |
| `python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py` | 23 test xanh (7 ca stream-json mới) |
| `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh` | `ok` (5 test Go dùng đường stream-json) |
| `tsc --noEmit -p agent/tsconfig.json --composite false` (grep tên file đụng) | không lỗi mới (còn `TS2322 ChangeReport` ở 2 dòng `let changes = undefined` CÓ SẴN của `agent-print-mode-exec.ts`) |
| `oxlint -c .oxlintrc.json` các file agent đụng | sạch, trừ `max-lines` của `agent-print-mode-exec.ts` (CÓ SẴN: 465 dòng đếm trước, 466 sau; phần mới tách ra 3 file riêng, không thêm disable) |
| Thực thi thật: driver esbuild gọi `handleAgentExecPrompt` với `claude` thật, chỉ-đọc, thư mục tạm có 1 file | notify: `· Session started (...)`, `» I'll read the file.`, `▸ Read a.txt`, `↳ ok · 2 lines`, `» The file ...`, `■ Done · 2 turns · 5.9s`; `stdout` = `The file `a.txt` contains a single line: "hello world file".\n`, exitCode 0 |
| Cùng driver với stub qua PATH | `stdout` = đúng JSON `answer` như chế độ text cũ |

### Blast radius (codegraph, rà tay)

`handleAgentExecPrompt` có 1 caller sản xuất (`dispatchAgentExecRpc`, method `agent.execPrompt`) + 2 file test. Người gọi phía Go (task-service `SimpleExecutor.runAgentPrompt`, request-service qua `task.execute`, contract executor) nhận cùng hình dạng kết quả; khác biệt nhìn thấy được: `agent.execPrompt.output` nay là dòng hoạt động có ký hiệu thay vì mảnh văn bản thô (task-service chỉ gom cộng dồn, không phân tích). `detectClaudeFlags` có 2 caller sản xuất (`handleAgentExecPrompt`, `handleAgentExecPromptStream`); `ClaudeFlagSupport` thêm trường `streamJson` (test của chính file cập nhật; `agent-compat-matrix.test.ts` mock không có trường này nên coi là false = chế độ text, vẫn xanh). `agent.execPromptStream` không đổi.

## F1: frontend

### Thiết kế

- `hooks/task-agent-output-store.ts`: MỘT luồng `task.activity.subscribe {taskId}` cho mỗi task, dùng chung giữa mọi view và giữ lại sau khi đóng sheet (tối đa 6 task rảnh, vượt thì đóng luồng cũ nhất). Lý do: client web chỉ huỷ phía local (`buildUnsubscribe` không có cho kênh này) và gateway chỉ dọn luồng khi socket đóng, nên mở sheet nhiều lần mà đăng ký mới mỗi lần sẽ dồn NATS subscription ở server (xem F7). Hệ quả tốt: mở lại sheet hiện ngay output gần nhất, kể cả phần đến khi sheet đóng. Frame mang TOÀN BỘ output cộng dồn của lượt chạy nên mỗi frame THAY thế, không nối; chỉ giữ 512 KiB đuôi. Rớt kênh (`onClose`, đã thêm tuỳ chọn cho `subscribeRuntimeStreamChannel`; trước đó hàm treo vô hạn nếu đóng trước ack) thì tự đăng ký lại với backoff 1s, 2s, 4s... tối đa 15s, tối đa 8 lần liên tiếp rồi báo "unavailable"; không đăng ký lại cho luồng không có người xem. `method_not_found`/`unsupported`/`forbidden` hoặc target desktop `local` (không có client push) → "unavailable", không thử lại.
- `hooks/useTaskAgentOutput.ts`: gắn store vào React, phân tích thành dòng (`agent-activity-lines.ts`: tách theo ký hiệu, cửa sổ 400 dòng mới nhất với `id` ổn định, tối đa 20 dòng stderr đặt sau stdout, dòng cắt 400 ký tự, che thêm các dạng bí mật còn sót: `sk-`, `gh*_`, `github_pat_`, `AKIA`, `Bearer`, JWT, khối khoá riêng).
- `components/task/agent-activity/TaskAgentActivityPanel.tsx`: đặt trong `TaskDetail` ngay dưới dòng "Status:", trên các tab (luôn thấy, thu gọn/mở được), chỉ cho task Engine 1 (`direct_agent` theo suy luận phía client của `ExecutionEngineBadge`); task Orchestration/Workflow không hiện. Mỗi dòng là `font-mono text-[11px]`, ký hiệu + văn bản (React text, không bao giờ HTML), màu theo token (`text-status-success` cho `■ Done`, `text-destructive` cho lỗi và stderr, `text-foreground` cho công cụ). `role="log" aria-live="polite"`, `scrollbar-sleek`, cao tối đa `max-h-56`. Cuộn: tự bám đáy khi người dùng ở trong 24 px cuối; cuộn lên đọc thì dừng bám, hiện nút "Jump to latest". Chỉ báo: huy hiệu "Live" (chấm nháy) khi task đang chạy (`isRunning` = đang dispatch hoặc `task.get` poll báo `in_progress`, đồng bộ với nút "Agent running…"), "Finished" khi có dòng và không còn chạy, "Idle" khi chưa có gì; "Reconnecting…" khi rớt kênh; "Older activity is not shown." khi đã cắt đầu.
- **Giới hạn đã nói rõ trên UI/ở đây**: backend KHÔNG lưu lịch sử. Mở sheet giữa chừng lúc agent đang chạy: nội dung hiện ở frame kế tiếp (tối đa 2 giây, vì mỗi frame mang cả lượt chạy), không phải ngay lập tức; tải lại trang sau khi chạy xong thì không còn gì (panel ghi "not kept after you reload the page"). Desktop Electron `local`: không hỗ trợ (ghi "needs a connected runtime environment"). Lịch sử bền cần F3 (backend) bên dưới.
- **F5**: `TaskDispatchStatusPanel` không hiện cho task thuộc Request (`!requestOwned`; chỉ task ngoài Request giữ thông báo Engine 2), và mọi chuỗi (cả chuỗi tiếng Việt cứng) qua i18n `auto.components.task.dispatch.*`. Comment lỗi thời trong `useTaskActivity.ts`/`TaskDetail.tsx` sửa cho đúng (poll `task.get` cho trạng thái; output là kênh push riêng).
- **F6**: `DevServerCard` có `data-testid="dev-server-card"` và `data-dev-server-id` (giữ `data-server-id` cũ).
- i18n: `auto.components.task.agentActivity.*` (11 khoá) và `auto.components.task.dispatch.*` (5 khoá) trong en/es/ja/ko/zh; test phủ khoá `i18n/task-agent-activity-locale-coverage.test.ts`.

### Selector cho spec live (agent N, `tests/stages/e2e/orca-flows/ui-live/**` không sửa ở task này)

| Selector | Ý nghĩa |
|---|---|
| `[data-testid=task-agent-activity]` (+ `data-state` = `running`/`finished`/`idle`) | panel; có mặt trong sheet của task Engine 1 (hàng `plan-task-row-<id>` mở sheet) |
| `[data-testid=task-agent-activity-live]` | huy hiệu "Live"; TỒN TẠI KHI VÀ CHỈ KHI task đang chạy (`isRunning`) |
| `[data-testid=task-agent-activity-status]` | huy hiệu khi KHÔNG chạy: `data-state` `finished` ("Finished") hoặc `idle` ("Idle") |
| `[data-testid=task-agent-activity-lines]` | vùng cuộn (`role=log`) |
| `[data-testid=task-agent-activity-line]` | mỗi dòng; `data-kind` = `info`/`text`/`tool`/`result`/`end`/`stderr`/`raw`, `data-tone` = `ok`/`error` (nếu có). `textContent` = ký hiệu + văn bản, ví dụ `▸Read src/calc.py`, `↳ok · 12 lines`, `■Done · 3 turns · 8.2s` |
| `task-agent-activity-empty`, `-older`, `-reconnecting`, `-jump`, `-toggle` | trạng thái rỗng / đã cắt đầu / rớt kênh / nút về cuối / thu gọn |
| `[data-testid=dev-server-card]` (+ `data-dev-server-id`) | thẻ dev server trong Settings → Dev Servers |
| `task-dispatch-none` | KHÔNG còn hiện cho task của Request |

Gợi ý phán quyết cho `live-agent-session-judge` (mã gap `AGENT-OUTPUT-NOT-STREAMED`, `UI-AGENT-OUTPUT-NOT-SHOWN`, `UI-NO-RUN-TRACE`): dấu vết đúng việc nay là dòng `[data-testid=task-agent-activity-line][data-kind=tool]` có `Edit src/calc.py` hoặc `Bash: python3 -m unittest`; frame `task.activity` nay có `stdout` gồm các dòng có ký hiệu (không còn là một đoạn câu trả lời cuối); cụm từ "28 ký tự của dòng cuối" nên lấy từ một dòng `»`/`▸`, không từ `■`.

### Việc cần làm (F1)

- [x] Hook + store `task.activity.subscribe`, đăng ký dùng chung, đăng ký lại an toàn, giới hạn bộ nhớ (cửa sổ 400 dòng, đuôi 512 KiB, 6 luồng rảnh).
- [x] Panel "Agent activity" trong sheet TaskDetail: thứ tự dòng, bám đáy có kiểm soát, chỉ báo đang chạy/đã xong/rớt kênh, không rò bí mật (che hai tầng), thu gọn.
- [x] Mở sheet giữa chừng (đồng bộ với `task.get` poll): Live badge theo `isRunning`; nội dung từ frame kế tiếp (không có lịch sử ở backend, đã nói rõ).
- [x] i18n 5 ngôn ngữ; data-testid ổn định.
- [x] F5 (i18n + ẩn "chưa có dispatch" cho task Request + comment), F6 (`dev-server-card`).
- [x] vitest (parser dòng 9, store/hook 16, panel 11, locale 5 ngôn ngữ, TaskDetail cho task Request/ngoài Request, DevServerCard, `subscribeRuntimeStreamChannel` onClose) + Playwright mock `request-web` (6 ca: luồng + Finished theo poll, không còn "no dispatch", mở giữa chừng + mở lại không đăng ký lần hai, server đóng luồng rồi tự nối lại, che bí mật + không chạy markup, 700 dòng chỉ giữ 400 + cuộn lên/Jump to latest).
- [ ] Kiểm với stack thật (cần build lại frontend + agent; agent N chạy `--real monitor`).
- [-] Lịch sử xem lại sau khi tải lại trang: cần backend (F3), không thuộc task này.

### Kiểm

| Lệnh | Kết quả |
|---|---|
| `cd frontend && NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts` (các file: `components/task/__tests__/{TaskDetail,TaskDetail-request-artifacts,TaskDispatchStatusPanel}.test.tsx`, `components/task/agent-activity`, `components/dev-server`, `runtime/runtime-rpc-client{,-stream}.test.ts`, `hooks/useTaskAgentOutput.test.ts`, `i18n`) | 38 file xanh; 1 lỗi CÓ SẴN `i18n/no-top-level-translate.test.ts` (`FleetServerStatusBadge.tsx`, file không đụng) và `TaskSourceBadge.test.tsx` không nạp được vì thiếu `jsdom` (CÓ SẴN) |
| `tsc --noEmit -p frontend/tsconfig.json --composite false` (grep file đụng) | không lỗi |
| `oxlint -c .oxlintrc.json` file đụng | sạch (TaskDetail.tsx vẫn dưới trần dòng) |
| `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web` | 41 passed, 1 skipped (CÓ SẴN); trong đó 6 ca mới. `test-results` ở gốc đã xoá |

### Blast radius

`subscribeRuntimeStreamChannel` (11 caller: agent orchestration, ephemeral VM, provider accounts...) chỉ thêm tham số tuỳ chọn thứ 5 và xử lý `onClose` (trước đó bị bỏ qua: nay đóng trước ack thì reject thay vì treo; sau ack thì gọi `options.onClose` nếu có; người gọi cũ không truyền nên không đổi). `TaskDetail` (1 caller `TaskPage`/sheet) thêm panel và bỏ dispatch panel cho task Request; test hiện có xanh. `TaskDispatchStatusPanel` chỉ còn 1 caller (`TaskDetail`). `DevServerCard` chỉ thêm thuộc tính. `mock-request-ws.ts` thêm `activity` và xử lý `task.activity.subscribe` (specs cũ không dùng).

## F3 (frontend): tab Result với `execution.get` (CONTRACT 2.8)

Gateway nay đăng ký `execution.get`: task Engine 1 (mọi task của Request) trả `{taskId, parseStatus:"missing", filesChanged:[], checksRun:[], stdoutTail, source:"last_execution_output", redacted?, truncated?}`; lượt hợp đồng trả `source:"execution_record"` kèm `attempt`, `status`, `summary`...; lỗi `TASK_NOT_FOUND` (UI đọc là `legacy`), `TASK_FORBIDDEN`, `TASK_UNAVAILABLE`.

- `ExecutionResult` thêm `source`, `redacted`, `truncated`; `parseExecutionResult` đọc cả ba (giá trị lạ bị bỏ). `TASK_UNAVAILABLE` được ánh xạ `unavailable` trong `request-errors.ts`.
- `ExecutionResultPanel`: khi `source = "last_execution_output"` hiện khung TRUNG TÍNH "Agent output" (`execution-agent-output`): `stdoutTail` là văn bản thuần trong `<pre>`, che thêm các dạng bí mật còn sót (cùng `maskSecretShapes` với panel hoạt động), giữ tối đa 200 dòng cuối, dòng cắt 400 ký tự, ghi chú "Secrets were hidden." / "Only the last part is shown." theo `redacted`/`truncated` (hoặc theo việc FE vừa cắt). Không còn cảnh báo đỏ `execution-malformed` cho trường hợp này, không có "Attempt", không có "Secret scan". Lượt hợp đồng (`execution_record`) giữ nguyên khung có cấu trúc và cảnh báo khi `parseStatus != ok`.
- Tab Result trong `TaskDetail`: chỉ hiện cho task của Request khi `execution.get` đã trả dữ liệu (`ready`) hoặc lỗi thật (`error`, kể cả `TASK_FORBIDDEN`/`TASK_UNAVAILABLE`); ẩn khi `legacy` (`TASK_NOT_FOUND`), `unsupported` (gateway cũ, target desktop `local`) và lúc tải lần đầu; nếu đang xem tab Result mà nó biến mất thì quay về Details. (Trước đây tab hiện cả khi `legacy`, với dòng "No output".)
- Test hợp đồng FE-E2E-001: bỏ `execution.get` khỏi `PENDING_GATEWAY_CHANNELS` (nếu còn là "chưa có" thì test "drops allowlist entries" đỏ), thêm ca kiểm `execution.get` là `unary` và `task.activity.subscribe`, `devServer.subscribeStatus` là `stream` trong `channel_inventory.json`.
- i18n: `auto.components.request.execution.agentOutput.{title,note,cut,redacted}` (5 ngôn ngữ).
- Test: `execution-agent-output.test.tsx` (7), `request-artifact-parsers.test.ts` (+1), `TaskDetail-request-artifacts.test.tsx` (+1: hiện cho last output, ẩn khi TASK_NOT_FOUND/không hỗ trợ), `request-rpc-contract.test.ts` (+1); Playwright mock `request-execution-output.web.e2e.ts` (4 ca: Agent output trung tính + che bí mật + ghi chú; lượt hợp đồng giữ khung cũ; ẩn tab khi `TASK_NOT_FOUND`; ẩn tab khi gateway thiếu kênh).

Selector: `execution-agent-output` (khung), `execution-agent-output-text` (nội dung), `execution-agent-output-empty` (rỗng); tab tên "Result" (`role=tab`) chỉ có khi có dữ liệu; `execution-result-panel` vẫn là khung có cấu trúc.

## F4 (frontend): dev server "bận"

`devServer.list`/`listForUser` thêm `activeExecutions` (số) và `busy`; `platform`/`arch`/`nodeVersion` được điền; kênh push `devServer.subscribeStatus` đẩy `devServer.statusChanged {devServerId, connected, activeExecutions, busy}` (ảnh chụp, tạm thời).

- `hooks/useDevServerStatusStream.ts`, gọi một lần ở gốc qua `useDevServersSync`: đăng ký `devServer.subscribeStatus`, rồi đọc `devServer.list` MỘT lần (sự kiện đã tới trong lúc đó thắng), sau đó sự kiện ghi đè vào store (`updateDevServerStatus(id, connected ? 'connected' : 'disconnected', {activeExecutions, busy})`). Rớt kênh: đăng ký lại với backoff 1s...15s, tối đa 8 lần liên tiếp, mỗi lần nối lại đọc `devServer.list` lại; hết lượt hoặc kênh không có (`method_not_found`/`unsupported`/`forbidden`) thì XOÁ `busy`/`activeExecutions` (không rõ tốt hơn sai cũ). Target desktop `local` bỏ qua êm (kiểm lại mỗi 2 giây tối đa 10 lần vì settings nạp sau khi app khởi động). Vòng poll `devServer.list` 3 giây có sẵn của bản web chỉ cập nhật status/platform/arch..., KHÔNG đụng `busy` nên không làm lùi sự kiện push.
- `DevServerCard`: khi đang `connected` và backend có báo `busy`: "Busy (N running)" (`dev-server-busy`, `data-busy`, `data-active-executions`) hoặc "Idle"; không hiện khi mất kết nối hoặc backend cũ (không có trường `busy`). Cạnh đó `dev-server-platform` = "macOS · arm64" khi handshake đã báo. `DevServer` thêm `activeExecutions?`/`busy?`; i18n `auto.components.devServer.card.{busy,idle}` (5 ngôn ngữ).
- Test: `useDevServerStatusStream.test.ts` (5), `DevServerCard.test.tsx` (+4), Playwright mock `dev-server-busy-state.web.e2e.ts` (3 ca: Idle → Busy (2 running) sau push → vẫn Busy sau lượt poll 3 giây → Idle → ẩn khi mất kết nối; server đóng luồng thì tự mở lại; gateway cũ không có badge). Mock `devServer.subscribeStatus` thêm vào `support/mock-request-ws.ts` (`backend.devServerStatus.push/end/openCount`).
- Vào cài đặt trong spec: Settings → mục "Dev Servers" (nút `Dev Servers` ở thanh bên, KHÔNG phải pane "servers" mặc định là "Remote Orca Servers").

Selector cho agent N: `[data-testid=dev-server-card][data-dev-server-id=<id>]`, `dev-server-busy` (`data-busy`, `data-active-executions`; text `Busy (N running)` / `Idle`), `dev-server-platform` (`macOS · arm64`).

## Lối vào nhanh tới panel hoạt động từ hàng task (mục 3)

Không thêm: hàng `plan-task-row-<id>` đã là nút mở sheet và panel nằm ngay trong sheet (không cần chuyển tab), thêm một lối nữa là trùng lặp.

## Phát hiện cho người điều phối (ngoài phạm vi)

- **F7 [backend api-gateway]** `task.activity.subscribe` chỉ được dọn khi socket đóng: client huỷ ở phía local (không có `task.activity.unsubscribe`), mỗi lần đăng ký là 7 đăng ký NATS ephemeral tồn tại đến hết kết nối WS. Frontend đã giảm thiểu bằng một luồng dùng chung mỗi task, nhưng nên có huỷ phía server (ví dụ method `*.unsubscribe` + `buildUnsubscribe` ở `web-preload-api`, hoặc dọn khi client ngừng nhận).
- **F8 [backend task-service]** `agent_output_partial` mang toàn bộ stdout/stderr cộng dồn mỗi 2 giây; agent đã tự chặn dòng hoạt động ở 96 KiB mỗi lượt nên an toàn, nhưng stderr thô không bị chặn ở agent (task-service nên cắt đuôi trước khi publish nếu lo giới hạn payload NATS).
- **F3 backend** nay có `execution.get` (stdout của lượt chạy thành công cuối); lượt chạy lỗi không để lại output ở đó (`lastError` nằm ở `backlog.tasks`). Panel hoạt động vẫn không có lịch sử sau khi tải lại trang; tab Result là chỗ xem lại output của lần chạy thành công.
- `agent.execPromptStream` (Engine 2) chưa có luồng hoạt động; nếu cần, dùng cùng `ClaudeStreamActivityParser` (đã tách riêng, không phụ thuộc handler).

## Việc người điều phối cần làm

1. Build lại agent và khởi động lại: `node agent/build.mjs` + `start-local-agent.sh` (agent đang chạy vẫn là bản text, panel sẽ chỉ thấy một đoạn cuối).
2. Dựng lại frontend của stack dev để có panel + `dev-server-card`.
3. Agent N: chạy `--real monitor` và cập nhật judge theo bảng selector trên.
