/**
 * WsSessionRouter Unit Tests
 *
 * Uses mocked AuthManager, SessionManager, and node:net — no real sockets.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock node:net at module level (required for ESM)
const upstreamMock = {
  on: vi.fn(),
  write: vi.fn(),
  end: vi.fn(),
  writable: true
}
vi.mock('node:net', () => ({
  createConnection: vi.fn(() => upstreamMock)
}))

// Mock readRuntimeMetadata so tests don't need real filesystem metadata files.
// Returns a valid unix transport so the router proceeds to create the upstream
// connection instead of closing early with 1011.
// Why path '../../runtime/runtime-metadata': test is in session/__tests__/,
// the module lives at main/runtime/runtime-metadata — two levels up.
vi.mock('../../runtime/runtime-metadata', () => ({
  readRuntimeMetadata: vi.fn(() => ({
    authToken: 'test-token',
    transports: [{ kind: 'unix', endpoint: '/tmp/orca-test/users/user-alice/orca.sock' }]
  }))
}))

import { WsSessionRouter } from '../ws-session-router'
import type { SessionManager } from '../session-manager'
import type { AuthManager } from '../../auth/auth-manager'

const mockSession = {
  sessionId:   'sid-abc',
  userId:      'user-alice',
  userEmail:   'alice@test.com',
  role:        'developer' as const,
  createdAt:   Date.now(),
  expiresAt:   Date.now() + 28800000,
  lastSeenAt:  null,
  ipAddress:   '127.0.0.1',
  userAgent:   'vitest'
}

describe('WsSessionRouter', () => {
  let router:         WsSessionRouter
  let sessionManager: SessionManager
  let authManager:    AuthManager

  beforeEach(() => {
    // Reset mocks
    vi.clearAllMocks()
    upstreamMock.on   = vi.fn()
    upstreamMock.write = vi.fn()
    upstreamMock.end  = vi.fn()

    sessionManager = {
      getOrSpawnUserProcess: vi.fn().mockResolvedValue({
        userId:       'user-alice',
        pid:          1234,
        socketPath:   '/data/orca/users/user-alice/orca.sock',
        startedAt:    Date.now(),
        lastSeenAt:   Date.now(),
        process:      null,
        respawnCount: 0
      }),
      touch: vi.fn(),
      // Why: WsSessionRouter accesses (sessionManager as any).config.baseDataPath
      // to read RuntimeMetadata. Provide the config stub so the access doesn't throw.
      config: { baseDataPath: '/tmp/orca-test' }
    } as unknown as SessionManager

    authManager = {
      validateRequest: vi.fn().mockResolvedValue(null)
    } as unknown as AuthManager

    router = new WsSessionRouter({ sessionManager, authManager })
  })

  // ── resolveUserFromRequest ─────────────────────────────────────────────────

  describe('resolveUserFromRequest', () => {
    it('returns userId for valid session cookie', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(mockSession)
      const userId = await router.resolveUserFromRequest({ headers: { cookie: 'orca_session=abc' } } as any)
      expect(userId).toBe('user-alice')
    })

    it('returns null for missing cookie', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(null)
      const userId = await router.resolveUserFromRequest({ headers: {} } as any)
      expect(userId).toBeNull()
    })

    it('returns null for expired/invalid session', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(null)
      const userId = await router.resolveUserFromRequest({ headers: { cookie: 'orca_session=bad' } } as any)
      expect(userId).toBeNull()
    })
  })

  // ── getOrCreateUserSocket ──────────────────────────────────────────────────

  describe('getOrCreateUserSocket', () => {
    it('calls sessionManager.getOrSpawnUserProcess with userId', async () => {
      const socketPath = await router.getOrCreateUserSocket('user-bob')
      expect(sessionManager.getOrSpawnUserProcess).toHaveBeenCalledWith('user-bob')
      expect(socketPath).toBe('/data/orca/users/user-alice/orca.sock')
    })
  })

  // ── handleConnection ───────────────────────────────────────────────────────

  describe('handleConnection', () => {
    it('closes with 4401 when no session', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(null)
      const ws: any = { readyState: 1, OPEN: 1, close: vi.fn(), on: vi.fn(), send: vi.fn() }

      await router.handleConnection(ws, { headers: {} } as any)
      expect(ws.close).toHaveBeenCalledWith(4401, expect.stringContaining('Authentication'))
      expect(sessionManager.getOrSpawnUserProcess).not.toHaveBeenCalled()
    })

    it('closes with 1011 when spawn fails', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(mockSession)
      vi.mocked(sessionManager.getOrSpawnUserProcess).mockRejectedValue(new Error('spawn error'))

      const ws: any = { readyState: 1, OPEN: 1, close: vi.fn(), on: vi.fn(), send: vi.fn() }
      await router.handleConnection(ws, { headers: { cookie: 'orca_session=abc' } } as any)
      expect(ws.close).toHaveBeenCalledWith(1011, expect.stringContaining('Internal error'))
    })

    it('touches session on WS close', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(mockSession)

      let wsCloseHandler: (() => void) | null = null
      const ws: any = {
        readyState: 1,
        OPEN: 1,
        close: vi.fn(),
        send: vi.fn(),
        on: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
          if (event === 'close') wsCloseHandler = cb
        })
      }

      await router.handleConnection(ws, { headers: { cookie: 'orca_session=abc' } } as any)

      // Simulate WS close
      wsCloseHandler?.()
      expect(sessionManager.touch).toHaveBeenCalledWith('user-alice')
      expect(upstreamMock.end).toHaveBeenCalled()
    })

    it('touches session before proxying (at start of handleConnection)', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(mockSession)
      const ws: any = { readyState: 1, OPEN: 1, close: vi.fn(), send: vi.fn(), on: vi.fn() }
      await router.handleConnection(ws, { headers: { cookie: 'x' } } as any)
      // touch() called once at connection start
      expect(sessionManager.touch).toHaveBeenCalledWith('user-alice')
    })
  })
})
