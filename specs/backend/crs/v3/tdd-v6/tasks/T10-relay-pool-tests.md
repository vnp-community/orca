# T10 — Write relay-connection-pool.test.ts

**Phase:** 2B  
**Effort:** ~1 hour  
**Depends on:** — (independent)  
**Solution ref:** [06-tdd19-project-workspace.md §2.1](../solutions/06-tdd19-project-workspace.md)  
**TDD ref:** TDD-19 (RelayConnectionPool)

---

## Mục tiêu

Viết tests cho `RelayConnectionPool` — pool lifecycle, ref counting, idle cleanup, reconnect.

**Target: ≥ 15 tests**

---

## Files Cần Đọc Trước

1. `src/main/dev-server/relay-connection-pool.ts` — đọc toàn bộ (126 lines)
2. `src/main/dev-server/dev-server-relay-bridge.ts` — xem interface (isAlive, disconnect, call)
3. `src/shared/dev-server-types.ts` — PersistedDevServer type

---

## File Cần Tạo

### `src/main/dev-server/__tests__/relay-connection-pool.test.ts`

```typescript
/**
 * Tests for RelayConnectionPool (TDD-19) — T10
 *
 * Uses vi.useFakeTimers() to control idle cleanup timers.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { RelayConnectionPool } from '../relay-connection-pool'
import type { PersistedDevServer } from '../../../shared/dev-server-types'

// ── Mock relay bridge ─────────────────────────────────────────────────────────

function makeMockBridge(alive = true) {
  return {
    isAlive: vi.fn().mockReturnValue(alive),
    disconnect: vi.fn().mockResolvedValue(undefined),
    call: vi.fn().mockResolvedValue({}),
  }
}

function makeServer(id = 'srv-001'): PersistedDevServer {
  return { id, name: 'Test', connectionType: 'direct-websocket', status: 'connected' } as PersistedDevServer
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('RelayConnectionPool', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  // ── getOrConnect ───────────────────────────────────────────────────────────
  describe('getOrConnect', () => {
    it('returns existing alive connection without re-connecting', async () => {
      const bridge = makeMockBridge(true)
      const connectFn = vi.fn().mockResolvedValue(bridge)
      const pool = new RelayConnectionPool(connectFn)

      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())

      expect(connectFn).toHaveBeenCalledTimes(1)
    })

    it('creates new connection when none exists', async () => {
      const bridge = makeMockBridge()
      const connectFn = vi.fn().mockResolvedValue(bridge)
      const pool = new RelayConnectionPool(connectFn)

      const result = await pool.getOrConnect('srv-001', makeServer())
      expect(connectFn).toHaveBeenCalledTimes(1)
      expect(result).toBe(bridge)
    })

    it('reconnects when existing connection is dead (isAlive = false)', async () => {
      const deadBridge = makeMockBridge(false)
      const aliveBridge = makeMockBridge(true)
      const connectFn = vi.fn()
        .mockResolvedValueOnce(deadBridge)
        .mockResolvedValueOnce(aliveBridge)

      const pool = new RelayConnectionPool(connectFn)

      await pool.getOrConnect('srv-001', makeServer())
      const result = await pool.getOrConnect('srv-001', makeServer())

      expect(connectFn).toHaveBeenCalledTimes(2)
      expect(result).toBe(aliveBridge)
    })

    it('cancels pending idle timer on re-acquire', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))

      await pool.getOrConnect('srv-001', makeServer())
      pool.release('srv-001') // starts idle timer
      await pool.getOrConnect('srv-001', makeServer()) // should cancel timer

      // Advance time past IDLE_CLEANUP_MS (5 minutes)
      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100)

      // Bridge should NOT have been disconnected
      expect(bridge.disconnect).not.toHaveBeenCalled()
    })

    it('increments ref count on each acquire', async () => {
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(makeMockBridge()))

      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())

      const status = pool.getStatus()
      expect(status['srv-001'].refCount).toBe(3)
    })
  })

  // ── release ────────────────────────────────────────────────────────────────
  describe('release', () => {
    it('decrements ref count', async () => {
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(makeMockBridge()))
      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())

      pool.release('srv-001')
      expect(pool.getStatus()['srv-001'].refCount).toBe(1)
    })

    it('does not disconnect immediately on release to 0', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))
      await pool.getOrConnect('srv-001', makeServer())

      pool.release('srv-001')
      expect(bridge.disconnect).not.toHaveBeenCalled()
    })

    it('schedules idle cleanup timer when count reaches 0', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))
      await pool.getOrConnect('srv-001', makeServer())
      pool.release('srv-001')

      // Advance past 5-minute idle timeout
      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100)
      expect(bridge.disconnect).toHaveBeenCalledTimes(1)
    })

    it('multiple users same server — cleanup only after ALL released', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))

      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())

      pool.release('srv-001') // count: 2 → 1
      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100)
      expect(bridge.disconnect).not.toHaveBeenCalled() // still 1 ref

      pool.release('srv-001') // count: 1 → 0
      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100)
      expect(bridge.disconnect).toHaveBeenCalledTimes(1)
    })

    it('timer cancelled if re-acquired before idle timeout', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))

      await pool.getOrConnect('srv-001', makeServer())
      pool.release('srv-001') // starts timer

      // Re-acquire BEFORE timeout
      await vi.advanceTimersByTimeAsync(60_000) // 1 min
      await pool.getOrConnect('srv-001', makeServer()) // cancels timer

      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100) // advance past original timeout
      expect(bridge.disconnect).not.toHaveBeenCalled()
    })
  })

  // ── disconnectAll ─────────────────────────────────────────────────────────
  describe('disconnectAll', () => {
    it('disconnects all active connections', async () => {
      const b1 = makeMockBridge()
      const b2 = makeMockBridge()
      let call = 0
      const pool = new RelayConnectionPool(vi.fn().mockImplementation(() =>
        Promise.resolve(call++ === 0 ? b1 : b2)
      ))
      await pool.getOrConnect('srv-001', makeServer('srv-001'))
      await pool.getOrConnect('srv-002', makeServer('srv-002'))

      await pool.disconnectAll()
      expect(b1.disconnect).toHaveBeenCalled()
      expect(b2.disconnect).toHaveBeenCalled()
    })

    it('cancels all idle timers during disconnectAll', async () => {
      const bridge = makeMockBridge()
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(bridge))
      await pool.getOrConnect('srv-001', makeServer())
      pool.release('srv-001') // timer started

      await pool.disconnectAll()
      // Timer should be cancelled — even if we advance past it, disconnect not called again
      bridge.disconnect.mockClear()
      await vi.advanceTimersByTimeAsync(5 * 60 * 1000 + 100)
      expect(bridge.disconnect).not.toHaveBeenCalled()
    })

    it('clears internal maps after disconnectAll', async () => {
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(makeMockBridge()))
      await pool.getOrConnect('srv-001', makeServer())
      await pool.disconnectAll()
      expect(pool.getStatus()).toEqual({})
    })
  })

  // ── getStatus ─────────────────────────────────────────────────────────────
  describe('getStatus', () => {
    it('returns refCount and alive status for each connection', async () => {
      const pool = new RelayConnectionPool(vi.fn().mockResolvedValue(makeMockBridge(true)))
      await pool.getOrConnect('srv-001', makeServer())
      await pool.getOrConnect('srv-001', makeServer())

      const status = pool.getStatus()
      expect(status['srv-001']).toEqual({ refCount: 2, alive: true })
    })

    it('returns empty object when no connections', () => {
      const pool = new RelayConnectionPool(vi.fn())
      expect(pool.getStatus()).toEqual({})
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/dev-server/__tests__/relay-connection-pool.test.ts` ✅
- [x] `pnpm vitest run src/main/dev-server/__tests__/relay-connection-pool.test.ts` → ≥15 tests passing ✅ (15 tests pass)
- [x] 0 TypeScript errors ✅
- [x] `vi.useFakeTimers()` được dùng để control idle timer ✅ (lines 130, 142, 157)
