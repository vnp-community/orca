# TASK-016: Tạo `src/main/session/ws-session-router.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §4.3, §3.2
**Depends on:** TASK-008 (AuthManager), TASK-014 (SessionManager)
**Blocks:** TASK-018, TASK-019

---

## Mục tiêu

Tạo `WsSessionRouter` — intercept WebSocket connections, resolve userId từ session cookie, proxy WS traffic đến user Unix socket.

---

## File cần tạo: `src/main/session/ws-session-router.ts`

```typescript
// src/main/session/ws-session-router.ts
import * as net from 'node:net'
import type { IncomingMessage } from 'node:http'
import type { WebSocket } from 'ws'
import type { SessionManager } from './session-manager'
import type { AuthManager } from '../auth/auth-manager'

export class WsSessionRouter {
  private readonly sessionManager: SessionManager
  private readonly authManager:    AuthManager

  constructor(opts: { sessionManager: SessionManager; authManager: AuthManager }) {
    this.sessionManager = opts.sessionManager
    this.authManager    = opts.authManager
  }

  resolveUserFromRequest(req: IncomingMessage): string | null {
    const session = this.authManager.validateRequest(req.headers.cookie)
    return session?.userId ?? null
  }

  async getOrCreateUserSocket(userId: string): Promise<string> {
    const proc = await this.sessionManager.getOrSpawnUserProcess(userId)
    return proc.socketPath
  }

  /**
   * Main entry — gọi từ WebSocket server 'connection' event.
   * Nếu có login session: proxy WS → user process Unix socket.
   * Nếu không có session: close với 4401 (require auth).
   */
  async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    const userId = this.resolveUserFromRequest(req)

    if (!userId) {
      // Không có login session → reject
      // NOTE: PairCode / deviceToken connections KHÔNG đi qua router này
      // Họ connect trực tiếp vào shared runtime (legacy path — ORCA_MULTI_USER=0)
      ws.close(4401, 'Authentication required. Please log in first.')
      return
    }

    this.sessionManager.touch(userId)

    let socketPath: string
    try {
      socketPath = await this.getOrCreateUserSocket(userId)
    } catch (err) {
      console.error(`[WsSessionRouter] Failed to spawn process: userId=${userId}`, err)
      ws.close(1011, 'Internal error: cannot start user session')
      return
    }

    // Proxy WS ↔ Unix socket (binary-safe)
    const upstream = net.createConnection(socketPath)

    upstream.on('error', (err) => {
      console.error(`[WsSessionRouter] Upstream error: userId=${userId}`, err)
      if (ws.readyState === ws.OPEN) ws.close(1011, 'User session unavailable')
    })

    ws.on('message', (data: Buffer | string, isBinary: boolean) => {
      if (upstream.writable) {
        upstream.write(isBinary ? data : Buffer.from(data as string))
      }
    })

    upstream.on('data', (chunk: Buffer) => {
      if (ws.readyState === ws.OPEN) ws.send(chunk)
    })

    ws.on('close', () => {
      upstream.end()
      this.sessionManager.touch(userId)
    })

    upstream.on('close', () => {
      if (ws.readyState === ws.OPEN) ws.close(1011, 'User session ended')
    })
  }
}
```

---

## File cần tạo: `src/main/session/__tests__/ws-session-router.test.ts`

