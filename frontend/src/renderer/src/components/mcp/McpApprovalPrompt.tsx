import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { useAppStore } from '@/store'
import { selectMcpKillSwitchActive } from '@/store/slices/mcp-slice'
import { useMcpApprovalDeadline } from '@/hooks/useMcpApprovalDeadline'
import { syncMcpPendingApprovals } from '@/lib/mcp-approval-sync'
import { McpApprovalArgs, McpApprovalClient, McpRiskBadge } from './McpApprovalDetails'
import { formatCountdown } from './mcp-approval-display'
import { useMcpApproveLock } from './use-mcp-approve-lock'
import { useMcpApprovalDecision } from './use-mcp-approval-decision'

/**
 * Global approval prompt. Security rules: never decides by itself; the only approve call-site
 * is the Approve button's own click; focus starts on Deny; Approve is delay-locked by risk.
 */
export function McpApprovalPrompt(): React.JSX.Element | null {
  const queue = useAppStore((s) => s.mcpApprovalQueue)
  const open = useAppStore((s) => s.mcpApprovalPromptOpen)
  const deadlines = useAppStore((s) => s.mcpApprovalLocalDeadline)
  const killSwitch = useAppStore(selectMcpKillSwitchActive)
  const current = queue[0]
  const denyRef = useRef<HTMLButtonElement>(null)
  const [reconciled, setReconciled] = useState<string | null>(null)

  const { decide, busy, error, lockEpoch } = useMcpApprovalDecision(current)
  const remaining = useMcpApprovalDeadline(current ? deadlines[current.id] : undefined)
  const lockLeft = useMcpApproveLock(
    current?.tool.risk ?? 'read',
    `${current?.id}:${current?.paramsHash}:${lockEpoch}`
  )

  const timedOut = Boolean(current) && remaining <= 0
  // Why: after local expiry ask the server once; it may still be pending (clock skew).
  useEffect(() => {
    if (!current || !timedOut || reconciled === current.id) {
      return
    }
    setReconciled(current.id)
    void syncMcpPendingApprovals().catch(() => {})
  }, [current, timedOut, reconciled])

  if (!current) {
    return null
  }

  const decideLater = (): void => {
    useAppStore.getState().setMcpApprovalPromptOpen(false)
    toast(
      translate('auto.mcp.approval.waiting', '{{count}} request(s) waiting for approval', {
        count: queue.length
      }),
      {
        action: {
          label: translate('auto.mcp.approval.review', 'Review'),
          onClick: () => useAppStore.getState().setMcpApprovalPromptOpen(true)
        }
      }
    )
  }

  const waitingReconcile = timedOut && reconciled !== current.id
  const approveLocked = lockLeft > 0 || waitingReconcile || killSwitch || busy
  const sameClient = queue.filter((q) => q.clientName === current.clientName).length
  const seconds = Math.ceil(lockLeft / 1000)
  const title = current.tool.title

  return (
    <Dialog open={open} onOpenChange={(o) => (!o ? decideLater() : undefined)}>
      <DialogContent
        className="sm:max-w-xl"
        aria-describedby="mcp-approval-desc"
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          denyRef.current?.focus()
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement)) {
            e.preventDefault()
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>
            {translate('auto.mcp.approval.title', 'An AI agent is asking permission')}
          </DialogTitle>
          <DialogDescription id="mcp-approval-desc">
            {translate(
              'auto.mcp.approval.desc',
              'Review exactly what will run. Nothing runs until you approve.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-2">
            <McpApprovalClient approval={current} />
            <span className="shrink-0 text-xs text-muted-foreground">
              {translate('auto.mcp.approval.position', 'Request {{index}} of {{total}}', {
                index: 1,
                total: queue.length
              })}
            </span>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <span className="truncate text-sm font-medium" title={title}>
                {title}
              </span>
              <code className="truncate font-mono text-xs text-muted-foreground">
                {current.tool.name}
              </code>
              <McpRiskBadge risk={current.tool.risk} />
            </div>
            <span className="text-xs text-muted-foreground">
              {waitingReconcile ? (
                translate('auto.mcp.approval.expired', 'Expired')
              ) : timedOut ? (
                translate('auto.mcp.approval.expiring', 'Expiring…')
              ) : (
                <>
                  {translate('auto.mcp.approval.expiresIn', 'Expires in')}{' '}
                  <time dateTime={current.expiresAt} aria-live="off">
                    {formatCountdown(remaining)}
                  </time>
                </>
              )}
            </span>
          </div>
          <McpApprovalArgs preview={current.argsPreview} />
          {killSwitch ? (
            <p role="status" className="text-sm text-destructive">
              {translate('auto.mcp.approval.suspended', 'MCP access is suspended')}
            </p>
          ) : null}
          {sameClient >= 3 ? (
            <p className="text-xs text-muted-foreground">
              {translate(
                'auto.mcp.approval.manyFromClient',
                'This client has {{count}} pending requests',
                {
                  count: sameClient
                }
              )}
            </p>
          ) : null}
          {error ? (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          ) : null}
        </div>
        <DialogFooter aria-busy={busy}>
          <Button variant="ghost" size="sm" onClick={decideLater} disabled={busy}>
            {translate('auto.mcp.approval.later', 'Decide later')}
          </Button>
          <Button
            ref={denyRef}
            variant="ghost"
            disabled={busy}
            aria-label={translate('auto.mcp.approval.denyLabel', 'Deny {{tool}}', { tool: title })}
            onClick={() => void decide('deny')}
          >
            {translate('auto.mcp.approval.deny', 'Deny')}
          </Button>
          <Button
            disabled={approveLocked}
            aria-disabled={approveLocked}
            aria-label={translate('auto.mcp.approval.approveLabel', 'Approve {{tool}}', {
              tool: title
            })}
            onClick={(e) => {
              if (approveLocked || !e.nativeEvent.isTrusted) {
                return
              }
              void decide('approve')
            }}
          >
            {lockLeft > 0
              ? translate('auto.mcp.approval.approveLocked', 'Approve ({{seconds}}s)', { seconds })
              : translate('auto.mcp.approval.approve', 'Approve')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
