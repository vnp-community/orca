/**
 * FindingsToolbar.tsx — FE-CV-TASK-059-05
 *
 * Filters for the structural findings list. Severity / dismissed / scope are server filters;
 * kind, origin and search are applied to the loaded pages.
 *
 * @module components/review-map/findings/FindingsToolbar
 */

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { IndexFreshness } from '../../../../../shared/code-intel-types'
import type { FindingCounts } from './finding-filter'
import { tf } from './findings-i18n'
import { severityLabel } from './FindingRow'
import type { FindingKindState, FindingSeverityState } from './finding-view-model'

export function kindLabel(kind: FindingKindState): string {
  switch (kind) {
    case 'layer_violation':
      return tf('kind.layer_violation', 'Layer violations')
    case 'dependency_cycle':
      return tf('kind.dependency_cycle', 'Dependency cycles')
    case 'hotspot':
      return tf('kind.hotspot', 'Hotspots')
    case 'missing_tenant_id':
      return tf('kind.missing_tenant_id', 'Missing tenant_id')
    case 'dead_code':
      return tf('kind.dead_code', 'Possibly unused code')
    case 'rls_removed':
      return tf('kind.rls_removed', 'RLS removed')
    default:
      return tf('kind.unknown', 'Other')
  }
}

export type FindingsToolbarState = {
  kinds: FindingKindState[]
  severities: FindingSeverityState[]
  onlyIntroduced: boolean
  includeDismissed: boolean
  query: string
}

export function FindingsToolbar({
  state,
  counts,
  dismissedCount,
  freshness,
  onChange
}: {
  state: FindingsToolbarState
  counts: FindingCounts
  dismissedCount: number
  freshness: IndexFreshness | null
  onChange: (next: FindingsToolbarState) => void
}): React.JSX.Element {
  const toggle = <T,>(list: T[], value: T): T[] =>
    list.includes(value) ? list.filter((v) => v !== value) : [...list, value]
  const kindEntries = Object.entries(counts.byKind) as [FindingKindState, number][]
  const stale = freshness && freshness.state !== 'fresh'
  return (
    <div className="space-y-1 border-b p-2 text-xs">
      <div className="flex flex-wrap items-center gap-1">
        {kindEntries.map(([kind, count]) => (
          <Button
            key={kind}
            type="button"
            size="xs"
            variant={state.kinds.includes(kind) ? 'secondary' : 'outline'}
            aria-pressed={state.kinds.includes(kind)}
            onClick={() => onChange({ ...state, kinds: toggle(state.kinds, kind) })}
          >
            {kindLabel(kind)} {count}
          </Button>
        ))}
        {(['error', 'warning', 'info'] as const).map((sev) => (
          <Button
            key={sev}
            type="button"
            size="xs"
            variant={state.severities.includes(sev) ? 'secondary' : 'outline'}
            aria-pressed={state.severities.includes(sev)}
            onClick={() => onChange({ ...state, severities: toggle(state.severities, sev) })}
          >
            {severityLabel(sev)} {counts.bySeverity[sev]}
          </Button>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1">
          <input
            type="checkbox"
            checked={state.onlyIntroduced}
            onChange={(e) => onChange({ ...state, onlyIntroduced: e.target.checked })}
          />
          {tf('filter.onlyIntroduced', 'Only from this change')}
        </label>
        <label className="flex items-center gap-1">
          <input
            type="checkbox"
            checked={state.includeDismissed}
            onChange={(e) => onChange({ ...state, includeDismissed: e.target.checked })}
          />
          {tf('filter.showDismissed', 'Show dismissed ({{count}})', { count: dismissedCount })}
        </label>
        <Input
          value={state.query}
          onChange={(e) => onChange({ ...state, query: e.target.value })}
          placeholder={tf('filter.search', 'Search findings')}
          aria-label={tf('filter.search', 'Search findings')}
          className="h-7 w-44 text-xs"
        />
        {stale ? (
          <span role="status" className="text-muted-foreground">
            {tf('freshness.stale', 'Data may be out of date')}
          </span>
        ) : null}
      </div>
    </div>
  )
}
