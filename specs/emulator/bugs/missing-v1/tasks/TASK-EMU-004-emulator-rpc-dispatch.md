# TASK-EMU-004: `emulator-rpc-dispatch.ts` — dispatch table

**Solution:** [SOL-EMU-003](../solutions/SOL-EMU-003-device-capability-and-list-handlers.md)
**Priority:** P1
**Status:** `[x]` DONE (control ops honest-stub, chờ TASK-EMU-010)

## Việc đã làm

`emulator/src/relay/emulator-rpc-dispatch.ts`: `createEmulatorRpcDispatcher(log)`
đăng ký đủ 9 method trong catalog (TDD-EMU-02) —
`device.capabilities`/`device.list`/`device.availability` gọi handler thật;
`device.attach/tap/gesture/button/rotate/shutdown` gọi `device-control-handler.ts`
(luôn throw `DeviceMethodNotImplementedError`, dispatch bắt lỗi này và trả
JSON-RPC `-32601` kèm đường dẫn TASK-EMU-010 — không phải throw không rõ
nguyên nhân). Method lạ cũng trả `-32601` chuẩn JSON-RPC 2.0.

## Verify (chạy thật trong pass này)

`emulator-rpc-dispatch.test.ts` (4 test, nằm trong 13 test pass ở
TASK-EMU-003) xác nhận: `device.capabilities`/`device.list` trả kết quả thật
không lỗi; `device.tap` trả đúng code `-32601` và message chứa
"TASK-EMU-010"; method lạ (`device.bogus`) cũng trả `-32601`.
