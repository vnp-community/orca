import { describe, expect, it, vi } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import type { Store } from '../../../persistence'

const {
  detectAgentsForDevServerMock,
  detectAgentsAllDevServersMock,
  getPreflightStatusForDevServerMock,
  setGitIdentityForDevServerMock,
  detectGhosttyConfigForDevServerMock,
  openGhAuthTerminalForDevServerMock,
  detectWindowsCapabilitiesForDevServerMock,
  markOnboardingChecklistItemMock,
  getActiveOnboardingStoreMock
} = vi.hoisted(() => ({
  detectAgentsForDevServerMock: vi.fn(),
  detectAgentsAllDevServersMock: vi.fn(),
  getPreflightStatusForDevServerMock: vi.fn(),
  setGitIdentityForDevServerMock: vi.fn(),
  detectGhosttyConfigForDevServerMock: vi.fn(),
  openGhAuthTerminalForDevServerMock: vi.fn(),
  detectWindowsCapabilitiesForDevServerMock: vi.fn(),
  markOnboardingChecklistItemMock: vi.fn(),
  getActiveOnboardingStoreMock: vi.fn()
}))

vi.mock('../../../ipc/onboarding-ipc', () => ({
  detectAgentsForDevServer: detectAgentsForDevServerMock,
  detectAgentsAllDevServers: detectAgentsAllDevServersMock,
  getPreflightStatusForDevServer: getPreflightStatusForDevServerMock,
  setGitIdentityForDevServer: setGitIdentityForDevServerMock,
  detectGhosttyConfigForDevServer: detectGhosttyConfigForDevServerMock,
  openGhAuthTerminalForDevServer: openGhAuthTerminalForDevServerMock,
  detectWindowsCapabilitiesForDevServer: detectWindowsCapabilitiesForDevServerMock,
  markOnboardingChecklistItem: markOnboardingChecklistItemMock
}))

// Why: onboarding.get/update/markChecklistItem now read the store lazily via
// getActiveOnboardingStore() instead of taking it as a constructor arg — mock
// the singleton getter instead of calling a (now-removed) factory function.
vi.mock('../../../ipc/onboarding', () => ({
  getActiveOnboardingStore: getActiveOnboardingStoreMock
}))

import { ONBOARDING_METHODS } from './onboarding'

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeDevServerManagerMock() {
  return { getRelay: vi.fn(), get: vi.fn(), list: vi.fn() }
}

describe('onboarding RPC methods (dev-server-backed)', () => {
  it('detects agents for a specific dev server through devServerManager', async () => {
    detectAgentsForDevServerMock.mockResolvedValueOnce({
      agents: ['claude'],
      platform: 'linux',
      devServerId: 'ds-1'
    })
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.detectAgents', { devServerId: 'ds-1' })
    )

    expect(detectAgentsForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1'
    })
    expect(response).toMatchObject({
      ok: true,
      result: { agents: ['claude'], platform: 'linux', devServerId: 'ds-1' }
    })
  })

  it('detects agents across all connected dev servers', async () => {
    detectAgentsAllDevServersMock.mockResolvedValueOnce({
      'ds-1': { agents: ['claude'], platform: 'linux' }
    })
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(makeRequest('onboarding.detectAgentsAllServers'))

    expect(detectAgentsAllDevServersMock).toHaveBeenCalledWith(devServerManager)
    expect(response).toMatchObject({ ok: true, result: { 'ds-1': { agents: ['claude'] } } })
  })

  it('proxies onboarding.getPreflightStatus to the dev server relay', async () => {
    const status = { devServerId: 'ds-1', platform: 'linux', checkedAt: 1, gh: {}, git: {} }
    getPreflightStatusForDevServerMock.mockResolvedValueOnce(status)
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.getPreflightStatus', { devServerId: 'ds-1', force: true })
    )

    expect(getPreflightStatusForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1',
      force: true
    })
    expect(response).toMatchObject({ ok: true, result: status })
  })

  it('sets git identity on the dev server', async () => {
    setGitIdentityForDevServerMock.mockResolvedValueOnce(undefined)
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.setGitIdentity', {
        devServerId: 'ds-1',
        name: 'Ada',
        email: 'ada@example.com'
      })
    )

    expect(setGitIdentityForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1',
      name: 'Ada',
      email: 'ada@example.com'
    })
    expect(response).toMatchObject({ ok: true })
  })

  it('detects the ghostty config on the dev server', async () => {
    detectGhosttyConfigForDevServerMock.mockResolvedValueOnce({
      configPath: '/home/user/.config/ghostty/config',
      themeDir: null
    })
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.detectGhosttyConfig', { devServerId: 'ds-1' })
    )

    expect(detectGhosttyConfigForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1'
    })
    expect(response).toMatchObject({
      ok: true,
      result: { configPath: '/home/user/.config/ghostty/config', themeDir: null }
    })
  })

  it('opens a gh auth login terminal on the dev server', async () => {
    openGhAuthTerminalForDevServerMock.mockResolvedValueOnce({ ptyId: 'pty-1', devServerId: 'ds-1' })
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.openGhAuthTerminal', { devServerId: 'ds-1' })
    )

    expect(openGhAuthTerminalForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1'
    })
    expect(response).toMatchObject({ ok: true, result: { ptyId: 'pty-1', devServerId: 'ds-1' } })
  })

  it('detects Windows terminal capabilities on the dev server', async () => {
    detectWindowsCapabilitiesForDevServerMock.mockResolvedValueOnce({
      wslAvailable: true,
      wslDistros: ['Ubuntu'],
      pwshAvailable: true,
      gitBashAvailable: false
    })
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.detectWindowsCapabilities', { devServerId: 'ds-1' })
    )

    expect(detectWindowsCapabilitiesForDevServerMock).toHaveBeenCalledWith(devServerManager, {
      devServerId: 'ds-1'
    })
    expect(response).toMatchObject({ ok: true, result: { wslAvailable: true } })
  })

  it('throws method_not_found-safe error when devServerManager is unavailable', async () => {
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: ONBOARDING_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.detectAgents', { devServerId: 'ds-1' })
    )

    expect(response).toMatchObject({ ok: false })
    expect(detectAgentsForDevServerMock).not.toHaveBeenCalled()
  })

  it('rejects onboarding.detectAgents when devServerId is missing from params', async () => {
    const devServerManager = makeDevServerManagerMock()
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS,
      devServerManager: devServerManager as never
    })

    const response = await dispatcher.dispatch(makeRequest('onboarding.detectAgents', {}))

    expect(response).toMatchObject({ ok: false })
  })
})

