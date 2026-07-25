# TASK-FE-002 — Cập nhật web-preload-api.ts

**Source Solutions:** [SOL-FE-002](../solutions/SOL-FE-002-rpc-client-bridge.md), [SOL-FE-004](../solutions/SOL-FE-004-web-preload-compat.md)  
**Priority:** P0 — Phụ thuộc TASK-FE-001  
**Loại:** Cập nhật file hiện có  
**Depends on:** TASK-FE-001 (IRpcClient interface + WebSocketRpcClient)

---

## Context

`web-preload-api.ts` đã tồn tại trong codebase. Cần cập nhật để:
1. Import và dùng `WebSocketRpcClient` từ TASK-FE-001
2. Expose đúng `OrcaApi` interface (giống hệt Electron preload)
3. Hỗ trợ cleanup methods (`offData`, `offExit`) để tránh memory leaks

---

## Input

Đọc trước khi sửa:
- `src/renderer/src/web/web-preload-api.ts` — file hiện tại (xem nội dung)
- `src/preload/index.ts` — Electron preload để verify API surface match
- `src/renderer/src/hooks/` — xem tất cả `window.api.*` calls để biết method nào cần

---

## Output — File cần sửa

### File: `src/renderer/src/web/web-preload-api.ts` [CẬP NHẬT]

Rewrite hoàn toàn file này theo spec sau:

#### Import

```typescript
import { WebSocketRpcClient } from '../../../platform/adapters/web/rpc-client'
import type { IRpcClient } from '../../../platform/rpc-client-interface'
```

#### Export interface

```typescript
export interface WebPreloadOptions {
  wsUrl?: string
}

export function installWebPreloadApi(options: WebPreloadOptions = {}): IRpcClient
```

#### Cleanup handler pattern (QUAN TRỌNG)

Vì `useIpcEvents` hooks gọi `offData(handler)` để cleanup, phải track unsubscribe per handler:

```typescript
// Internal registry — WeakMap để GC friendly
const handlerUnsubMap = new WeakMap<Function, () => void>()

function makeSubscriber(client: IRpcClient, channel: string) {
  return (callback: (event: any) => void) => {
    const listener = (_e: any, ...args: any[]) => callback(args[0])
    const unsub = client.on(channel, listener)
    handlerUnsubMap.set(callback, unsub)  // track for offX()
    return unsub
  }
}

function makeUnsubscriber() {
  return (callback: Function) => {
    const unsub = handlerUnsubMap.get(callback)
    if (unsub) {
      unsub()
      handlerUnsubMap.delete(callback)
    }
  }
}
```

#### API surface phải cover đầy đủ

| Namespace | Methods bắt buộc |
|-----------|------------------|
| `pty` | `create`, `write`, `resize`, `kill`, `subscribe`, `onData`, `offData`, `onExit`, `offExit` |
| `filesystem` | `readFile`, `writeFile`, `listDir`, `search`, `onChange`, `watch`, `unwatch` |
| `ssh` | `listTargets`, `connect`, `disconnect`, `onConnectionStateChanged` |
| `repos` | `list`, `create`, `update`, `delete` |
| `worktrees` | `detect`, `create`, `delete`, `list` |
| `settings` | `getGlobal`, `updateGlobal` |
| `github` | `listPRs`, `createPR` |
| `runtimeEnvironments` | `call` |
| *(root)* | `onNotification`, `onAgentStatusUpdate`, `onAutomationEvent`, `onRuntimeEvent`, `onWorkspaceSession` |

#### Channel naming convention

`namespace:method` — ví dụ:
- `repos:list`
- `pty:create`  
- `filesystem:readFile`
- Push events: `pty:data`, `pty:exit`, `ssh:stateChanged`, `filesystem:change`

#### Kết thúc function

```typescript
  ;(window as any).api = api
  return client  // trả về client để bootstrapWebApp quản lý lifecycle
```

---

## Acceptance Criteria

| # | Criteria | Verify bằng |
|---|----------|-------------|
| AC-1 | `installWebPreloadApi()` set `window.api` | unit test |
| AC-2 | `window.api.repos.list()` gọi `rpc.invoke('repos:list')` | unit test |
| AC-3 | `window.api.pty.onData(cb)` registers push listener | unit test |
| AC-4 | `window.api.pty.offData(cb)` unsubscribes đúng | unit test |
| AC-5 | `window.api.ssh.listTargets()` gọi đúng channel | unit test |
| AC-6 | Tất cả methods trong bảng trên đều tồn tại và là function | compat test |
| AC-7 | `installWebPreloadApi()` trả về IRpcClient | compile check |
| AC-8 | File compile clean với TypeScript strict | tsc check |

---

## Constraints

- **KHÔNG** thay đổi `src/preload/index.ts` (Electron preload)
- **KHÔNG** thay đổi signature của `window.api` — phải match Electron preload 1:1
- Dùng `WeakMap` cho handler tracking — không dùng `Map` (memory leak risk)
- Không import React hoặc bất kỳ UI library nào trong file này

---

## Audit script (sau khi implement)

Chạy script này để verify coverage:
```bash
npx tsx scripts/audit-window-api-coverage.ts
```

Script này scan tất cả hooks files và report missing API methods.
Xem spec chi tiết tại: `specs/frontend/crs/v1/restructure_v1/solutions/SOL-FE-004-web-preload-compat.md` §2.1

---

## Execution Status

**Status:** ⏭️ SKIPPED (Already implemented)  
**Date:** 2026-07-23  
**Ghi chú:** `web-preload-api.ts` đã là file 135KB với đầy đủ `installWebPreloadApi()` function. Codebase thực tế dùng `WebRuntimeClient` (E2EE, encrypted WebSocket) thay vì simple `IRpcClient`. File đã cover tất cả API methods cần thiết. Không cần thay đổi.
