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
// because it also serves the mobile pairing bridge). ClaudeAccountService's
// addAccount()/reauthenticateAccount() spawn an interactive `claude login`
// PTY that opens a system browser for OAuth; that's safe to wrap here
// because the local RPC dispatcher runs in-process with ipcMain (same main
// process, same PTY-spawning capability the existing ipc/claude-accounts.ts
// handler already relies on) -- unlike the mobile WebSocket bridge, which is
// a genuinely different device with no local browser to complete the OAuth
// redirect. cancelPendingLogin() lets the renderer abort an in-flight login
// PTY (e.g. user closes the add-account dialog before OAuth completes).
export const CLAUDE_ACCOUNTS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'claudeAccounts.list',
    params: null,
    handler: async (_params, { runtime }) => runtime.getClaudeAccountService().listAccounts()
  }),
  defineMethod({
    name: 'claudeAccounts.add',
    params: AccountTargetParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeAccountService().addAccount(params ?? undefined)
  }),
  defineMethod({
    name: 'claudeAccounts.cancelPendingLogin',
    params: null,
    handler: async (_params, { runtime }) => runtime.getClaudeAccountService().cancelPendingLogin()
  }),
  defineMethod({
    name: 'claudeAccounts.reauthenticate',
    params: AccountIdParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeAccountService().reauthenticateAccount(params.accountId)
  }),
  defineMethod({
    name: 'claudeAccounts.remove',
    params: AccountIdParams,
    handler: async (params, { runtime }) => runtime.getClaudeAccountService().removeAccount(params.accountId)
  }),
  defineMethod({
    name: 'claudeAccounts.select',
    params: SelectAccountParams,
    handler: async (params, { runtime }) => {
      const claudeAccounts = runtime.getClaudeAccountService()
      if (!params.runtime) {
        return claudeAccounts.selectAccount(params.accountId)
      }
      return claudeAccounts.selectAccountForTarget(params.accountId, params)
    }
  })
]
