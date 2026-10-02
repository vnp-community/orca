import { useEffect, useState } from 'react'
import { BotIcon } from 'lucide-react'
import type { McpAuditEntry, McpToolView } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { useMcpQuery } from '@/hooks/useMcpQuery'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpAuditFilters, auditDecisionLabel } from './McpAuditFilters'
import { McpAuditRowDetail } from './McpAuditRowDetail'
import { downloadMcpAuditCsv, toCsv } from './mcp-audit-csv'
import {
  EMPTY_AUDIT_FILTERS,
  auditFilterIssue,
  hasAuditFilters,
  type McpAuditFilterState
} from './mcp-audit-filters'
import { useMcpAuditQuery } from './useMcpAuditQuery'

const DEBOUNCE_MS = 300

function decisionVariant(d: McpAuditEntry['decision']): 'secondary' | 'destructive' | 'outline' {
  if (d === 'allow' || d === 'approved') {
    return 'secondary'
  }
  return d === 'expired' ? 'outline' : 'destructive'
}

export function McpAuditTab(): React.JSX.Element {
  const [form, setForm] = useState<McpAuditFilterState>(EMPTY_AUDIT_FILTERS)
  const [applied, setApplied] = useState<McpAuditFilterState>(EMPTY_AUDIT_FILTERS)
  const [detail, setDetail] = useState<McpAuditEntry | null>(null)
  const tools = useMcpQuery('mcp.admin.tool.list', {}, [] as McpToolView[])
  const q = useMcpAuditQuery(applied)

  useEffect(() => {
    const t = window.setTimeout(() => setApplied(form), DEBOUNCE_MS)
    return () => window.clearTimeout(t)
  }, [form])

  const issue = auditFilterIssue(form)
  const filtered = hasAuditFilters(applied)

  let body: React.ReactNode
  if (q.status === 'loading') {
    body = <McpListSkeleton rows={5} />
  } else if (q.status === 'forbidden') {
    body = (
      <McpMutedNote>
        {translate('auto.mcp.admin.required', 'Administrator access required.')}
      </McpMutedNote>
    )
  } else if (q.status === 'unavailable') {
    body = (
      <McpMutedNote>
        {translate(
          'auto.mcp.audit.unavailable',
          "The audit log isn't available on this server yet."
        )}
      </McpMutedNote>
    )
  } else if (q.status === 'error') {
    body = <McpInlineAlert message={q.error ?? ''} onRetry={q.reload} />
  } else if (q.status === 'idle') {
    body = null
  } else if (q.entries.length === 0) {
    body = (
      <div className="space-y-2">
        <McpMutedNote>
          {filtered
            ? translate('auto.mcp.audit.emptyFiltered', 'No entries match these filters.')
            : translate(
                'auto.mcp.audit.empty',
                'No agent activity yet. Actions taken by connected AI clients will appear here.'
              )}
        </McpMutedNote>
        {filtered ? (
          <Button variant="ghost" size="sm" onClick={() => setForm(EMPTY_AUDIT_FILTERS)}>
            {translate('auto.mcp.audit.clear', 'Clear filters')}
          </Button>
        ) : null}
      </div>
    )
  } else {
    body = (
      <div className="space-y-2">
        <div className="overflow-x-auto">
          <Table>
            <caption className="sr-only">
              {translate('auto.mcp.audit.caption', 'Agent actions')}
            </caption>
            <TableHeader>
              <TableRow>
                {(
                  [
                    'time',
                    'user',
                    'client',
                    'tool',
                    'risk',
                    'decision',
                    'approver',
                    'result',
                    'duration'
                  ] as const
                ).map((c) => (
                  <TableHead key={c} scope="col">
                    {translate(`auto.mcp.audit.col.${c}`, c.charAt(0).toUpperCase() + c.slice(1))}
                  </TableHead>
                ))}
                <TableHead scope="col">
                  <span className="sr-only">
                    {translate('auto.mcp.sessions.col.actions', 'Actions')}
                  </span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {q.entries.map((e) => {
                const when = new Date(e.at).toLocaleString()
                return (
                  <TableRow
                    key={e.id}
                    tabIndex={0}
                    onKeyDown={(ev) =>
                      ev.key === 'Enter' && ev.target === ev.currentTarget && setDetail(e)
                    }
                  >
                    <TableCell>
                      <time dateTime={e.at} title={e.at}>
                        {when}
                      </time>
                    </TableCell>
                    <TableCell title={e.userId}>{e.userName ?? e.userId.slice(0, 8)}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        <BotIcon aria-hidden />
                        {translate('auto.mcp.audit.agentBadge', 'Agent')}
                      </Badge>{' '}
                      <span className="text-xs">{e.clientName}</span>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{e.tool}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          e.risk === 'read'
                            ? 'secondary'
                            : e.risk === 'destructive' || e.risk === 'admin'
                              ? 'destructive'
                              : 'outline'
                        }
                      >
                        {mcpRiskLabel(e.risk)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={decisionVariant(e.decision)}>
                        {auditDecisionLabel(e.decision)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs">{e.approver ?? '—'}</TableCell>
                    <TableCell className="text-xs">{e.result ?? '—'}</TableCell>
                    <TableCell className="text-xs">
                      {e.durationMs !== undefined ? `${e.durationMs} ms` : '—'}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={translate(
                          'auto.mcp.audit.openDetails',
                          'Open details for {{tool}} at {{time}}',
                          { tool: e.tool, time: when }
                        )}
                        onClick={() => setDetail(e)}
                      >
                        {translate('auto.mcp.audit.details', 'Details')}
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
        {q.moreError ? <McpInlineAlert message={q.moreError} onRetry={q.loadMore} /> : null}
        {q.nextCursor ? (
          <Button
            variant="outline"
            size="sm"
            disabled={q.loadingMore}
            aria-busy={q.loadingMore}
            onClick={q.loadMore}
          >
            {translate('auto.mcp.audit.loadMore', 'Load more')}
          </Button>
        ) : (
          <p className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.end', 'End of results · {{count}} entries loaded', {
              count: q.entries.length
            })}
          </p>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.audit.intro',
          'Actions taken by AI agents through MCP. Direct actions by people are in Organization › Audit Log.'
        )}
      </p>
      <McpAuditFilters
        value={form}
        onChange={setForm}
        issue={issue}
        toolNames={tools.data.map((t) => t.name)}
      />
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={q.entries.length === 0}
          title={translate(
            'auto.mcp.audit.exportHint',
            'Exports only the {{count}} rows currently loaded. Load more or narrow the date range first.',
            { count: q.entries.length }
          )}
          onClick={() => downloadMcpAuditCsv(toCsv(q.entries))}
        >
          {translate('auto.mcp.audit.export', 'Export loaded rows (CSV)')}
        </Button>
        <span aria-live="polite" className="text-xs text-muted-foreground">
          {q.status === 'ready'
            ? translate('auto.mcp.audit.loaded', '{{count}} entries loaded', {
                count: q.entries.length
              })
            : ''}
        </span>
      </div>
      {body}
      {detail ? <McpAuditRowDetail entry={detail} onClose={() => setDetail(null)} /> : null}
    </div>
  )
}
