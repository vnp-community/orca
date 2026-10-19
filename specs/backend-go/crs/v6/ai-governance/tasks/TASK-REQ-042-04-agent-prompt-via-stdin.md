# TASK-REQ-042-04: Agent: prompt `agent.execPrompt` qua stdin, có công tắc quay lui

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `agent`
**File:** `agent/src/relay/agent-print-mode-exec.ts` (sửa), `agent/src/relay/agent-print-mode-exec.test.ts` (sửa/mới), `agent/src/relay/agent-claude-prompt-input.ts` + `agent-print-mode-args.ts` (mới, để file dưới 300 dòng), tài liệu `deploy/agent/README.md`
**Depends on:** Không
**Status:** [x] DONE (2026-10-19) - kiểm chứng trên macOS với `claude` 2.1.266 thật; **Windows, Linux và e2e giao diện chưa kiểm** (cổng đợt 4, xem Ghi chú triển khai)

---

## Context

- Hiện `args = ['--print', prompt]` và `stdio: ['ignore', ...]`; chỉ `claude` được hỗ trợ.
- `ai-complete-cli-session.ts` là mẫu stdin (`child.stdin.end`, `child.stdin.on('error')`).
- Giới hạn đối số: Linux ~128 KB mỗi đối số, Windows 32 KB cả dòng lệnh; argv hiện trong danh sách tiến trình.

## Việc cần làm

1. `args = ['--print']` rồi các cờ như cũ (`--output-format stream-json --verbose`, `--permission-mode`, `--allowedTools`, `--mcp-config`, ...); prompt không còn trong `args`.
2. `stdio: ['pipe','pipe','pipe']`; ngay sau `spawn`: `child.stdin.on('error', () => undefined)` rồi `child.stdin.end(initFile ? initFile + '\n' + prompt : prompt)` (dùng `write` có xử lý backpressure nếu prompt lớn).
3. Công tắc `ORCA_AGENT_PROMPT_VIA_ARGV=1`: dùng đường cũ; ghi một dòng log khi bật (không in prompt).
4. Tách phần dựng đầu vào/đối số ra file riêng nếu `agent-print-mode-exec.ts` (383 dòng) vượt `max-lines` sau thay đổi.
5. Thử với `claude` thật: tổ hợp `--print` + stdin + `stream-json` + `--permission-mode plan` + `--mcp-config` (chỉ-đọc kèm knowledge hub) và hồ sơ ghi; ghi kết quả vào PR. Nếu `claude` thật không đọc stdin ở tổ hợp nào thì **không** coi stdin là mặc định cho tổ hợp đó: giữ argv cho tổ hợp đó với lý do ghi trong mã, và báo lại.
6. Cập nhật `deploy/agent/README.md`: biến môi trường, hành vi mặc định mới.

## Kiểm thử

- Test spawn với `claude` giả đọc stdin: prompt thường, prompt 500 KB, prompt bắt đầu bằng `-`, prompt có ký tự điều khiển/UTF-8 nhiều byte, `EPIPE` khi tiến trình thoát sớm, công tắc quay lui (argv).
- Kiểm `ps`-like: chạy lệnh dò đối số của tiến trình con trong test và khẳng định không chứa nội dung prompt.
- Chạy test trên Linux và macOS; Windows: chạy trên runner Windows nếu có, nếu không ghi giới hạn và kiểm tay.
- `cd agent && npx vitest run src/relay/agent-print-mode-exec` và toàn bộ `src/relay`.
- e2e: `run_ui_flow.py --ai-stub` rồi `--real native-execute`.

## Tiêu chí hoàn thành

- [x] Mọi tổ hợp cờ đã dùng hoạt động với stdin hoặc có lý do ghi rõ vì sao còn argv.
- [x] Prompt 500 KB chạy được; không xuất hiện trong danh sách tiến trình.
- [x] Công tắc quay lui có test; log không chứa prompt.
- [x] `agent` build, lint và test xanh; backend không đổi. (build + lint + toàn bộ test liên quan xanh. Hai test đỏ không liên quan, `agent-rpc-dispatch-mcp.test.ts > advertises the feature` và `__tests__/agent-rpc-dispatch.test.ts > notification.send ... delivered:false`, **đã xác nhận đỏ sẵn trên HEAD sạch `f0ab5c1a9`** bằng worktree tạm, không do thay đổi này.)

## Rủi ro và lưu ý

- Bản `claude` quá cũ có thể không đọc stdin ở `--print`: đã có thăm dò cờ (`detectClaudeFlags`); nếu cần, thêm thăm dò và rơi về argv có cảnh báo.
- Sau khi build agent mới phải thay cả pty-daemon? Không: đây là đường `agent.execPrompt` của tiến trình agent, không phải pty-daemon; nhưng khởi động lại agent làm gián đoạn lượt chạy đang có, báo trước khi triển khai.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Ngày:** 2026-10-19. Trạng thái: PARTIAL. Mã cho `agent.execPrompt` xong và đã kiểm chứng với `claude` thật; Windows/Linux, e2e và 2 test đỏ không liên quan ghi ở cuối.

