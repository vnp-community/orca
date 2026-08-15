/**
 * Tests for fleet.health.checkAll.
 *
 * fleet.ts is a thin batch wrapper over SshConnectionManager.getState() and
 * fleetHealthStore's existing collection primitives (fleet-status-service.ts's
 * getFleetStatus() maps the same fields per target) — mock both singletons,
 * following ssh.test.ts's vi.mock pattern.
 *
 * @module main/runtime/rpc/methods/__tests__/fleet.test
 */
import { describe, it, expect, vi } from 'vitest'
import type { RpcContext } from '../../core'

vi.mock('../../../../ipc/ssh', () => ({
  getSshConnectionManager: vi.fn()
}))
vi.mock('../../../../ssh/fleet-health-store.js', () => ({
  fleetHealthStore: {
    getLastRecord: vi.fn(),
    getConnectedSince: vi.fn()
  }
}))

import { getSshConnectionManager } from '../../../../ipc/ssh'
import { fleetHealthStore } from '../../../../ssh/fleet-health-store.js'
import { FLEET_METHODS } from '../fleet'

function findMethod(name: string) {
  const method = FLEET_METHODS.find((m) => m.name === name)
  if (!method) {throw new Error(`method not found: ${name}`)}
  return method
}

const FAKE_CTX = {} as RpcContext

describe('fleet.health.checkAll', () => {
  it('returns ServerHealthMetrics for every requested serverId', async () => {
    vi.mocked(getSshConnectionManager).mockReturnValue({
      getState: (id: string) =>
        id === 'server-1' ? { status: 'connected' } : { status: 'disconnected' }
    } as ReturnType<typeof getSshConnectionManager>)
    vi.mocked(fleetHealthStore.getLastRecord).mockImplementation((id: string) =>
      id === 'server-1'
        ? { targetId: id, timestamp: 123, status: 'connected', relayVersion: '1.6.0', cpuPercent: 12, ramPercent: 34, diskPercent: 56 }
        : null
    )
    vi.mocked(fleetHealthStore.getConnectedSince).mockImplementation((id: string) =>
      id === 'server-1' ? 1000 : null
    )

    const method = findMethod('fleet.health.checkAll')
    const result = (await method.handler(
      { serverIds: ['server-1', 'server-2'] },
      FAKE_CTX
    )) as Array<Record<string, unknown>>

    expect(result).toHaveLength(2)
    expect(result[0]).toMatchObject({
      serverId: 'server-1',
      isReachable: true,
      relayVersion: '1.6.0',
      cpuUsagePercent: 12,
      memUsagePercent: 34,
      diskUsagePercent: 56
    })
    expect(result[1]).toMatchObject({
      serverId: 'server-2',
      isReachable: false,
      relayVersion: null,
      uptimeSeconds: null
    })
  })

  it('rejects a missing serverIds param', () => {
    const method = findMethod('fleet.health.checkAll')
    expect(() => method.params?.parse({})).toThrow()
  })

  it('returns an empty array for an empty serverIds list', async () => {
    vi.mocked(getSshConnectionManager).mockReturnValue({
      getState: () => undefined
    } as unknown as ReturnType<typeof getSshConnectionManager>)

    const method = findMethod('fleet.health.checkAll')
    const result = await method.handler({ serverIds: [] }, FAKE_CTX)
    expect(result).toEqual([])
  })
})
