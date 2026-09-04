# TASK-EMU-003: `device.capabilities` / `device.list` / `device.availability`

**Solution:** [SOL-EMU-003](../solutions/SOL-EMU-003-device-capability-and-list-handlers.md)
**Priority:** P1
**Status:** `[x]` DONE

## Việc đã làm

- `emulator/src/relay/device-android-discovery.ts`: dò `ANDROID_HOME`/`ANDROID_SDK_ROOT` + đường dẫn cài đặt mặc định theo OS (`~/Library/Android/sdk` trên darwin, `%LOCALAPPDATA%\Android\Sdk` trên win32, `~/Android/Sdk` còn lại), xác nhận có `platform-tools/adb`.
- `emulator/src/relay/device-ios-discovery.ts`: `xcrun -find simctl` trên darwin; trả lời "requires macOS" ngay không shell-out trên OS khác.
- `emulator/src/relay/device-capabilities-handler.ts`: gộp 2 discovery trên thành `device.capabilities`.
- `emulator/src/relay/device-list-handler.ts`: `adb devices -l` (parse `parseAdbDevicesOutput`) + `xcrun simctl list devices -j` (parse `parseSimctlListOutput`), gộp thành `device.list`; `device.availability` suy ra từ danh sách thiết bị + capabilities.

## Verify (chạy thật trong pass này)

```
$ node vitest.mjs run --config vitest.config.ts
✓ device-android-discovery.test.ts (3 tests)
✓ device-ios-discovery.test.ts (3 tests)
✓ device-list-handler.test.ts (3 tests)
Test Files  4 passed (4)
     Tests  13 passed (13)
```

Chạy thử thật trên host sandbox (Linux, không có Android SDK/Xcode) qua
`out/emulator.js` (stdio mode — xem TASK-EMU-005) xác nhận kết quả đúng thực
tế của máy: `sdkFound: false`, `simctlOk: false` (message "iOS Simulator
requires macOS"), `devices: []` — không phải dữ liệu giả lập.
