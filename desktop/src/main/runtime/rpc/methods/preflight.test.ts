import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { PREFLIGHT_METHODS } from './preflight'

const {
  detectInstalledAgentsWithShellPathHydrationMock,
  detectRemoteAgentsMock,
  detectRemoteWindowsTerminalCapabilitiesMock,
  refreshShellPathAndDetectAgentsMock,
  runPreflightCheckMock
} = vi.hoisted(() => ({
  detectInstalledAgentsWithShellPathHydrationMock: vi.fn(),
  detectRemoteAgentsMock: vi.fn(),
  detectRemoteWindowsTerminalCapabilitiesMock: vi.fn(),
  refreshShellPathAndDetectAgentsMock: vi.fn(),
  runPreflightCheckMock: vi.fn()
}))

vi.mock('../../../ipc/preflight', () => ({
  detectInstalledAgentsWithShellPathHydration: detectInstalledAgentsWithShellPathHydrationMock,
  detectRemoteAgents: detectRemoteAgentsMock,
  detectRemoteWindowsTerminalCapabilities: detectRemoteWindowsTerminalCapabilitiesMock,
  refreshShellPathAndDetectAgents: refreshShellPathAndDetectAgentsMock,
  runPreflightCheck: runPreflightCheckMock
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

describe('preflight RPC methods', () => {
  it('runs the server-side preflight check through runtime RPC', async () => {
    const status = {
      git: { installed: true },
      gh: { installed: true, authenticated: true },
      glab: { installed: false, authenticated: false },
      bitbucket: { configured: false, authenticated: false, account: null }
    }
    runPreflightCheckMock.mockResolvedValueOnce(status)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS })

    const response = await dispatcher.dispatch(makeRequest('preflight.check', { force: true }))

    expect(runPreflightCheckMock).toHaveBeenCalledWith(true)
    expect(response).toMatchObject({ ok: true, result: status })
  })

  it('detects agents and refreshes PATH on the server through runtime RPC', async () => {
    detectInstalledAgentsWithShellPathHydrationMock.mockResolvedValueOnce(['codex'])
    refreshShellPathAndDetectAgentsMock.mockResolvedValueOnce({
      agents: ['codex', 'claude'],
      addedPathSegments: ['/opt/bin'],
      shellHydrationOk: true,
      pathSource: 'shell_hydrate',
      pathFailureReason: 'none'
    })
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS })

    const detected = await dispatcher.dispatch(makeRequest('preflight.detectAgents'))
    const refreshed = await dispatcher.dispatch(makeRequest('preflight.refreshAgents'))

    expect(detectInstalledAgentsWithShellPathHydrationMock).toHaveBeenCalled()
    expect(refreshShellPathAndDetectAgentsMock).toHaveBeenCalled()
    expect(detected).toMatchObject({ ok: true, result: ['codex'] })
    expect(refreshed).toMatchObject({
      ok: true,
      result: { agents: ['codex', 'claude'], shellHydrationOk: true }
    })
  })

  it('detects agents on remote SSH connections through runtime RPC', async () => {
    detectRemoteAgentsMock.mockResolvedValueOnce(['claude'])
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('preflight.detectRemoteAgents', { connectionId: 'ssh-1' })
    )

    expect(detectRemoteAgentsMock).toHaveBeenCalledWith({ connectionId: 'ssh-1' })
    expect(response).toMatchObject({ ok: true, result: ['claude'] })
  })

  it('detects remote Windows terminal capabilities through runtime RPC', async () => {
    detectRemoteWindowsTerminalCapabilitiesMock.mockResolvedValueOnce({
      wslAvailable: true,
      wslDistros: ['Ubuntu'],
      pwshAvailable: true,
      gitBashAvailable: true,
      hostPlatform: 'win32'
    })
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('preflight.detectRemoteWindowsTerminalCapabilities', {
        connectionId: 'ssh-1'
      })
    )

    expect(detectRemoteWindowsTerminalCapabilitiesMock).toHaveBeenCalledWith({
      connectionId: 'ssh-1'
    })
    expect(response).toMatchObject({
      ok: true,
      result: {
        wslAvailable: true,
        wslDistros: ['Ubuntu'],
        pwshAvailable: true,
        gitBashAvailable: true,
        hostPlatform: 'win32'
      }
    })
  })
})

// ── TASK-04: preflight.check proxy via devServerId ────────────────────────────
describe('preflight.check with devServerId (Web mode proxy)', () => {
  const cliStatusFromRelay = {
    platform: 'linux' as NodeJS.Platform,
    gh: { installed: true, authenticated: true, version: '2.40.0' },
    glab: { installed: true, authenticated: false },
    git: { installed: true, version: '2.45.0', hasUserName: true, hasUserEmail: true }
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  function makeRelayMock(cliStatus: unknown) {
    return {
      call: vi.fn().mockResolvedValue(cliStatus)
    }
  }

  function makeDevServerManagerMock(relay: { call: ReturnType<typeof vi.fn> } | null) {
    return {
      getRelay: vi.fn().mockReturnValue(relay)
    }
  }

  it('proxies preflight.check to relay when devServerId is provided', async () => {
    const relay = makeRelayMock(cliStatusFromRelay)
    const devServerManager = makeDevServerManagerMock(relay)
    runPreflightCheckMock.mockResolvedValue({ git: { installed: true }, gh: { installed: false } })

    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS, devServerManager: devServerManager as never })

    const response = await dispatcher.dispatch(makeRequest('preflight.check', { devServerId: 'ds-abc' }))

    expect(relay.call).toHaveBeenCalledWith('preflight.check', expect.anything(), 30_000)
    expect(runPreflightCheckMock).not.toHaveBeenCalled()
    expect(response).toMatchObject({
      ok: true,
      result: expect.objectContaining({
        gh: expect.objectContaining({ installed: true, authenticated: true }),
        glab: expect.objectContaining({ installed: true, authenticated: false }),
        git: expect.objectContaining({ installed: true })
      })
    })
  })

  it('falls back to runPreflightCheck() when no devServerId provided', async () => {
    const localStatus = { git: { installed: true }, gh: { installed: true, authenticated: true } }
    runPreflightCheckMock.mockResolvedValueOnce(localStatus)

    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS })

    const response = await dispatcher.dispatch(makeRequest('preflight.check', {}))

    expect(runPreflightCheckMock).toHaveBeenCalled()
    expect(response).toMatchObject({ ok: true, result: localStatus })
  })

  it('throws when devServerId is given but relay is not connected', async () => {
    const devServerManager = makeDevServerManagerMock(null) // relay not found
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: PREFLIGHT_METHODS, devServerManager: devServerManager as never })

    const response = await dispatcher.dispatch(makeRequest('preflight.check', { devServerId: 'ds-missing' }))

    // Should return error response (ok: false) not throw
    expect(response).toMatchObject({ ok: false })
    const resp = response as { ok: false; error: { message?: string } }
    expect(resp.error?.message ?? '').toMatch(/relay|dev server/i)
  })
})
