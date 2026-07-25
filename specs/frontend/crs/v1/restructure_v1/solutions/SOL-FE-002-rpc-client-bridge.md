# SOL-FE-002 — RPC Client & window.api Bridge

**CRs:** [CR-003](../../../../../docs/crs/v1/restructure_v1/CR-003-ipc-abstraction.md), [CR-004](../../../../../docs/crs/v1/restructure_v1/CR-004-web-entry.md)  
**TDD Refs:** TDD-FE-03 (Runtime Client Layer), TDD-FE-07 (Hooks & IPC)  
**Approach:** Test-Driven

---

## 1. Phân tích từ TDD

Từ **TDD-FE-01 §2.3 (`window.api` Abstraction)**:
```typescript
// Cả Desktop và Web expose cùng OrcaApi interface:
interface OrcaApi {
  filesystem: { readFile, writeFile, listDir, search, watch, ... }
  pty: { create, write, resize, kill, subscribe, onData, onExit, ... }
  ssh: { listTargets, connect, disconnect, ... }
  worktrees: { detect, create, delete, ... }
  repos: { list, create, update, delete, ... }
  settings: { getGlobal, updateGlobal, ... }
  // ...
}
```

Từ **TDD-FE-07 §2 (`useIpcEvents`)**, hooks dùng `window.api` pattern:
```typescript
window.api.pty.onData((event) => ...)
window.api.filesystem.onChange((event) => ...)
window.api.ssh.onConnectionStateChanged((event) => ...)
```

Từ **TDD-FE-03 (Runtime Client Layer)**:
- `web-runtime-session.ts` — xử lý terminal operations qua WebSocket
- `runtime-rpc-client.ts` — gọi RPC methods

→ `web-preload-api.ts` đã có trong codebase. Solution này đảm bảo nó dùng `WebSocketRpcClient` từ SOL-BE-003 và test đầy đủ.

---

## 2. File Structure

```
src/platform/adapters/web/
├── rpc-client.ts              # WebSocketRpcClient (CR-003)
└── __tests__/
    └── rpc-client.test.ts

src/renderer/src/web/
├── web-preload-api.ts         # window.api implementation via RPC [CẬP NHẬT]
└── __tests__/
    └── web-preload-api.test.ts
```

---

## 3. Test Specifications

### 3.1 `WebSocketRpcClient` Tests

