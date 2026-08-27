/* Why: mirrors runtime-github-client.ts's hybrid-routing shape for the
   `star-nag:*` preload methods. Desktop-local calls stay on window.api.starNag
   (real IPC, unchanged behavior); paired/web callers route through the
   `starNag.*` runtime RPC instead. */
import type { GlobalSettings } from '../../../shared/types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type RuntimeStarNagSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export type RuntimeStarNagVisibilityEvent =
  | { type: 'show'; mode?: 'gh' | 'web'; surface?: 'card' | 'toast' }
  | { type: 'hide' }

function isStarNagVisibilityEvent(value: unknown): value is RuntimeStarNagVisibilityEvent {
  return (
    typeof value === 'object' &&
    value !== null &&
    'type' in value &&
    ((value as { type: unknown }).type === 'show' || (value as { type: unknown }).type === 'hide')
  )
}

// Why: star-nag:show/star-nag:hide (BrowserWindow.webContents.send broadcasts
// in the desktop main process) reach a paired/web client through
// starNag.subscribe's streaming RPC instead — see StarNagService's
// onVisibilityChanged fan-out (desktop/src/main/star-nag/service.ts), the
// same bridge notifications.subscribe/remoteWorkspace.subscribeChanged use.
export function subscribeToRuntimeStarNagVisibility(
  settings: RuntimeStarNagSettings | null | undefined,
  callbacks: {
    onShow: (payload?: { mode?: 'gh' | 'web'; surface?: 'card' | 'toast' }) => void
    onHide: () => void
  }
): () => void {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    const unsubscribeShow = window.api.starNag.onShow(callbacks.onShow)
    const unsubscribeHide = window.api.starNag.onHide(callbacks.onHide)
    return () => {
      unsubscribeShow()
      unsubscribeHide()
    }
  }
  let handle: { unsubscribe: () => void } | null = null
  let cancelled = false
  window.api.runtimeEnvironments
    .subscribe(
      { selector: target.environmentId, method: 'starNag.subscribe' },
      {
        onResponse: (response) => {
          const payload = (response as RuntimeRpcResponse<unknown>).ok
            ? (response as { result: unknown }).result
            : undefined
          if (!isStarNagVisibilityEvent(payload)) {
            return
          }
          if (payload.type === 'show') {
            callbacks.onShow(payload)
          } else {
            callbacks.onHide()
          }
        }
      }
    )
    .then((subscription) => {
      handle = subscription
      if (cancelled) {
        subscription.unsubscribe()
      }
    })
    .catch(() => {})
  return () => {
    cancelled = true
    handle?.unsubscribe()
  }
}

export function dismissRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.dismiss> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.dismiss()
  }
  return callRuntimeRpc(target, 'starNag.dismiss', {}, { timeoutMs: 15_000 })
}

export function deferRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.later> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.later()
  }
  return callRuntimeRpc(target, 'starNag.later', {}, { timeoutMs: 15_000 })
}

export function completeRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.complete> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.complete()
  }
  return callRuntimeRpc(target, 'starNag.complete', {}, { timeoutMs: 15_000 })
}

export function disableRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.disable> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.disable()
  }
  return callRuntimeRpc(target, 'starNag.disable', {}, { timeoutMs: 15_000 })
}

export function openWebRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.openWeb> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.openWeb()
  }
  return callRuntimeRpc(target, 'starNag.openWeb', {}, { timeoutMs: 15_000 })
}

export function starRuntimeOrcaFromNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.starOrca> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.starOrca()
  }
  return callRuntimeRpc(target, 'starNag.starOrca', {}, { timeoutMs: 15_000 })
}

export function forceShowRuntimeStarNag(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.forceShow> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.forceShow()
  }
  return callRuntimeRpc(target, 'starNag.forceShow', {}, { timeoutMs: 15_000 })
}

export function prepareRuntimeStarNagAgentValueMoment(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.agentValueMoment> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.agentValueMoment()
  }
  return callRuntimeRpc(target, 'starNag.agentValueMoment', {}, { timeoutMs: 15_000 })
}

export function showRuntimeStarNagAgentValueMoment(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.showAgentValueMoment> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.showAgentValueMoment()
  }
  return callRuntimeRpc(target, 'starNag.showAgentValueMoment', {}, { timeoutMs: 15_000 })
}

export function notifyRuntimeStarNagOnboardingCompleted(
  settings: RuntimeStarNagSettings | null | undefined
): ReturnType<typeof window.api.starNag.onboardingCompleted> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.starNag.onboardingCompleted()
  }
  return callRuntimeRpc(target, 'starNag.onboardingCompleted', {}, { timeoutMs: 15_000 })
}