### File đã đổi (chỉ trong `agent/` + README + task này)

- `agent/src/relay/agent-claude-prompt-input.ts` (mới): `promptViaArgvEnabled` (đọc `ORCA_AGENT_PROMPT_VIA_ARGV` = `1`/`true` từ env của chính tiến trình agent, **không** từ `env` của request), `composePromptInput` (`initFile + '\n' + prompt`, không đổi), `resolvePromptDelivery` (log đúng một dòng khi bật công tắc, không có nội dung prompt), `printModeLeadArgs`, `writePromptToStdin` (ghi theo khối 64 KB, chờ `drain`, `stdin.on('error')` nuốt `EPIPE`, `end()` ở cuối).
- `agent/src/relay/agent-print-mode-args.ts` (mới): tách phần dựng đối số (`--print`, `stream-json`, hồ sơ quyền, YOLO, chỉ-đọc) ra khỏi handler; hành vi giữ nguyên. Cần vì `agent-print-mode-exec.ts` đã chạm trần `max-lines` 300 (oxlint báo 313 sau khi thêm mã stdin; còn dưới 300 sau khi tách; không dùng disable).
- `agent/src/relay/agent-print-mode-exec.ts`: `args = ['--print', ...cờ]` (không còn prompt), `stdio: ['pipe','pipe','pipe']`, ghi prompt vào stdin ngay sau `spawn`; công tắc argv dùng `stdio: ['ignore',...]` và đường cũ. Cờ knowledge hub (`--mcp-config`, `--strict-mcp-config`, `--allowedTools`, `--disallowedTools`) vẫn là đối số riêng, không đổi. `AGENT_FEATURES` không đổi.
- Test: `agent-print-mode-exec.test.ts` và `agent-print-mode-exec-knowledge-hub.test.ts` (cập nhật kỳ vọng argv sang `['--print', ...]`, fake child có `stdin`), `agent-print-mode-exec-stdin.test.ts` (mới, 21 test, spawn thật một `claude` giả).
- `deploy/agent/README.md`: mục "Prompt của `agent.execPrompt` đi qua stdin (CR-REQ-042)" và bảng biến `ORCA_AGENT_PROMPT_VIA_ARGV`.

### Kiểm chứng với `claude` thật

`claude` 2.1.266 (`~/.local/bin/claude`), đã đăng nhập. Mỗi lệnh dưới đây là `printf '%s' "<prompt>" | claude --print <cờ>` trong thư mục tạm; kết quả là `exit=0`, `result.subtype=success`, `is_error=false`.

| Tổ hợp | Cờ (sau `--print`) | Kết quả |
|---|---|---|
| thường | (không) | `OK` |
| stream-json | `--output-format stream-json --verbose` | sự kiện `system/init` ... `result: OK` |
| chỉ-đọc | `--permission-mode plan --tools Read,Glob,Grep` (prompt: đọc `hello.txt`) | trả đúng `hello-from-file` (công cụ Read chạy) |
| hồ sơ ghi `openspec-author` | `--output-format stream-json --verbose --permission-mode acceptEdits --tools Read,Glob,Grep,Write,Edit,Bash --allowedTools 'Read,Glob,Grep,Write,Edit,Bash(openspec:*),Bash(tdd-agent -C * status:*)'` | file `created.txt` được tạo với nội dung `DONE`, `result` bắt đầu bằng `OK` |
| YOLO + prompt bắt đầu bằng `--` | `--output-format stream-json --verbose --dangerously-skip-permissions` (prompt `--dangerously-skip-permissions-looking prompt: reply ...`) | `OK`, không bị hiểu là cờ |
| prompt lớn | `claude --print < file` (file 400 055 byte) | `OK` |
| chỉ-đọc + knowledge hub (qua mã thật) | `ORCA_LIVE_CLAUDE=1 ORCA_LIVE_KNOWLEDGE_ROOT=<vnp-qa-platform> npx vitest run src/relay/agent-print-mode-exec-knowledge-hub.live.test.ts` | 2/2 pass: chạy `handleAgentExecPrompt` thật, `stream-json` + `--permission-mode plan` + `--mcp-config ... --strict-mcp-config` + `--allowedTools/--disallowedTools` qua stdin; `kg_search` trả `FR-SYS-07`, cấu hình MCP tạm bị xoá |

Kết luận: mọi tổ hợp cờ agent đang dùng đọc được prompt từ stdin; **không** có tổ hợp nào phải giữ argv. Không cần thêm thăm dò cờ: đọc stdin ở `--print` là hành vi cơ bản của CLI, không phải cờ mới. Không thử `--bare` (không dùng ở đường này).

### Kiểm thử đơn vị

