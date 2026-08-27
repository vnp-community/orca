// Why: claudeAccounts.add/.cancelPendingLogin/.reauthenticate spawn an
// interactive `claude login` PTY in the desktop main process -- list/select/
// remove already route through the accounts.* mobile-bridge namespace via
// runtime-provider-accounts-client.ts, so this file only covers the
// PTY-login surface that namespace deliberately excludes. Local-only: no
// backend/agent implementation exists for remote runtime environments.
import type { ClaudeRateLimitAccountsState } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

export type ClaudeAccountAddTarget = {
  runtime?: 'host' | 'wsl'
  wslDistro?: string | null
}

export async function addClaudeAccount(
  target?: ClaudeAccountAddTarget
): Promise<ClaudeRateLimitAccountsState> {
  return callRuntimeRpc<ClaudeRateLimitAccountsState>(
    { kind: 'local' },
    'claudeAccounts.add',
    target ?? null
  )
}

export async function cancelPendingClaudeAccountLogin(): Promise<boolean> {
  return callRuntimeRpc<boolean>({ kind: 'local' }, 'claudeAccounts.cancelPendingLogin')
}

export async function reauthenticateClaudeAccount(
  accountId: string
): Promise<ClaudeRateLimitAccountsState> {
  return callRuntimeRpc<ClaudeRateLimitAccountsState>(
    { kind: 'local' },
    'claudeAccounts.reauthenticate',
    { accountId }
  )
}
