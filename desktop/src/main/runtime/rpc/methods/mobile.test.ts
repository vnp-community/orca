import { describe, expect, it, vi, beforeEach } from 'vitest'

// Why: avoid importing RpcDispatcher's transitive Electron-toolkit deps
// (see header note below) while still exercising the getActiveRuntimeRpcServer
// singleton wiring the same way star-nag.test.ts/onboarding.test.ts do.
const { mockGetActiveRuntimeRpcServer } = vi.hoisted(() => ({
  mockGetActiveRuntimeRpcServer: vi.fn()
}))

// Why: avoid importing RpcDispatcher here — it eagerly imports the full
// ALL_RPC_METHODS aggregator (every namespace file), which transitively
// pulls in Electron-toolkit modules (app-icon.ts et al.) that this sandbox's
// electron CJS/ESM interop can't load standalone. Invoking the method
// handler directly keeps this suite scoped to mobile.ts's own dependency
// graph (ipc/mobile.ts only needs app/ipcMain/shell).
vi.mock('electron', () => ({
  app: { isPackaged: false },
  ipcMain: { handle: vi.fn() },
  shell: { openExternal: vi.fn() }
}))

vi.mock('../../../ipc/mobile', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../ipc/mobile')>()
  return {
    ...actual,
    getActiveRuntimeRpcServer: mockGetActiveRuntimeRpcServer
  }
})

import type { RpcMethod } from '../core'
import { MOBILE_METHODS } from './mobile'

function findMethod(name: string): RpcMethod {
  const method = MOBILE_METHODS.find((m) => m.name === name)
  if (!method || 'stream' in method) {
    throw new Error(`Expected a non-streaming method named ${name}`)
  }
  return method
}

const ctx = { runtime: { getRuntimeId: () => 'test-runtime' } as never }

beforeEach(() => {
  vi.clearAllMocks()
})

describe('mobile RPC methods (listNetworkInterfaces subset)', () => {
  it('mobile.listNetworkInterfaces returns the same shape as the ipcMain handler', async () => {
    const method = findMethod('mobile.listNetworkInterfaces')

    const result = (await method.handler(undefined, ctx)) as { interfaces: unknown[] }

    expect(Array.isArray(result.interfaces)).toBe(true)
  })
})

