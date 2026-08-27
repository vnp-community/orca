import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'

const AccountTargetParams = z
  .object({
    runtime: z.enum(['host', 'wsl']).optional(),
    wslDistro: z.string().nullable().optional()
  })
  .nullish()

const AccountIdParams = z.object({
  accountId: z.string().min(1, 'Missing accountId')
})

const SelectAccountParams = z.object({
  accountId: z.string().nullable(),
  runtime: z.enum(['host', 'wsl']).optional(),
  wslDistro: z.string().nullable().optional()
})

// Why: additive local-desktop namespace -- NOT merged into accounts.* (that
// namespace's read+switch+remove-only mobile-bridge contract must stay as-is
// because it also serves the mobile pairing bridge). CodexAccountService's
// addAccount()/reauthenticateAccount() spawn an interactive `codex login`
// PTY that opens a system browser for OAuth; safe to wrap here because the
// local RPC dispatcher runs in-process with ipcMain (same main process, same
// PTY-spawning capability ipc/codex-accounts.ts already relies on) -- unlike
// the mobile WebSocket bridge, a genuinely different device with no local
// browser to complete the OAuth redirect. Unlike Claude, CodexAccountService
// has no cancelPendingLogin() -- the ipc layer doesn't expose one either, so
// none is added here.
export const CODEX_ACCOUNTS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'codexAccounts.list',
    params: null,
    handler: async (_params, { runtime }) => runtime.getCodexAccountService().listAccounts()
  }),
  defineMethod({
    name: 'codexAccounts.add',
    params: AccountTargetParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexAccountService().addAccount(params ?? undefined)
  }),
  defineMethod({
    name: 'codexAccounts.reauthenticate',
    params: AccountIdParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexAccountService().reauthenticateAccount(params.accountId)
  }),
  defineMethod({
    name: 'codexAccounts.remove',
    params: AccountIdParams,
    handler: async (params, { runtime }) => runtime.getCodexAccountService().removeAccount(params.accountId)
  }),
  defineMethod({
    name: 'codexAccounts.select',
    params: SelectAccountParams,
    handler: async (params, { runtime }) => {
      const codexAccounts = runtime.getCodexAccountService()
      if (!params.runtime) {
        // Why: older renderer surfaces selected by account id only. Let the
        // service infer the account's runtime instead of treating missing
        // runtime as Windows/host and rejecting valid WSL accounts. Mirrors
        // ipc/codex-accounts.ts's 'codexAccounts:select' handler exactly.
        return codexAccounts.selectAccount(params.accountId)
      }
      return codexAccounts.selectAccountForTarget(params.accountId, params)
    }
  })
]
