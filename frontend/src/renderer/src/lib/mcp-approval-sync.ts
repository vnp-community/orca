import { mcpClient } from '@/runtime/runtime-mcp-client'
import { useAppStore } from '@/store'

/** Pulls the pending set from the server (source of truth; events are only hints). */
export async function syncMcpPendingApprovals(): Promise<void> {
  const res = await mcpClient.call('mcp.approval.list', { status: 'pending', limit: 50 })
  useAppStore.getState().replaceMcpApprovals(res.approvals, Date.now())
}