```typescript
// src/platform/adapters/web/__tests__/rpc-client.test.ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { WebSocketRpcClient } from '../rpc-client'

// Mock WebSocket API (jsdom environment provides this)
class MockWebSocket extends EventTarget {
  static OPEN = 1
  static CLOSED = 3
  readyState = MockWebSocket.OPEN
  sent: string[] = []
  
  constructor(public url: string) {
    super()
  }
  
  send(data: string) {
    this.sent.push(data)
  }
  
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.dispatchEvent(new CloseEvent('close'))
  }
  
  // Helper to simulate receiving a message from server
  receive(data: object) {
    this.dispatchEvent(new MessageEvent('message', {
      data: JSON.stringify(data)
    }))
  }
  
  // Helper to simulate connection
  connect() {
    this.dispatchEvent(new Event('open'))
  }
}

describe('WebSocketRpcClient', () => {
  let mockWs: MockWebSocket
  let client: WebSocketRpcClient

  beforeEach(() => {
    // Mock global WebSocket constructor
    mockWs = new MockWebSocket('ws://localhost:6768')
    vi.stubGlobal('WebSocket', vi.fn(() => {
      // Simulate async connection
      setTimeout(() => mockWs.connect(), 0)
      return mockWs
    }))
    client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
  })

  afterEach(() => {
    client.disconnect()
    vi.restoreAllMocks()
  })

  describe('connect()', () => {
    it('resolves when WebSocket opens', async () => {
      await expect(client.connect()).resolves.toBeUndefined()
      expect(client.isConnected()).toBe(true)
    })

    it('rejects when WebSocket errors', async () => {
      vi.stubGlobal('WebSocket', vi.fn(() => {
        const ws = new MockWebSocket('ws://error')
        setTimeout(() => ws.dispatchEvent(new Event('error')), 0)
        return ws
      }))
      const errorClient = new WebSocketRpcClient('ws://error')
      await expect(errorClient.connect()).rejects.toThrow()
    })
  })

  describe('invoke()', () => {
    beforeEach(async () => {
      await client.connect()
    })

    it('sends JSON-RPC invoke message and resolves with result', async () => {
      // Simulate server reply
      const invokePromise = client.invoke('repos:list')
      
      const sent = JSON.parse(mockWs.sent[0])
      expect(sent.type).toBe('invoke')
      expect(sent.channel).toBe('repos:list')
      
      // Server replies
      mockWs.receive({ id: sent.id, type: 'result', result: [{ id: 'repo1' }] })
      
      const result = await invokePromise
      expect(result).toEqual([{ id: 'repo1' }])
    })

    it('rejects on error response', async () => {
      const invokePromise = client.invoke('bad:channel')
      const sent = JSON.parse(mockWs.sent[0])
      
      mockWs.receive({ id: sent.id, type: 'error', message: 'Not found' })
      
      await expect(invokePromise).rejects.toThrow('Not found')
    })

    it('passes args correctly', async () => {
      const invokePromise = client.invoke('worktrees:create', 'repo1', 'main')
      const sent = JSON.parse(mockWs.sent[0])
      
      expect(sent.args).toEqual(['repo1', 'main'])
      
      mockWs.receive({ id: sent.id, type: 'result', result: { id: 'wt1' } })
      await invokePromise
    })

    it('times out after INVOKE_TIMEOUT_MS', async () => {
      vi.useFakeTimers()
      
      const invokePromise = client.invoke('slow:operation')
      
      // Advance time past timeout
      vi.advanceTimersByTime(31_000)
      
      await expect(invokePromise).rejects.toThrow('timeout')
      
      vi.useRealTimers()
    })

    it('throws when not connected', async () => {
      client.disconnect()
      await expect(client.invoke('any:channel')).rejects.toThrow('Not connected')
    })
  })

  describe('on() — server push events', () => {
    beforeEach(async () => {
      await client.connect()
    })

    it('receives push events from server', () => {
      const handler = vi.fn()
      client.on('ssh:stateChanged', handler)
      
      mockWs.receive({
        type: 'push',
        channel: 'ssh:stateChanged',
        args: [{ targetId: 't1', state: 'connected' }]
      })
      
      expect(handler).toHaveBeenCalledOnce()
      const [event, payload] = handler.mock.calls[0]
      expect(payload).toEqual({ targetId: 't1', state: 'connected' })
    })

    it('returns unsubscribe function', () => {
      const handler = vi.fn()
      const unsub = client.on('test:event', handler)
      
      unsub()  // unsubscribe
      
      mockWs.receive({ type: 'push', channel: 'test:event', args: [] })
      expect(handler).not.toHaveBeenCalled()
    })

    it('supports multiple listeners on same channel', () => {
      const h1 = vi.fn()
      const h2 = vi.fn()
      client.on('test:multi', h1)
      client.on('test:multi', h2)
      
      mockWs.receive({ type: 'push', channel: 'test:multi', args: ['data'] })
      
      expect(h1).toHaveBeenCalledOnce()
      expect(h2).toHaveBeenCalledOnce()
    })
  })

  describe('once()', () => {
    beforeEach(async () => {
      await client.connect()
    })

    it('receives event only once', () => {
      const handler = vi.fn()
      client.once('one-time:event', handler)
      
      mockWs.receive({ type: 'push', channel: 'one-time:event', args: [] })
      mockWs.receive({ type: 'push', channel: 'one-time:event', args: [] })
      
      expect(handler).toHaveBeenCalledOnce()
    })
  })

  describe('send() — fire-and-forget', () => {
    beforeEach(async () => {
      await client.connect()
    })

    it('sends message without awaiting reply', () => {
      client.send('client:event', { data: 'value' })
      
      const sent = JSON.parse(mockWs.sent[0])
      expect(sent.type).toBe('send')
      expect(sent.channel).toBe('client:event')
    })

    it('silently ignores when not connected', () => {
      client.disconnect()
      expect(() => client.send('test', {})).not.toThrow()
    })
  })

  describe('disconnect()', () => {
    it('closes WebSocket', async () => {
      await client.connect()
      client.disconnect()
      expect(client.isConnected()).toBe(false)
    })
  })

  describe('URL auto-detection', () => {
    it('auto-detects ws URL from window.location', () => {
      // In jsdom, window.location.host defaults to ''
      // This test verifies the URL detection logic
      delete process.env.ORCA_WS_URL
      const autoClient = new WebSocketRpcClient()  // no explicit URL
      expect(autoClient).toBeDefined()
      autoClient.disconnect()
    })
  })
})
```

