// Why: additive local-desktop namespace mirroring ipc/claude-usage.ts's
// claudeUsage.* ipcMain channels 1:1 through the RPC dispatcher -- same
// ClaudeUsageStore instance, reached via runtime.getClaudeUsageStore() on
// the desktop side. Return shapes match the untyped preload contract
// (window.api.claudeUsage.*), which already returns Promise<unknown>.
import { callRuntimeRpc } from './runtime-rpc-client'

export async function getClaudeUsageScanState(): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getScanState')
}

export async function setClaudeUsageEnabled(enabled: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.setEnabled', { enabled })
}

export async function refreshClaudeUsage(force?: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.refresh', { force })
}

export async function getClaudeUsageSnapshot(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getSnapshot', { scope, range, limit })
}

export async function getClaudeUsageSummary(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getSummary', { scope, range })
}

export async function getClaudeUsageDaily(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getDaily', { scope, range })
}

export async function getClaudeUsageBreakdown(
  scope: string,
  range: string,
  kind: string
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getBreakdown', { scope, range, kind })
}

export async function getClaudeUsageRecentSessions(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'claudeUsage.getRecentSessions', { scope, range, limit })
}
