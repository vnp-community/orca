import { z } from 'zod'
import { defineStreamingMethod, defineMethod, type RpcAnyMethod } from '../core'
import { AGENT_STATUS_STATES } from '../../../../shared/agent-status-types'
import type {
  NotificationDispatchRequest,
  NotificationDispatchResult,
  NotificationPermissionStatusResult,
  NotificationDeliveryProbeResult,
  NotificationSoundDataResult
} from '../../../../shared/types'

// Why: monotonically increasing per-process counter eliminates the
// Date.now() collision that could fire when two near-simultaneous
// notifications.subscribe calls landed on the same millisecond.
let notificationsSubscriptionSeq = 0

const NotificationUnsubscribeParams = z.object({
  subscriptionId: z
    .unknown()
    .transform((value) => (typeof value === 'string' && value.length > 0 ? value : ''))
    .pipe(z.string().min(1, 'Missing subscriptionId'))
})

// Why: these five methods (dispatch/getPermissionStatus/openSystemSettings/
// playSound/probeDelivery) mirror the desktop-only 'notifications:*' ipcMain
// handlers (desktop/src/main/ipc/notifications.ts) — all five are about the
// OS-native notification system of the machine RUNNING Orca. In desktop that
// machine is the user's own device; in server mode it's a shared Orca Server
// host that no browser client's notification permissions/sounds/settings
// have anything to do with (a browser has its own separate Notification API
// and permission model, unrelated to the server's OS). Rather than throwing
// "Unknown method" — which the frontend would surface as a hard error — each
// returns the same "not applicable here" value desktop itself already
// returns on platforms that don't support the feature (e.g. Windows/Linux
// already get `{ state: 'unsupported' }` from probeDelivery), so no frontend
// changes are needed to handle server mode gracefully.

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
  // Why: real notification delivery to a server-mode caller goes through
  // notifications.subscribe (WebSocket push) above, not this native-OS
  // dispatch path — so 'not-supported' here does not mean notifications
  // don't work in server mode, only that this particular native-toast
  // mechanism doesn't apply.
  defineMethod({
    name: 'notifications.dispatch',
    params: NotificationDispatchParams,
    handler: (): NotificationDispatchResult => ({ delivered: false, reason: 'not-supported' })
  }),
  defineMethod({
    name: 'notifications.getPermissionStatus',
    params: null,
    handler: (): NotificationPermissionStatusResult => ({
      supported: false,
      platform: process.platform,
      requested: false
    })
  }),
  defineMethod({
    name: 'notifications.openSystemSettings',
    params: null,
    handler: (): void => {
      // Why: no-op — opening the server host's own OS notification settings
      // panel has no meaning for a remote browser client.
    }
  }),
  defineMethod({
    name: 'notifications.probeDelivery',
    params: NotificationProbeDeliveryParams,
    handler: (): NotificationDeliveryProbeResult => ({ state: 'unsupported', authoritative: false })
  }),
  // Why: unlike the other four, this ISN'T a semantic mismatch — desktop's
  // own playSound already returns sound bytes for the CALLER to play
  // locally rather than playing audio in the main process (see desktop's
  // rpc/methods/notifications.ts comment). The real blocker here is that the
  // notification sound .mp3 assets are bundled into desktop's build via
  // electron-vite's `?asset` imports (desktop/resources/notification-sounds/
  // *.mp3); backend has no resources/ directory or asset-bundling step, so
  // there are no sound files on disk to read. 'missing-path' is the same
  // failure shape desktop returns when a user's configured sound file can't
  // be found — accurate here too, since none exist in this build.
  defineMethod({
    name: 'notifications.playSound',
    params: null,
    handler: (): NotificationSoundDataResult => ({ ok: false, reason: 'missing-path' })
  })
]
