import type { McpAuditEntry } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import {
  EMPTY_AUDIT_FILTERS,
  hasAuditFilters,
  type McpAuditFilterIssue,
  type McpAuditFilterState
} from './mcp-audit-filters'

const ALL = '__all__'
type Decision = McpAuditEntry['decision']

export function auditDecisionLabel(d: Decision): string {
  switch (d) {
    case 'allow':
      return translate('auto.mcp.audit.decision.allow', 'Allowed')
    case 'deny':
      return translate('auto.mcp.audit.decision.deny', 'Denied')
    case 'approved':
      return translate('auto.mcp.audit.decision.approved', 'Approved')
    case 'denied':
      return translate('auto.mcp.audit.decision.denied', 'Rejected (denied after prompt)')
    default:
      return translate('auto.mcp.audit.decision.expired', 'Expired')
  }
}

const isoDay = (d: Date): string =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

export function McpAuditFilters({
  value,
  onChange,
  issue,
  toolNames
}: {
  value: McpAuditFilterState
  onChange: (next: McpAuditFilterState) => void
  issue: McpAuditFilterIssue
  toolNames: string[]
}): React.JSX.Element {
  const set = (p: Partial<McpAuditFilterState>): void => onChange({ ...value, ...p })
  const lastDays = (days: number): void => {
    const now = new Date()
    set({ from: isoDay(new Date(now.getTime() - days * 86_400_000)), to: isoDay(now) })
  }
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label htmlFor="mcp-audit-from" className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.from', 'From')}
          </Label>
          <Input
            id="mcp-audit-from"
            type="date"
            className="h-8"
            value={value.from}
            onChange={(e) => set({ from: e.target.value })}
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor="mcp-audit-to" className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.to', 'To')}
          </Label>
          <Input
            id="mcp-audit-to"
            type="date"
            className="h-8"
            value={value.to}
            onChange={(e) => set({ to: e.target.value })}
          />
        </div>
        <Button variant="ghost" size="sm" onClick={() => lastDays(1)}>
          {translate('auto.mcp.audit.last24h', '24h')}
        </Button>
        <Button variant="ghost" size="sm" onClick={() => lastDays(7)}>
          {translate('auto.mcp.audit.last7d', '7 days')}
        </Button>
        <Button variant="ghost" size="sm" onClick={() => lastDays(30)}>
          {translate('auto.mcp.audit.last30d', '30 days')}
        </Button>
        <div className="space-y-1">
          <Label htmlFor="mcp-audit-user" className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.user', 'User ID')}
          </Label>
          <Input
            id="mcp-audit-user"
            className="h-8 w-72 font-mono text-xs"
            aria-invalid={issue === 'userId' ? true : undefined}
            aria-describedby="mcp-audit-filter-msg"
            value={value.userId}
            onChange={(e) => set({ userId: e.target.value })}
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor="mcp-audit-tool" className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.tool', 'Tool')}
          </Label>
          <Input
            id="mcp-audit-tool"
            list="mcp-audit-tools"
            className="h-8 font-mono text-xs"
            value={value.tool}
            onChange={(e) => set({ tool: e.target.value })}
          />
          <datalist id="mcp-audit-tools">
            {toolNames.map((n) => (
              <option key={n} value={n} />
            ))}
          </datalist>
        </div>
        <div className="space-y-1">
          <Label htmlFor="mcp-audit-decision" className="text-xs text-muted-foreground">
            {translate('auto.mcp.audit.decisionLabel', 'Decision')}
          </Label>
          <Select
            value={value.decision || ALL}
            onValueChange={(v) => set({ decision: v === ALL ? '' : (v as Decision) })}
          >
            <SelectTrigger id="mcp-audit-decision" size="sm" className="w-56">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{translate('auto.mcp.audit.all', 'All')}</SelectItem>
              {(['allow', 'deny', 'approved', 'denied', 'expired'] as Decision[]).map((d) => (
                <SelectItem key={d} value={d}>
                  {auditDecisionLabel(d)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {hasAuditFilters(value) ? (
          <Button variant="ghost" size="sm" onClick={() => onChange(EMPTY_AUDIT_FILTERS)}>
            {translate('auto.mcp.audit.clear', 'Clear filters')}
          </Button>
        ) : null}
      </div>
      <p
        id="mcp-audit-filter-msg"
        role={issue ? 'alert' : undefined}
        className="text-xs text-destructive"
      >
        {issue === 'userId'
          ? translate('auto.mcp.audit.userIdInvalid', 'User ID must be a UUID.')
          : null}
        {issue === 'range'
          ? translate(
              'auto.mcp.audit.rangeInvalid',
              'The start date must not be after the end date.'
            )
          : null}
      </p>
    </div>
  )
}
