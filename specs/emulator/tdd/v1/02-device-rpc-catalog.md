# TDD-EMU-02: `device.*` RPC Catalog

Method names và field JSON phải khớp đúng với những gì
`backend-go/services/infra-fleet-service/internal/usecase/emulator_relay.go`
và `get_host_capabilities.go` đã relay/decode sẵn — không cần đổi gì phía
backend-go khi các method này được cài đặt thật (xem doc comment của 2 file
đó: *"The moment agent/ adds a device.* handler, these calls start working
with zero further backend-go changes"*).

| Method | Params | Result | Nguồn quyết định shape |
|---|---|---|---|
| `device.capabilities` | `{}` | `{ platform, android: { sdkFound, sdkPath?, message }, ios: { simctlOk, serveSimOk, message? } }` | tự định nghĩa mới — dùng cho setup guide, không phải field bắt buộc của `emulator_relay.go` |
| `device.list` | `{}` | `{ devices: [{ id, name, platform, state }] }` | `decodeEmulatorDevices` trong `emulator_relay.go:168-187` |
| `device.availability` | `{}` | `{ available: boolean, reason?: string }` | `EmulatorRelay.GetAvailability` trong `emulator_relay.go:114-139` |
| `device.attach` | `{ deviceId: string }` | `{ sessionId, deviceId, platform }` | `decodeEmulatorSession` trong `emulator_relay.go:189-196` |
| `device.tap` | `{ sessionId?, deviceId?, x, y }` | `{}` (fire-and-forget) | `SendCommand` trong `emulator_relay.go:159-166`, params xây bởi `server_emulator_host.go`'s `SendEmulatorTap` từ `SendEmulatorTapRequest` (`int32 x, y` — pixel thiết bị thật, không phải 0..1 chuẩn hoá) |
| `device.gesture` | `{ sessionId?, deviceId?, startX, startY, endX, endY, durationMs? }` | `{}` | **Sửa (2026-09-03, TASK-EMU-010):** bảng gốc ghi `points: [{x,y}]` — sai, không khớp code thật. `server_emulator_host.go`'s `SendEmulatorGesture` build params từ `SendEmulatorGestureRequest` (`infrafleet.proto`), toàn `int32`, chỉ vuốt thẳng 2 điểm — không có mảng `points` nhiều điểm |
| `device.button` | `{ sessionId?, deviceId?, button: string }` | `{}` | **Sửa (2026-09-03, TASK-EMU-010):** bảng gốc ghi `name` — sai. `server_emulator_host.go`'s `SendEmulatorButton` gửi khoá `button` (khớp `SendEmulatorButtonRequest.button`) |
| `device.rotate` | `{ sessionId?, deviceId?, orientation: string }` | `{}` | `RotateEmulator` trong `server_emulator_host.go`, khớp `RotateEmulatorRequest` |
| `device.shutdown` | `{ sessionId?, deviceId? }` | `{}` | `ShutdownEmulator` trong `server_emulator_host.go`, khớp `ShutdownEmulatorRequest` |

## Lỗi "chưa cài đặt"

**Cập nhật (2026-09-03, TASK-EMU-010 DONE cho Android):**
`device.attach/tap/gesture/button/rotate/shutdown` chạy adb thật khi thiết bị
target là Android. Cho iOS (chưa port — xem SOL-EMU-004), 5 method điều khiển
(không phải `attach`, vốn chỉ bookkeeping và thành công cho cả 2 platform)
vẫn trả JSON-RPC error thật (không phải throw không rõ nguyên nhân):

```json
{ "code": -32601, "message": "device.tap not implemented yet for iOS — Android device control is real (adb); iOS needs the serve-sim helper ... — see specs/emulator/bugs/missing-v1/tasks/TASK-EMU-010-device-control-handlers-port.md" }
```

Mã lỗi `-32601` (Method Not Found) khớp đúng với những gì
`infra-fleet-service`'s `devserveragent.Client.Exec` đã biết cách nhận diện
(`domain.ErrAgentMethodNotFound`, xem `emulator_relay.go:83-93`) và dịch
thành `apperrors.KindFailedPrecondition` / `INFRA_EMULATOR_UNSUPPORTED` —
tức là dù chưa cài xong, hành vi phía UI vẫn là lỗi rõ ràng, không phải
timeout hay treo. Một lỗi adb thật (device không tìm thấy, exit code ≠ 0,
thiếu `sessionId`/`deviceId`, tham số sai kiểu) trả `-32000` (Internal Error)
với message mô tả rõ nguyên nhân — khác `-32601`, vốn chỉ dành riêng cho
"method này chưa cài cho platform này".
