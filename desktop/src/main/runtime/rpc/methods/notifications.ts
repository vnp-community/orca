import { z } from 'zod'
import { defineStreamingMethod, defineMethod, type RpcAnyMethod } from '../core'
import {
  dispatchNotification,
  getNotificationPermissionStatus,
  loadNotificationSoundData,
  openNotificationSystemSettings,
  probeNotificationDeliveryStatus
} from '../../../ipc/notifications'
import { AGENT_STATUS_STATES } from '../../../../shared/agent-status-types'
import type { NotificationDispatchRequest } from '../../../../shared/types'

// Why: monotonically increasing per-process counter eliminates the
// Date.now() collision that could fire when two near-simultaneous
// notifications.subscribe calls landed on the same millisecond.
let notificationsSubscriptionSeq = 0

// Why: RPC-triggered dispatches (mobile/web/local-self callers) get their
// own cooldown/dedupe scope, separate from the desktop ipcMain handler's
// per-registration map — dispatchNotification() takes the map as a plain
// param specifically so callers don't have to share mutable state.
const rpcRecentNotifications = new Map<string, number>()

const NotificationUnsubscribeParams = z.object({
  subscriptionId: z
    .unknown()
    .transform((value) => (typeof value === 'string' && value.length > 0 ? value : ''))
    .pipe(z.string().min(1, 'Missing subscriptionId'))
})

const NotificationDispatchParams = z.object({
  source: z.enum(['agent-task-complete', 'terminal-bell', 'test']),
  notificationId: z.string().optional(),
  requireDisplayConfirmation: z.boolean().optional(),
  worktreeId: z.string().optional(),
  paneKey: z.string().optional(),
  repoLabel: z.string().optional(),
  worktreeLabel: z.string().optional(),
  hasMultipleActiveRepos: z.boolean().optional(),
  terminalTitle: z.string().optional(),
  isActiveWorktree: z.boolean().optional(),
  agentType: z.string().optional(),
  agentState: z.enum(AGENT_STATUS_STATES).optional(),
  agentPrompt: z.string().optional(),
  agentToolName: z.string().optional(),
  agentToolInput: z.string().optional(),
  agentLastAssistantMessage: z.string().optional(),
  agentInterrupted: z.boolean().optional()
}) satisfies z.ZodType<NotificationDispatchRequest>

const NotificationProbeDeliveryParams = z
  .object({
    force: z.boolean().optional()
  })
  .nullish()

// Why: notifications.subscribe streams desktop notification events to mobile
// clients over WebSocket. The mobile client shows a local push notification
// for each event. This avoids requiring Firebase/APNs — the existing
// persistent WebSocket connection doubles as the push channel.
export const NOTIFICATION_METHODS: readonly RpcAnyMethod[] = [
  defineStreamingMethod({
    name: 'notifications.subscribe',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        const unsubscribe = runtime.onNotificationDispatched((event) => {
          emit(event)
        })

        // Why: scope by per-ws connectionId + per-process counter so
        // concurrent subscribes never collide on the cleanup map.
        const seq = ++notificationsSubscriptionSeq
        const subscriptionId = `notifications-${connectionId ?? 'inproc'}-${seq}`
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
    name: 'notifications.unsubscribe',
    params: NotificationUnsubscribeParams,
    handler: async (params, { runtime }) => {
      runtime.cleanupSubscription(params.subscriptionId)
      return { unsubscribed: true }
    }
  }),
  // Why: these five mirror the desktop-only 'notifications:*' ipcMain
  // handlers (see ipc/notifications.ts) for callers that only have the
  // generic RPC channel — a remote/web runtime target, or the local
  // window.api.runtime.call path. Each delegates to the exact function the
  // ipcMain handler calls so behavior can't drift between the two entry
  // points.
  defineMethod({
    name: 'notifications.dispatch',
    params: NotificationDispatchParams,
    handler: (params, { runtime }) =>
      dispatchNotification(params, {
        getNotificationSettings: () => runtime.getNotificationSettings(),
        runtime,
        recentNotifications: rpcRecentNotifications
      })
  }),
  defineMethod({
    name: 'notifications.getPermissionStatus',
    params: null,
    handler: (_params, { runtime }) =>
      getNotificationPermissionStatus(runtime.getUIState().notificationPermissionRequested === true)
  }),
  defineMethod({
    name: 'notifications.openSystemSettings',
    params: null,
    handler: () => {
      openNotificationSystemSettings()
    }
  }),
  defineMethod({
    name: 'notifications.probeDelivery',
    params: NotificationProbeDeliveryParams,
    handler: (params, { runtime }) =>
      probeNotificationDeliveryStatus(
        {
          getPermissionRequested: () =>
            runtime.getUIState().notificationPermissionRequested === true,
          markPermissionRequested: () =>
            void runtime.updateUIState({ notificationPermissionRequested: true })
        },
        params ?? undefined
      )
  }),
  // Why: unlike the preload's playSound() — which plays audio through an
  // Audio element that only exists in a renderer/preload context — a
  // main-process RPC handler cannot play sound itself. This returns the same
  // resolved sound bytes the preload fetches via 'notifications:loadSound'
  // (data/mimeType/path) so a remote/local RPC caller can play them on its
  // own side, mirroring the git-client local-vs-remote split pattern.
  defineMethod({
    name: 'notifications.playSound',
    params: null,
    handler: (_params, { runtime }) =>
      loadNotificationSoundData(() => runtime.getNotificationSettings())
  })
]