describe('onboarding RPC methods (store-backed)', () => {
  function makeStoreMock(onboarding: unknown) {
    return {
      getOnboarding: vi.fn().mockReturnValue(onboarding),
      updateOnboarding: vi.fn().mockReturnValue(onboarding),
      mutate: vi.fn()
    } as unknown as Store
  }

  it('reads the persisted onboarding state', async () => {
    const onboarding = {
      flowVersion: 1,
      closedAt: null,
      outcome: null,
      lastCompletedStep: -1,
      checklist: {}
    }
    const store = makeStoreMock(onboarding)
    getActiveOnboardingStoreMock.mockReturnValue(store)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS as never
    })

    const response = await dispatcher.dispatch(makeRequest('onboarding.get'))

    expect(store.getOnboarding).toHaveBeenCalled()
    expect(response).toMatchObject({ ok: true, result: onboarding })
  })

  it('sanitizes and persists an onboarding update', async () => {
    const onboarding = {
      flowVersion: 1,
      closedAt: 123,
      outcome: 'completed',
      lastCompletedStep: 5,
      checklist: {}
    }
    const store = makeStoreMock(onboarding)
    getActiveOnboardingStoreMock.mockReturnValue(store)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.update', { closedAt: 123, outcome: 'completed', lastCompletedStep: 5 })
    )

    expect(store.updateOnboarding).toHaveBeenCalledWith({
      closedAt: 123,
      outcome: 'completed',
      lastCompletedStep: 5
    })
    expect(response).toMatchObject({ ok: true, result: onboarding })
  })

  it('drops unknown/malformed fields from onboarding.update before persisting', async () => {
    const onboarding = {
      flowVersion: 1,
      closedAt: null,
      outcome: null,
      lastCompletedStep: -1,
      checklist: {}
    }
    const store = makeStoreMock(onboarding)
    getActiveOnboardingStoreMock.mockReturnValue(store)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS as never
    })

    await dispatcher.dispatch(
      makeRequest('onboarding.update', { __proto__: 'evil', notARealField: 'x' })
    )

    expect(store.updateOnboarding).toHaveBeenCalledWith({})
  })

  it('marks a global checklist item through the shared mutation', async () => {
    const onboarding = {
      flowVersion: 1,
      closedAt: null,
      outcome: null,
      lastCompletedStep: -1,
      checklist: {}
    }
    const store = makeStoreMock(onboarding)
    getActiveOnboardingStoreMock.mockReturnValue(store)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS as never
    })

    const response = await dispatcher.dispatch(
      makeRequest('onboarding.markChecklistItem', { item: 'addedRepo', value: true })
    )

    expect(markOnboardingChecklistItemMock).toHaveBeenCalledWith(store, {
      item: 'addedRepo',
      devServerId: undefined,
      value: true
    })
    expect(response).toMatchObject({ ok: true, result: { marked: true } })
  })

  it('marks a per-server checklist item', async () => {
    const store = makeStoreMock({
      flowVersion: 1,
      closedAt: null,
      outcome: null,
      lastCompletedStep: -1,
      checklist: {}
    })
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({
      runtime,
      methods: ONBOARDING_METHODS as never
    })

    await dispatcher.dispatch(
      makeRequest('onboarding.markChecklistItem', {
        item: 'preflightPassed',
        devServerId: 'ds-1',
        value: false
      })
    )

    expect(markOnboardingChecklistItemMock).toHaveBeenCalledWith(store, {
      item: 'preflightPassed',
      devServerId: 'ds-1',
      value: false
    })
  })
})
