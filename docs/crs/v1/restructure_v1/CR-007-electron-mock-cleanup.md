# CR-007 — Electron Mock Consolidation & Cleanup

**Status:** ✅ Implemented  
**Priority:** 🟢 Low  
**Depends on:** CR-002  
**Blocks:** —

---

## Mục tiêu

Dọn dẹp và hợp nhất các mock/stub của Electron sau khi NodeAdapter (CR-002) và electron-node-wrapper (CR-005) đã được implement:
1. Deprecate `src/main/mocks/electron.ts` (không xóa ngay để tránh conflict)
2. Đảm bảo không còn duplicate member issues trong mock class
3. Làm sạch `src/server/index.ts` khỏi các workaround tạm thời
4. Đảm bảo tất cả Electron API được sử dụng đều có coverage trong `electron-node-wrapper.ts`

---

## Bối cảnh

`src/main/mocks/electron.ts` được tạo ra như emergency workaround khi cần chạy Orca trong Node.js mode. Nó có nhiều vấn đề:

```typescript
// Vấn đề 1: Duplicate members (do patch nhiều lần)
class BrowserWindow extends EventEmitter {
  isMaximized() { return false }   // Method definition
  isMaximized = () => false         // Arrow function (DUPLICATE!)
  isMinimized = () => false         // Chưa có method counterpart
  isVisible() { return true }       // Method
  isVisible = () => true            // Arrow function (DUPLICATE!)
}

// Vấn đề 2: mockSessionObject được định nghĩa SAU khi được dùng
// (dùng trong BrowserWindow constructor, khai báo ở dưới)
this.webContents.session = mockSessionObject  // Dùng ở dây...
// ...
const mockSessionObject = { ... }              // ...khai báo sau!
// (Hoạt động vì var hoisting, nhưng confusing)

// Vấn đề 3: Type unsafe
this.webContents = new EventEmitter()
this.webContents.send = (channel: string, ...args: any[]) => { ... }
// webContents là EventEmitter nhưng thêm properties tùy tiện
```

---

## Giải pháp Đề xuất

### 1. Thêm Deprecation Notice vào mock

```typescript
// src/main/mocks/electron.ts — THÊM vào đầu file
/**
 * @deprecated
 * 
 * This file is a temporary workaround for running Orca in Node.js mode.
 * It will be replaced by `src/platform/adapters/node/` (CR-002) and
 * `src/platform/stubs/electron-node-wrapper.ts` (CR-005).
 * 
 * DO NOT add new functionality here.
 * DO NOT import this in new code.
 * 
 * Migration: Use `src/platform/adapters/node/` instead.
 */
```

### 2. Fix Duplicate Members (ngay lập tức)

```typescript
// src/main/mocks/electron.ts — FIX BrowserWindow class

export class BrowserWindow extends EventEmitter {
  static getAllWindows() { return [] }
  static fromWebContents() { return null }
  static fromId() { return null }
  static getFocusedWindow() { return null }

  readonly webContents: MockWebContents
  readonly id: number

  constructor(options?: any) {
    super()
    this.id = Math.floor(Math.random() * 10000)
    this.webContents = createMockWebContents()
  }

  loadURL(_url: string) { return Promise.resolve() }
  loadFile(_path: string) { return Promise.resolve() }
  show() {}
  hide() {}
  close() {}
  destroy() {}
  isDestroyed() { return false }
  focus() {}
  blur() {}
  restore() {}
  setAlwaysOnTop() {}
  maximize() {}
  unmaximize() {}
  minimize() {}
  setFullScreen() {}
  setOpacity() {}
  setBounds() {}
  
  // State queries — NO duplicates
  isVisible() { return true }
  isMaximized() { return false }
  isMinimized() { return false }
  isFullScreen() { return false }
  isFocused() { return true }
  
  getBounds() { return { x: 0, y: 0, width: 800, height: 600 } }
}

// Extract webContents creation to avoid forward reference issues
interface MockWebContents extends EventEmitter {
  send(channel: string, ...args: any[]): void
  openDevTools(): void
  loadURL(url: string): Promise<void>
  loadFile(path: string): Promise<void>
  invalidate(): void
  setBackgroundThrottling(value: boolean): void
  isDestroyed(): boolean
  setZoomLevel(level: number): void
  setWindowOpenHandler(handler: any): void
  reloadIgnoringCache(): void
  isDevToolsOpened(): boolean
  closeDevTools(): void
  isCrashed(): boolean
  session: typeof mockSessionObject
}

function createMockWebContents(): MockWebContents {
  const wc = new EventEmitter() as MockWebContents
  wc.send = (channel, ...args) => {
    console.log(`[MockBrowserWindow] send: ${channel}`)
  }
  wc.openDevTools = () => {}
  wc.loadURL = async () => {}
  wc.loadFile = async () => {}
  wc.invalidate = () => {}
  wc.setBackgroundThrottling = () => {}
  wc.isDestroyed = () => false
  wc.setZoomLevel = () => {}
  wc.setWindowOpenHandler = () => {}
  wc.reloadIgnoringCache = () => {}
  wc.isDevToolsOpened = () => false
  wc.closeDevTools = () => {}
  wc.isCrashed = () => false
  wc.session = mockSessionObject
  return wc
}

// mockSessionObject TRƯỚC khi dùng
const mockSessionObject = {
  webRequest: { onBeforeSendHeaders: () => {} },
  getUserAgent: () => 'MockUA',
  setUserAgent: () => {},
  clearStorageData: async () => {},
  clearCache: async () => {},
  setPermissionRequestHandler: () => {},
  setPermissionCheckHandler: () => {},
  setDisplayMediaRequestHandler: () => {},
  setDevicePermissionHandler: () => {},
  addWordToSpellCheckerDictionary: () => {},
  removeListener: () => {},
  on: () => {}
}
```