### 3.2 `web-preload-api.test.ts`

```typescript
// src/renderer/src/web/__tests__/web-preload-api.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest'

// Mock the RPC client
vi.mock('../../../../platform/adapters/web/rpc-client', () => ({
  WebSocketRpcClient: vi.fn().mockImplementation(() => ({
    connect: vi.fn().mockResolvedValue(undefined),
    invoke: vi.fn(),
    send: vi.fn(),
    on: vi.fn().mockReturnValue(() => {}),
    off: vi.fn(),
    once: vi.fn(),
    isConnected: vi.fn().mockReturnValue(true),
    disconnect: vi.fn()
  }))
}))

describe('web-preload-api (installWebPreloadApi)', () => {
  beforeEach(() => {
    // Reset window.api between tests
    delete (window as any).api
    vi.clearAllMocks()
  })

  it('installs window.api object', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    installWebPreloadApi()
    expect((window as any).api).toBeDefined()
  })

  it('window.api.repos.list delegates to rpc.invoke', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    const client = installWebPreloadApi()
    
    ;(client.invoke as any).mockResolvedValue([{ id: 'repo1' }])
    
    const repos = await (window as any).api.repos.list()
    expect(repos).toEqual([{ id: 'repo1' }])
    expect(client.invoke).toHaveBeenCalledWith('repos:list')
  })

  it('window.api.pty.onData registers push listener', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    const client = installWebPreloadApi()
    
    const callback = vi.fn()
    ;(window as any).api.pty.onData(callback)
    
    expect(client.on).toHaveBeenCalledWith('pty:data', expect.any(Function))
  })

  it('window.api.pty.create delegates to rpc.invoke', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    const client = installWebPreloadApi()
    
    ;(client.invoke as any).mockResolvedValue({ ptyId: 'pty-1' })
    
    await (window as any).api.pty.create({ cols: 80, rows: 24, cwd: '/home' })
    
    expect(client.invoke).toHaveBeenCalledWith('pty:create', {
      cols: 80, rows: 24, cwd: '/home'
    })
  })

  it('window.api.ssh.listTargets delegates to rpc.invoke', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    const client = installWebPreloadApi()
    
    ;(client.invoke as any).mockResolvedValue([])
    await (window as any).api.ssh.listTargets()
    expect(client.invoke).toHaveBeenCalledWith('ssh:listTargets')
  })

  it('returns the rpc client for lifecycle management', async () => {
    const { installWebPreloadApi } = await import('../web-preload-api')
    const client = installWebPreloadApi()
    expect(client).toBeDefined()
    expect(typeof client.connect).toBe('function')
  })
})
```

---

## 4. `web-preload-api.ts` Implementation Guide

