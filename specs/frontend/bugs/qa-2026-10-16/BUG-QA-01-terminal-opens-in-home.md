# BUG-QA-01: Terminal của worktree luôn mở ở thư mục `$HOME`

**Mức độ:** P1 (mọi terminal trên web mở sai chỗ; lệnh git/build chạy nhầm thư mục) · **Khu vực:** frontend (web runtime) + api-gateway + agent · **Trạng thái:** đã sửa 2026-10-16 (FE gửi cwd, gateway lấy cwd từ selector, agent báo `cwdFellBack`); QA-TERM-01..03 và QA-UI-TERM-01 xanh; spec UI phải chạy với agent thật, không dùng `--ai-stub`

## Triệu chứng

Chọn một worktree ở thanh bên rồi mở terminal: dấu nhắc hiện `~` và `pwd` trả `/Users/<user>`, không phải thư mục của worktree (primary hoặc worktree phụ).

## Tái hiện (đã chạy 2026-10-16 trên stack local, web `localhost:28080`)

1. Mở web, bấm worktree `claude/tms-bug-triage-report` (primary của `vnp-qa-platform`): terminal mở ở `~`.
2. Bấm worktree phụ `task/0a03c74e-...`: cũng `~`.
3. Cơ sở dữ liệu: `infra.terminal_sessions.cwd` của 12 phiên gần nhất đều là `/Users/datbeohbbh`.
4. Bắt khung WebSocket của trang (`WebSocket.prototype.send`), lệnh `terminal.create` do giao diện gửi **không có trường `cwd`**:
   `{"method":"terminal.create","params":{"worktree":"id:<worktreeId>[::<đường dẫn>]","connectionId":"<devServerId>","env":{...},"tabId":...,"leafId":...}}`
5. Đối chứng: gọi thẳng `terminal.create` với `cwd` tồn tại trả về đúng `cwd` đó; với `cwd` rỗng hoặc không tồn tại trả `/Users/<user>`. Tức là backend làm đúng việc được yêu cầu, chỉ là không ai gửi `cwd`.

## Nguyên nhân gốc (ba lớp)

1. **Frontend làm rơi `cwd`.** `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts:3181` dựng `transportOptions` có `cwd: deps.cwd` (= `tab.startupCwd ?? workspace.path`, `Terminal.tsx:2229`), nhưng `createRemoteRuntimePtyTransport` (`remote-runtime-pty-transport.ts:145-173`) không lấy `cwd` ra khỏi `opts`, và lời gọi `terminal.create` (`:888`) không gửi nó. Chỉ có `worktree` (selector) và `connectionId`.
2. **Gateway bỏ qua `worktree`.** `channels_terminal.go` (`terminalCreateArgs`) chỉ đọc `connectionId`, `cwd`, `shell`, `cols`, `rows`; comment ở transport nói rõ "chỉ đọc connectionId, không đọc worktree". Không có chỗ nào đổi selector worktree thành đường dẫn.
3. **Agent im lặng lùi về `$HOME`.** `agent/src/relay/pty-handler.ts` `validatePtyCwd` trả `homedir()` khi `cwd` rỗng, không tồn tại hay không phải thư mục, và `terminal.create` không báo là đã lùi. Vì vậy một `cwd` sai (ví dụ đường dẫn của host khác, nhất là qua SSH) cũng cho kết quả giống hệt lỗi này mà không ai biết.

## Ảnh hưởng

- Web và mọi môi trường dev server/SSH; desktop Electron không bị vì IPC `pty.spawn` có `cwd`.
- Terminal chạy agent (`launchAgent`) và lệnh khởi động (`command`) cũng chạy sai thư mục.
- Phiên gắn lại (`reattach`) giữ `cwd` ban đầu nên cũng sai.

## Giải pháp và task

Xem [solutions/SOL-QA-01-terminal-cwd.md](./solutions/SOL-QA-01-terminal-cwd.md) và [tasks/README.md](./tasks/README.md) (T-QA-01-xx). Test: `suite_qa_regressions.py` (QA-TERM-01..03), `ui-live/qa-regressions.web.e2e.ts` (QA-UI-TERM-01), checklist [qa/QA-CHECKLIST.md](./qa/QA-CHECKLIST.md) mục 1.
