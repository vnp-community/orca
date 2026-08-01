# BUG-CLI-001: CLI-03 Headless Mode — AutomationService yêu cầu `WebContents` (Electron) không có trong server context

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-CLI-001  
**Note:** PtyDaemon wires CLI commands to OrcaRuntime  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

Xem BUG-AT-001 — `AutomationService` dùng `webContents.send()`.

Trong CLI headless context (`$ orca daemon start`):
1. Không có Electron BrowserWindow
2. `webContents = null`  
3. Automation dispatch fallback to `headlessDispatcher`
4. `headlessDispatcher` cần được inject — có code (`src/main/automations/headless-dispatch.ts`) nhưng chưa được wire vào headless startup

## Thêm: CLI binary không tồn tại

Grep:
```bash
find src -name "cli.ts" -o -name "cli-entry.ts" -o -path "*/bin/orca*"
→ src/cli/ directory exists
```

`src/cli/` có handlers nhưng CLI binary entry point cần verify.

## Ảnh hưởng

1. `$ orca daemon start` → nếu không wire headlessDispatcher → automations fail silently
2. `$ orca run --automation` → dispatch_failed status  

## Fix đề xuất

Trong headless server bootstrap:
```typescript
import { HeadlessAutomationDispatcher } from './automations/headless-dispatch'

const headlessDispatcher = new HeadlessAutomationDispatcher({
  relayPool,
  spawnLocalAgent: (opts) => localPtyManager.spawn(opts)
})

automationService.setHeadlessDispatcher(headlessDispatcher)
automationService.start()
```

## Files liên quan

- `src/main/automations/headless-dispatch.ts`: HeadlessDispatcher implementation
- `src/main/index.ts`: Electron main entry - cần headless branch
- `src/cli/`: CLI handlers
