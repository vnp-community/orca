import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'

// Why: monotonically increasing per-process counter avoids Date.now()
// collisions when two near-simultaneous rateLimits.subscribe calls land on
// the same millisecond -- mirrors accounts.subscribe/notifications.subscribe.
let rateLimitsSubscriptionSeq = 0

const RuntimeTargetParams = z.object({
  runtime: z.enum(['host', 'wsl']),
  wslDistro: z.string().nullable()
})

const SetPollingIntervalParams = z.object({
  ms: z.number()
})

const RateLimitsUnsubscribeParams = z.object({
  subscriptionId: z
    .unknown()
    .transform((value) => (typeof value === 'string' && value.length > 0 ? value : ''))
    .pipe(z.string().min(1, 'Missing subscriptionId'))
})

// Why: additive local-desktop namespace mirroring ipc/rate-limits.ts 1:1 --
// same RateLimitService instance the ipcMain channel calls, reached through
// runtime.getRateLimitService() (wired via setAccountServices in
// desktop/src/main/index.ts). NOT merged into accounts.* -- that namespace's
// read+switch+remove-only mobile-bridge contract stays as-is.
export const RATE_LIMIT_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'rateLimits.get',
    params: null,
    handler: async (_params, { runtime }) => runtime.getRateLimitService().getState()
  }),
  defineMethod({
    name: 'rateLimits.refresh',
    params: null,
    handler: async (_params, { runtime }) => runtime.getRateLimitService().refresh()
  }),
  defineMethod({
    name: 'rateLimits.refreshCodexForTarget',
    params: RuntimeTargetParams,
    handler: async (params, { runtime }) => runtime.getRateLimitService().refreshCodexForTarget(params)
  }),
  defineMethod({
    name: 'rateLimits.consumeCodexResetCredit',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.getRateLimitService().consumeCodexRateLimitResetCredit()
  }),
  defineMethod({
    name: 'rateLimits.refreshClaudeForTarget',
    params: RuntimeTargetParams,
    handler: async (params, { runtime }) => runtime.getRateLimitService().refreshClaudeForTarget(params)
  }),
  defineMethod({
    name: 'rateLimits.setPollingInterval',
    params: SetPollingIntervalParams,
    handler: async (params, { runtime }) => runtime.getRateLimitService().setPollingInterval(params.ms)
  }),
  defineMethod({
    name: 'rateLimits.fetchInactiveClaudeAccounts',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.getRateLimitService().fetchInactiveClaudeAccountsOnOpen()
  }),
  defineMethod({
    name: 'rateLimits.fetchInactiveCodexAccounts',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.getRateLimitService().fetchInactiveCodexAccountsOnOpen()
  }),
  defineMethod({
    name: 'rateLimits.refreshMiniMax',
    // Why: mirrors ipc/rate-limits.ts's 'rateLimits:refreshMiniMax' channel,
    // which itself just calls the general refresh() -- MiniMax has no
    // dedicated refresh path on RateLimitService.
    params: null,
    handler: async (_params, { runtime }) => runtime.getRateLimitService().refresh()
  }),
  defineMethod({
    name: 'rateLimits.refreshGrok',
    params: null,
    handler: async (_params, { runtime }) => runtime.getRateLimitService().refreshGrok()
  }),
  // Why: desktop's ipc/rate-limits.ts pushes 'rateLimits:update' to the
  // renderer via RateLimitService.onStateChange whenever polling completes or
  // an account switch triggers a refresh. The local RPC dispatcher has no
  // main->renderer IPC channel, so this streams the same event shape over the
  // request/response RPC transport -- mirrors accounts.subscribe's shape.
  defineStreamingMethod({
    name: 'rateLimits.subscribe',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        const unsubscribe = runtime.getRateLimitService().onStateChange((state) => {
          emit({ type: 'update', state })
        })

        const seq = ++rateLimitsSubscriptionSeq
        const subscriptionId = `rateLimits-${connectionId ?? 'inproc'}-${seq}`
        runtime.registerSubscriptionCleanup(
          subscriptionId,
          () => {
            unsubscribe()
            emit({ type: 'end' })
            resolve()
          },
          connectionId
        )

        emit({ type: 'ready', subscriptionId, state: runtime.getRateLimitService().getState() })
      })
    }
  }),
  defineMethod({
    name: 'rateLimits.unsubscribe',
    params: RateLimitsUnsubscribeParams,
    handler: async (params, { runtime }) => {
      runtime.cleanupSubscription(params.subscriptionId)
      return { unsubscribed: true }
    }
  })
]
