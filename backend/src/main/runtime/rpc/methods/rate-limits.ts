import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import type { RateLimitState } from '../../../../shared/rate-limit-types'

// Why: monotonically increasing per-process counter avoids Date.now()
// collisions when two near-simultaneous rateLimits.subscribe calls land on
// the same millisecond -- mirrors accounts.subscribe/notifications.subscribe.
let rateLimitsSubscriptionSeq = 0

// Why: server mode never calls runtime.setAccountServices() today (see
// hasAccountServices()'s doc comment in orca-runtime-account-services.ts) --
// every handler below checks this first and returns a safe empty state
// instead of letting getRateLimitService() throw an uncaught rejection on
// every page load. Revisit once account-service wiring lands for server mode.
const EMPTY_RATE_LIMIT_STATE: RateLimitState = {
  claude: null,
  codex: null,
  gemini: null,
  opencodeGo: null,
  kimi: null,
  antigravity: null,
  minimax: null,
  grok: null,
  minimaxCookieConfigured: false,
  grokAuthConfigured: false,
  claudeTarget: { runtime: 'host', wslDistro: null },
  codexTarget: { runtime: 'host', wslDistro: null },
  inactiveClaudeAccounts: [],
  inactiveCodexAccounts: []
}

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

// Why: server-mode port of desktop/src/main/runtime/rpc/methods/rate-limits.ts
// -- same RateLimitService public API (backend/src/main/rate-limits/service.ts),
// reached through runtime.getRateLimitService() (wired via setAccountServices in
// backend's server-bootstrap.ts). NOT merged into accounts.* -- that namespace's
// read+switch+remove-only mobile-bridge contract stays as-is.
export const RATE_LIMIT_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'rateLimits.get',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.hasAccountServices() ? runtime.getRateLimitService().getState() : EMPTY_RATE_LIMIT_STATE
  }),
  defineMethod({
    name: 'rateLimits.refresh',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.hasAccountServices() ? runtime.getRateLimitService().refresh() : EMPTY_RATE_LIMIT_STATE
  }),
  defineMethod({
    name: 'rateLimits.refreshCodexForTarget',
    params: RuntimeTargetParams,
    handler: async (params, { runtime }) =>
      runtime.hasAccountServices()
        ? runtime.getRateLimitService().refreshCodexForTarget(params)
        : EMPTY_RATE_LIMIT_STATE
  }),
  defineMethod({
    name: 'rateLimits.consumeCodexResetCredit',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.hasAccountServices()
        ? runtime.getRateLimitService().consumeCodexRateLimitResetCredit()
        : { outcome: 'nothingToReset' as const, state: EMPTY_RATE_LIMIT_STATE }
  }),
  defineMethod({
    name: 'rateLimits.refreshClaudeForTarget',
    params: RuntimeTargetParams,
    handler: async (params, { runtime }) =>
      runtime.hasAccountServices()
        ? runtime.getRateLimitService().refreshClaudeForTarget(params)
        : EMPTY_RATE_LIMIT_STATE
  }),
  defineMethod({
    name: 'rateLimits.setPollingInterval',
    params: SetPollingIntervalParams,
    handler: async (params, { runtime }) => {
      if (runtime.hasAccountServices()) {runtime.getRateLimitService().setPollingInterval(params.ms)}
    }
  }),
  defineMethod({
    name: 'rateLimits.fetchInactiveClaudeAccounts',
    params: null,
    handler: async (_params, { runtime }) => {
      if (runtime.hasAccountServices()) {await runtime.getRateLimitService().fetchInactiveClaudeAccountsOnOpen()}
    }
  }),
  defineMethod({
    name: 'rateLimits.fetchInactiveCodexAccounts',
    params: null,
    handler: async (_params, { runtime }) => {
      if (runtime.hasAccountServices()) {await runtime.getRateLimitService().fetchInactiveCodexAccountsOnOpen()}
    }
  }),
  defineMethod({
    name: 'rateLimits.refreshMiniMax',
    // Why: mirrors desktop's rateLimits.refreshMiniMax, which itself just
    // calls the general refresh() -- MiniMax has no dedicated refresh path on
    // RateLimitService.
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.hasAccountServices() ? runtime.getRateLimitService().refresh() : EMPTY_RATE_LIMIT_STATE
  }),
  defineMethod({
    name: 'rateLimits.refreshGrok',
    params: null,
    handler: async (_params, { runtime }) =>
      runtime.hasAccountServices() ? runtime.getRateLimitService().refreshGrok() : EMPTY_RATE_LIMIT_STATE
  }),
  // Why: desktop's ipc/rate-limits.ts pushes 'rateLimits:update' to the
  // renderer via RateLimitService.onStateChange whenever polling completes or
  // an account switch triggers a refresh. Server mode has no main->renderer
  // IPC channel, so this streams the same event shape over the RPC transport
  // -- mirrors accounts.subscribe's shape.
  defineStreamingMethod({
    name: 'rateLimits.subscribe',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        // Why: no-op unsubscribe when account services aren't configured --
        // there's no RateLimitService instance to attach a listener to, only
        // the one-shot 'ready' emit below (matching what a real subscriber
        // would see: current state, then silence until reconfigured).
        const unsubscribe = runtime.hasAccountServices()
          ? runtime.getRateLimitService().onStateChange((state) => {
              emit({ type: 'update', state })
            })
          : () => {}

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

        emit({
          type: 'ready',
          subscriptionId,
          state: runtime.hasAccountServices() ? runtime.getRateLimitService().getState() : EMPTY_RATE_LIMIT_STATE
        })
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