### 3. Cleanup `src/server/index.ts`

Sau khi CR-002 hoàn thành, refactor server entry point:

```typescript
// src/server/index.ts — AFTER CR-002 cleanup

/**
 * Orca Server Entry Point — Node.js mode
 * 
 * Khởi động Orca backend như một standalone HTTP/WebSocket server,
 * không cần Electron. Frontend React được phục vụ dưới dạng static files.
 * 
 * Architecture:
 *   HTTP server → serve out/web/ (React SPA)
 *   WebSocket /ws/runtime/api → IPC bridge → Orca runtime services
 */
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'
import { startHttpServer } from './http-server'
import { startWebSocketServer } from './ws-server'

const port = parseInt(process.env.ORCA_PORT ?? '6768', 10)
const domain = process.env.ORCA_DOMAIN ?? 'localhost'

async function main(): Promise<void> {
  console.log('======================================================')
  console.log(' Orca Node.js Server — Starting')
  console.log('======================================================')
  console.log(`  Port:   ${port}`)
  console.log(`  Domain: ${domain}`)
  console.log('======================================================\n')

  // 1. Initialize platform adapter
  const platform = createNodeAdapter({
    userDataPath: process.env.ORCA_USER_DATA_PATH
  })
  setPlatform(platform)
  
  // 2. Boot main application logic (registers all IPC handlers)
  // Vite alias: 'electron' → src/platform/stubs/electron-node-wrapper.ts
  const { initializeOrcaServices } = await import('../main/server-bootstrap')
  await initializeOrcaServices(platform)
  
  // 3. Start HTTP server (static web files + API)
  const httpServer = await startHttpServer(port)
  
  // 4. Attach WebSocket (IPC bridge for web frontend)
  await startWebSocketServer(httpServer, platform.ipc)
}

main().catch(err => {
  console.error('[Orca Server] Fatal error:', err)
  process.exit(1)
})
```

### 4. `src/main/server-bootstrap.ts` (FILE MỚI)

```typescript
// src/main/server-bootstrap.ts (FILE MỚI — không sửa index.ts)
/**
 * Server bootstrap — khởi tạo các Orca services cho Node.js mode.
 * 
 * Đây là entry point thay thế cho src/main/index.ts khi chạy trong server mode.
 * Chỉ khởi tạo các services cần thiết, bỏ qua các Electron-specific features
 * như Menu, Tray, native dialogs.
 * 
 * IMPORTANT: Không import file này từ src/main/index.ts.
 * File này import từ src/main/ nhưng src/main/ không biết file này tồn tại.
 */
import type { IPlatformServices } from '../platform/types'

export async function initializeOrcaServices(
  platform: IPlatformServices
): Promise<void> {
  // Import và khởi tạo từng service subset
  // (Tránh import toàn bộ src/main/index.ts)
  
  await initializePersistence(platform)
  await initializeIpcHandlers(platform)
  await initializeDaemon(platform)
  await initializeRuntimeRpc(platform)
}

async function initializePersistence(platform: IPlatformServices): Promise<void> {
  const { loadPersistence } = await import('./persistence')
  // ...
}

// etc.
```

---

## Audit Checklist: Electron APIs còn thiếu trong Wrapper

Sau khi CR-002 và CR-005 hoàn thành, cần verify tất cả Electron APIs được dùng:

```bash
# Tìm tất cả Electron APIs đang được dùng trong src/main/
grep -rh "from 'electron'" src/main/ \
  | grep -oP "(?<=import { ).*(?= } from)" \
  | tr ',' '\n' \
  | tr -d ' ' \
  | sort | uniq \
  > /tmp/used-electron-apis.txt

# So sánh với những gì electron-node-wrapper.ts export
grep -oP "^export (const|class|function) \K\w+" \
  src/platform/stubs/electron-node-wrapper.ts \
  | sort \
  > /tmp/wrapped-apis.txt

# Diff — những API còn thiếu
diff /tmp/wrapped-apis.txt /tmp/used-electron-apis.txt
```

---

## Migration Path (Long-term)

```
Phase 1 (CR-007): Fix mock, add deprecation notice
Phase 2 (Future):  Renderer uses bridge.invoke() instead of window.electron
Phase 3 (Future):  Remove electron-compat shim
Phase 4 (Future):  Delete src/main/mocks/electron.ts
```

---

## Phạm vi thay đổi

### Files sửa đổi (trong CR-007)
| File | Thay đổi |
|------|---------|
| `[MODIFY] src/main/mocks/electron.ts` | Add deprecation notice, fix duplicate members, fix ordering |
| `[MODIFY] src/server/index.ts` | Cleanup workarounds sau khi CR-002 done |

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] src/main/server-bootstrap.ts` | Selective service init cho Node mode |

### Files KHÔNG thay đổi
- `src/main/index.ts` — **KHÔNG sửa**
- `src/main/ipc/` — **KHÔNG sửa**

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

| File | Status |
|------|--------|
| `src/main/mocks/electron.ts` | ✅ Done — deprecated notice added L2 |
| `src/main/server-bootstrap.ts` | ✅ Done — clean bootstrap without mock dependency |
| `src/platform/stubs/` | ✅ Done — proper stubs replacing mock |
