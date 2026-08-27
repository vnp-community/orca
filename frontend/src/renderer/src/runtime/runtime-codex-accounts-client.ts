// Why: codexAccounts.add/.reauthenticate spawn an interactive `codex login`
// PTY in the desktop main process -- list/select/remove already route through
// the accounts.* mobile-bridge namespace via runtime-provider-accounts-client.ts,
// so this file only covers the PTY-login surface that namespace deliberately
// excludes. Local-only: no backend/agent implementation exists for remote
// runtime environments. Unlike Claude, Codex has no cancelPendingLogin.
import type { CodexRateLimitAccountsState } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

export type CodexAccountAddTarget = {
  runtime?: 'host' | 'wsl'
  wslDistro?: string | null
}

export async function addCodexAccount(
  target?: CodexAccountAddTarget
): Promise<CodexRateLimitAccountsState> {
  return callRuntimeRpc<CodexRateLimitAccountsState>(
    { kind: 'local' },
    'codexAccounts.add',
    target ?? null
  )
}

export async function reauthenticateCodexAccount(
  accountId: string
): Promise<CodexRateLimitAccountsState> {
  return callRuntimeRpc<CodexRateLimitAccountsState>(
    { kind: 'local' },
    'codexAccounts.reauthenticate',
    { accountId }
  )
}
