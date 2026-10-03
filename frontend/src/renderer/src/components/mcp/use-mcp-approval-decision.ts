import { useState } from 'react'
import { toast } from 'sonner'
import type { McpApproval } from '../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { syncMcpPendingApprovals } from '@/lib/mcp-approval-sync'
import { trackMcpApprovalDecided } from '@/lib/mcp-telemetry'

/**
 * Sends a decision. The caller is a user click; nothing here decides on its own.
 * The paramsHash is always the one the server sent for this approval.
 */
export function useMcpApprovalDecision(approval: McpApproval | undefined) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Bumped after a failure so the Approve lock restarts (never auto-resubmits).
  const [lockEpoch, setLockEpoch] = useState(0)

  const decide = async (decision: 'approve' | 'deny'): Promise<void> => {
    if (!approval || busy) {
      return
    }
    const store = useAppStore.getState()
    setBusy(true)
    setError(null)
    try {
      await mcpClient.call('mcp.approval.decide', {
        approvalId: approval.id,
        decision,
        paramsHash: approval.paramsHash
      })
      store.resolveMcpApproval(approval.id)
      trackMcpApprovalDecided({
        decision,
        risk: approval.tool.risk,
        via: 'dialog',
        latencyMs: Math.max(0, Date.now() - Date.parse(approval.createdAt) || 0)
      })
      toast.success(
        decision === 'approve'
          ? translate('auto.mcp.approval.approved', 'Approved')
          : translate('auto.mcp.approval.denied', 'Denied')
      )
    } catch (e) {
      const err = parseMcpError(e)
      switch (err.code) {
        case 'MCP_APPROVAL_EXPIRED':
          toast.info(translate('auto.mcp.approval.expiredToast', 'This request expired'))
          store.resolveMcpApproval(approval.id)
          break
        case 'MCP_APPROVAL_ALREADY_DECIDED':
          toast.info(
            translate('auto.mcp.approval.alreadyToast', 'Already handled (another device)')
          )
          store.resolveMcpApproval(approval.id)
          break
        case 'MCP_NOT_FOUND':
          store.resolveMcpApproval(approval.id)
          break
        case 'MCP_APPROVAL_HASH_MISMATCH':
          setError(translate('auto.mcp.approval.changed', 'This request changed. Review it again.'))
          setLockEpoch((n) => n + 1)
          // Replace with the server's current version; never resend automatically.
          void syncMcpPendingApprovals().catch(() => {})
          break
        case 'MCP_KILL_SWITCH_ACTIVE':
          setError(translate('auto.mcp.approval.killSwitch', 'MCP access is suspended.'))
          void store.refreshMcpServerInfo()
          break
        default:
          setError(err.detail)
          setLockEpoch((n) => n + 1)
      }
    } finally {
      setBusy(false)
    }
  }

  return { decide, busy, error, lockEpoch }
}
