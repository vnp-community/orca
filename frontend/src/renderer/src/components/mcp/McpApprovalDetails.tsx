import { BotIcon, ShieldAlertIcon, TerminalIcon } from 'lucide-react'
import type { McpApproval, McpRisk } from '../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import { visualizeControlChars } from './mcp-approval-display'

/** Risk is conveyed by icon + text, never colour alone. */
export function McpRiskBadge({ risk }: { risk: McpRisk }): React.JSX.Element {
  const variant =
    risk === 'destructive' || risk === 'admin'
      ? 'destructive'
      : risk === 'read'
        ? 'secondary'
        : 'outline'
  return (
    <Badge variant={variant}>
      {risk === 'exec' ? <TerminalIcon aria-hidden /> : null}
      {risk === 'destructive' || risk === 'admin' ? <ShieldAlertIcon aria-hidden /> : null}
      {mcpRiskLabel(risk)}
    </Badge>
  )
}

/** Exact arguments as a plain text node in a mono block (never HTML, markdown or links). */
export function McpApprovalArgs({
  preview,
  id
}: {
  preview: McpApproval['argsPreview']
  id?: string
}): React.JSX.Element {
  return (
    <div className="space-y-1">
      <pre
        id={id}
        tabIndex={0}
        aria-label={translate('auto.mcp.approval.argsLabel', 'Exact tool arguments')}
        className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md border border-border bg-muted p-3 font-mono text-xs"
      >
        {visualizeControlChars(preview.text)}
      </pre>
      {preview.redacted ? (
        <Badge variant="outline">{translate('auto.mcp.approval.redacted', 'Secrets hidden')}</Badge>
      ) : null}
    </div>
  )
}

export function McpApprovalClient({ approval }: { approval: McpApproval }): React.JSX.Element {
  return (
    <p className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
      <BotIcon className="size-4 shrink-0" aria-hidden />
      <span className="truncate" title={approval.clientName}>
        {approval.clientName}
      </span>
      <span className="shrink-0">· …{approval.sessionId.slice(-6)}</span>
    </p>
  )
}
