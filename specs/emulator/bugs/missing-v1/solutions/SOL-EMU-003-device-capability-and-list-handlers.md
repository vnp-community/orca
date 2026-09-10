# SOL-EMU-003: `device.capabilities` / `device.list` / `device.availability`

**Resolves:** CR-DS-009 §2.3, Phase 2
**Service:** `emulator/`
**Affected files:** `emulator/src/relay/device-android-discovery.ts`, `device-ios-discovery.ts`, `device-capabilities-handler.ts`, `device-list-handler.ts`, `emulator-rpc-dispatch.ts`
**Status:** ✅ Implemented — xem [TASK-EMU-003](../tasks/TASK-EMU-003-device-capabilities-and-list-handlers.md), [TASK-EMU-004](../tasks/TASK-EMU-004-emulator-rpc-dispatch.md)

## Giải pháp

Cài mới (không port nguyên `backend/src/main/emulator/**` — hệ thống đó gắn
với `EmulatorBridge`/backends đầy đủ ~650+ dòng cho riêng phần discovery,
chưa kể phần điều khiển; port toàn bộ thuộc TASK-EMU-010) hai module dò khả
dụng thuần Node.js, không phụ thuộc Electron:

- `device-android-discovery.ts`: dò `ANDROID_HOME`/`ANDROID_SDK_ROOT`, các
  đường dẫn cài đặt Android Studio mặc định theo từng OS, xác nhận có
  `platform-tools/adb` (hoặc `emulator/emulator` binary).
- `device-ios-discovery.ts`: trên `darwin`, chạy `xcrun -find simctl` để xác
  nhận Xcode command line tools có `simctl`; không làm gì trên
  Linux/Windows.

`device-list-handler.ts` gọi `adb devices -l` (nếu có adb) và
`xcrun simctl list devices -j` (nếu trên macOS và có simctl) để trả về danh
sách thiết bị theo đúng shape `emulator_relay.go`'s `decodeEmulatorDevices`
mong đợi (`{id, name, platform, state}`).

`emulator-rpc-dispatch.ts` đăng ký toàn bộ 9 method trong catalog
(TDD-EMU-02); `device.attach/tap/gesture/button/rotate/shutdown` trả lỗi
`-32601` kèm đường dẫn tới TASK-EMU-010 (honest stub, khớp cách
`infra-fleet-service` đã tự thiết kế để nhận diện "method not found").
