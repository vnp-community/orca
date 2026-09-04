# TDD-EMU-01: Architecture

**Source:** `emulator/src/relay/*`
**CR Ref:** CR-DS-009 §2

## Vị trí trong hệ thống

```
backend-go (infra-fleet-service): Agent Registry (kind-aware, TASK-EMU-007)
         │ wss:// git.*, fs.*, pty.*        │ wss:// device.*
┌────────▼───────────┐          ┌───────────▼─────────────┐
│ agent/              │          │ emulator/                │
│ orca-agent          │          │ orca-emulator-agent       │
│ chạy trên Dev Server│          │ chạy trên máy có Android  │
│ (thường remote)     │          │ Studio/Xcode (thường máy  │
│                     │          │ cá nhân của dev)          │
└─────────────────────┘          └───────────────────────────┘
```

## Ranh giới trách nhiệm

`emulator/` **không** có:
- git/worktree ops
- fs read/write/watch
- pty spawn
- browser automation, CLI install, AI provider credential

`emulator/` **chỉ** có:
- `device.list` / `device.availability` / `device.capabilities` — liệt kê & dò khả dụng
- `device.attach` — gắn 1 session điều khiển vào 1 thiết bị
- `device.tap` / `device.gesture` / `device.button` / `device.rotate` — điều khiển
- `device.shutdown` — tắt thiết bị/emulator

## Vì sao tách gói thay vì thêm `case 'device.*'` vào `agent/src/relay/agent-rpc-dispatch.ts`

1. **Sai vị trí vật lý** — Dev Server Agent chạy trên máy chứa code (thường
   remote Linux), hiếm khi có Android Studio/Xcode. iOS Simulator chỉ chạy
   trên macOS.
2. **Sai ranh giới bảo mật/triển khai** — bắt agent nắm git/fs/pty của
   project cũng phải cài trên laptop cá nhân (chỉ vì máy đó có Android
   Studio) là thừa quyền và sai mô hình triển khai (Dev Server Agent cài như
   systemd/Docker trên server, không phải trên máy cá nhân).

→ `emulator/` là tiến trình riêng, gói riêng, đăng ký riêng (`AgentKind =
AGENT_KIND_MOBILE_EMULATOR`, xem TASK-EMU-007), có thể chạy trên một máy
hoàn toàn khác với Dev Server Agent của cùng project.

## Cây thư mục

```
emulator/
├── package.json        # name: "orca-emulator-agent"
├── tsconfig.json        # self-contained, không extend @electron-toolkit
├── build.mjs             # esbuild bundle emulator-entry.ts → out/emulator.js
├── vitest.config.ts
└── src/relay/
    ├── emulator-config.ts               # env vars: ORCA_BACKEND_URL, ORCA_AGENT_TOKEN, ORCA_EMULATOR_LOG_LEVEL...
    ├── emulator-logger.ts
    ├── device-android-discovery.ts      # probe Android SDK (ANDROID_HOME/ANDROID_SDK_ROOT + common paths)
    ├── device-ios-discovery.ts          # probe xcrun/simctl trên darwin
    ├── device-capabilities-handler.ts   # device.capabilities
    ├── device-list-handler.ts           # device.list, device.availability
    ├── device-control-handler.ts        # device.attach/tap/gesture/button/rotate/shutdown — honest "not implemented" (TASK-EMU-010)
    ├── emulator-rpc-dispatch.ts         # JSON-RPC dispatch table
    ├── emulator-entry.ts                # main() — hiện tại: stdio debug mode (xem TDD-EMU-04)
    └── *.test.ts
```
