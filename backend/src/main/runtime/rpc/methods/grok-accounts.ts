import { defineMethod, type RpcAnyMethod } from '../core'
import { getGrokAccountStatus } from '../../../grok-accounts/status'

// Why: ports desktop/src/main/runtime/rpc/methods/grok-accounts.ts. Grok has
// no interactive-login PTY today — status is derived by reading the Grok
// CLI's own auth.json off disk (already relied on server-side by
// RateLimitService for Grok usage fetching, see rate-limits/service.ts), so
// a single read-only method covers it here too. Signing in still happens
// outside Orca (`grok login` in a terminal on whichever machine runs this
// process) — this method only reports what's already on disk.
export const GROK_ACCOUNTS_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'grokAccounts.getStatus',
    params: null,
    handler: async (_params, _ctx) => getGrokAccountStatus()
  })
]
