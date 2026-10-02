import { forwardRef, useState } from 'react'
import type { McpApproval } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { McpApprovalArgs, McpRiskBadge } from '@/components/mcp/McpApprovalDetails'

function statusLabel(s: McpApproval['status']): string {
  return translate(`auto.mcp.approval.status.${s}`, s.charAt(0).toUpperCase() + s.slice(1))
}

function viaLabel(v: McpApproval['decidedVia']): string | null {
  if (!v) {
    return null
  }
  if (v === 'web') {
    return translate('auto.mcp.approval.via.web', 'Web')
  }
  return v === 'mobile'
    ? translate('auto.mcp.approval.via.mobile', 'Mobile')
    : translate('auto.mcp.approval.via.elicitation', 'Client prompt')
}

type Props = { approval: McpApproval; highlighted?: boolean; onReview?: (id: string) => void }

export const McpApprovalRow = forwardRef<HTMLLIElement, Props>(function McpApprovalRow(
  { approval: a, highlighted, onReview },
  ref
) {
  // Collapsed by default: arguments can be sensitive when others can see the screen.
  const [shown, setShown] = useState(false)
  const via = viaLabel(a.decidedVia)
  return (
    <li
      ref={ref}
      data-testid={`mcp-approval-${a.id}`}
      className={cn(
        'space-y-2 rounded-md border border-border px-3 py-2',
        highlighted && 'ring-2 ring-ring'
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="truncate text-sm font-medium" title={a.tool.title}>
            {a.tool.title}
          </span>
          <code className="font-mono text-xs text-muted-foreground">{a.tool.name}</code>
          <McpRiskBadge risk={a.tool.risk} />
          <Badge variant={a.status === 'approved' ? 'secondary' : 'outline'}>
            {statusLabel(a.status)}
          </Badge>
        </div>
        <div className="flex items-center gap-2">
          {a.status === 'pending' && onReview ? (
            <Button size="sm" onClick={() => onReview(a.id)}>
              {translate('auto.mcp.approval.review', 'Review')}
            </Button>
          ) : null}
          <Button size="sm" variant="ghost" aria-expanded={shown} onClick={() => setShown(!shown)}>
            {shown
              ? translate('auto.mcp.approval.hideArgs', 'Hide arguments')
              : translate('auto.mcp.approval.showArgs', 'Show arguments')}
          </Button>
        </div>
      </div>
      <p className="truncate text-xs text-muted-foreground" title={a.clientName}>
        {a.clientName} · {formatMcpRelativeTime(a.decidedAt ?? a.createdAt)}
        {via ? ` · ${via}` : ''}
      </p>
      {shown ? <McpApprovalArgs preview={a.argsPreview} /> : null}
    </li>
  )
})
