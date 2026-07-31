# TASK-002: RelayConnectionPool

**Phase:** 1 — Foundation  
**Solution ref:** [SOL-V5-006](../solutions/SOL-V5-006-project-workspace.md) §2  
**Prerequisite:** None (độc lập)  
**Status:** ✅ DONE — 2026-07-28

> **Kết quả:** `relay-connection-pool.ts` + `isAlive()` added to `DevServerRelayBridge`. 15/15 tests pass.


---

## Mô tả

Tạo `RelayConnectionPool` — singleton class quản lý relay connections với ref-counting và idle cleanup. Đây là prerequisite cho TDD-15 (ProjectServerRouter), TDD-16 (AIProviderService), TDD-19 (WorkspaceService).

---

## File cần tạo: `src/main/dev-server/relay-connection-pool.ts`

```typescript
import type { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { PersistedDevServer } from '../../shared/dev-server-types'

const IDLE_CLEANUP_MS = 5 * 60 * 1000  // 5 minutes

/**
 * RelayConnectionPool — manages reuse of DevServerRelayBridge connections.
 *
 * Ref-counting: multiple callers can hold a reference to the same relay.
 * When all refs are released, connection is kept alive for IDLE_CLEANUP_MS
 * then disconnected automatically.
 *
 * Usage:
 *   const relay = await pool.getOrConnect(devServerId, server)
 *   // use relay...
 *   pool.release(devServerId)
 */
export class RelayConnectionPool {
  private readonly connections = new Map<string, DevServerRelayBridge>()
  private readonly refCounts = new Map<string, number>()
  private readonly idleTimers = new Map<string, ReturnType<typeof setTimeout>>()

  constructor(
    private readonly connectFn: (server: PersistedDevServer) => Promise<DevServerRelayBridge>
  ) {}

  async getOrConnect(devServerId: string, server: PersistedDevServer): Promise<DevServerRelayBridge> {
    // Cancel pending idle cleanup
    const timer = this.idleTimers.get(devServerId)
    if (timer) {
      clearTimeout(timer)
      this.idleTimers.delete(devServerId)
    }

    const existing = this.connections.get(devServerId)
    if (existing?.isAlive()) {
      this.refCounts.set(devServerId, (this.refCounts.get(devServerId) ?? 0) + 1)
      return existing
    }

    // Remove stale dead connection
    if (existing) {
      this.connections.delete(devServerId)
    }

    const relay = await this.connectFn(server)
    this.connections.set(devServerId, relay)
    this.refCounts.set(devServerId, 1)
    return relay
  }

  release(devServerId: string): void {
    const count = Math.max(0, (this.refCounts.get(devServerId) ?? 0) - 1)
    this.refCounts.set(devServerId, count)

    if (count === 0) {
      const timer = setTimeout(() => {
        this.connections.get(devServerId)?.disconnect()
        this.connections.delete(devServerId)
        this.refCounts.delete(devServerId)
        this.idleTimers.delete(devServerId)
      }, IDLE_CLEANUP_MS)
      this.idleTimers.set(devServerId, timer)
    }
  }

  async disconnectAll(): Promise<void> {
    for (const [, relay] of this.connections) {
      try { await relay.disconnect() } catch { /* ignore */ }
    }
    this.connections.clear()
    this.refCounts.clear()
    for (const timer of this.idleTimers.values()) clearTimeout(timer)
    this.idleTimers.clear()
  }

  getStatus(): Record<string, { refCount: number; alive: boolean }> {
    return Object.fromEntries(
      [...this.connections.entries()].map(([id, relay]) => [
        id,
        { refCount: this.refCounts.get(id) ?? 0, alive: relay.isAlive() }
      ])
    )
  }
}
```

---

## File cần tạo: `src/main/dev-server/__tests__/relay-connection-pool.test.ts`

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { RelayConnectionPool } from '../relay-connection-pool'
import type { DevServerRelayBridge } from '../dev-server-relay-bridge'
import type { PersistedDevServer } from '../../../shared/dev-server-types'

function makeMockRelay(alive = true): DevServerRelayBridge {
  return {
    isAlive: vi.fn(() => alive),
    disconnect: vi.fn().mockResolvedValue(undefined),
    call: vi.fn(),
  } as unknown as DevServerRelayBridge
}

const fakeServer = { id: 'srv-1' } as PersistedDevServer

describe('RelayConnectionPool', () => {
  let connectFn: ReturnType<typeof vi.fn>
  let pool: RelayConnectionPool

  beforeEach(() => {
    connectFn = vi.fn()
    pool = new RelayConnectionPool(connectFn)
  })

  it('should connect on first call', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    const result = await pool.getOrConnect('srv-1', fakeServer)
    expect(connectFn).toHaveBeenCalledTimes(1)
    expect(result).toBe(relay)
  })

  it('should reuse alive connection on second call', async () => {
    const relay = makeMockRelay(true)
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)
    expect(connectFn).toHaveBeenCalledTimes(1)
  })

  it('should reconnect if existing connection is dead', async () => {
    const dead = makeMockRelay(false)
    const alive = makeMockRelay(true)
    connectFn.mockResolvedValueOnce(dead).mockResolvedValueOnce(alive)

    await pool.getOrConnect('srv-1', fakeServer)
    const result = await pool.getOrConnect('srv-1', fakeServer)
    expect(connectFn).toHaveBeenCalledTimes(2)
    expect(result).toBe(alive)
  })

  it('should decrement ref count on release', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')
    expect(pool.getStatus()['srv-1'].refCount).toBe(1)
  })

  it('should schedule cleanup when all refs released', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')

    // Should not disconnect immediately
    expect(relay.disconnect).not.toHaveBeenCalled()

    // After idle timeout
    vi.advanceTimersByTime(5 * 60 * 1000 + 100)
    expect(relay.disconnect).toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('should not cleanup if new connection acquired during idle window', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')

    // Re-acquire before timeout
    vi.advanceTimersByTime(2 * 60 * 1000)
    await pool.getOrConnect('srv-1', fakeServer)
    vi.advanceTimersByTime(5 * 60 * 1000)

    expect(relay.disconnect).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('should disconnectAll close all connections', async () => {
    const r1 = makeMockRelay()
    const r2 = makeMockRelay()
    connectFn.mockResolvedValueOnce(r1).mockResolvedValueOnce(r2)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-2', { id: 'srv-2' } as PersistedDevServer)
    await pool.disconnectAll()

    expect(r1.disconnect).toHaveBeenCalled()
    expect(r2.disconnect).toHaveBeenCalled()
    expect(pool.getStatus()).toEqual({})
  })

  it('getStatus returns correct alive/refCount', async () => {
    const relay = makeMockRelay(true)
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)
    const status = pool.getStatus()

    expect(status['srv-1']).toEqual({ refCount: 2, alive: true })
  })
})
```

---

## Verification

```bash
pnpm test --run src/main/dev-server/__tests__/relay-connection-pool.test.ts
```

## Acceptance Criteria

- [x] `relay-connection-pool.ts` tạo thành công
- [x] `getOrConnect` reuse alive connection ✓
- [x] `getOrConnect` reconnect nếu dead ✓
- [x] `release` ref count decrement ✓
- [x] Idle cleanup sau 5 min ✓
- [x] `disconnectAll` đóng tất cả ✓
- [x] **≥ 8 tests pass**