describe('mobile RPC methods backed by OrcaRuntimeRpcServer', () => {
  it('throws runtime_rpc_server_unavailable before bootstrap completes', async () => {
    mockGetActiveRuntimeRpcServer.mockReturnValue(null)
    const method = findMethod('mobile.listDevices')

    expect(() => method.handler(undefined, ctx)).toThrow('runtime_rpc_server_unavailable')
  })

  it('mobile.getPairingQR creates a mobile-scoped pairing offer', async () => {
    const createPairingOffer = vi.fn().mockReturnValue({
      available: true,
      pairingUrl: 'orca://pair#mobile',
      endpoint: 'ws://100.102.47.57:6768',
      deviceId: 'mobile-1'
    })
    mockGetActiveRuntimeRpcServer.mockReturnValue({ createPairingOffer })
    const method = findMethod('mobile.getPairingQR')

    const result = await method.handler({ address: '100.102.47.57' }, ctx)

    expect(result).toMatchObject({
      available: true,
      pairingUrl: 'orca://pair#mobile',
      deviceId: 'mobile-1'
    })
    expect(createPairingOffer).toHaveBeenCalledWith({
      address: '100.102.47.57',
      rotate: undefined,
      name: expect.stringMatching(/^Mobile /),
      scope: 'mobile'
    })
  })

  it('mobile.getRuntimePairingUrl creates a runtime-scoped pairing offer', async () => {
    const createPairingOffer = vi.fn().mockReturnValue({
      available: true,
      pairingUrl: 'orca://pair#runtime',
      webClientUrl: 'http://100.64.1.20:6768/web-index.html?pairing=runtime',
      endpoint: 'ws://100.64.1.20:6768',
      deviceId: 'runtime-1'
    })
    mockGetActiveRuntimeRpcServer.mockReturnValue({ createPairingOffer })
    const method = findMethod('mobile.getRuntimePairingUrl')

    const result = await method.handler({ address: '100.64.1.20', rotate: true }, ctx)

    expect(result).toEqual({
      available: true,
      pairingUrl: 'orca://pair#runtime',
      webClientUrl: 'http://100.64.1.20:6768/web-index.html?pairing=runtime',
      endpoint: 'ws://100.64.1.20:6768',
      deviceId: 'runtime-1'
    })
  })

  it('mobile.listDevices lists only paired mobile-scoped devices', async () => {
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getDeviceRegistry: () => ({
        listDevices: () => [
          { deviceId: 'mobile-1', name: 'Phone', scope: 'mobile', pairedAt: 1, lastSeenAt: 2 },
          { deviceId: 'runtime-1', name: 'CLI', scope: 'runtime', pairedAt: 1, lastSeenAt: 2 },
          {
            deviceId: 'pending-mobile',
            name: 'Pending',
            scope: 'mobile',
            pairedAt: 1,
            lastSeenAt: 0
          }
        ]
      })
    })
    const method = findMethod('mobile.listDevices')

    const result = await method.handler(undefined, ctx)

    expect(result).toEqual({
      devices: [{ deviceId: 'mobile-1', name: 'Phone', pairedAt: 1, lastSeenAt: 2 }]
    })
  })

  it('mobile.listRuntimeAccessGrants includes unused generated links', async () => {
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getDeviceRegistry: () => ({
        listDevices: () => [
          { deviceId: 'runtime-1', name: 'Browser', scope: 'runtime', pairedAt: 3, lastSeenAt: 4 },
          {
            deviceId: 'pending-runtime',
            name: 'Copied link',
            scope: 'runtime',
            pairedAt: 5,
            lastSeenAt: 0
          }
        ]
      })
    })
    const method = findMethod('mobile.listRuntimeAccessGrants')

    const result = await method.handler(undefined, ctx)

    expect(result).toEqual({
      grants: [
        { deviceId: 'pending-runtime', name: 'Copied link', createdAt: 5, lastSeenAt: null },
        { deviceId: 'runtime-1', name: 'Browser', createdAt: 3, lastSeenAt: 4 }
      ]
    })
  })

  it('mobile.revokeDevice revokes through the runtime server', async () => {
    const revokeMobileDevice = vi.fn().mockReturnValue(true)
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getDeviceRegistry: () => ({}),
      revokeMobileDevice
    })
    const method = findMethod('mobile.revokeDevice')

    const result = await method.handler({ deviceId: 'mobile-1' }, ctx)

    expect(result).toEqual({ revoked: true })
    expect(revokeMobileDevice).toHaveBeenCalledWith('mobile-1')
  })

  it('mobile.revokeRuntimeAccess revokes through the runtime server', async () => {
    const revokeRuntimeAccess = vi.fn().mockReturnValue(true)
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getDeviceRegistry: () => ({}),
      revokeRuntimeAccess
    })
    const method = findMethod('mobile.revokeRuntimeAccess')

    const result = await method.handler({ deviceId: 'runtime-1' }, ctx)

    expect(result).toEqual({ revoked: true })
    expect(revokeRuntimeAccess).toHaveBeenCalledWith('runtime-1')
  })

  it('mobile.isWebSocketReady reports the live websocket endpoint', async () => {
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getWebSocketEndpoint: () => 'ws://0.0.0.0:6768'
    })
    const method = findMethod('mobile.isWebSocketReady')

    const result = await method.handler(undefined, ctx)

    expect(result).toEqual({ ready: true, endpoint: 'ws://0.0.0.0:6768' })
  })

  it('mobile.getWindowsFirewallStatus reports unsupported on non-Windows', async () => {
    mockGetActiveRuntimeRpcServer.mockReturnValue({
      getWebSocketEndpoint: () => 'ws://0.0.0.0:6768'
    })
    const method = findMethod('mobile.getWindowsFirewallStatus')

    const result = await method.handler({ address: '192.168.0.108' }, ctx)

    expect(result).toMatchObject({ supported: false })
  })
})
