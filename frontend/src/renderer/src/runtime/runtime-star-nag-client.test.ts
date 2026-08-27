import { beforeEach, describe, expect, it, vi } from 'vitest'
import { subscribeToRuntimeStarNagVisibility } from './runtime-star-nag-client'

const LOCAL = { activeRuntimeEnvironmentId: null }
const REMOTE = { activeRuntimeEnvironmentId: 'env-1' }

type SubscriptionCallbacks = { onResponse: (response: unknown) => void }

const onShowLocal = vi.fn()
const onHideLocal = vi.fn()
const unsubscribeShowLocal = vi.fn()
const unsubscribeHideLocal = vi.fn()
const runtimeEnvironmentSubscribe = vi.fn()
const unsubscribeRemote = vi.fn()

let subscriptionCallbacks: SubscriptionCallbacks | null = null

beforeEach(() => {
  for (const mock of [
    onShowLocal,
    onHideLocal,
    unsubscribeShowLocal,
    unsubscribeHideLocal,
    runtimeEnvironmentSubscribe,
    unsubscribeRemote
  ]) {
    mock.mockReset()
  }
  subscriptionCallbacks = null
  onShowLocal.mockImplementation(() => unsubscribeShowLocal)
  onHideLocal.mockImplementation(() => unsubscribeHideLocal)
  runtimeEnvironmentSubscribe.mockImplementation(
    async (_args: unknown, callbacks: SubscriptionCallbacks) => {
      subscriptionCallbacks = callbacks
      return { unsubscribe: unsubscribeRemote }
    }
  )
  vi.stubGlobal('window', {
    api: {
      starNag: { onShow: onShowLocal, onHide: onHideLocal },
      runtimeEnvironments: { subscribe: runtimeEnvironmentSubscribe }
    }
  })
})

async function flushMicrotasks(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}

describe('subscribeToRuntimeStarNagVisibility', () => {
  it('subscribes via window.api.starNag when no runtime environment is active', () => {
    const onShow = vi.fn()
    const onHide = vi.fn()
    const unsubscribe = subscribeToRuntimeStarNagVisibility(LOCAL, { onShow, onHide })

    expect(onShowLocal).toHaveBeenCalledWith(onShow)
    expect(onHideLocal).toHaveBeenCalledWith(onHide)
    expect(runtimeEnvironmentSubscribe).not.toHaveBeenCalled()

    unsubscribe()
    expect(unsubscribeShowLocal).toHaveBeenCalledTimes(1)
    expect(unsubscribeHideLocal).toHaveBeenCalledTimes(1)
  })

  it('routes to starNag.subscribe and dispatches show/hide events for a paired environment', async () => {
    const onShow = vi.fn()
    const onHide = vi.fn()
    subscribeToRuntimeStarNagVisibility(REMOTE, { onShow, onHide })
    await flushMicrotasks()

    expect(runtimeEnvironmentSubscribe).toHaveBeenCalledWith(
      expect.objectContaining({ selector: 'env-1', method: 'starNag.subscribe' }),
      expect.any(Object)
    )

    subscriptionCallbacks?.onResponse({ ok: true, result: { type: 'ready', subscriptionId: 's-1' } })
    subscriptionCallbacks?.onResponse({
      ok: true,
      result: { type: 'show', mode: 'gh', surface: 'card' }
    })
    subscriptionCallbacks?.onResponse({ ok: true, result: { type: 'hide' } })
    subscriptionCallbacks?.onResponse({ ok: false, error: { code: 'x', message: 'boom' } })

    expect(onShow).toHaveBeenCalledTimes(1)
    expect(onShow).toHaveBeenCalledWith({ type: 'show', mode: 'gh', surface: 'card' })
    expect(onHide).toHaveBeenCalledTimes(1)
  })

  it('unsubscribes the remote stream on cleanup', async () => {
    const unsubscribe = subscribeToRuntimeStarNagVisibility(REMOTE, {
      onShow: vi.fn(),
      onHide: vi.fn()
    })
    await flushMicrotasks()

    unsubscribe()
    expect(unsubscribeRemote).toHaveBeenCalledTimes(1)
  })
})
