import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import { getActiveStarNagService } from '../../../star-nag/service'
import type { AgentValueMomentPreparation } from '../../../star-nag/agent-value-moment'

// Why: monotonically increasing per-process counter, matching
// notifications.subscribe's collision-avoidance for near-simultaneous
// starNag.subscribe calls landing in the same millisecond.
let starNagSubscriptionSeq = 0

const StarNagUnsubscribeParams = z.object({
  subscriptionId: z
    .unknown()
    .transform((value) => (typeof value === 'string' && value.length > 0 ? value : ''))
    .pipe(z.string().min(1, 'Missing subscriptionId'))
})

// Why: star-nag state (promptVisible/cooldown/threshold) lives on the single
// StarNagService instance constructed in main/index.ts. RPC calls that land
// before that instance exists (or after shutdown) throw a stable error code
// rather than silently no-op-ing.
function requireStarNagService(): NonNullable<ReturnType<typeof getActiveStarNagService>> {
  const service = getActiveStarNagService()
  if (!service) {
    throw new Error('star_nag_unavailable')
  }
  return service
}

// Why: one wrapper per `star-nag:*` ipcMain channel (see
// desktop/src/main/star-nag/service.ts#registerIpcHandlers), calling the exact
// same StarNagService methods so remote/mobile RPC callers observe identical
// state transitions to the desktop's own renderer.
export const STAR_NAG_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'starNag.dismiss',
    params: null,
    handler: async () => {
      requireStarNagService().dismiss()
    }
  }),
  defineMethod({
    name: 'starNag.later',
    params: null,
    handler: async () => {
      requireStarNagService().defer('later')
    }
  }),
  defineMethod({
    name: 'starNag.complete',
    params: null,
    handler: async () => {
      requireStarNagService().markCompleted()
    }
  }),
  defineMethod({
    name: 'starNag.disable',
    params: null,
    handler: async () => {
      requireStarNagService().disable()
    }
  }),
  defineMethod({
    name: 'starNag.openWeb',
    params: null,
    handler: async () => {
      requireStarNagService().openWeb()
    }
  }),
  defineMethod({
    name: 'starNag.starOrca',
    params: null,
    handler: async (): Promise<boolean> => requireStarNagService().starOrcaFromNag()
  }),
  defineMethod({
    name: 'starNag.forceShow',
    params: null,
    handler: async () => {
      requireStarNagService().forceShow()
    }
  }),
  defineMethod({
    name: 'starNag.agentValueMoment',
    params: null,
    handler: async (): Promise<AgentValueMomentPreparation> =>
      requireStarNagService().prepareAgentValueMoment()
  }),
  defineMethod({
    name: 'starNag.showAgentValueMoment',
    params: null,
    handler: async () => {
      requireStarNagService().showPreparedAgentValueMoment()
    }
  }),
  defineMethod({
    name: 'starNag.onboardingCompleted',
    params: null,
    handler: async () => {
      await requireStarNagService().onboardingCompleted()
    }
  }),
  // Why: starNag.onShow/onHide push events (star-nag:show/star-nag:hide) are
  // desktop-native BrowserWindow.webContents.send broadcasts with no
  // request/response shape. This mirrors notifications.subscribe's
  // streaming pattern so remote/mobile RPC callers observe the same
  // show/hide transitions the desktop's own renderer gets over IPC.
  defineStreamingMethod({
    name: 'starNag.subscribe',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        const unsubscribe = requireStarNagService().onVisibilityChanged((event) => {
          emit(event)
        })

        // Why: scope by per-connection connectionId + per-process counter so
        // concurrent subscribes never collide on the cleanup map.
        const seq = ++starNagSubscriptionSeq
        const subscriptionId = `star-nag-${connectionId ?? 'inproc'}-${seq}`
        runtime.registerSubscriptionCleanup(
          subscriptionId,
          () => {
            unsubscribe()
            emit({ type: 'end' })
            resolve()
          },
          connectionId
        )

        emit({ type: 'ready', subscriptionId })
      })
    }
  }),
  defineMethod({
    name: 'starNag.unsubscribe',
    params: StarNagUnsubscribeParams,
    handler: async (params, { runtime }) => {
      runtime.cleanupSubscription(params.subscriptionId)
      return { unsubscribed: true }
    }
  })
]
