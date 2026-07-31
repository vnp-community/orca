# TDD-BE-15: Platform Abstraction Layer

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/platform/`

---

## 1. Mục tiêu

Tách biệt `src/main/` khỏi Electron API — cho phép chạy trong Node.js server mode mà không cần bất kỳ `import 'electron'` nào.

---

## 2. Interface Hierarchy

```typescript
// src/platform/types.ts
export interface IPlatformServices {
  app:           IAppInterface
  ipc:           IIpcInterface
  windowManager: IWindowInterface
  storage:       IStorageInterface
  system:        ISystemInterface
}

// src/platform/app-interface.ts
export interface IAppInterface {
  getPath(name: 'userData' | 'logs' | 'temp' | 'cache'): string
  getVersion(): string
  getName(): string
  quit(): void
  isReady(): boolean
}

// src/platform/ipc-interface.ts
export interface IIpcInterface {
  on(channel: string, listener: (...args: any[]) => void): () => void
  handle(channel: string, handler: (...args: any[]) => Promise<any>): () => void
  send(webContents: unknown, channel: string, ...args: any[]): void
}

// src/platform/storage-interface.ts
export interface IStorageInterface {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  deleteItem(key: string): void
}

// src/platform/window-interface.ts
export interface IWindowInterface {
  showWindow(): void
  hideWindow(): void
  openExternal(url: string): Promise<void>
  setTitle(title: string): void
}

// src/platform/system-interface.ts
export interface ISystemInterface {
  platform: NodeJS.Platform
  homedir: string
  env: Record<string, string | undefined>
}
```

---

## 3. NodeAdapter (src/platform/adapters/node/)

```typescript
export function createNodeAdapter(opts: {
  userDataPath?: string
} = {}): IPlatformServices {
  return {
    app:           new NodeApp(opts.userDataPath),
    ipc:           new NodeIpcBridge(),
    windowManager: new NodeWindowManager(),
    storage:       new NodeSecureStorage(opts.userDataPath),
    system:        new NodeSystemInterface()
  }
}
```

### NodeApp
```typescript
class NodeApp implements IAppInterface {
  getPath(name: string): string {
    switch (name) {
      case 'userData': return this.userDataPath  // ORCA_USER_DATA_PATH || ~/.orca
      case 'logs':     return join(this.userDataPath, 'logs')
      case 'temp':     return os.tmpdir()
      case 'cache':    return join(this.userDataPath, 'cache')
    }
  }
  getVersion(): string { return process.env['ORCA_VERSION'] ?? '0.0.0' }
  quit():       void   { process.exit(0) }
}
```

### NodeIpcBridge (no-op)
```typescript
class NodeIpcBridge implements IIpcInterface {
  on():     () => void  { return () => {} }  // No-op (không có Electron renderer)
  handle(): () => void  { return () => {} }
  send():   void        { }                   // No-op
}
```

### NodeSecureStorage
```typescript
class NodeSecureStorage implements IStorageInterface {
  // Encrypt với AES-256-GCM (crypto.createCipheriv)
  // Key derivation: PBKDF2 từ machine-unique seed (hostname + process.pid)
  // File: userData/secure-storage/<key>.enc
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  deleteItem(key: string): void
}
```

---

## 4. Context Singleton

```typescript
// src/platform/context.ts
let _platform: IPlatformServices | null = null

export function setPlatform(p: IPlatformServices): void {
  if (_platform) console.warn('Platform already set — overwriting')
  _platform = p
}

export function getPlatform(): IPlatformServices {
  if (!_platform) throw new Error('[platform] Not initialized. Call setPlatform() first.')
  return _platform
}

export function resetPlatform(): void {
  _platform = null  // Test only
}
```

---

## 5. Stubs (Testing)

```typescript
// src/platform/stubs/
export function createStubPlatform(overrides?: Partial<IPlatformServices>): IPlatformServices {
  return {
    app:           createStubApp(),
    ipc:           createStubIpc(),
    windowManager: createStubWindow(),
    storage:       createStubStorage(),
    system:        createStubSystem(),
    ...overrides
  }
}
```

---

## 6. Anti-patterns (bị cấm)

```typescript
// ❌ FORBIDDEN — gây lỗi khi import trong Node.js server mode
import { app } from 'electron'
import { safeStorage } from 'electron'
import { ipcMain } from 'electron'

// ✅ CORRECT — dùng platform abstraction
import { getPlatform } from '../platform/context'
const userData = getPlatform().app.getPath('userData')
```

Lint rule: No `import 'electron'` ngoài `src/platform/adapters/electron/` (nếu có).
