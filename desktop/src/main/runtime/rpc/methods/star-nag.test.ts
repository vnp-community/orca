import { describe, expect, it, vi, beforeEach } from 'vitest'

const {
  mockDismiss,
  mockDefer,
  mockMarkCompleted,
  mockDisable,
  mockOpenWeb,
  mockStarOrcaFromNag,
  mockForceShow,
  mockPrepareAgentValueMoment,
  mockShowPreparedAgentValueMoment,
  mockOnboardingCompleted,
  mockOnVisibilityChanged,
  mockGetActiveStarNagService
} = vi.hoisted(() => ({
  mockDismiss: vi.fn(),
  mockDefer: vi.fn(),
  mockMarkCompleted: vi.fn(),
  mockDisable: vi.fn(),
  mockOpenWeb: vi.fn(),
  mockStarOrcaFromNag: vi.fn().mockResolvedValue(true),
  mockForceShow: vi.fn(),
  mockPrepareAgentValueMoment: vi.fn().mockResolvedValue({ status: 'ready', mode: 'gh' }),
  mockShowPreparedAgentValueMoment: vi.fn(),
  mockOnboardingCompleted: vi.fn().mockResolvedValue(undefined),
  mockOnVisibilityChanged: vi.fn(),
  mockGetActiveStarNagService: vi.fn()
}))

vi.mock('../../../star-nag/service', () => ({
  getActiveStarNagService: mockGetActiveStarNagService
}))

import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { STAR_NAG_METHODS } from './star-nag'

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeDispatcher() {
  const runtime = { getRuntimeId: () => 'test' } as unknown as OrcaRuntimeService
  return new RpcDispatcher({ runtime, methods: STAR_NAG_METHODS })
}

const fakeService = {
  dismiss: mockDismiss,
  defer: mockDefer,
  markCompleted: mockMarkCompleted,
  disable: mockDisable,
  openWeb: mockOpenWeb,
  starOrcaFromNag: mockStarOrcaFromNag,
  forceShow: mockForceShow,
  prepareAgentValueMoment: mockPrepareAgentValueMoment,
  showPreparedAgentValueMoment: mockShowPreparedAgentValueMoment,
  onboardingCompleted: mockOnboardingCompleted,
  onVisibilityChanged: mockOnVisibilityChanged
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetActiveStarNagService.mockReturnValue(fakeService)
})

describe('starNag RPC methods', () => {
  it('starNag.dismiss calls StarNagService.dismiss', async () => {
    const response = await makeDispatcher().dispatch(makeRequest('starNag.dismiss'))
    expect(mockDismiss).toHaveBeenCalledTimes(1)
    expect(response.ok).toBe(true)
  })

  it('starNag.later calls StarNagService.defer("later")', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.later'))
    expect(mockDefer).toHaveBeenCalledWith('later')
  })

  it('starNag.complete calls StarNagService.markCompleted', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.complete'))
    expect(mockMarkCompleted).toHaveBeenCalledTimes(1)
  })

  it('starNag.disable calls StarNagService.disable', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.disable'))
    expect(mockDisable).toHaveBeenCalledTimes(1)
  })

  it('starNag.openWeb calls StarNagService.openWeb', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.openWeb'))
    expect(mockOpenWeb).toHaveBeenCalledTimes(1)
  })

  it('starNag.starOrca returns the star result', async () => {
    const response = await makeDispatcher().dispatch(makeRequest('starNag.starOrca'))
    expect(mockStarOrcaFromNag).toHaveBeenCalledTimes(1)
    expect(response).toMatchObject({ ok: true, result: true })
  })

  it('starNag.forceShow calls StarNagService.forceShow', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.forceShow'))
    expect(mockForceShow).toHaveBeenCalledTimes(1)
  })

  it('starNag.agentValueMoment returns the preparation result', async () => {
    const response = await makeDispatcher().dispatch(makeRequest('starNag.agentValueMoment'))
    expect(mockPrepareAgentValueMoment).toHaveBeenCalledTimes(1)
    expect(response).toMatchObject({ ok: true, result: { status: 'ready', mode: 'gh' } })
  })

  it('starNag.showAgentValueMoment calls StarNagService.showPreparedAgentValueMoment', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.showAgentValueMoment'))
    expect(mockShowPreparedAgentValueMoment).toHaveBeenCalledTimes(1)
  })

  it('starNag.onboardingCompleted calls StarNagService.onboardingCompleted', async () => {
    await makeDispatcher().dispatch(makeRequest('starNag.onboardingCompleted'))
    expect(mockOnboardingCompleted).toHaveBeenCalledTimes(1)
  })

  it('throws star_nag_unavailable when no active StarNagService is registered', async () => {
    mockGetActiveStarNagService.mockReturnValue(null)
    const response = await makeDispatcher().dispatch(makeRequest('starNag.dismiss'))
    expect(response.ok).toBe(false)
    if (!response.ok) {
      expect(response.error.message).toContain('star_nag_unavailable')
    }
  })

  it('starNag.subscribe streams show/hide events from StarNagService.onVisibilityChanged', async () => {
    const unsubscribe = vi.fn()
    const listeners: ((event: unknown) => void)[] = []
    mockOnVisibilityChanged.mockImplementation((listener: (event: unknown) => void) => {
      listeners.push(listener)
      return unsubscribe
    })
    // Why: starNag.subscribe blocks (mirrors notifications.subscribe) until
    // registerSubscriptionCleanup's cleanup callback resolves it — capture that
    // callback instead of awaiting dispatchStreaming directly, or the test hangs.
    let cleanup: (() => void) | undefined
    const runtime = {
      getRuntimeId: () => 'test',
      registerSubscriptionCleanup: vi.fn((_id: string, cleanupFn: () => void) => {
        cleanup = cleanupFn
      })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: STAR_NAG_METHODS })
    const messages: string[] = []

    const streamPromise = dispatcher.dispatchStreaming(
      makeRequest('starNag.subscribe'),
      (message) => messages.push(message),
      { connectionId: 'conn-1' }
    )
    await vi.waitFor(() => expect(listeners.length).toBe(1))
    listeners[0]?.({ type: 'show', mode: 'gh', surface: 'card' })

    expect(mockOnVisibilityChanged).toHaveBeenCalledTimes(1)
    expect(runtime.registerSubscriptionCleanup).toHaveBeenCalledWith(
      expect.stringContaining('star-nag-conn-1-'),
      expect.any(Function),
      'conn-1'
    )
    const results = messages.map((message) => JSON.parse(message).result)
    expect(results[0]).toMatchObject({ type: 'ready' })
    expect(results[1]).toEqual({ type: 'show', mode: 'gh', surface: 'card' })

    cleanup?.()
    await streamPromise
  })

  it('starNag.unsubscribe cleans up the registered subscription', async () => {
    const runtime = {
      getRuntimeId: () => 'test',
      cleanupSubscription: vi.fn()
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: STAR_NAG_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('starNag.unsubscribe', { subscriptionId: 'star-nag-conn-1-1' })
    )

    expect(runtime.cleanupSubscription).toHaveBeenCalledWith('star-nag-conn-1-1')
    expect(response).toMatchObject({ ok: true, result: { unsubscribed: true } })
  })
})
