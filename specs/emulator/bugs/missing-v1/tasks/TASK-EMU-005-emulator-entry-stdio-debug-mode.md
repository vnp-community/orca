# TASK-EMU-005: `emulator-entry.ts` — chế độ stdio debug

**Solution:** [SOL-EMU-003](../solutions/SOL-EMU-003-device-capability-and-list-handlers.md)
**Priority:** P1
**Status:** `[x]` DONE — chỉ chế độ stdio debug; chế độ WS thật là TASK-EMU-006 (blocked on TASK-EMU-001)

## Việc đã làm

`emulator/src/relay/emulator-entry.ts`: đọc từng dòng stdin bằng
`node:readline`, parse JSON-RPC request, gọi
`createEmulatorRpcDispatcher(log).dispatch(...)`, ghi response ra một dòng
stdout. Log (bao gồm cảnh báo thiếu `ORCA_BACKEND_URL`) ghi ra stderr để
không lẫn vào output JSON-RPC trên stdout. `emulator-config.ts` đọc
`ORCA_BACKEND_URL`/`ORCA_AGENT_TOKEN`/`ORCA_EMULATOR_LOG_LEVEL`/
`ORCA_ANDROID_SDK_PATH`.

## Verify (chạy thật trong pass này)

Build ra `out/emulator.js` (esbuild, 10.9kb) rồi chạy thật:

```
$ printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"device.capabilities","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"device.list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"device.tap","params":{"x":1,"y":2}}' \
  | node out/emulator.js
{"jsonrpc":"2.0","id":1,"result":{"platform":"linux","android":{...},"ios":{...}}}
{"jsonrpc":"2.0","id":2,"result":{"devices":[]}}
{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"device.tap not implemented yet — ..."}}
```

Cả 3 request được xử lý đúng, binary chạy độc lập không cần backend-go hay
`agent/`. (Thứ tự dòng output không đảm bảo — mỗi request dispatch async độc
lập; caller khớp theo `id`, đúng ngữ nghĩa JSON-RPC 2.0.)
