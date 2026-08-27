/**
 * Tests for ssh.getUserAccount.
 *
 * There is no per-user Linux account provisioning backend-side today — SSH
 * targets connect as whichever username is configured on the target. This
 * covers that the RPC method reports that configured username honestly,
 * rather than a fabricated multi-step provisioning result.
 *
 * @module main/runtime/rpc/methods/__tests__/ssh.test
 */
import { describe, it, expect, vi } from 'vitest'
import type { RpcContext } from '../../core'

vi.mock('../../../../ipc/ssh', () => ({
  connectRegisteredSshTarget: vi.fn(),
  getSshConnectionStore: vi.fn(),
  getRegisteredSshState: vi.fn(),
  getRegisteredSshTarget: vi.fn(),
  listRegisteredAllConnectionStates: vi.fn(),
  listRegisteredFilteredTargets: vi.fn(),
  listRegisteredRemovedSshTargetLabels: vi.fn(),
  listRegisteredSshProjects: vi.fn(),
  listRegisteredSshTargets: vi.fn(),
  listRegisteredSshTeams: vi.fn()
}))
vi.mock('../../../../ssh/fleet-bootstrap-service.js', () => ({ bootstrapServer: vi.fn() }))
vi.mock('../../../../ssh/fleet-status-service.js', () => ({ getFleetStatus: vi.fn() }))
vi.mock('../../../../ssh/fleet-health-store.js', () => ({
  fleetHealthStore: { getUptimeForTarget: vi.fn() }
}))

import { getRegisteredSshTarget } from '../../../../ipc/ssh'
import { SSH_METHODS } from '../ssh'

function findMethod(name: string) {
  const method = SSH_METHODS.find((m) => m.name === name)
  if (!method) {throw new Error(`method not found: ${name}`)}
  return method
}

const FAKE_CTX = {} as RpcContext

describe('ssh.getUserAccount', () => {
  it('reports the target-configured username when the target is registered', async () => {
    vi.mocked(getRegisteredSshTarget).mockReturnValue({
      id: 'server-1',
      label: 'Server 1',
      host: 'example.com',
      port: 22,
      username: 'ubuntu'
    } as ReturnType<typeof getRegisteredSshTarget>)

    const method = findMethod('ssh.getUserAccount')
    const result = await method.handler({ serverId: 'server-1' }, FAKE_CTX)

    expect(result).toEqual({ linuxUsername: 'ubuntu', provisioned: true })
  })

  it('returns null/unprovisioned when the target is unknown', async () => {
    vi.mocked(getRegisteredSshTarget).mockReturnValue(undefined)

    const method = findMethod('ssh.getUserAccount')
    const result = await method.handler({ serverId: 'missing' }, FAKE_CTX)

    expect(result).toEqual({ linuxUsername: null, provisioned: false })
  })

  it('rejects a missing serverId', () => {
    const method = findMethod('ssh.getUserAccount')
    expect(() => method.params?.parse({})).toThrow()
  })
})