```typescript
// src/main/session/__tests__/ws-session-router.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { WsSessionRouter } from '../ws-session-router'
import type { SessionManager } from '../session-manager'
import type { AuthManager } from '../../auth/auth-manager'

describe('WsSessionRouter', () => {
  let router: WsSessionRouter
  let sessionManager: SessionManager
  let authManager: AuthManager

  const mockSession = {
    sessionId: 'sid-1', userId: 'user-alice', userEmail: 'alice@test.com',
    role: 'developer' as const, createdAt: Date.now(), expiresAt: Date.now() + 28800000,
    lastSeenAt: null, ipAddress: '127.0.0.1', userAgent: 'ua'
  }

  beforeEach(() => {
    sessionManager = {
      getOrSpawnUserProcess: vi.fn().mockResolvedValue({
        userId: 'user-alice', pid: 1234,
        socketPath: '/data/orca/users/user-alice/orca.sock',
        startedAt: Date.now(), lastSeenAt: Date.now(),
        process: null, respawnCount: 0
      }),
      touch: vi.fn()
    } as any

    authManager = {
      validateRequest: vi.fn()
    } as any

    router = new WsSessionRouter({ sessionManager, authManager })
  })

  describe('resolveUserFromRequest', () => {
    it('returns userId for valid session cookie', () => {
      vi.mocked(authManager.validateRequest).mockReturnValue(mockSession)
      const userId = router.resolveUserFromRequest({ headers: { cookie: 'orca_session=abc' } } as any)
      expect(userId).toBe('user-alice')
    })

    it('returns null for missing cookie', () => {
      vi.mocked(authManager.validateRequest).mockReturnValue(null)
      const userId = router.resolveUserFromRequest({ headers: {} } as any)
      expect(userId).toBeNull()
    })

    it('returns null for expired/invalid session', () => {
      vi.mocked(authManager.validateRequest).mockReturnValue(null)
      const userId = router.resolveUserFromRequest({ headers: { cookie: 'orca_session=bad' } } as any)
      expect(userId).toBeNull()
    })
  })

  describe('getOrCreateUserSocket', () => {
    it('calls sessionManager.getOrSpawnUserProcess with userId', async () => {
      const socketPath = await router.getOrCreateUserSocket('user-bob')
      expect(sessionManager.getOrSpawnUserProcess).toHaveBeenCalledWith('user-bob')
      expect(socketPath).toBe('/data/orca/users/user-alice/orca.sock')
    })
  })

  describe('handleConnection', () => {
    it('closes with 4401 when no session', async () => {
      vi.mocked(authManager.validateRequest).mockReturnValue(null)
      const ws: any = { readyState: 1, close: vi.fn(), on: vi.fn() }
      await router.handleConnection(ws, { headers: {} } as any)
      expect(ws.close).toHaveBeenCalledWith(4401, expect.stringContaining('Authentication'))
      expect(sessionManager.getOrSpawnUserProcess).not.toHaveBeenCalled()
    })

    it('touches session on WS close', async () => {
      vi.mocked(authManager.validateRequest).mockReturnValue(mockSession)
      vi.mocked(sessionManager.getOrSpawnUserProcess).mockResolvedValue({
        userId: 'user-alice', pid: 1234,
        socketPath: '/tmp/test-alice.sock',
        startedAt: Date.now(), lastSeenAt: Date.now(),
        process: null as any, respawnCount: 0
      })

      // Mock net.createConnection
      const upstreamMock = { on: vi.fn(), write: vi.fn(), end: vi.fn(), writable: true }
      const netMock = await import('node:net')
      vi.spyOn(netMock, 'createConnection').mockReturnValue(upstreamMock as any)

      let wsCloseHandler: (() => void) | null = null
      const ws: any = {
        readyState: 1,
        OPEN: 1,
        close: vi.fn(),
        send: vi.fn(),
        on: vi.fn((event: string, cb: (...args: any[]) => void) => {
          if (event === 'close') wsCloseHandler = cb
        })
      }

      await router.handleConnection(ws, { headers: { cookie: 'orca_session=abc' } } as any)

      // Simulate WS close
      wsCloseHandler?.()
      expect(sessionManager.touch).toHaveBeenCalledWith('user-alice')
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/session/__tests__/ws-session-router.test.ts
```

---

## Acceptance Criteria

- [x] `ws-session-router.ts` tồn tại, TypeScript compile sạch
- [x] `resolveUserFromRequest()` → userId hoặc null
- [x] `handleConnection()` close 4401 khi không có session
- [x] `handleConnection()` gọi `sessionManager.touch()` khi WS close
- [x] Test: tất cả test cases pass (≥ 6 cases)
