import { Suspense, lazy, useCallback, useEffect, useMemo, useState } from 'react'
import { Info } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { useCodeIntelQuery } from '@/hooks/useCodeIntelQuery'
import type { CodeIntelQueryError, CodeIntelQueryResult } from '@/hooks/useCodeIntelQuery'
import { usePerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { openReviewFileInEditor } from '@/lib/review-diff-navigation'
import type { ImpactGraph } from '../../../../../shared/code-intel-graph-types'
import type { ReviewLensProps } from '../review-lens-registry'
import { overlayFlagsWithData } from '../review-overlay-model'
import type { SymbolRefView } from '../review-wire-types'
import { ReviewLoadingStage } from '../shell/ReviewLoadingStage'
import { layoutImpactColumns } from './impact-column-layout'
import {
  clampImpactDepth,
  collectAffectedKeys,
  defaultImpactView,
  describeRisk,
  findChangedSymbol,
  resolveImpactCenter
} from './impact-lens-model'
import { ImpactColumnsList } from './ImpactColumnsList'
import { ImpactToolbar } from './ImpactToolbar'
import type { ImpactDirection, ImpactView } from './ImpactToolbar'
import type { ImpactNodeActions } from './ImpactNodeCard'
import { ambiguousCandidates } from './use-symbol-detail'
import type { SymbolTarget } from './use-symbol-detail'

const ImpactGraphCanvas = lazy(() => import('./ImpactGraphCanvas'))
const t = translate

function errorText(error: CodeIntelQueryError): string {
  switch (error.kind) {
    case 'timeout':
      return t('auto.components.reviewMap.impact.errTimeout', 'The impact query timed out.')
    case 'too-large':
      return t(
        'auto.components.reviewMap.impact.errTooLarge',
        'The result is too large. Lower the depth or pick another symbol.'
      )
    case 'not-found':
      return t(
        'auto.components.reviewMap.impact.errNotFound',
        'This symbol was not found in the index; the index may be out of date.'
      )
    default:
      return t('auto.components.reviewMap.impact.errFailed', 'The impact query failed.')
  }
}

function useImpactQuery(
  props: Pick<ReviewLensProps, 'worktreeId' | 'environmentId'>,
  target: SymbolTarget | null,
  direction: 'upstream' | 'downstream',
  enabled: boolean,
  depth: number
): CodeIntelQueryResult<ImpactGraph> {
  return useCodeIntelQuery<ImpactGraph>(props.worktreeId, props.environmentId, {
    method: 'impact',
    enabled: enabled && target !== null,
    params: {
      target: target ?? {},
      direction,
      depth: clampImpactDepth(depth),
      includeTests: false
    }
  })
}

/** Impact lens: callers/dependencies of one changed symbol, laid out by depth. */
export default function ImpactLens(props: ReviewLensProps): React.JSX.Element {
  const { worktreeId, overlay, selectedSymbolKey, onSelectSymbol, onOpenDiff } = props
  const focusKey = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.impactFocusKey ?? null)
  const centerKey = resolveImpactCenter(focusKey, selectedSymbolKey, overlay)
  const [override, setOverride] = useState<SymbolTarget | null>(null)
  const [direction, setDirection] = useState<ImpactDirection>('both')
  const [depth, setDepth] = useState(2)
  const [viewChoice, setViewChoice] = useState<ImpactView | null>(null)
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set())

  useEffect(() => setOverride(null), [centerKey])
  useEffect(() => setExpanded(new Set()), [centerKey, depth, direction])

  const target: SymbolTarget | null = override ?? (centerKey ? { key: centerKey } : null)
  const up = useImpactQuery(props, target, 'upstream', direction !== 'downstream', depth)
  const down = useImpactQuery(props, target, 'downstream', direction !== 'upstream', depth)

  const queries = [direction !== 'downstream' ? up : null, direction !== 'upstream' ? down : null]
  const active = queries.filter((q): q is CodeIntelQueryResult<ImpactGraph> => q !== null)
  const pending = active.some((q) => q.status === 'loading' || q.status === 'idle')
  const stage = usePerceivedLoadingStage(Boolean(target) && pending, {
    remote: props.environmentId !== null
  })

  // Ambiguous names: ask once, then continue with the chosen target.
  const ambiguous = active.find((q) => q.error?.kind === 'ambiguous')?.error ?? null
  useEffect(() => {
    if (!ambiguous) {
      return
    }
    let cancelled = false
    void props.requestSymbolChoice(ambiguousCandidates(ambiguous)).then((choice) => {
      if (cancelled || !choice) {
        return
      }
      setOverride(choice)
    })
    return () => {
      cancelled = true
    }
    // requestSymbolChoice identity is not stable across renders in every host.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ambiguous])

  const upGraph = direction !== 'downstream' && up.status === 'success' ? up.data : null
  const downGraph = direction !== 'upstream' && down.status === 'success' ? down.data : null
  const centerSymbol: SymbolRefView | null =
    (centerKey ? findChangedSymbol(overlay, centerKey) : null) ??
    upGraph?.target ??
    downGraph?.target ??
    null

  const layout = useMemo(
    () =>
      centerSymbol
        ? layoutImpactColumns(centerSymbol, upGraph, downGraph, {
            expandedColumns: expanded
          })
        : null,
    [centerSymbol, upGraph, downGraph, expanded]
  )
  const impact = useMemo(
    () => ({
      affectedKeys: collectAffectedKeys([upGraph, downGraph], overlay)
    }),
    [upGraph, downGraph, overlay]
  )
  const hasImpactData = impact.affectedKeys.size > 0
  const legendFlags = overlayFlagsWithData(overlay, {
    hasImpact: hasImpactData
  })
  const view = viewChoice ?? defaultImpactView(upGraph, downGraph)

  const actions: ImpactNodeActions = useMemo(
    () => ({
      onSelect: (key) => onSelectSymbol(key),
      onOpenDiff,
      onSetCenter: (key) => useAppStore.getState().setReviewImpactFocus(worktreeId, key),
      onOpenEditor: (symbol) => {
        if (!openReviewFileInEditor(worktreeId, symbol).ok) {
          toast.error(
            t(
              'auto.components.reviewMap.impact.openPathBlocked',
              'This path is outside the worktree and was not opened.'
            )
          )
        }
      }
    }),
    [onSelectSymbol, onOpenDiff, worktreeId]
  )
  const expandColumn = useCallback(
    (column: number) => setExpanded((prev) => new Set(prev).add(column)),
    []
  )

  const toolbar = (
    <ImpactToolbar
      direction={direction}
      depth={depth}
      view={view}
      legendFlags={legendFlags}
      onDirection={setDirection}
      onDepth={(d) => setDepth(clampImpactDepth(d))}
      onView={setViewChoice}
    />
  )

  if (!target) {
    return (
      <div className="flex h-full flex-col">
        {toolbar}
        <p className="p-6 text-sm text-muted-foreground" data-testid="impact-empty">
          {t(
            'auto.components.reviewMap.impact.pick',
            'Select a changed symbol to see what it affects.'
          )}
        </p>
      </div>
    )
  }

  const failed = active.filter((q) => q.status === 'error' && q.error?.kind !== 'ambiguous')
  const truncated = active.some((q) => q.truncated)
  const risk = describeRisk(upGraph?.risk ?? downGraph?.risk)
  const total = (upGraph?.impactedCount ?? 0) + (downGraph?.impactedCount ?? 0)
  const nothingFound =
    !pending && failed.length === 0 && layout && layout.nodes.length === 1 && active.length > 0

  return (
    <div className="flex h-full min-h-0 flex-col">
      {toolbar}
      <div className="flex flex-col gap-1 px-3 py-2 text-xs text-muted-foreground">
        <p className="flex items-center gap-1" data-testid="impact-no-edges-note">
          <Info className="size-3.5 shrink-0" aria-hidden />
          {t(
            'auto.components.reviewMap.impact.noEdges',
            'The data has no edge information; symbols are shown by level.'
          )}
        </p>
        {risk ? (
          <p data-testid="impact-risk">
            {t(`auto.components.reviewMap.impact.risk.${risk.id}`, risk.fallback)}
            {total > 0 ? ` · ${total}` : ''}
          </p>
        ) : null}
        {truncated ? (
          <p role="status">
            {t('auto.components.reviewMap.impact.truncated', 'Showing only part of the result.')}
          </p>
        ) : null}
      </div>
      {failed.map((q, i) => (
        <div key={i} role="alert" className="flex items-center gap-2 px-3 pb-2 text-xs">
          <span>{errorText(q.error!)}</span>
          <Button type="button" size="sm" variant="outline" onClick={q.refetch}>
            {t('auto.components.reviewMap.symbolDetail.retry', 'Retry')}
          </Button>
        </div>
      ))}
      <div className="min-h-0 flex-1">
        {pending && !layout ? (
          <ReviewLoadingStage stage={stage} />
        ) : nothingFound ? (
          <p className="p-6 text-sm text-muted-foreground" data-testid="impact-none">
            {t(
              'auto.components.reviewMap.impact.none',
              'No dependencies found in the index for this symbol.'
            )}
          </p>
        ) : layout ? (
          view === 'list' ? (
            <ImpactColumnsList
              layout={layout}
              overlay={overlay}
              impact={impact}
              selectedKey={selectedSymbolKey}
              actions={actions}
              onExpandColumn={expandColumn}
            />
          ) : (
            <Suspense fallback={<ReviewLoadingStage stage="busy" rows={3} />}>
              <ImpactGraphCanvas
                layout={layout}
                overlay={overlay}
                impact={impact}
                selectedKey={selectedSymbolKey}
                actions={actions}
                onExpandColumn={expandColumn}
                worktreeId={worktreeId}
              />
            </Suspense>
          )
        ) : null}
      </div>
    </div>
  )
}
