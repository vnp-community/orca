# SOL-QA-01: Terminal mở đúng thư mục của worktree

**Bug:** [BUG-QA-01](../BUG-QA-01-terminal-opens-in-home.md) · **Hướng chọn:** sửa ở cả ba lớp (phòng thủ chiều sâu), thứ tự A → B → C.

## A. Frontend gửi `cwd` (sửa chính, nhỏ, rủi ro thấp)

- `remote-runtime-pty-transport.ts`: lấy `cwd` từ `opts`; thêm vào tham số `terminal.create`: `...(cwd ? { cwd } : {})` (đúng kiểu `command`, `env` đang làm).
- Nguồn `cwd`: `deps.cwd` là đường dẫn worktree **theo máy chạy PTY** (dev server hoặc SSH), không phải máy trình duyệt. Không chuẩn hoá bằng `path.*` của trình duyệt; giữ nguyên chuỗi (AGENTS.md: hỗ trợ SSH, không giả định `/` hay `\`).
- Đường `createWebRuntimeSessionTerminal` (`web-runtime-session.ts:54,83`) đã chuyển `cwd`; thêm test để không thụt lùi.
- Terminal "floating"/setup (`FLOATING_TERMINAL_WORKTREE_ID`): không có worktree thật, giữ `$HOME` có chủ đích; ghi comment "why".

## B. Gateway tự suy `cwd` từ `worktree` khi thiếu (dự phòng cho client cũ, MCP, mobile)

- `terminalCreateArgs` thêm `Worktree string`. Khi `Cwd == ""` và `Worktree` có dạng `id:<repoId>::<path>` thì lấy `<path>`; dạng `id:<worktreeId>` thì hỏi git-gateway (`ListWorktrees`/`GetWorktree` theo id) lấy đường dẫn. Không tìm được thì để trống (không đoán).
- Không tin tuyệt đối đường dẫn từ client: agent vẫn kiểm (`validatePtyCwd`), nhưng gateway kiểm `cwd` nằm trong repo/worktree của tenant trước khi chuyển (chống dùng `terminal.create` mở thư mục tuỳ ý ngoài worktree; hiện đã có kiểm traversal ở agent, TM-002).

## C. Không lùi im lặng về `$HOME`

- Agent `pty.spawn` trả thêm `cwdFellBack: boolean` và `cwdRequested` khi `cwd` được yêu cầu nhưng không dùng được; infra-fleet và gateway chuyển tiếp vào kết quả `terminal.create` (`terminalCreateHandleView.cwdFellBack`).
- Frontend: khi `cwdFellBack` hiện một dòng nhắc trong terminal ("Directory <x> was not found on <host>; opened your home directory.") thay vì để người dùng tự phát hiện. Dòng nhắc thuần văn bản, qua `translate()` đủ năm locale.
- Tuỳ chọn cấu hình: `strictCwd=true` cho client muốn lỗi thay vì lùi.

## Kiểm thử cần có cùng bản sửa

| Tầng | Test |
|---|---|
| FE unit | `remote-runtime-pty-transport.test.ts`: `terminal.create` có `cwd` khi `opts.cwd` có; không có khi trống |
| FE unit | `pty-connection` test: remote runtime không bỏ `cwd` khỏi `transportOptions` |
| Gateway Go | `channels_terminal_test.go`: `worktree=id:<repo>::<path>` không `cwd` → `SpawnTerminalSessionRequest.Cwd == path`; `cwd` có sẵn thắng `worktree` |
| Agent | `pty-handler` test: `cwdFellBack` đúng khi thư mục không tồn tại |
| Acceptance | `suite_qa_regressions.py` QA-TERM-01..03 |
| UI live | `qa-regressions.web.e2e.ts` QA-UI-TERM-01: `pwd` trong terminal của worktree in đúng đường dẫn |

## Rủi ro

- Worktree id không có đường dẫn trong selector (ca worktree phụ): B cần tra cứu, tốn thêm một RPC; A đã đủ cho giao diện nên B là dự phòng, không chặn phát hành A.
- SSH: `cwd` là đường dẫn của máy xa; không kiểm tồn tại ở gateway (chỉ agent biết).
