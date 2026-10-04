# BUG-AG-ORCH-014 — Phiên làm việc (terminal/agent PTY) trên worktree bị ngắt khi terminal disconnect nhanh / khi thoát app, dù grace-period 120s đã được implement

**Mức độ:** 🟡 Medium (hedged — xem "Chưa confirm" bên dưới)
**Status:** 🟡 Root cause CHƯA confirm bằng live repro — ghi nhận theo yêu cầu "log bug trước" của phiên làm việc, đúng tinh thần BUG-012's cách xử lý (hedge rõ ràng thay vì đoán).
**Module:** `agent/src/relay/agent-session.ts`, `agent-pty-registry.ts`, `pty-agent-bridge.ts`; frontend `WorkspaceTerminalPanel.tsx`/terminal-pane
**Phát hiện:** 2026-09-15, user report trực tiếp: *"nhánh worktree đang làm việc đang là sync theo terminal => nhanh bị ngắt và khi out app bị ngắt"*.

---

## User report

Khi đang làm việc trên 1 worktree (agent/terminal đang chạy), nếu:
1. Terminal bị ngắt kết nối (network blip, tab reload…) → công việc **bị ngắt nhanh**, không tiếp tục được.
2. Thoát app (đóng Orca) → công việc **cũng bị ngắt**.

Kỳ vọng: reconnect lại thì công việc tiếp tục đúng chỗ cũ, không mất.

## Đã confirm bằng đọc source — có 1 cơ chế grace-period ĐÃ TỒN TẠI để giải quyết ĐÚNG vấn đề này

`agent-session.ts:235-244` (comment nguyên văn):
> "CR-STORAGE-008(b) (2026-09-07): agent-spawned (agent.spawn) PTYs used to be killed immediately here (ORCH-011)... That meant a 1s network blip killed every running AI-agent CLI session. **Now mirrors pty.create terminals' own grace-period behavior instead**: arm a timer per PTY, cancelled by rebindAgentSpawnConnection() on the next successful reconnect."

Cơ chế thật (`agent-pty-registry.ts`):
- `AGENT_SPAWN_PTY_GRACE_PERIOD_MS = 120_000` (**120 giây**).
- Khi WS đóng (`ws.on('close', ...)`, **không phân biệt** close code — kể cả đóng "sạch" khi thoát app đàng hoàng), `stop()` gọi `scheduleAgentSpawnGracePeriod(log)` — set timer 120s cho mỗi PTY đang chạy, **không kill ngay**.
- Nếu reconnect trong 120s (`rebindAgentSpawnConnection()`) → huỷ timer, PTY sống tiếp, không mất việc.
- Chỉ khi hết 120s thật sự mới `kill('SIGTERM')`.
- `pty.create` (terminal thường, không phải AI-agent CLI) có cơ chế **tương tự, độc lập** trong `pty-agent-bridge.ts` (`PTY_GRACE_PERIOD_MS`, cũng 120s) — sống trong 1 **daemon process riêng** (`pty-daemon-server.ts`), tách khỏi agent process, để **sống sót cả khi agent process tự restart** (deploy, crash), không chỉ khi WS rớt.

→ **Về mặt thiết kế, cả 2 loại PTY (terminal thường + AI-agent CLI) đều đã có cơ chế chống mất việc khi disconnect ngắn hạn (≤120s).** Nếu user thấy "ngắt nhanh" thật (không phải chờ đủ 2 phút), điều đó **mâu thuẫn với code hiện tại** — nghĩa là có 1 trong các khả năng sau, CHƯA xác định được khả năng nào đúng:

## Chưa confirm — 3 giả thuyết, cần live repro để phân biệt

1. **UI báo "ngắt" sớm hơn thực tế** — frontend có thể hiển thị trạng thái "disconnected"/dừng ngay khi WS rớt, dù backend vẫn giữ PTY sống trong 120s grace window phía sau — cùng họ vấn đề với saga `BUG-FE-PTY-001` (SSH_SESSION_EXPIRED) đã ghi nhận trước đây (RESOLVED lần đó, nhưng có thể đây là biến thể mới/khác chỗ).
2. **Reconnect không match đúng `ptyId` cũ** — `rebindAgentSpawnConnection()` cần đúng id để huỷ timer; nếu phiên reconnect (sau khi mở lại app) tạo session/id mới thay vì gọi đúng cơ chế reattach, PTY cũ vẫn "sống" trong 120s theo lý thuyết nhưng **không ai nối lại được vào nó** → với người dùng, cảm giác y hệt "bị ngắt", dù kỹ thuật khác nhau.
3. **`out app` (thoát hẳn) có thể không đi qua đường tái sử dụng grace-period đúng cách** — ví dụ nếu app đóng làm rớt kết nối tới **nhiều dev server cùng lúc** hoặc theo cách khác với 1 network blip đơn thuần, cần xác nhận có đúng 1 `ws.on('close')` event bắn ra theo đúng luồng đã đọc ở trên hay không.

## Đề xuất bước xác minh (giải pháp chẩn đoán, chưa phải code fix)

Theo đúng phương pháp đã dùng hiệu quả cho BUG-009/BUG-010 trong phiên này (log-cause trước, đừng đoán): lần tới khi tái hiện được, đối chiếu **đúng theo thời gian thực**:
1. Log phía agent (`journalctl -u orca-agent-<host>` hoặc file log agent) — tìm dòng `"Session closed code=..."` (agent-session.ts:217) và `"scheduleAgentSpawnGracePeriod: armed grace timers..."` — xác nhận grace timer THẬT SỰ được arm đúng lúc disconnect.
2. Nếu reconnect trong 120s: tìm log `rebindAgentSpawnConnection` — xác nhận nó match đúng ptyId cũ, không tạo session mới.
3. Đối chiếu với UI: tại đúng thời điểm đó, frontend có hiển thị "đã ngắt/mất việc" hay không — nếu UI báo ngắt NGAY LẬP TỨC (trước khi hết 120s) trong khi log backend cho thấy PTY vẫn sống → xác nhận giả thuyết #1 (UI bug, không phải backend bug).

## Việc phụ phát hiện: tài liệu spec bị lỗi thời (stale)

`specs/agent/crs/v3/storage/solutions/SOL-AG-STORAGE-003-agent-spawn-pty-daemon-grace-period.md` đang ghi trạng thái **"🔲 Designed — chưa implement"** — nhưng code thật (`agent-session.ts`/`agent-pty-registry.ts`) cho thấy **đã implement xong** (comment ghi ngày 2026-09-07, code gọi `scheduleAgentSpawnGracePeriod` thật). Cần cập nhật lại trạng thái doc này cho khớp thực tế — việc nhỏ, không liên quan trực tiếp bug chính nhưng gây hiểu nhầm nếu ai đó đọc theo TDD/SOL mà không đối chiếu code thật (đúng bài học "phải verify bằng source thật" mà chính bug này áp dụng).

## Liên quan

- [BUG-AG-ORCH-011](./BUG-AG-ORCH-011-pty-registry-orphaned-on-ws-disconnect.md) — bug gốc dẫn tới việc PTY bị kill ngay khi disconnect (fix ban đầu ưu tiên tránh leak resource, đánh đổi mất khả năng resume) — bug này (014) là hệ quả/tiếp nối của đúng câu chuyện đó, sau khi CR-STORAGE-008(b) đã cố gắng cân bằng lại.
- Memory: "BUG-FE-PTY-001 investigation — SSH_SESSION_EXPIRED terminal saga: RESOLVED" (phiên trước) — rất có thể cùng họ vấn đề (UI báo ngắt sớm), cần đối chiếu khi có log thật.
