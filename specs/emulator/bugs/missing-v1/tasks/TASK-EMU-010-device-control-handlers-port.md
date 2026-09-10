# TASK-EMU-010: Port `device.attach/tap/gesture/button/rotate/shutdown`

**Solution:** [SOL-EMU-004](../solutions/SOL-EMU-004-device-control-handlers.md)
**Priority:** P2
**Status:** `[x]` DONE cho Android (adb thật); iOS vẫn honest-stub có chủ đích — xem "Vì sao iOS chưa port" bên dưới.

## Việc đã làm (khác kế hoạch ban đầu — xem lý do trong SOL-EMU-004)

1. **Không** tách `serve-sim-execution.ts`/`android/scrcpy-server-download.ts` — không
   cần dùng tới: Android control port này chỉ cần `adb shell input`/`adb emu kill`
   (không đụng tới scrcpy streaming), và iOS control (thứ duy nhất cần
   `serve-sim-execution.ts`) vẫn ở honest-stub pass này. `backend/` giữ nguyên,
   không sửa gì.
2. Port Android control thành bản **tự viết, gọn, độc lập** (không phải copy
   nguyên `backends/android-emulator-backend.ts` + `android/android-input-commands.ts`
   + `AndroidCommandRunner`/`AndroidSdkPaths` — những thứ đó gắn chặt với
   `EmulatorBridge`/`AndroidSdkState` mà `emulator/` không có) — cùng tinh thần
   với `device-android-discovery.ts` đã có (xem doc comment của nó). File mới:
   - `emulator/src/relay/device-android-control.ts` — `adb shell input tap/swipe/keyevent`,
     `adb shell settings put system accelerometer_rotation/user_rotation`,
     `adb emu kill`; arg-builder thuần (`androidTapArgs`, `androidSwipeArgs`,
     `androidButtonArgs`, `androidRotateArgs`, `androidShutdownArgs`) tách khỏi
     phần exec để test được không cần mock `child_process`.
   - `emulator/src/relay/device-session-registry.ts` — map `sessionId ↔
     {deviceId, platform}` trong bộ nhớ tiến trình (không persist).
3. Nối vào `emulator/src/relay/device-control-handler.ts`:
   `attach` luôn thành công cho cả 2 platform (chỉ tạo bookkeeping session, gọi
   `device-list-handler.ts`'s `listDevices()` để biết platform thật của
   `deviceId`); `tap/gesture/button/rotate/shutdown` chạy adb thật khi
   `platform === 'android'`, ném `DeviceMethodNotImplementedError` (giữ
   `-32601`) riêng cho từng method khi `platform === 'ios'`.
4. `emulator-rpc-dispatch.ts`: nay truyền `rpc.params` xuống
   `attach/tap/gesture/button/rotate/shutdown` (trước đó gọi không tham số vì
   toàn bộ là honest-stub).
5. **Sửa lệch giữa spec và code thật đã phát hiện khi port**: TDD-EMU-02 mô tả
   `device.gesture` params là `{sessionId?, deviceId?, points: [{x,y}]}` và
   `device.button` là `{..., name}` — nhưng
   `backend-go/services/infra-fleet-service/internal/adapter/grpc/server_emulator_host.go`
   (code thật, đã merge trước pass này) build params thật là
   `device.gesture: {sessionId, startX, startY, endX, endY, durationMs}` và
   `device.button: {sessionId, button}` (khớp
   `infrafleet.proto`'s `SendEmulatorGestureRequest`/`SendEmulatorButtonRequest`,
   đều `int32`, không phải mảng điểm chuẩn hoá 0..1). Cài đặt ở đây theo đúng
   **code thật** (nguồn chân lý), không theo mô tả cũ trong TDD-EMU-02:
   - `device.tap`: `{sessionId?, deviceId?, x, y}` — `x`/`y` là **pixel thiết
     bị thật** (int), không phải 0..1 chuẩn hoá như đường `emulator.*` cục bộ
     (`{kind:'local'}` → `EmulatorBridge`/serve-sim) — 2 hệ toạ độ khác nhau,
     không liên quan.
   - `device.gesture`: `{sessionId?, deviceId?, startX, startY, endX, endY,
     durationMs?}` — vuốt thẳng 2 điểm (khớp đúng những gì `adb input swipe`
     hỗ trợ; mảng `points` nhiều điểm không được hỗ trợ vì backend-go thật
     không gửi nó).
   - `device.button`: `{sessionId?, deviceId?, button}` — cũng chấp nhận khoá
     `name` như phương án dự phòng (không hại gì, phòng khi có caller khác
     dùng đúng shape tài liệu cũ).
   - `device.rotate`/`device.shutdown`: khớp đúng TDD-EMU-02.
