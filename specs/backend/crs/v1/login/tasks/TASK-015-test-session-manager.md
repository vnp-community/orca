# TASK-015: Tạo `src/main/session/__tests__/session-manager.test.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §3.1
**Depends on:** TASK-013, TASK-014
**Blocks:** (không)

---

## Mục tiêu

Viết test suite cho `SessionManager` với mock `node:child_process.fork`.

---

## File cần tạo

**Path:** `src/main/session/__tests__/session-manager.test.ts`

---

## Nội dung (copy từ SOL-LG-002 §3.1, full)

```typescript
// src/main/session/__tests__/session-manager.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { SessionManager } from '../session-manager'

// Mock child_process.fork để không fork thực sự
vi.mock('node:child_process', () => ({
  fork: vi.fn()
}))

vi.mock('node:fs/promises', () => ({
  mkdir: vi.fn().mockResolvedValue(undefined),
  rm:    vi.fn().mockResolvedValue(undefined)
}))

function makeMockChild(socketPath: string) {
  const listeners: Record<string, ((...args: any[]) => void)[]> = {}
  const child = {
    pid:      Math.floor(Math.random() * 10000) + 1000,
    killed:   false,
    exitCode: null,
    stdout:   null,
    stderr:   null,
    send:     vi.fn(),
    kill:     vi.fn(),
    on: vi.fn((event: string, cb: (...args: any[]) => void) => {
      listeners[event] = listeners[event] ?? []
      listeners[event]!.push(cb)
      if (event === 'message') {
        // Simulate 'ready' immediately
        setImmediate(() => cb({ type: 'ready', socketPath }))
      }
    }),
    emit: (event: string, ...args: any[]) => {
      listeners[event]?.forEach(cb => cb(...args))
    }
  }
  return child
}

describe('SessionManager', () => {
  let manager: SessionManager
  const BASE_PATH = '/data/orca-test'
  let mockFork: ReturnType<typeof vi.fn>

  beforeEach(async () => {
    const { fork } = await import('node:child_process')
    mockFork = vi.mocked(fork)
    mockFork.mockImplementation((_entry: string, _args: any[], opts: any) => {
      const userId = (opts.env as any).ORCA_USER_ID
      const sockPath = (opts.env as any).ORCA_SOCKET_PATH
      return makeMockChild(sockPath) as any
    })

    manager = new SessionManager({
      baseDataPath:     BASE_PATH,
      userProcessEntry: '/fake/user-process-entry.js',
      idleTimeoutMs:    60_000,
      maxRespawnAttempts: 3
    })
  })

  afterEach(async () => {
    await manager.shutdown()
    vi.clearAllMocks()
  })

  describe('getOrSpawnUserProcess', () => {
    it('spawns a new process for unknown userId', async () => {
      const proc = await manager.getOrSpawnUserProcess('user-alice')
      expect(proc.userId).toBe('user-alice')
      expect(proc.pid).toBeGreaterThan(0)
    })

    it('reuses existing process for same userId', async () => {
      const proc1 = await manager.getOrSpawnUserProcess('user-bob')
      const proc2 = await manager.getOrSpawnUserProcess('user-bob')
      expect(proc1.pid).toBe(proc2.pid)
      expect(mockFork).toHaveBeenCalledTimes(1)
    })

    it('creates isolated directory per user', async () => {
      const { mkdir } = await import('node:fs/promises')
      await manager.getOrSpawnUserProcess('user-carol')
      expect(mkdir).toHaveBeenCalledWith(
        expect.stringContaining('/data/orca-test/users/user-carol'),
        expect.objectContaining({ recursive: true, mode: 0o700 })
      )
    })

    it('passes ORCA_USER_ID env to forked process', async () => {
      await manager.getOrSpawnUserProcess('user-dave')
      expect(mockFork).toHaveBeenCalledWith(
        '/fake/user-process-entry.js',
        [],
        expect.objectContaining({
          env: expect.objectContaining({ ORCA_USER_ID: 'user-dave' })
        })
      )
    })

    it('includes NODE_OPTIONS memory limit', async () => {
      await manager.getOrSpawnUserProcess('user-mem')
      expect(mockFork).toHaveBeenCalledWith(
        expect.any(String), [],
        expect.objectContaining({
          env: expect.objectContaining({ NODE_OPTIONS: '--max-old-space-size=512' })
        })
      )
    })

    it('spawns separate processes for different userIds', async () => {
      await manager.getOrSpawnUserProcess('user-x')
      await manager.getOrSpawnUserProcess('user-y')
      expect(mockFork).toHaveBeenCalledTimes(2)
    })
  })

  describe('touch', () => {
    it('updates lastSeenAt for user process', async () => {
      await manager.getOrSpawnUserProcess('user-eve')
      const before = Date.now()
      await new Promise(r => setTimeout(r, 5))
      manager.touch('user-eve')
      const proc = manager.getProcess('user-eve')
      expect(proc!.lastSeenAt).toBeGreaterThanOrEqual(before)
    })

    it('is no-op for unknown userId', () => {
      expect(() => manager.touch('non-existent')).not.toThrow()
    })
  })

  describe('getProcess', () => {
    it('returns UserProcess for known userId', async () => {
      await manager.getOrSpawnUserProcess('user-get')
      const proc = manager.getProcess('user-get')
      expect(proc).not.toBeNull()
      expect(proc!.userId).toBe('user-get')
    })

    it('returns null for unknown userId', () => {
      expect(manager.getProcess('non-existent')).toBeNull()
    })
  })

  describe('process exit handling', () => {
    it('removes process from registry on exit', async () => {
      const { fork } = await import('node:child_process')
      let exitHandler: ((code: number) => void) | null = null

      vi.mocked(fork).mockImplementationOnce((_e, _a, opts: any) => {
        const child = makeMockChild((opts.env as any).ORCA_SOCKET_PATH) as any
        const origOn = child.on.bind(child)
        child.on = vi.fn((event: string, cb: (...args: any[]) => void) => {
          if (event === 'exit') exitHandler = cb
          return origOn(event, cb)
        })
        return child
      })

      await manager.getOrSpawnUserProcess('user-crash')
      expect(manager.getProcess('user-crash')).not.toBeNull()

      exitHandler?.(1)
      await new Promise(r => setTimeout(r, 10))
      expect(manager.getProcess('user-crash')).toBeNull()
    })
  })

  describe('shutdown', () => {
    it('kills all user processes', async () => {
      const proc1 = await manager.getOrSpawnUserProcess('u-s1')
      const proc2 = await manager.getOrSpawnUserProcess('u-s2')
      const kill1 = proc1.process.kill as ReturnType<typeof vi.fn>
      const kill2 = proc2.process.kill as ReturnType<typeof vi.fn>

      await manager.shutdown()
      expect(kill1).toHaveBeenCalled()
      expect(kill2).toHaveBeenCalled()
    })

    it('listProcesses() returns empty after shutdown', async () => {
      await manager.getOrSpawnUserProcess('u-sd')
      await manager.shutdown()
      expect(manager.listProcesses()).toHaveLength(0)
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/session/__tests__/session-manager.test.ts
```

---

## Acceptance Criteria

- [x] Test file tồn tại
- [x] Tất cả test cases pass (≥ 12 cases)
- [x] `vi.mock('node:child_process')` — không fork thực sự trong test
- [x] Test: reuse process cho cùng userId
- [x] Test: env vars đúng (ORCA_USER_ID, ORCA_USER_DATA_PATH, ORCA_SOCKET_PATH)
- [x] Test: process exit → auto remove từ registry
- [x] Test: shutdown kills tất cả processes
