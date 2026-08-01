# BUG-AT-001: `AutomationService` dùng Electron `webContents.send()` — không hoạt động trong headless server mode

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-AT-001  
**Note:** automations/service.ts: WebContents → RendererBridge interface  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/main/automations/service.ts:1`:
```typescript
import type { WebContents } from 'electron'
```

`src/main/automations/service.ts:248`:
```typescript
webContents.send('automations:dispatchRequested', payload)
```

`AutomationService` **phụ thuộc vào Electron's WebContents** để dispatch automation runs đến Renderer process.

Khi chạy headless (`orca --headless` hoặc server mode), sẽ không có BrowserWindow → `webContents = null`.

Kiểm tra guard trong code:
```typescript
// service.ts:230:
const webContents = this.webContents
if (!webContents) {
  if (!this.headlessDispatcher) {
    // ...update status dispatch_failed
  }
}
```

**`headlessDispatcher`** được check → có khả năng headless mode có alternative path. Nhưng:
- `headlessDispatcher` phải được inject qua constructor option
- Nếu không inject → tất cả scheduled automations sẽ `status='dispatch_failed'`

## Root Cause

HLD mô tả Automation là "Daemon Process" với AutomationEngine. Thực tế implementation là Electron-coupled service. Headless mode đang được thêm dần qua `headlessDispatcher` nhưng chưa complete.

## Ảnh hưởng

1. `BL-AT-02` (Schedule Automation): Không trigger được khi headless
2. `BL-CLI-03` (Headless Mode): Automation commands thất bại
3. CI/CD use case: `orca run --automation nightly` → fail

## Fix đề xuất

`headlessDispatcher` cần được wire đầy đủ vào server bootstrap:
```typescript
// server-bootstrap.ts hoặc electron main:
const headlessDispatcher = new HeadlessAutomationDispatcher(relayPool)
const automationService = new AutomationService(store, {
  headlessDispatcher,
  allowRemoteHostScheduling: true
})
```

## Files liên quan

- `src/main/automations/service.ts:1,29,248`: Electron dependency
- `src/main/automations/headless-dispatch.ts`: HeadlessDispatcher (có nhưng cần wiring)
- `src/main/index.ts`: Electron main entry
