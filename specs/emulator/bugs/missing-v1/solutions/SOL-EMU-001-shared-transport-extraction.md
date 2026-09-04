# SOL-EMU-001: Trích transport dùng chung giữa `agent/` và `emulator/`

**Resolves:** CR-DS-009 §2.2, Phase 1
**Service:** `agent/`, `emulator/`, (package mới) `packages/dev-agent-transport/`
**Status:** ✅ DONE — `packages/dev-agent-transport/` trích xong, verify xanh
100% (`agent/`'s test suite zero regression), xem
[TASK-EMU-001](../tasks/TASK-EMU-001-extract-dev-agent-transport-package.md)
để có bằng chứng chi tiết. `emulator-session.ts`/`emulator-connection-direct.ts`
nối vào package này — xem [TASK-EMU-006](../tasks/TASK-EMU-006-wire-protocol-transport-integration.md).
**Task(s):** [TASK-EMU-001](../tasks/TASK-EMU-001-extract-dev-agent-transport-package.md), [TASK-EMU-006](../tasks/TASK-EMU-006-wire-protocol-transport-integration.md)

## Vấn đề

Không phải để tách ranh giới domain — việc đó đã xong khi `emulator/` trở
thành workspace riêng (xem SOL-EMU-002/003). Vấn đề còn lại là **trùng lặp
code wire-protocol** (khung nhị phân 13-byte, seq/ack, keepalive, handshake)
nếu `emulator/` phải tự cài lại protocol này để nói chuyện thật với backend.
Không tách thì chỉ còn 2 lựa chọn đều tệ: (a) `emulator/` tự viết lại
protocol riêng — rủi ro lệch/không tương thích, phải tự đồng bộ tay mỗi khi
`agent/` sửa; (b) `emulator/` import thẳng `agent-session.ts` — nhưng file
đó hardcode `createRpcDispatcher` (~150 case git/fs/pty), nên làm vậy sẽ
**tái tạo đúng lỗ hổng bảo mật CR-DS-009 muốn triệt tiêu**: một request
`git.exec` route nhầm tới Mobile Emulator Agent sẽ thực thi thật.

## Giải pháp

Khảo sát `agent/src/main/ssh/relay-protocol.ts` (348 dòng — `encodeFrame`,
`FrameDecoder`, `parseJsonRpcMessage`, `MessageType`, error codes) và
`agent/src/relay/agent-wire.ts` (103 dòng — state seq/ack trên
`relay-protocol.ts`) cho thấy **2 file này đã là codec thuần**, không
hardcode dispatcher/domain logic, và đã được dùng chung trong `agent/` giữa
2 chế độ kết nối. Chỉ cần **move** 2 file này sang
`packages/dev-agent-transport/` — không cần refactor DI trên
`agent-session.ts` như kế hoạch ban đầu.

`agent-session.ts` (nơi thực sự gắn domain: dispatcher, capability
detection, PTY cleanup) **không trích, không dùng chung**. Mỗi agent tự viết
bản session/handshake orchestration riêng (của `emulator/` nhỏ hơn nhiều —
không capability-detection cho git/pty, không PTY cleanup), gọi vào
`packages/dev-agent-transport` để mã hoá/giải mã khung.

## Đã làm trong pass này

`git mv` 2 file thuần sang `packages/dev-agent-transport/`, cập nhật import
trong 15 file `agent/` sang package name `orca-dev-agent-transport` (chỉ
dòng import, không đụng dispatcher/domain logic ở `agent-session.ts`/
`agent-rpc-dispatch.ts`), `pnpm install` để tạo symlink workspace thật. Chạy
`agent/`'s test suite trước/sau move — **giống hệt nhau**: 3 test file lỗi
pre-existing không liên quan (esbuild resolve, 1 assertion pty-handler cũ, 1
test thiếu thư mục renderer), 336 file pass / 2 skip, 3816 test pass / 2 lỗi
pre-existing / 30 skip — zero regression. Chi tiết + lệnh thật xem
[TASK-EMU-001](../tasks/TASK-EMU-001-extract-dev-agent-transport-package.md).

Tiếp theo, `emulator/` đã nối vào package này thật (không còn stdio-debug-only):
`emulator-session.ts` (session/handshake nhỏ, chỉ `capabilities: ['device']`,
không dispatcher git/fs/pty) + `emulator-connection-direct.ts` (dial WS
outbound tới `ORCA_BACKEND_URL`) dùng
`createWireState`/`encodeDataFrame`/`decodeFrame`/`parseJsonPayload`/
`encodeKeepaliveFrame` từ `orca-dev-agent-transport`. `emulator-entry.ts`
chọn chế độ theo `ORCA_BACKEND_URL` có set hay không. Xem
[TASK-EMU-006](../tasks/TASK-EMU-006-wire-protocol-transport-integration.md).
