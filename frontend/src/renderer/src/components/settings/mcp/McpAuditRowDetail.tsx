import type { McpAuditEntry } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { McpCopyButton } from './McpCopyButton'
import { auditDecisionLabel } from './McpAuditFilters'

function Field({
  label,
  children
}: {
  label: string
  children: React.ReactNode
}): React.JSX.Element {
  return (
    <div className="grid grid-cols-[8rem_1fr] gap-2 text-sm">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-all">{children}</dd>
    </div>
  )
}

export function McpAuditRowDetail({
  entry,
  onClose
}: {
  entry: McpAuditEntry
  onClose: () => void
}): React.JSX.Element {
  const dash = '—'
  return (
    <Sheet open onOpenChange={(o) => (!o ? onClose() : undefined)}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle className="font-mono text-base">{entry.tool}</SheetTitle>
          <SheetDescription>{new Date(entry.at).toLocaleString()}</SheetDescription>
        </SheetHeader>
        <dl className="space-y-2 px-4 pb-4">
          <Field label="ID">
            <span className="font-mono text-xs">{entry.id}</span>
          </Field>
          <Field label={translate('auto.mcp.audit.time', 'Time')}>
            {new Date(entry.at).toLocaleString()}{' '}
            <span className="text-xs text-muted-foreground">({entry.at})</span>
          </Field>
          <Field label={translate('auto.mcp.audit.actingUser', 'Acting user')}>
            {entry.userName ?? entry.userId}
          </Field>
          <Field label={translate('auto.mcp.audit.agent', 'Agent')}>{entry.clientName}</Field>
          <Field label={translate('auto.mcp.audit.session', 'MCP session')}>
            <span className="font-mono text-xs">{entry.sessionId}</span>
          </Field>
          <Field label={translate('auto.mcp.audit.tool', 'Tool')}>
            <span className="font-mono text-xs">{entry.tool}</span>
          </Field>
          <Field label={translate('auto.mcp.audit.risk', 'Risk')}>{mcpRiskLabel(entry.risk)}</Field>
          <Field label={translate('auto.mcp.audit.decisionLabel', 'Decision')}>
            {auditDecisionLabel(entry.decision)}
          </Field>
          <Field label={translate('auto.mcp.audit.approver', 'Approver')}>
            {entry.approver ?? dash}
          </Field>
          <Field label={translate('auto.mcp.audit.result', 'Result')}>{entry.result ?? dash}</Field>
          <Field label={translate('auto.mcp.audit.duration', 'Duration')}>
            {entry.durationMs !== undefined ? `${entry.durationMs} ms` : dash}
          </Field>
          <Field label={translate('auto.mcp.audit.trace', 'Trace ID')}>
            {entry.traceId ? (
              <span className="inline-flex items-center gap-1">
                <span className="font-mono text-xs">{entry.traceId}</span>
                <McpCopyButton
                  text={entry.traceId}
                  ariaLabel={translate('auto.mcp.audit.copyTrace', 'Copy trace ID')}
                />
              </span>
            ) : (
              dash
            )}
          </Field>
        </dl>
        <div className="space-y-1 px-4 pb-4">
          <h4 className="text-sm font-medium">
            {translate('auto.mcp.audit.args', 'Arguments summary')}
          </h4>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md border border-border bg-muted p-3 font-mono text-xs">
            {entry.argsSummary}
          </pre>
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.mcp.audit.argsNote',
              'Secrets are redacted before logging. Full arguments are never stored.'
            )}
          </p>
        </div>
      </SheetContent>
    </Sheet>
  )
}
