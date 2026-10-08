/**
 * ErdLens.tsx — FE-CV-TASK-057-05
 *
 * `erd` review lens: tables, keys, logical links between services, and the columns a
 * migration in this change touches. Errors and warnings are inline (never toasts).
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useAppStore } from '@/store'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { usePerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import { isEditableTarget } from '@/lib/editable-target'
import { useCodeIntelErd } from '../../../hooks/useCodeIntelErd'
import type { ReviewLensProps } from '../review-lens-registry'
import { ReviewLoadingStage } from '../shell/ReviewLoadingStage'
import { ErdCanvas } from './ErdCanvas'
import { ErdLegend } from './ErdLegend'
import { ErdTableDetail } from './ErdTableDetail'
import { ErdTableList } from './ErdTableList'
import { ErdToolbar, type ErdViewMode } from './ErdToolbar'
import { ErdWarningsStrip } from './ErdWarningsStrip'
import { layoutErdTables } from './erd-layout'
import { defaultErdFilterMode, filterErdTables, type ErdFilterMode } from './erd-table-filter'
import { buildErdViewModel } from './erd-view-model'

const t = translate
const EMPTY_SET: ReadonlySet<string> = new Set()

function errorMessage(kind: string | undefined): string {
  switch (kind) {
    case 'offline':
      return t(
        'auto.components.reviewMap.ErdLens.error.offline',
        'The connection to the workspace is down.'
      )
    case 'forbidden':
      return t(
        'auto.components.reviewMap.ErdLens.error.forbidden',
        'You do not have access to this repository.'
      )
    case 'too-large':
      return t(
        'auto.components.reviewMap.ErdLens.error.tooLarge',
        'The ERD is too large to load at once.'
      )
    case 'timeout':
      return t('auto.components.reviewMap.ErdLens.error.timeout', 'Building the ERD took too long.')
    case 'no-binding':
    case 'unsupported':
    case 'tool-unavailable':
      return t(
        'auto.components.reviewMap.ErdLens.error.unavailable',
        'The ERD is not available for this repository.'
      )
    default:
      return t('auto.components.reviewMap.ErdLens.error.generic', 'The ERD could not be loaded.')
  }
}

function InlineError({
  kind,
  onRetry
}: {
  kind: string | undefined
  onRetry: () => void
}): React.JSX.Element {
  return (
    <div role="alert" className="flex flex-col items-start gap-2 p-4 text-sm">
      <p>{errorMessage(kind)}</p>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        {t('auto.components.reviewMap.ErdLens.retry', 'Retry')}
      </Button>
    </div>
  )
}

export default function ErdLens(props: ReviewLensProps): React.JSX.Element {
  const { worktreeId, environmentId, scope, overlay, onOpenDiff, onSelectSymbol } = props
  const service = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.erdService ?? null)
  const hasHistory = useAppStore(
    (s) => (s.reviewUiByWorktree[worktreeId]?.erdServiceHistory?.length ?? 0) > 0
  )
  const selectedKey = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.selectedErdTable ?? null)
  const setErdService = useAppStore((s) => s.setErdService)
  const goBackErdService = useAppStore((s) => s.goBackErdService)
  const selectErdTable = useAppStore((s) => s.selectErdTable)

  const [dialect, setDialect] = useState<'postgres' | 'mysql' | null>(null)
  const [query, setQuery] = useState('')
  const [schemas, setSchemas] = useState<ReadonlySet<string>>(EMPTY_SET)
  const [modeChoice, setModeChoice] = useState<ErdFilterMode | null>(null)
  const [view, setView] = useState<ErdViewMode>('graph')
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(EMPTY_SET)
  const searchRef = useRef<HTMLInputElement>(null)

  const erd = useCodeIntelErd({ worktreeId, environmentId, scope, service, dialect })
  const model = erd.model.data
  const pending =
    erd.services.status === 'idle' ||
    erd.services.status === 'loading' ||
    erd.model.status === 'loading'
  const stage = usePerceivedLoadingStage(pending, { remote: environmentId !== null })

  const vm = useMemo(() => (model ? buildErdViewModel(model, overlay) : null), [model, overlay])
  const mode: ErdFilterMode =
    modeChoice ??
    (vm && vm.tables.some((x) => x.tableChange !== 'untouched')
      ? defaultErdFilterMode(vm.tables.length)
      : 'all')
  const filtered = useMemo(
    () =>
      vm
        ? filterErdTables({ tables: vm.tables, relations: vm.relations, query, schemas, mode })
        : null,
    [vm, query, schemas, mode]
  )
  const schemaList = useMemo(
    () => [...new Set((vm?.tables ?? []).map((x) => x.schema))].sort(),
    [vm]
  )
  const layout = useMemo(
    () =>
      filtered && vm
        ? layoutErdTables({
            tables: filtered.visible,
            relations: vm.relations,
            expandedTables: expanded,
            groupBySchema: schemaList.length > 1
          })
        : null,
    [filtered, vm, expanded, schemaList.length]
  )

  useEffect(() => setExpanded(EMPTY_SET), [model?.service])

  const selectedTable = vm?.tables.find((x) => x.key === selectedKey) ?? null
  const toggleExpand = (key: string): void =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (!next.delete(key)) {
        next.add(key)
      }
      return next
    })
  const openService = (name: string): void => {
    setDialect(null)
    setErdService(worktreeId, name)
  }

  const onKeyDown = (e: React.KeyboardEvent): void => {
    if (isEditableTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) {
      return
    }
    if (e.key === '/') {
      e.preventDefault()
      searchRef.current?.focus()
    } else if (e.key === 'f') {
      setModeChoice(mode === 'focus' ? 'all' : 'focus')
    } else if (e.key === 'g') {
      setView('graph')
    } else if (e.key === 'l') {
      setView('list')
    } else if (e.key === 'Escape' && selectedKey) {
      selectErdTable(worktreeId, null)
    }
  }

  const services = erd.services.data ?? []
  const toolbar = (
    <ErdToolbar
      ref={searchRef}
      services={services}
      service={erd.resolved.service}
      dialect={erd.resolved.dialect}
      dialects={services.find((x) => x.name === erd.resolved.service)?.dialects ?? []}
      schemas={schemaList}
      selectedSchemas={schemas}
      query={query}
      mode={mode}
      view={view}
      asOfMigration={model?.asOfMigration ?? null}
      disabled={pending}
      onService={openService}
      onDialect={setDialect}
      onSchemas={(list) => setSchemas(new Set(list))}
      onQuery={setQuery}
      onMode={setModeChoice}
      onView={setView}
    />
  )

  let body: React.JSX.Element
  if (erd.services.status === 'error') {
    body = <InlineError kind={erd.services.error?.kind} onRetry={erd.reload} />
  } else if (pending || !erd.resolved.service) {
    body =
      !pending && erd.services.status === 'success' ? (
        <p className="p-4 text-sm text-muted-foreground">
          {t(
            'auto.components.reviewMap.ErdLens.noMigrations',
            'No services with migrations were found in this repository.'
          )}
        </p>
      ) : (
        <ReviewLoadingStage
          stage={stage}
          stageLabel={t('auto.components.reviewMap.ErdLens.building', 'Building the ERD...')}
        />
      )
  } else if (erd.model.status === 'error') {
    body = <InlineError kind={erd.model.error?.kind} onRetry={erd.reload} />
  } else if (!vm || !filtered || !layout) {
    body = (
      <ReviewLoadingStage
        stage={stage}
        stageLabel={t('auto.components.reviewMap.ErdLens.building', 'Building the ERD...')}
      />
    )
  } else if (vm.tables.length === 0) {
    body = (
      <p className="p-4 text-sm text-muted-foreground">
        {t(
          'auto.components.reviewMap.ErdLens.emptyService',
          'This service has no tables in its migrations.'
        )}
      </p>
    )
  } else {
    const degraded = vm.tables.filter((x) => x.degraded).map((x) => x.name)
    const noLinks = vm.tables.every((x) => x.accessedBy.length === 0)
    body = (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex flex-col gap-1 px-3 py-1.5">
          <ErdLegend />
          <ErdWarningsStrip warnings={model?.warnings ?? []} degradedTables={degraded} />
          {erd.isStale ? (
            <Button
              type="button"
              variant="outline"
              size="xs"
              className="w-fit"
              onClick={erd.reload}
            >
              <RefreshCw aria-hidden="true" />
              {t('auto.components.reviewMap.ErdLens.newData', 'New data available - refresh')}
            </Button>
          ) : null}
          {erd.model.truncated ? (
            <p role="status" className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.ErdLens.backendTruncated',
                'The backend limited this result.'
              )}
            </p>
          ) : null}
          {filtered.truncated.capped ? (
            <p role="status" className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.ErdLens.showingCount',
                'Showing {{shown}} of {{total}} tables; the rest are in the list.',
                filtered.truncated
              )}
            </p>
          ) : null}
          {vm.degradedToTableLevel ? (
            <p className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.ErdLens.tableLevelOnly',
                'Column details are not available; changed tables are highlighted.'
              )}
            </p>
          ) : null}
          {noLinks ? (
            <p className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.ErdLens.noCodeLinks',
                'No links to code yet (the code index may be missing).'
              )}
            </p>
          ) : null}
        </div>
        <div className="flex min-h-0 flex-1">
          {filtered.visible.length === 0 ? (
            <div className="flex flex-1 flex-col items-start gap-2 p-4 text-sm text-muted-foreground">
              <p>
                {t(
                  'auto.components.reviewMap.ErdLens.noMatches',
                  'No tables match the current filters.'
                )}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setQuery('')
                  setSchemas(EMPTY_SET)
                  setModeChoice('all')
                }}
              >
                {t('auto.components.reviewMap.ErdLens.clearFilters', 'Clear filters')} (
                {vm.tables.length})
              </Button>
            </div>
          ) : view === 'graph' ? (
            <ErdCanvas
              service={model?.service ?? ''}
              tables={filtered.visible}
              relations={vm.relations}
              ghosts={vm.ghosts}
              layout={layout}
              expandedTables={expanded}
              query={query}
              dimmed={filtered.dimmed}
              selectedKey={selectedKey}
              onSelect={(key) => selectErdTable(worktreeId, key)}
              onToggleExpand={toggleExpand}
              onOpenService={openService}
            />
          ) : (
            <ErdTableList
              tables={filtered.visible}
              selectedKey={selectedKey}
              dimmed={filtered.dimmed}
              onSelect={(key) => selectErdTable(worktreeId, key)}
            />
          )}
          {selectedTable ? (
            <ErdTableDetail
              worktreeId={worktreeId}
              table={selectedTable}
              relations={vm.relations}
              changes={model?.changes ?? []}
              service={model?.service ?? ''}
              overlay={overlay}
              onOpenSymbol={onSelectSymbol}
              onOpenDiff={onOpenDiff}
              onOpenService={openService}
              onClose={() => selectErdTable(worktreeId, null)}
            />
          ) : null}
        </div>
      </div>
    )
  }

  return (
    <section
      className="flex min-h-0 flex-1 flex-col"
      aria-label={t('auto.components.reviewMap.ErdLens.label', 'Entity-relationship diagram')}
      onKeyDown={onKeyDown}
    >
      {toolbar}
      {hasHistory ? (
        <div className="border-b px-3 py-1">
          <Button
            type="button"
            variant="ghost"
            size="xs"
            onClick={() => {
              setDialect(null)
              goBackErdService(worktreeId)
            }}
          >
            {t('auto.components.reviewMap.ErdLens.back', 'Back to previous service')}
          </Button>
        </div>
      ) : null}
      {body}
    </section>
  )
}
