// Why: additive local-desktop namespace mirroring ipc/openCode-usage.ts's
// openCodeUsage.* ipcMain channels 1:1 through the RPC dispatcher -- same
// OpenCodeUsageStore instance, reached via runtime.getOpenCodeUsageStore() on
// the desktop side. Return shapes match the untyped preload contract
// (window.api.openCodeUsage.*), which already returns Promise<unknown>.
import { callRuntimeRpc } from './runtime-rpc-client'

export async function getOpenCodeUsageScanState(): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getScanState')
}

export async function setOpenCodeUsageEnabled(enabled: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.setEnabled', { enabled })
}

export async function refreshOpenCodeUsage(force?: boolean): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.refresh', { force })
}

export async function getOpenCodeUsageSnapshot(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getSnapshot', { scope, range, limit })
}

export async function getOpenCodeUsageSummary(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getSummary', { scope, range })
}

export async function getOpenCodeUsageDaily(scope: string, range: string): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getDaily', { scope, range })
}

export async function getOpenCodeUsageBreakdown(
  scope: string,
  range: string,
  kind: string
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getBreakdown', { scope, range, kind })
}

export async function getOpenCodeUsageRecentSessions(
  scope: string,
  range: string,
  limit?: number
): Promise<unknown> {
  return callRuntimeRpc({ kind: 'local' }, 'openCodeUsage.getRecentSessions', { scope, range, limit })
}