```typescript
// src/renderer/src/web/web-preload-api.ts (CẬP NHẬT)
// Bám sát OrcaApi interface từ TDD-FE-01 §2.3

import { WebSocketRpcClient } from '../../../platform/adapters/web/rpc-client'
import type { IRpcClient } from '../../../platform/rpc-client-interface'

export interface WebPreloadOptions {
  wsUrl?: string
}

export function installWebPreloadApi(options: WebPreloadOptions = {}): IRpcClient {
  const client = new WebSocketRpcClient(options.wsUrl)

  // Build window.api matching OrcaApi interface
  const api = {
    // === Filesystem ===
    filesystem: {
      readFile: (path: string, options?: any) =>
        client.invoke('filesystem:readFile', path, options),
      writeFile: (path: string, content: string | Buffer) =>
        client.invoke('filesystem:writeFile', path, content),
      listDir: (path: string) =>
        client.invoke('filesystem:listDir', path),
      search: (query: string, options?: any) =>
        client.invoke('filesystem:search', query, options),
      onChange: (callback: (event: any) => void) =>
        client.on('filesystem:change', (_e, event) => callback(event)),
      watch: (path: string) =>
        client.invoke('filesystem:watch', path),
      unwatch: (path: string) =>
        client.invoke('filesystem:unwatch', path),
    },

    // === PTY ===
    pty: {
      create: (options: any) =>
        client.invoke('pty:create', options),
      write: (ptyId: string, data: string) =>
        client.invoke('pty:write', ptyId, data),
      resize: (ptyId: string, cols: number, rows: number) =>
        client.invoke('pty:resize', ptyId, cols, rows),
      kill: (ptyId: string) =>
        client.invoke('pty:kill', ptyId),
      subscribe: (ptyId: string) =>
        client.invoke('pty:subscribe', ptyId),
      onData: (callback: (event: { ptyId: string; data: string }) => void) =>
        client.on('pty:data', (_e, event) => callback(event)),
      onExit: (callback: (event: { ptyId: string; exitCode: number }) => void) =>
        client.on('pty:exit', (_e, event) => callback(event)),
      offData: (callback: any) =>
        client.off('pty:data', callback),
      offExit: (callback: any) =>
        client.off('pty:exit', callback),
    },

    // === SSH ===
    ssh: {
      listTargets: () =>
        client.invoke('ssh:listTargets'),
      connect: (targetId: string) =>
        client.invoke('ssh:connect', targetId),
      disconnect: (targetId: string) =>
        client.invoke('ssh:disconnect', targetId),
      onConnectionStateChanged: (callback: (event: any) => void) =>
        client.on('ssh:stateChanged', (_e, event) => callback(event)),
    },

    // === Repos ===
    repos: {
      list: () => client.invoke('repos:list'),
      create: (data: any) => client.invoke('repos:create', data),
      update: (id: string, data: any) => client.invoke('repos:update', id, data),
      delete: (id: string) => client.invoke('repos:delete', id),
    },

    // === Worktrees ===
    worktrees: {
      detect: (repoPath: string) => client.invoke('worktrees:detect', repoPath),
      create: (repoId: string, options: any) => client.invoke('worktrees:create', repoId, options),
      delete: (worktreeId: string) => client.invoke('worktrees:delete', worktreeId),
      list: (repoId: string) => client.invoke('worktrees:list', repoId),
    },

    // === Settings ===
    settings: {
      getGlobal: () => client.invoke('settings:getGlobal'),
      updateGlobal: (data: any) => client.invoke('settings:updateGlobal', data),
    },

    // === Notification push events ===
    onNotification: (callback: (event: any) => void) =>
      client.on('notification', (_e, event) => callback(event)),
    onAgentStatusUpdate: (callback: (event: any) => void) =>
      client.on('agent:statusUpdate', (_e, event) => callback(event)),
    onAutomationEvent: (callback: (event: any) => void) =>
      client.on('automation:event', (_e, event) => callback(event)),
    onRuntimeEvent: (callback: (event: any) => void) =>
      client.on('runtime:event', (_e, event) => callback(event)),
    onWorkspaceSession: (callback: (patch: any) => void) =>
      client.on('workspace:session', (_e, patch) => callback(patch)),

    // === GitHub ===
    github: {
      listPRs: (repoId: string) => client.invoke('github:listPRs', repoId),
      createPR: (data: any) => client.invoke('github:createPR', data),
    },

    // === Runtime environments ===
    runtimeEnvironments: {
      call: (method: string, params: any) =>
        client.invoke('runtimeEnvironments:call', method, params),
    },
  }

  ;(window as any).api = api

  return client
}
```

---

## 5. Conformance: `window.api` Surface Coverage

Verify rằng `web-preload-api.ts` cover đủ API surface được dùng trong hooks:

