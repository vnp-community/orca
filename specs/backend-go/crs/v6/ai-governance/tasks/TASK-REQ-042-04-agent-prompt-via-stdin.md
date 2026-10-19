# TASK-REQ-042-04: Agent: prompt `agent.execPrompt` qua stdin, có công tắc quay lui

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `agent`
**File:** `agent/src/relay/agent-print-mode-exec.ts` (sửa), `agent/src/relay/agent-print-mode-exec.test.ts` (sửa/mới), có thể `agent/src/relay/agent-claude-prompt-input.ts` (mới, nếu cần để file dưới 300 dòng), tài liệu `deploy/agent/README.md`
**Depends on:** Không
**Status:** [ ] TODO

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

- [ ] Mọi tổ hợp cờ đã dùng hoạt động với stdin hoặc có lý do ghi rõ vì sao còn argv.
- [ ] Prompt 500 KB chạy được; không xuất hiện trong danh sách tiến trình.
- [ ] Công tắc quay lui có test; log không chứa prompt.
- [ ] `agent` build, lint và test xanh; backend không đổi.

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
