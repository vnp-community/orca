import { RefreshCwIcon } from 'lucide-react'
import type { McpDecision, McpRisk } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { decisionLabel } from './McpToolDecisionBadge'
import {
  RISK_ORDER,
  hasActiveToolFilters,
  DEFAULT_TOOL_FILTERS,
  type ToolFilters,
  type ToolGroupBy
} from './mcp-tool-catalog-grouping'

type Props = {
  filters: ToolFilters
  onFiltersChange: (next: ToolFilters) => void
  groupBy: ToolGroupBy
  onGroupByChange: (next: ToolGroupBy) => void
  namespaces: string[]
  disabled: boolean
  onRefresh: () => void
}

const DECISIONS: McpDecision[] = ['allow', 'require_approval', 'deny']

function FilterSelect({
  label,
  value,
  onChange,
  allLabel,
  options,
  disabled
}: {
  label: string
  value: string
  onChange: (v: string) => void
  allLabel: string
  options: { value: string; label: string }[]
  disabled: boolean
}): React.JSX.Element {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger size="sm" aria-label={label} className="w-40">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">{allLabel}</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export function McpToolsToolbar({
  filters,
  onFiltersChange,
  groupBy,
  onGroupByChange,
  namespaces,
  disabled,
  onRefresh
}: Props): React.JSX.Element {
  const set = (patch: Partial<ToolFilters>): void => onFiltersChange({ ...filters, ...patch })
  const search = translate('auto.mcp.tools.search', 'Search tools')
  return (
    <div
      className="flex flex-wrap items-center gap-2"
      role="group"
      aria-label={translate('auto.mcp.tools.filters', 'Tool filters')}
    >
      <Input
        value={filters.query}
        disabled={disabled}
        onChange={(e) => set({ query: e.target.value })}
        aria-label={search}
        placeholder={search}
        className="h-8 w-48"
      />
      <FilterSelect
        label={translate('auto.mcp.tools.filter.namespace', 'Namespace')}
        allLabel={translate('auto.mcp.tools.filter.allNamespaces', 'All namespaces')}
        value={filters.namespace}
        disabled={disabled}
        onChange={(v) => set({ namespace: v })}
        options={namespaces.map((n) => ({ value: n, label: n }))}
      />
      <FilterSelect
        label={translate('auto.mcp.tools.filter.risk', 'Risk')}
        allLabel={translate('auto.mcp.tools.filter.allRisks', 'All risks')}
        value={filters.risk}
        disabled={disabled}
        onChange={(v) => set({ risk: v as McpRisk | 'all' })}
        options={RISK_ORDER.map((r) => ({ value: r, label: mcpRiskLabel(r) }))}
      />
      <FilterSelect
        label={translate('auto.mcp.tools.filter.decision', 'Policy')}
        allLabel={translate('auto.mcp.tools.filter.allDecisions', 'All policies')}
        value={filters.decision}
        disabled={disabled}
        onChange={(v) => set({ decision: v as McpDecision | 'all' })}
        options={DECISIONS.map((d) => ({ value: d, label: decisionLabel(d) }))}
      />
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        disabled={disabled}
        value={String(filters.pack)}
        aria-label={translate('auto.mcp.tools.filter.pack', 'Pack')}
        onValueChange={(v) =>
          v && set({ pack: v === 'all' ? 'all' : (Number(v) as 1 | 2 | 3 | 4) })
        }
      >
        <ToggleGroupItem value="all">
          {translate('auto.mcp.tools.filter.allPacks', 'All')}
        </ToggleGroupItem>
        {[1, 2, 3, 4].map((n) => (
          <ToggleGroupItem key={n} value={String(n)}>
            {n}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <div className="flex items-center gap-1.5">
        <Checkbox
          id="mcp-tools-hide-blocked"
          disabled={disabled}
          checked={filters.hideHardDenied}
          onCheckedChange={(c) => set({ hideHardDenied: c === true })}
        />
        <Label htmlFor="mcp-tools-hide-blocked" className="text-sm font-normal">
          {translate('auto.mcp.tools.filter.hideBlocked', 'Hide always-blocked')}
        </Label>
      </div>
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        disabled={disabled}
        value={groupBy}
        aria-label={translate('auto.mcp.tools.groupByLabel', 'Group by')}
        onValueChange={(v) => v && onGroupByChange(v as ToolGroupBy)}
      >
        <ToggleGroupItem value="namespace">
          {translate('auto.mcp.tools.groupBy.namespace', 'Namespace')}
        </ToggleGroupItem>
        <ToggleGroupItem value="pack">
          {translate('auto.mcp.tools.groupBy.pack', 'Pack')}
        </ToggleGroupItem>
        <ToggleGroupItem value="risk">
          {translate('auto.mcp.tools.groupBy.risk', 'Risk')}
        </ToggleGroupItem>
      </ToggleGroup>
      {hasActiveToolFilters(filters) ? (
        <Button variant="ghost" size="sm" onClick={() => onFiltersChange(DEFAULT_TOOL_FILTERS)}>
          {translate('auto.mcp.tools.clearFilters', 'Clear filters')}
        </Button>
      ) : null}
      <Button variant="ghost" size="sm" disabled={disabled} onClick={onRefresh}>
        <RefreshCwIcon aria-hidden />
        {translate('auto.mcp.tools.refresh', 'Refresh')}
      </Button>
    </div>
  )
}
