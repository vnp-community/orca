# TDD-EMU-03: Tái sử dụng transport của `agent/` — phạm vi thật sau khảo sát code

CR-DS-009 giả định lớp "transport/lifecycle dùng chung" của `agent/src/relay/`
(`agent-wire.ts`, `agent-connection-*.ts`, `agent-session.ts`, …) là một khối
độc lập, có thể trích thẳng ra `packages/dev-agent-transport/` mà không đụng
domain logic. Khảo sát thật cho thấy điều đó đúng cho **một phần** của khối
này, không đúng cho phần còn lại — hai phần đó cần xử lý khác nhau.

## Phần THUẦN transport — an toàn để trích ngay (`relay-protocol.ts` + `agent-wire.ts`)

`agent/src/main/ssh/relay-protocol.ts` (348 dòng) là module codec khung nhị
phân đã tồn tại: `HEADER_LENGTH`, `MessageType`, `encodeFrame`,
`FrameDecoder`, `parseJsonRpcMessage`, `RelayErrorCode`/`JsonRpcErrorCode`.
`agent/src/relay/agent-wire.ts` (103 dòng) chỉ bọc thêm state seq/ack
(`createWireState`, `encodeDataFrame`, `decodeFrame`) lên trên. **Cả hai file
này đã được dùng chung** bên trong `agent/` bởi cả chế độ SSH relay lẫn chế
độ direct-websocket (`agent-connection-direct.ts` → `agent-wire.ts` →
`relay-protocol.ts`) — nghĩa là chúng vốn đã tách khỏi domain logic, không
hardcode dispatcher, không phụ thuộc git/pty (2 hằng số
`GIT_RESPONSE_STREAM_THRESHOLD`/`GIT_RESPONSE_CHUNK_SIZE` trong
`relay-protocol.ts` là ngoại lệ vô hại — hằng số thuần, không logic, `emulator/`
đơn giản không import chúng).

→ **Trích ngay 2 file này** sang `packages/dev-agent-transport/` là một
`git mv` + cập nhật import, KHÔNG cần refactor DI, KHÔNG đụng file nào khác
của `agent/`. Đây là phạm vi thật của TASK-EMU-001 (đã thu hẹp so với bản kế
hoạch gốc).

## Phần GẮN VỚI DOMAIN — không trích, mỗi agent tự viết bản của mình (`agent-session.ts`)

`agent-session.ts` không phải transport thuần — nó hardcode dispatcher
(`createRpcDispatcher` từ `agent-rpc-dispatch.ts`, ~150 case git/fs/pty), tự
xây capability list gắn với git/pty (`checkGitAvailable`, `checkPtyAvailable`),
và tự gọi cleanup PTY/fs-watch khi đóng session
(`cleanupAllPtys`/`notifyDaemonSessionClosed`/`cleanupAgentWatches`).

**Dùng nguyên `agent-session.ts` cho `emulator/` không chỉ lãng phí — nó tái
tạo đúng lỗ hổng bảo mật mà CR-DS-009 muốn triệt tiêu**: nếu backend lỡ route
một lệnh `git.exec` tới connection của Mobile Emulator Agent (chạy trên
laptop cá nhân của dev), dispatcher hardcode trong `agent-session.ts` sẽ thực
thi thật. Ranh giới domain phải giữ ở tầng session, không chỉ tầng gói/binary.

→ `emulator/` **không dùng `agent-session.ts`**. Nó tự viết một bản session
riêng, nhỏ hơn nhiều (không cần capability-detection cho git/pty, không cần
PTY cleanup), nhưng gọi vào `packages/dev-agent-transport` để mã hoá/giải mã
khung — nhờ vậy hai agent không lệch protocol theo thời gian mà vẫn giữ đúng
ranh giới bảo mật.

## Trạng thái hiện tại (trước khi TASK-EMU-001 làm bước trích ở trên)

`emulator/` **không import gì từ `agent/`** và chạy ở **chế độ stdio debug**
(đọc JSON-RPC request từ stdin, ghi response ra stdout) thay vì tự chế lại
giao thức khung nhị phân 13-byte + handshake chỉ dựa trên mô tả trong
`specs/agent/tdd/v5/02-wire-protocol.md`/`04-handshake-session.md` — tự cài
đặt lại một giao thức đã có, đã test kỹ, mà không có backend thật để kiểm
chứng có rủi ro tạo ra bản triển khai *trông giống* nhưng sai lệch tinh vi
(sai xử lý seq/ack, sai timing keepalive). Một khi TASK-EMU-001 trích xong
`relay-protocol.ts`/`agent-wire.ts`, `emulator/` chuyển sang dùng trực tiếp
bộ mã hoá khung thật đó thay vì tự viết lại (xem TASK-EMU-006).
