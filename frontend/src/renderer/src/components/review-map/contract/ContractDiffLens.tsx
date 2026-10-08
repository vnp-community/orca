/**
 * ContractDiffLens.tsx — FE-CV-TASK-059-04
 *
 * The `contract` review lens: proto / ws-channel / route / migration changes grouped by service
 * and kind, with a before/after table. The UI only maps the backend's compatibility; it never
 * says a change is safe, and "no consumers found" is not "unused".
 *
 * @module components/review-map/contract/ContractDiffLens
 */

import { useMemo, useState } from 'react'
import { ChevronRight, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { useAppStore } from '@/store'
import { cn } from '@/lib/utils'
import { useCodeIntelContractDiff } from '../../../hooks/useCodeIntelContractDiff'
import type { ContractDiffKindFilter } from '../../../hooks/useCodeIntelContractDiff'
import { REVIEW_LENS_DEFINITIONS } from '../review-lens-registry'
import type { ReviewLensProps } from '../review-lens-registry'
import {
  filterContractChanges,
  groupContractChanges,
  listContractServices,
  NO_SERVICE_GROUP_KEY
} from './contract-grouping'
import type { CompatibilityState } from './contract-grouping'
import { tc } from './contract-i18n'
import { compatibilityLabel } from './ContractCompatibilityBadge'
import { ContractChangeDetail } from './ContractChangeDetail'
import { ContractChangeTable } from './ContractChangeTable'
import { ContractMigrationGroup } from './ContractMigrationGroup'

const KIND_OPTIONS: ContractDiffKindFilter[] = ['proto', 'ws-channel', 'route', 'migration']

function kindLabel(kind: string): string {
  switch (kind) {
    case 'proto':
      return tc('kind.proto', 'Proto')
    case 'ws-channel':
      return tc('kind.wsChannel', 'WS channel')
    case 'route':
      return tc('kind.route', 'Route')
    case 'migration':
      return tc('kind.migration', 'Migration')
    default:
      return tc('kind.unknown', 'Other')
  }
}

export default function ContractDiffLens({
  worktreeId,
  environmentId,
  overlay,
  onOpenDiff,
  onSelectSymbol
}: ReviewLensProps): React.JSX.Element {
  const [kinds, setKinds] = useState<ContractDiffKindFilter[]>([])
  const [service, setService] = useState('')
  const [onlyBreaking, setOnlyBreaking] = useState(false)
  const [onlyChanged, setOnlyChanged] = useState(false)
  const [query, setQuery] = useState('')
  const [chip, setChip] = useState<CompatibilityState | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const { status, error, diff, reload } = useCodeIntelContractDiff(worktreeId, environmentId, {
    kinds,
    detail: 'full'
  })
  const setReviewLens = useAppStore((s) => s.setReviewLens)
  const setErdService = useAppStore((s) => s.setErdService)
  const selectErdTable = useAppStore((s) => s.selectErdTable)
  const erdAvailable = REVIEW_LENS_DEFINITIONS.some((d) => d.id === 'erd' && d.load)

  const changedFiles = useMemo(() => new Set(overlay.changedFiles.map((f) => f.path)), [overlay.changedFiles])
  const changes = useMemo(() => diff?.changes ?? [], [diff])
  const visible = useMemo(
    () =>
      filterContractChanges(changes, {
        service,
        onlyBreaking,
        onlyChangedByAgent: onlyChanged,
        changedFiles,
        compatibility: chip,
        query
      }),
    [changes, service, onlyBreaking, onlyChanged, changedFiles, chip, query]
  )
  const groups = useMemo(() => groupContractChanges(visible), [visible])
  const selected = visible.find((c) => c.id === selectedId) ?? null

  const openErd = (table: string, tableService: string | undefined): void => {
    setReviewLens(worktreeId, 'erd')
    if (tableService) {
      setErdService(worktreeId, tableService)
    }
    selectErdTable(worktreeId, table)
  }

  if (status === 'idle' || status === 'loading') {
    return (
      <div role="status" aria-live="polite" className="space-y-2 p-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="size-3.5 animate-spin" aria-hidden />
          {tc('loading', 'Loading contract changes...')}
        </div>
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    )
  }
  if (status === 'error' || !diff) {
    return (
      <div className="space-y-2 p-3 text-xs">
        <p className="text-muted-foreground">
          {error?.kind === 'index-missing' || error?.kind === 'tool-unavailable'
            ? tc('error.noIndex', 'The code index is not available, so contract changes cannot be listed yet.')
            : tc('error.load', 'Could not load contract changes.')}
        </p>
        <Button type="button" variant="outline" size="xs" onClick={reload}>
          {tc('retry', 'Retry')}
        </Button>
      </div>
    )
  }

  const chips: { state: CompatibilityState; count: number }[] = [
    { state: 'breaking', count: diff.summary.breaking },
    { state: 'risky', count: diff.summary.risky },
    { state: 'unknown', count: diff.summary.unknown },
    { state: 'compatible', count: diff.summary.compatible }
  ]
  const hasContent = diff.changes.length > 0 || diff.migrations.length > 0
  const services = listContractServices(changes)

  return (
    <div className="flex min-h-0 flex-col gap-2 overflow-y-auto p-3 text-xs">
      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup
          type="multiple"
          size="sm"
          value={kinds}
          onValueChange={(v) => setKinds(v as ContractDiffKindFilter[])}
          aria-label={tc('filter.kind', 'Contract kind')}
        >
          {KIND_OPTIONS.map((k) => (
            <ToggleGroupItem key={k} value={k} aria-label={kindLabel(k)}>
              {kindLabel(k)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        {services.length > 0 ? (
          <select
            aria-label={tc('filter.service', 'Service')}
            value={service}
            onChange={(e) => setService(e.target.value)}
            className="h-7 rounded-md border bg-background px-2 text-xs"
          >
            <option value="">{tc('filter.allServices', 'All services')}</option>
            {services.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        ) : null}
        <label className="flex items-center gap-1">
          <input type="checkbox" checked={onlyBreaking} onChange={(e) => setOnlyBreaking(e.target.checked)} />
          {tc('filter.onlyBreaking', 'Breaking only')}
        </label>
        <label className="flex items-center gap-1">
          <input type="checkbox" checked={onlyChanged} onChange={(e) => setOnlyChanged(e.target.checked)} />
          {tc('filter.onlyChanged', 'Only from this change')}
        </label>
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={tc('filter.search', 'Search contracts')}
          aria-label={tc('filter.search', 'Search contracts')}
          className="h-7 w-44 text-xs"
        />
      </div>

      <div role="group" aria-label={tc('summary.label', 'Compatibility summary')} className="flex flex-wrap gap-1">
        {chips.map(({ state, count }) => (
          <Button
            key={state}
            type="button"
            size="xs"
            variant={chip === state ? 'secondary' : 'outline'}
            aria-pressed={chip === state}
            onClick={() => setChip(chip === state ? null : state)}
          >
            {count} {compatibilityLabel(state)}
          </Button>
        ))}
      </div>

      {diff.truncated ? (
        <p role="status" className="text-muted-foreground">
          {tc('truncated', 'Showing {{shown}} of {{total}} changes. Narrow the filters to see the rest.', {
            shown: diff.changes.length,
            total: diff.totalCount
          })}
        </p>
      ) : null}

      {!hasContent ? (
        <p className="text-muted-foreground">
          {tc('empty', 'No contract changes were detected in this scope (proto, WS channels, routes, migrations).')}
        </p>
      ) : visible.length === 0 && diff.changes.length > 0 ? (
        <p className="text-muted-foreground">{tc('emptyFiltered', 'No changes match the current filters.')}</p>
      ) : null}

      {groups.map((group) => (
        <Collapsible key={group.service || NO_SERVICE_GROUP_KEY} defaultOpen className="rounded-md border">
          <CollapsibleTrigger className="group flex w-full items-center gap-1 px-2 py-1.5 text-left font-medium">
            <ChevronRight className="size-3.5 transition-transform group-data-[state=open]:rotate-90" aria-hidden />
            {group.service || tc('group.noService', '(unknown service)')}
            <span className="text-muted-foreground">({group.total})</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            {group.kinds.map((bucket) => (
              <div key={bucket.kindGroup} className={cn('border-t')}>
                <div className="px-2 py-1 text-[11px] font-medium text-muted-foreground">
                  {kindLabel(bucket.kindGroup)} ({bucket.changes.length})
                </div>
                <ContractChangeTable changes={bucket.changes} selectedId={selectedId} onSelect={setSelectedId} />
              </div>
            ))}
          </CollapsibleContent>
        </Collapsible>
      ))}

      {diff.migrations.length > 0 && (kinds.length === 0 || kinds.includes('migration')) ? (
        <div className="space-y-1">
          {diff.migrations.map((m) => (
            <Collapsible key={m.service} className="rounded-md border">
              <CollapsibleTrigger className="group flex w-full items-center gap-1 px-2 py-1.5 text-left font-medium">
                <ChevronRight className="size-3.5 transition-transform group-data-[state=open]:rotate-90" aria-hidden />
                {tc('migration.title', 'Migration · {{service}}', { service: m.service })}
              </CollapsibleTrigger>
              <CollapsibleContent>
                <ContractMigrationGroup migration={m} onOpenErd={erdAvailable ? openErd : undefined} />
              </CollapsibleContent>
            </Collapsible>
          ))}
        </div>
      ) : null}

      {selected ? (
        <div className="rounded-md border">
          <ContractChangeDetail
            change={selected}
            worktreeId={worktreeId}
            changedFiles={changedFiles}
            onOpenDiff={onOpenDiff}
            onSelectSymbol={onSelectSymbol}
            onOpenErd={erdAvailable ? openErd : undefined}
          />
        </div>
      ) : null}
    </div>
  )
}
