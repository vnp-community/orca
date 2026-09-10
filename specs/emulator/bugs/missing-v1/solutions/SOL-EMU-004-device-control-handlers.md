# SOL-EMU-004: `device.attach/tap/gesture/button/rotate/shutdown` — điều khiển thật

**Resolves:** CR-DS-009 §2.3, Phase 2 (phần điều khiển thiết bị)
**Service:** `emulator/`
**Status:** ✅ DONE cho Android (adb thật, unit test xanh); iOS honest-stub có chủ đích (xem TASK-EMU-010)
**Task(s):** [TASK-EMU-010](../tasks/TASK-EMU-010-device-control-handlers-port.md)

## Vấn đề (bối cảnh ban đầu)

Điều khiển thiết bị thật (tap/gesture/button/rotate, gắn/tắt session) nằm
trong `backend/src/main/emulator/emulator-bridge.ts` (329 dòng) +
`backends/*` (adapter ADB/iOS) + `android/*` (19 file: adb-devices,
android-input-commands, android-input-mapping, scrcpy-stream-session, …) +
`emulator-gesture-sender.ts`, `emulator-session-registry.ts`,
`serve-sim-*.ts` — tổng cộng hàng nghìn dòng, có 2 file phụ thuộc Electron
(`serve-sim-execution.ts`, `android/scrcpy-server-download.ts` — dùng
`app.getPath`), toàn bộ gắn chặt với `EmulatorBridge`'s router/session
registry/stream lifecycle mà `emulator/` (agent điều khiển thuần, không
stream) không có.

## Giải pháp đã triển khai

Thay vì port nguyên khối `EmulatorBridge` + `backends/*` + `android/*` (kế
hoạch ban đầu bên dưới), phần Android được viết lại **gọn, tự chứa** —
đúng tinh thần `device-android-discovery.ts` đã có trước đó trong `emulator/`
(doc comment của nó: *"fresh, self-contained implementation, not a port of
backend/src/main/emulator/android/*"*):

1. `emulator/src/relay/device-android-control.ts` — `adb shell input
   tap/swipe/keyevent`, `adb shell settings put system
   accelerometer_rotation/user_rotation` (rotate), `adb emu kill`
   (shutdown). Arg-builder thuần tách khỏi phần exec cho test không cần mock
   `child_process`. Toạ độ tap/gesture là **pixel thiết bị thật** (khớp kiểu
   `int32` của `SendEmulatorTapRequest`/`SendEmulatorGestureRequest` trong
   `infrafleet.proto`) — không cần đọc `wm size` trước mỗi lần tap như bản
   `backend/` (bản đó dùng toạ độ 0..1 chuẩn hoá vì phục vụ đường
   `emulator.*` cục bộ khác, không phải đường `device.*` remote này).
2. `emulator/src/relay/device-session-registry.ts` — map `sessionId ↔
   {deviceId, platform}` trong bộ nhớ tiến trình, thay cho
   `EmulatorSessionRegistry` (bản gốc còn theo dõi cả stream/helper
   lifecycle mà agent này không sở hữu).
3. `device-control-handler.ts`: `attach` thành công cho mọi platform (chỉ
   bookkeeping); `tap/gesture/button/rotate/shutdown` chạy adb thật khi
   platform là `android`, ném `DeviceMethodNotImplementedError` (`-32601`,
   message method-cụ thể) khi platform là `ios`.
4. Không tách 2 file dính Electron (`serve-sim-execution.ts`,
   `android/scrcpy-server-download.ts`) — **không cần dùng tới** pass này:
   Android control không đụng scrcpy streaming, và iOS control (thứ duy nhất
   cần `serve-sim-execution.ts`) vẫn ở honest-stub. `backend/` giữ nguyên
   100%.
5. Phát hiện + sửa theo lệch giữa `specs/emulator/tdd/v1/02-device-rpc-catalog.md`
   và code backend-go thật (`server_emulator_host.go` + `infrafleet.proto`,
   đã merge trước pass này) — implement theo **code thật**: `device.gesture`
   dùng `{startX, startY, endX, endY, durationMs}` (không phải mảng
   `points`), `device.button` dùng khoá `button` (không phải `name`, dù vẫn
   chấp nhận `name` dự phòng). Chi tiết đầy đủ trong TASK-EMU-010.

### Kế hoạch ban đầu (không theo — ghi lại để đối chiếu)

1. ~~Tách 2 file dính Electron: thay `app.getPath('userData')` bằng
   `process.env.ORCA_EMULATOR_DATA_DIR` (fallback `os.tmpdir()`)~~ — không
   cần, xem trên.
2. ~~Port `backends/emulator-backend.ts` (interface) + 2 implementation gần
   như nguyên trạng~~ — thay bằng bản tự viết gọn hơn nhiều (không có
   interface đa-backend, không cần vì iOS chưa port).

## iOS — vẫn honest-stub, có chủ đích

`backends/ios-emulator-backend.ts` (nguồn logic iOS duy nhất trong
`backend/`) shell-out ra binary bên thứ ba `serve-sim`, cần Xcode thật + dylib
camera-injection ký riêng + Simulator.app đã boot với framebuffer sống.
Không có gì trong số đó ở sandbox này để verify — xem lý do đầy đủ trong
TASK-EMU-010's "Vì sao iOS chưa port".

## Test

Unit test mock `child_process.execFile` — verify đúng binary/args cho từng
adb command (tap/swipe/keyevent/settings/emu kill), bảng mã phím Android +
alias, session registry attach/lookup/cleanup, lỗi adb thật không bị nuốt,
lỗi thiếu target/tham số rõ ràng, honest-stub `-32601` đúng cho session iOS.
Không cần thiết bị Android/iOS thật để chạy các test này — xem TASK-EMU-010
để biết integration test thật (máy Android SDK/Xcode thật) còn thiếu.