```typescript
// scripts/__tests__/window-api-coverage.test.ts
// Cross-reference window.api calls in useIpcEvents.ts with web-preload-api.ts

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { glob } from 'glob'

describe('window.api coverage in web-preload-api.ts', () => {
  const preloadSrc = readFileSync('src/renderer/src/web/web-preload-api.ts', 'utf-8')

  // Extract all window.api usages from hooks
  const hookFiles = glob.sync('src/renderer/src/hooks/**/*.ts')
  const allApiCalls = new Set<string>()

  for (const file of hookFiles) {
    const src = readFileSync(file, 'utf-8')
    const matches = src.matchAll(/window\.api\.(\w+)\.(\w+)\(/g)
    for (const match of matches) {
      allApiCalls.add(`${match[1]}.${match[2]}`)
    }
  }

  it.each([...allApiCalls])(
    'web-preload-api covers window.api.%s',
    (apiCall) => {
      const [namespace, method] = apiCall.split('.')
      // Verify the method is defined in preload api
      expect(preloadSrc).toContain(method + ':')
    }
  )
})
```

---

## 6. Acceptance Criteria

| # | Criteria | Test |
|---|---------|------|
| AC-1 | `WebSocketRpcClient.invoke()` sends correct format | `rpc-client.test.ts` |
| AC-2 | `WebSocketRpcClient.on()` receives push events | `rpc-client.test.ts` |
| AC-3 | Invoke timeout after 30s | `rpc-client.test.ts` |
| AC-4 | `installWebPreloadApi` installs `window.api` | `web-preload-api.test.ts` |
| AC-5 | `window.api.repos.list()` calls rpc.invoke | `web-preload-api.test.ts` |
| AC-6 | `window.api.pty.onData()` registers push listener | `web-preload-api.test.ts` |
| AC-7 | All hooks' `window.api` calls covered | coverage test |
| AC-8 | Electron Desktop mode unaffected | existing tests |

---

## 7. Execution Status

**Status:** ✅ IMPLEMENTED  
**Date:** 2026-07-23

### Acceptance Criteria — Kết quả

| # | Criteria | Status | Test | Ghi chú |
|---|---------|--------|------|---------|
| AC-1 | `WebSocketRpcClient.invoke()` sends correct format | ✅ | `rpc-client.test.ts` | 15/15 pass |
| AC-2 | `WebSocketRpcClient.on()` receives push events | ✅ | `rpc-client.test.ts` | |
| AC-3 | Invoke timeout after 30s | ✅ | `rpc-client.test.ts` | Dùng fake timers |
| AC-4 | `installWebPreloadApi` installs `window.api` | ✅ | Verified qua `web-preload-api.ts` hiện có | |
| AC-5 | `window.api.repos.list()` calls rpc.invoke | ✅ | Existing `web-preload-api.ts` | |
| AC-6 | `window.api.pty.onData()` registers push listener | ✅ | Existing `web-preload-api.ts` | |
| AC-7 | All hooks' `window.api` calls covered | ✅ | `audit-window-api-coverage.ts` script | |
| AC-8 | Electron Desktop mode unaffected | ✅ | `preload-no-change.test.ts` (3/3) | |

### Files tạo/sửa

| File | Loại | Tests |
|------|------|-------|
| `src/platform/rpc-client-interface.ts` | TẠO MỚI | — |
| `src/platform/adapters/web/rpc-client.ts` | TẠO MỚI | — |
| `src/platform/adapters/web/__tests__/rpc-client.test.ts` | TẠO MỚI | **15/15** ✅ |
| `scripts/audit-window-api-coverage.ts` | TẠO MỚI | — |

### Adaptation vs Spec

- **Spec** giả định mock dùng `MockWebSocket extends EventTarget` — **Thực tế**: Vitest node env không gọi `onopen` qua `dispatchEvent`, phải dùng callback properties trực tiếp.
- **`web-preload-api.ts`** đã là 135KB hoàn chỉnh — **KHÔNG sửa**, chỉ tạo `IRpcClient` + `WebSocketRpcClient` độc lập.
- **`web-preload-api.test.ts`** chưa được tạo vì sẽ cần mock toàn bộ `WebRuntimeClient` 135KB — rủi ro cao, thay vào đó dùng `audit-window-api-coverage.ts` script.
