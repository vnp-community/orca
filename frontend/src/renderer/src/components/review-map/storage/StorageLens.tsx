/**
 * StorageLens.tsx — FE-CV-TASK-058-04
 *
 * `storage` review lens: service -> data store -> topic -> secret, read-only. Never says a
 * change is safe: an unchanged map is "nothing detected within the indexed scope".
 */

import { useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { usePerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import { useCodeIntelStorage } from '../../../hooks/useCodeIntelStorage'
import type { ReviewLensProps } from '../review-lens-registry'
import { ReviewLoadingStage } from '../shell/ReviewLoadingStage'
import { setReviewLensUnavailable } from '../shell/review-lens-availability'
import { StorageCanvas } from './StorageCanvas'
import { StorageInferenceNotice } from './StorageInferenceNotice'
import { StorageLegend } from './StorageLegend'
import { StorageNodeDetail } from './StorageNodeDetail'
import { StorageTextView } from './StorageTextView'
import { StorageToolbar } from './StorageToolbar'
import { computeStorageChangeMarks, normalizeStoragePath } from './storage-change-marks'
import { layoutStorageLanes } from './storage-layout'
import { buildStorageViewModel } from './storage-view-model'

const t = translate
const EMPTY: ReadonlySet<string> = new Set()

function errorText(kind: string | undefined): string {
  if (kind === 'offline') {
    return t(
      'auto.components.reviewMap.StorageLens.error.offline',
      'The connection to the workspace is down.'
    )
  }
  if (kind === 'forbidden') {
    return t(
      'auto.components.reviewMap.StorageLens.error.forbidden',
      'You do not have access to this repository.'
    )
  }
  return t(
    'auto.components.reviewMap.StorageLens.error.generic',
    'The storage map could not be loaded.'
  )
}

export default function StorageLens(props: ReviewLensProps): React.JSX.Element {
  const { worktreeId, environmentId, overlay, onOpenDiff } = props
  const env = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.storageEnv ?? 'dev')
  const selectedId = useAppStore(
    (s) => s.reviewUiByWorktree[worktreeId]?.selectedStorageNodeId ?? null
  )
  const setStorageEnv = useAppStore((s) => s.setStorageEnv)
  const selectStorageNode = useAppStore((s) => s.selectStorageNode)
  const setErdService = useAppStore((s) => s.setErdService)
  const setReviewLens = useAppStore((s) => s.setReviewLens)

  const [includeLegacy, setIncludeLegacy] = useState(false)
  const [onlyChangedChoice, setOnlyChangedChoice] = useState<boolean | null>(null)
  const [kinds, setKinds] = useState<ReadonlySet<string>>(EMPTY)

  const storage = useCodeIntelStorage({ worktreeId, environmentId, env, includeLegacy })
  // Why: a backend that cannot serve `storage` hides the tab instead of leaving a dead end.
  useEffect(() => {
    setReviewLensUnavailable(worktreeId, 'storage', !storage.available)
  }, [worktreeId, storage.available])
  const pending = storage.status === 'idle' || storage.status === 'loading'
  const stage = usePerceivedLoadingStage(pending, { remote: environmentId !== null })

  const vm = useMemo(
    () => (storage.data ? buildStorageViewModel(storage.data) : null),
    [storage.data]
  )
  const changed = useMemo(
    () => (vm ? computeStorageChangeMarks(vm, overlay.changedFiles) : null),
    [vm, overlay.changedFiles]
  )
  const hasDirect = changed ? [...changed.marks.values()].some((m) => m !== 'related') : false
  const onlyChanged = hasDirect && (onlyChangedChoice ?? true)

  const visibleIds = useMemo(() => {
    if (!vm) {
      return undefined
    }
    let ids: Set<string> | undefined
    if (onlyChanged && changed) {
      ids = new Set(changed.marks.keys())
    }
    if (kinds.size > 0) {
      const base = ids ?? new Set(vm.nodes.map((n) => n.id))
      ids = new Set(
        vm.nodes
          .filter((n) => base.has(n.id) && (n.lane !== 'store' || kinds.has(n.kind ?? '')))
          .map((n) => n.id)
      )
    }
    return ids
  }, [vm, onlyChanged, changed, kinds])

  const layout = useMemo(
    () =>
      vm ? layoutStorageLanes({ nodes: vm.nodes, edges: vm.edges, filter: visibleIds }) : null,
    [vm, visibleIds]
  )
  const changedPaths = useMemo(
    () => new Set(overlay.changedFiles.map((f) => normalizeStoragePath(f.path))),
    [overlay.changedFiles]
  )
  const kindList = useMemo(
    () =>
      [
        ...new Set(
          (vm?.nodes ?? []).filter((n) => n.lane === 'store').map((n) => n.kind ?? 'other')
        )
      ].sort(),
    [vm]
  )
  const selected = vm?.nodes.find((n) => n.id === selectedId) ?? null

  const toolbar = (
    <StorageToolbar
      env={env}
      includeLegacy={includeLegacy}
      onlyChanged={onlyChanged}
      canFilterChanged={hasDirect}
      kinds={kindList}
      selectedKinds={kinds}
      asOfCommit={storage.data?.asOfCommit ?? null}
      disabled={pending}
      onEnv={(e) => setStorageEnv(worktreeId, e)}
      onLegacy={setIncludeLegacy}
      onOnlyChanged={setOnlyChangedChoice}
      onToggleKind={(k) =>
        setKinds((prev) => {
          const next = new Set(prev)
          if (!next.delete(k)) {
            next.add(k)
          }
          return next
        })
      }
    />
  )

  let body: React.JSX.Element
  if (!storage.available) {
    body = (
      <p role="status" className="p-4 text-sm text-muted-foreground">
        {t(
          'auto.components.reviewMap.StorageLens.unavailable',
          'The storage map is not available for this backend.'
        )}
      </p>
    )
  } else if (storage.status === 'error') {
    body = (
      <div role="alert" className="flex flex-col items-start gap-2 p-4 text-sm">
        <p>{errorText(storage.error?.kind)}</p>
        <Button type="button" variant="outline" size="sm" onClick={storage.reload}>
          {t('auto.components.reviewMap.StorageLens.retry', 'Retry')}
        </Button>
      </div>
    )
  } else if (!vm || !layout || !changed) {
    body = (
      <ReviewLoadingStage
        stage={stage}
        stageLabel={t(
          'auto.components.reviewMap.StorageLens.building',
          'Reading storage configuration...'
        )}
      />
    )
  } else if (vm.nodes.length === 0) {
    body = (
      <div className="space-y-2 p-4 text-sm text-muted-foreground">
        <p>
          {t(
            'auto.components.reviewMap.StorageLens.empty',
            'No storage configuration was found in this repository.'
          )}
        </p>
        {vm.warnings.map((w, i) => (
          <p key={i}>{w.text}</p>
        ))}
      </div>
    )
  } else {
    const inferred = vm.nodes.filter(
      (n) => n.confidence === 'inferred' || n.confidence === 'derived'
    ).length
    body = (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex flex-col gap-1 px-3 py-1.5">
          <StorageLegend />
          <StorageInferenceNotice
            inferredCount={inferred}
            redactedCount={vm.redactedCount}
            warnings={vm.warnings}
          />
          {storage.isStale ? (
            <Button
              type="button"
              variant="outline"
              size="xs"
              className="w-fit"
              onClick={storage.reload}
            >
              {t('auto.components.reviewMap.StorageLens.newData', 'New data available - refresh')}
            </Button>
          ) : null}
          {storage.truncated ? (
            <p role="status" className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.StorageLens.truncated',
                'The backend limited this result.'
              )}
            </p>
          ) : null}
          {changed.unknown ? (
            <p role="status" className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.StorageLens.unknownChange',
                'Cannot tell which components changed: the backend did not supply source evidence.'
              )}
            </p>
          ) : !hasDirect && overlay.changedFiles.length > 0 ? (
            <p role="status" className="text-xs text-muted-foreground">
              {t(
                'auto.components.reviewMap.StorageLens.noStorageChange',
                'No change to storage configuration was detected within the indexed scope.'
              )}
            </p>
          ) : null}
        </div>
        <div className="flex min-h-0 flex-1">
          <div className="flex min-w-0 flex-1 flex-col">
            <StorageCanvas
              nodes={vm.nodes}
              edges={vm.edges}
              positions={layout.positions}
              marks={changed.marks}
              selectedId={selectedId}
              onSelect={(id) => selectStorageNode(worktreeId, id)}
            />
            <StorageTextView
              nodes={vm.nodes.filter((n) => layout.positions.has(n.id))}
              edges={vm.edges}
              marks={changed.marks}
              onSelect={(id) => selectStorageNode(worktreeId, id)}
            />
          </div>
          {selected ? (
            <StorageNodeDetail
              node={selected}
              nodes={vm.nodes}
              edges={vm.edges}
              mark={changed.marks.get(selected.id)}
              changedPaths={changedPaths}
              onOpenDiff={onOpenDiff}
              onSelectNode={(id) => selectStorageNode(worktreeId, id)}
              onOpenErd={(service) => {
                setErdService(worktreeId, service)
                setReviewLens(worktreeId, 'erd')
              }}
              onClose={() => selectStorageNode(worktreeId, null)}
            />
          ) : null}
        </div>
      </div>
    )
  }

  return (
    <section
      className="flex min-h-0 flex-1 flex-col"
      aria-label={t('auto.components.reviewMap.StorageLens.label', 'Storage map')}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && selectedId && !(e.target instanceof HTMLInputElement)) {
          selectStorageNode(worktreeId, null)
        }
      }}
    >
      {toolbar}
      {body}
    </section>
  )
}
