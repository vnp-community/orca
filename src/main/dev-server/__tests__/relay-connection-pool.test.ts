/**
 * Tests for RelayConnectionPool
 *
 * @module main/dev-server/__tests__/relay-connection-pool.test
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { RelayConnectionPool } from '../relay-connection-pool'
import type { DevServerRelayBridge } from '../dev-server-relay-bridge'
import type { PersistedDevServer } from '../../../shared/dev-server-types'

// ── helpers ────────────────────────────────────────────────────────────────

function makeMockRelay(alive = true): DevServerRelayBridge {
  return {
    isAlive: vi.fn(() => alive),
    disconnect: vi.fn().mockResolvedValue(undefined),
    call: vi.fn(),
    connect: vi.fn(),
  } as unknown as DevServerRelayBridge
}

const fakeServer = { id: 'srv-1' } as PersistedDevServer
const fakeServer2 = { id: 'srv-2' } as PersistedDevServer

// ── tests ──────────────────────────────────────────────────────────────────

describe('RelayConnectionPool', () => {
  let connectFn: ReturnType<typeof vi.fn>
  let pool: RelayConnectionPool

  beforeEach(() => {
    connectFn = vi.fn()
    pool = new RelayConnectionPool(connectFn)
  })

  afterEach(async () => {
    await pool.disconnectAll()
    vi.useRealTimers()
  })

  // ── connection establishment ───────────────────────────────────────────────

  it('getOrConnect: calls connectFn on first request', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    const result = await pool.getOrConnect('srv-1', fakeServer)

    expect(connectFn).toHaveBeenCalledTimes(1)
    expect(connectFn).toHaveBeenCalledWith(fakeServer)
    expect(result).toBe(relay)
  })

  it('getOrConnect: reuses alive connection on second call', async () => {
    const relay = makeMockRelay(true)
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    const result = await pool.getOrConnect('srv-1', fakeServer)

    expect(connectFn).toHaveBeenCalledTimes(1)
    expect(result).toBe(relay)
  })

  it('getOrConnect: reconnects if existing connection is dead', async () => {
    const dead = makeMockRelay(false)
    const alive = makeMockRelay(true)
    connectFn.mockResolvedValueOnce(dead).mockResolvedValueOnce(alive)

    await pool.getOrConnect('srv-1', fakeServer)
    const result = await pool.getOrConnect('srv-1', fakeServer)

    expect(connectFn).toHaveBeenCalledTimes(2)
    expect(result).toBe(alive)
  })

  it('getOrConnect: tracks different servers independently', async () => {
    const r1 = makeMockRelay()
    const r2 = makeMockRelay()
    connectFn.mockResolvedValueOnce(r1).mockResolvedValueOnce(r2)

    const result1 = await pool.getOrConnect('srv-1', fakeServer)
    const result2 = await pool.getOrConnect('srv-2', fakeServer2)

    expect(result1).toBe(r1)
    expect(result2).toBe(r2)
    expect(connectFn).toHaveBeenCalledTimes(2)
  })

  // ── ref counting ──────────────────────────────────────────────────────────

  it('getOrConnect: increments ref count on each call', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)

    expect(pool.getStatus()['srv-1'].refCount).toBe(3)
  })

  it('release: decrements ref count', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')

    expect(pool.getStatus()['srv-1'].refCount).toBe(1)
  })

  it('release: ref count does not go below 0', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1') // refCount → 0, timer starts
    pool.release('srv-1') // should not error or go negative

    // Pool entry still exists (timer hasn't fired yet)
    expect(pool.getStatus()['srv-1']?.refCount).toBe(0)
  })

  // ── idle cleanup ──────────────────────────────────────────────────────────

  it('release: schedules cleanup after all refs released (does not disconnect immediately)', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')

    // Disconnect should not be called immediately
    expect(relay.disconnect).not.toHaveBeenCalled()
  })

  it('release: disconnects after idle timeout fires', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1')

    vi.advanceTimersByTime(5 * 60 * 1000 + 100)
    await vi.runAllTimersAsync()

    expect(relay.disconnect).toHaveBeenCalledTimes(1)
    expect(pool.getStatus()['srv-1']).toBeUndefined()
  })

  it('getOrConnect: cancels pending idle timer on re-acquire', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1') // starts idle timer

    // Advance part-way through idle window
    vi.advanceTimersByTime(2 * 60 * 1000)

    // Re-acquire — should cancel the timer
    await pool.getOrConnect('srv-1', fakeServer)

    // Advance past where the original timer would have fired
    vi.advanceTimersByTime(5 * 60 * 1000)
    await vi.runAllTimersAsync()

    // Connection should still be alive
    expect(relay.disconnect).not.toHaveBeenCalled()
  })

  // ── disconnectAll ─────────────────────────────────────────────────────────

  it('disconnectAll: disconnects all active connections', async () => {
    const r1 = makeMockRelay()
    const r2 = makeMockRelay()
    connectFn.mockResolvedValueOnce(r1).mockResolvedValueOnce(r2)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-2', fakeServer2)

    await pool.disconnectAll()

    expect(r1.disconnect).toHaveBeenCalledTimes(1)
    expect(r2.disconnect).toHaveBeenCalledTimes(1)
  })

  it('disconnectAll: clears pool status after disconnect', async () => {
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.disconnectAll()

    expect(pool.getStatus()).toEqual({})
  })

  it('disconnectAll: cancels pending idle timers', async () => {
    vi.useFakeTimers()
    const relay = makeMockRelay()
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    pool.release('srv-1') // starts timer

    await pool.disconnectAll()

    // Timer cancelled — advance past idle timeout, no second disconnect
    vi.advanceTimersByTime(5 * 60 * 1000 + 100)
    await vi.runAllTimersAsync()

    expect(relay.disconnect).toHaveBeenCalledTimes(1) // only from disconnectAll
  })

  // ── getStatus ─────────────────────────────────────────────────────────────

  it('getStatus: returns correct refCount and alive for active connection', async () => {
    const relay = makeMockRelay(true)
    connectFn.mockResolvedValue(relay)

    await pool.getOrConnect('srv-1', fakeServer)
    await pool.getOrConnect('srv-1', fakeServer)

    const status = pool.getStatus()

    expect(status['srv-1']).toEqual({ refCount: 2, alive: true })
  })

  it('getStatus: returns empty object when no connections', () => {
    expect(pool.getStatus()).toEqual({})
  })
})