6. Unit test mới (mock `child_process.execFile`, không cần thiết bị thật):
   - `device-android-control.test.ts` — verify từng arg array (`adb -s
     <serial> shell input tap/swipe/keyevent`, `settings put ...`, `emu kill`),
     bảng mã phím + alias, làm tròn toạ độ.
   - `device-session-registry.test.ts` — attach/get/findByDeviceId/
     removeByDeviceId/clear.
   - `device-control-handler.test.ts` — `attach` tạo/tái dùng session đúng
     platform; `tap/gesture/button/rotate/shutdown` gọi đúng lệnh adb qua
     `sessionId` lẫn `deviceId` trực tiếp; lỗi adb thật (exit code ≠ 0) được
     ném ra chứ không nuốt; session iOS ném đúng `DeviceMethodNotImplementedError`
     (`code === -32601`); thiếu `sessionId`/`deviceId` báo lỗi rõ ràng.
   - Cập nhật `emulator-rpc-dispatch.test.ts`: bỏ test cũ giả định
     `device.tap` luôn trả `-32601` (nay chỉ đúng cho session iOS), thêm test
     xác nhận lỗi thiếu target không còn là `-32601`.

## Vì sao iOS chưa port (quyết định có chủ đích, không phải bỏ sót)

Cơ chế điều khiển iOS Simulator duy nhất có trong `backend/` là
`backends/ios-emulator-backend.ts` shell-out ra binary bên thứ ba `serve-sim`
(`serve-sim-execution.ts`) — cần Xcode thật, một dylib camera-injection ký
riêng cho kiến trúc iOS-simulator, và (theo chính thông báo lỗi của nó) một
Simulator.app đã boot với framebuffer sống để tap/gesture vào được. Không có
`xcrun`/Xcode/Simulator thật trong sandbox này để verify — viết code "trông
đúng" cho phần này có rủi ro cao là sai lệch tinh vi (ví dụ tọa độ, timing
gesture) mà không cách nào phát hiện qua unit test mock. `device.tap/gesture/
button/rotate/shutdown` cho platform iOS tiếp tục ném
`DeviceMethodNotImplementedError` (`-32601`) — message method-cụ thể, không
lẫn với lỗi chung chung — cho tới khi có máy macOS/Xcode thật để làm và verify
riêng.

## Verify đã chạy (kết quả thật)

- `cd emulator && node /opt/repos/orca/agent/node_modules/typescript/bin/tsc --noEmit -p tsconfig.json` → sạch, không lỗi.
- `cd emulator && node /opt/repos/orca/agent/node_modules/vitest/vitest.mjs run --config vitest.config.ts` → **61/61 test pass**, 8 file test (bao gồm 3 file test mới + toàn bộ test cũ, không có test nào fail/skip).
- `node build.mjs` → build thành công, `out/emulator.js` (~152kb).

## KHÔNG thể verify (ghi rõ, không claim đã test)

- **Không có thiết bị/emulator Android thật** trong sandbox này (không có
  `adb`, không có AVD) — Android control ở trên chỉ được verify bằng unit
  test mock `child_process.execFile`, xác nhận đúng binary/args được gọi,
  KHÔNG xác nhận hành vi thật trên máy Android SDK thật. Cần 1 máy có Android
  SDK + AVD/thiết bị thật để manual-verify: `device.attach` → `device.tap` tại
  toạ độ biết trước → xác nhận điểm chạm đúng vị trí trên màn hình; tương tự
  cho `gesture`/`button`/`rotate`/`shutdown`.
- **Không có macOS/Xcode thật** — iOS control hoàn toàn chưa port, không có
  gì để verify ở đây pass này.
