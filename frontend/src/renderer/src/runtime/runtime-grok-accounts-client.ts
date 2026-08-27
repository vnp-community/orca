// Why: Grok status is derived from the CLI's own config on disk (no PTY
// login) -- additive namespace, local-only, mirrors the desktop ipcMain
// 'grokAccounts:getStatus' channel through the RPC dispatcher.
import type { GrokAccountStatus } from '../../../shared/rate-limit-types'
import { callRuntimeRpc } from './runtime-rpc-client'

export async function getGrokAccountStatus(): Promise<GrokAccountStatus> {
  return callRuntimeRpc<GrokAccountStatus>({ kind: 'local' }, 'grokAccounts.getStatus')
}
