import { defineMethod, type RpcMethod } from '../core'
import { getGrokAccountStatus } from '../../../grok-accounts/status'

// Why: additive local-desktop namespace -- NOT merged into accounts.* (that
// namespace's read+switch+remove-only mobile-bridge contract must stay as-is).
// Grok has no interactive-login PTY today (status is derived from the CLI's
// own config on disk), so a single read-only method covers it.
export const GROK_ACCOUNTS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'grokAccounts.getStatus',
    params: null,
    handler: async (_params, _ctx) => getGrokAccountStatus()
  })
]
