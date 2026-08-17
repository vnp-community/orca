// Why: additive local-desktop namespace mirroring ipc/codex-usage.ts's
// codexUsage.* ipcMain channels 1:1 through the RPC dispatcher -- same
// CodexUsageStore instance, reached via runtime.getCodexUsageStore() on
// the desktop side. Return shapes match the untyped preload contract
// (window.api.codexUsage.*), which already returns Promise<unknown>.
import { callRuntimeRpc } from './runtime-rpc-client'

export async function getCodexUsageScanState(): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getScanState')
}

export async function setCodexUsageEnabled(enabled: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.setEnabled', { enabled })
}

export async function refreshCodexUsage(force?: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.refresh', { force })
}

export async function getCodexUsageSnapshot(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getSnapshot', { scope, range, limit })
}

export async function getCodexUsageSummary(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getSummary', { scope, range })
}

export async function getCodexUsageDaily(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getDaily', { scope, range })
}

export async function getCodexUsageBreakdown(
  scope: string,
  range: string,
  kind: string
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getBreakdown', { scope, range, kind })
}

export async function getCodexUsageRecentSessions(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'codexUsage.getRecentSessions', { scope, range, limit })
}
