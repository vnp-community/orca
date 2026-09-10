# TDD-EMU-04: Deployment & Chế độ chạy

## Build

```
cd emulator && node build.mjs        # → out/emulator.js (esbuild CJS bundle, node22)
```

## Chế độ chạy hiện tại: stdio debug (TASK-EMU-005)

Cho tới khi TASK-EMU-001/006 xong (transport thật nối `backend-go`),
`emulator/` chạy ở chế độ đọc/ghi qua stdio — mỗi dòng stdin là một JSON-RPC
2.0 request, mỗi response ghi ra một dòng stdout:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"device.capabilities","params":{}}' | node out/emulator.js
```

Chế độ này dùng để:
- Test thủ công/CI logic `device.*` mà không cần backend-go/infra-fleet-service chạy thật
- Là nền tảng cho `emulator-entry.ts` cắm transport thật vào sau (chỉ cần thay nguồn đọc/ghi từ stdio sang WS frame, dispatcher giữ nguyên)

## Chế độ chạy mục tiêu (sau TASK-EMU-001/006): direct-websocket

Giống `agent/`'s `direct-websocket` mode (`agent-connection-direct.ts`) —
phù hợp nhất cho máy cá nhân (Mac/Windows) đứng sau NAT: agent tự dial ra
ngoài, không cần mở port vào.

## Biến môi trường (mục tiêu, khớp quy ước `agent/`)

| Variable | Mô tả |
|---|---|
| `ORCA_BACKEND_URL` | Gateway WebSocket URL |
| `ORCA_AGENT_TOKEN` | Token đăng ký (từ luồng pairing F28, `--kind=mobile-emulator`) |
| `ORCA_AGENT_KIND` | `mobile-emulator` — cố định cho package này |
| `ORCA_EMULATOR_LOG_LEVEL` | debug/info/warn/error |
| `ORCA_ANDROID_SDK_PATH` | Override đường dẫn Android SDK (nếu không tự dò được) |