- `cd agent && npx vitest run src/relay/agent-print-mode-exec src/relay/agent-compat-matrix`: 4 file pass, 72 pass, 2 skipped (live).
- File mới `agent-print-mode-exec-stdin.test.ts` (21 test) spawn thật một script `claude` giả (shebang `process.execPath`): prompt thường; `initFile + '\n' + prompt`; 500 KB (hash SHA-256 khớp, `argv == ['--print']`, `ps -o args=` của tiến trình con không chứa nội dung prompt); prompt bắt đầu bằng `-`; UTF-8 nhiều byte vượt nhiều khối 64 KB (ranh giới khối rơi giữa ký tự); NUL/ESC/CRLF; `EPIPE` (con thoát ngay, prompt 4 MB: trả `exitCode 7`, không có `uncaughtException`/`unhandledRejection`; đã chạy thử đột biến bằng cách bỏ `stdin.on('error')` thì test đỏ với `write EPIPE`); công tắc bật (argv `['--print', 'CTX\nprompt']`, stdin rỗng, đúng một dòng log, log không chứa prompt) và tắt; `env` của request không bật được công tắc; `writePromptToStdin` với stream chậm (backpressure), stream đã huỷ, lỗi giữa chừng. Nhóm spawn thật bỏ qua trên Windows (script giả dùng shebang và `ps`).
- Quét theo file các test liên quan trong `src/relay` (`agent-*`, `__tests__/agent-*`, `ai-complete*`): đều xanh, trừ 2 test **không liên quan** tới thay đổi này và fail ở cây làm việc hiện tại: `agent-rpc-dispatch-mcp.test.ts > advertises the feature` (thiếu `mcp.callTool` trong feature) và `__tests__/agent-rpc-dispatch.test.ts > notification.send ... delivered:false` (phụ thuộc thông báo OS của máy chạy). Không file nào trong hai test đó thuộc file tôi đã sửa.
- `npx vitest run src/relay` toàn thư mục trong một lần bị tiến trình vitest chết đột ngột (exit 144) sau cảnh báo SQLite, nên đã chạy theo từng file. Chưa xác định test nào gây ra (không nằm trong các file đã sửa).
- `oxlint` sạch trên 6 file đã sửa/mới (gồm `max-lines`); `node build.mjs` ra `agent/out/agent.js` thành công. `tsc --noEmit` có 2 lỗi nằm sẵn trong file đã sửa nhưng không do thay đổi này (`ChangeReport` ở phần `changes`, kiểu `AgentConfig` thiếu `orcaHttpUrl/apiSecret` trong test cũ); không có lỗi mới.

### Blast radius (không có GitNexus: dò bằng grep)

- `handleAgentExecPrompt`: gọi từ `agent-rpc-dispatch-agent-exec.ts` (`agent.execPrompt`); test: `agent-print-mode-exec*.test.ts`, `agent-compat-matrix.test.ts`, `__tests__/agent-rpc-dispatch.test.ts` (chỉ kiểm định tuyến). Mọi caller backend chỉ thấy cùng JSON-RPC; backend không đổi.
- Ký hiệu mới (`buildPrintModeArgs`, `resolvePromptDelivery`, `writePromptToStdin`, `printModeLeadArgs`) chỉ được dùng trong `agent-print-mode-exec.ts`.
- Không đổi: `agent-exec-prompt-stream.ts` (`agent.execPromptStream`, không có caller backend nào gửi), `agent-tool-registry.ts` (`claude_code` tool), `ai-complete-cli-session.ts`.

### Lệch so với task / lưu ý

- Thêm `agent-print-mode-args.ts` (không có trong danh sách ban đầu) vì `max-lines`.
- `agent.execPromptStream` vẫn truyền prompt qua argv (ngoài phạm vi task; tài liệu README ghi rõ). Nên làm riêng nếu có caller.
- Công tắc nhận cả `true` ngoài `1`.
- Triển khai: khởi động lại agent làm gián đoạn lượt chạy đang có; chưa khởi động lại agent local nào trong lượt này.

### Chưa kiểm chứng

- Windows (chạy test spawn trên runner Windows, thực tế `claude.cmd`): chưa; test spawn thật bị bỏ qua trên Windows. Với `claude.cmd` qua shell, cần kiểm tay việc stdin được chuyển tiếp.
- Linux: chưa chạy (chỉ macOS/Darwin); cách dò `ps -o args=` dùng được trên cả hai nhưng chưa chạy trên Linux.
- e2e `run_ui_flow.py --ai-stub` và `--real native-execute`: không chạy (ngoài phạm vi lượt này, cấm đụng stack).
- `agent-print-mode-exec-knowledge-hub.live.test.ts` chỉ chạy với kho `vnp-qa-platform` cục bộ.

### Kiểm chứng của người điều phối (2026-10-19)

- Chạy lại `npx vitest run src/relay/agent-print-mode-exec src/relay/agent-compat-matrix`: 72 passed, 2 skipped (live). `oxlint` sạch trên ba file chính.
- Hai test đỏ ngoài phạm vi được chạy trên worktree tạm của HEAD `f0ab5c1a9` (chưa có thay đổi này): cả hai đỏ như cũ, nên là lỗi có sẵn.
- Còn phải kiểm ở đợt 4: chạy trên Linux và Windows (CI hoặc máy thật), `run_ui_flow.py --ai-stub` và `--real native-execute` sau khi khởi động lại agent local.
