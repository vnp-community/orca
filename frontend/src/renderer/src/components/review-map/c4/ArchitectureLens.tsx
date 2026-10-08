/**
 * ArchitectureLens.tsx — FE-CV-TASK-055-03
 *
 * C4 level-3 component view of one container, built by the server from folder structure and
 * hexagonal rules (plus c4.yaml overrides). Always presented as inferred.
 */

import { useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import type { C4Relation, ContainerRef } from '../../../../../shared/code-intel-architecture-types'
import type { ReviewLensProps } from '../review-lens-registry'
import { useC4Architecture } from '../../../hooks/useC4Architecture'
import { ArchitectureToolbar, type C4ViewMode } from './ArchitectureToolbar'
import { C4InferredNotice } from './C4InferredNotice'
import { C4DiagramCanvas } from './C4DiagramCanvas'
import { C4RelationsTable, relationKey } from './C4RelationsTable'
import { C4ComponentDetail, type C4Selection } from './C4ComponentDetail'
import { C4EdgeLegend } from './C4EdgeLegend'
import { C4OverrideEditor } from './C4OverrideEditor'
import { layoutC4Layers } from './c4-layer-layout'
import { pickDefaultContainer } from './c4-container-default'
import { computeC4OverlayFlags, computeC4RelationFlags, type C4OverlayFlags } from './c4-overlay-model'

export const C4_MAX_GRAPH_NODES = 150
export const C4_MAX_GRAPH_RELATIONS = 500

export default function ArchitectureLens(props: ReviewLensProps): React.JSX.Element {
  const { worktreeId, environmentId, overlay, onOpenDiff } = props
  const storedContainer = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.c4ContainerId ?? null)
  const draft = useAppStore((s) => {
    const id = s.reviewUiByWorktree[worktreeId]?.c4ContainerId
    return id ? (s.reviewUiByWorktree[worktreeId]?.c4Drafts?.[id] ?? null) : null
  })
  const setContainer = useAppStore((s) => s.setReviewC4Container)
  const setDraft = useAppStore((s) => s.setReviewC4Draft)

  const [includeHidden, setIncludeHidden] = useState(false)
  const [mode, setMode] = useState<C4ViewMode | null>(null)
  const [selection, setSelection] = useState<{ id?: string; edge?: string } | null>(null)
  const [editing, setEditing] = useState(false)
  const [lastContainers, setLastContainers] = useState<ContainerRef[]>([])

  const load = useC4Architecture(worktreeId, environmentId, { container: storedContainer, includeHidden })
  const containers = load.data?.containers ?? lastContainers
  const view = load.data?.view ?? null

  useEffect(() => {
    if (load.data && load.data.containers.length > 0) {
      setLastContainers(load.data.containers)
    }
  }, [load.data])

  // Default selection: persisted per tab so switching lenses keeps the choice.
  useEffect(() => {
    if (storedContainer === null && containers.length > 0) {
      const pick = pickDefaultContainer(containers, overlay.changedFiles.map((f) => f.path))
      if (pick) {
        setContainer(worktreeId, pick.id)
      }
    }
  }, [storedContainer, containers, overlay.changedFiles, setContainer, worktreeId])

  const model = useMemo(() => {
    if (!view) {
      return null
    }
    const layout = layoutC4Layers(view)
    const names = new Map<string, string>()
    for (const c of view.components) {names.set(c.id, c.name)}
    for (const e of view.externals) {names.set(e.id, e.name)}
    const flags = new Map<string, C4OverlayFlags>()
    for (const c of view.components) {
      flags.set(c.id, computeC4OverlayFlags(c, overlay, null, view.container.path))
    }
    const changedEdges = new Set<string>()
    for (const r of layout.relations) {
      if (computeC4RelationFlags(r, overlay).touchesChange) {changedEdges.add(relationKey(r))}
    }
    const nodeCount = view.components.length + view.externals.length
    return { layout, names, flags, changedEdges, nodeCount, tooBig: nodeCount > C4_MAX_GRAPH_NODES || view.relations.length > C4_MAX_GRAPH_RELATIONS }
  }, [view, overlay])

  const effectiveMode: C4ViewMode = mode ?? (model?.tooBig ? 'list' : 'graph')

  const selected: C4Selection | null = useMemo(() => {
    if (!view || !selection) {return null}
    if (selection.edge) {
      const r = view.relations.find((x) => relationKey(x) === selection.edge)
      return r ? { kind: 'relation', relation: r } : null
    }
    const c = view.components.find((x) => x.id === selection.id)
    if (c) {
      return { kind: 'component', component: c, flags: model?.flags.get(c.id) ?? { changed: false, untested: false, violation: false, affected: false } }
    }
    const e = view.externals.find((x) => x.id === selection.id)
    return e ? { kind: 'external', external: e } : null
  }, [view, selection, model])

  const selectRelation = (r: C4Relation): void => setSelection({ edge: relationKey(r) })
  const chosenContainer = storedContainer

  let body: React.JSX.Element
  if (load.status === 'error') {
    body = (
      <div role="alert" className="flex flex-col items-start gap-2 p-4 text-sm">
        <p>{translate('auto.components.reviewMap.c4.error', 'Could not load the architecture view.')}</p>
        <p className="text-xs text-muted-foreground">{load.error?.message}</p>
        <Button size="sm" variant="outline" onClick={load.refetch}>
          {translate('auto.components.reviewMap.c4.retry', 'Retry')}
        </Button>
      </div>
    )
  } else if (load.status === 'loading' || load.status === 'idle') {
    body = (
      <div role="status" aria-label={translate('auto.components.reviewMap.c4.loading', 'Loading architecture')} className="space-y-2 p-4">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  } else if (containers.length === 0) {
    body = (
      <p className="p-4 text-sm text-muted-foreground">
        {translate('auto.components.reviewMap.c4.noContainers', 'No containers were inferred; check the folder conventions.')}
      </p>
    )
  } else if (!view || !model) {
    body = (
      <p className="p-4 text-sm text-muted-foreground">
        {translate('auto.components.reviewMap.c4.noView', 'No component view is available for this container yet.')}
      </p>
    )
  } else {
    body = (
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          {view.warnings.length > 0 ? (
            <ul className="border-b px-3 py-1 text-xs text-muted-foreground">
              {view.warnings.map((w, i) => (
                <li key={`${w.code}-${i}`}>{w.message}</li>
              ))}
            </ul>
          ) : null}
          {effectiveMode === 'graph' ? (
            <>
              <C4DiagramCanvas
                layout={model.layout}
                flagsById={model.flags}
                changedEdgeKeys={model.changedEdges}
                selectedId={selection?.id ?? null}
                selectedEdgeKey={selection?.edge ?? null}
                onSelectNode={(id) => setSelection({ id })}
                onSelectEdge={(edge) => setSelection({ edge })}
              />
              <div className="border-t px-3 py-1.5">
                <C4EdgeLegend />
              </div>
            </>
          ) : (
            <C4RelationsTable
              relations={model.layout.relations}
              names={model.names}
              total={view.relations.length}
              changedKeys={model.changedEdges}
              selectedKey={selection?.edge ?? null}
              onSelect={selectRelation}
            />
          )}
        </div>
        {selected ? (
          <C4ComponentDetail
            selection={selected}
            names={model.names}
            relations={view.relations}
            onSelectRelation={selectRelation}
            onOpenDiff={onOpenDiff}
            onAddDescription={() => setEditing(true)}
            onClose={() => setSelection(null)}
          />
        ) : null}
      </div>
    )
  }

  return (
    <section aria-label={translate('auto.components.reviewMap.lens.architecture.label', 'Architecture')} className="flex min-h-0 flex-1 flex-col">
      <ArchitectureToolbar
        containers={containers}
        container={chosenContainer}
        onContainerChange={(id) => {
          setSelection(null)
          setContainer(worktreeId, id)
        }}
        mode={effectiveMode}
        onModeChange={setMode}
        includeHidden={includeHidden}
        onIncludeHiddenChange={setIncludeHidden}
        onEdit={() => setEditing(true)}
      />
      <C4InferredNotice hasOverrides={view?.hasOverrides ?? false} onEdit={chosenContainer ? () => setEditing(true) : undefined} />
      {body}
      {editing && chosenContainer ? (
        <C4OverrideEditor
          open
          worktreeId={worktreeId}
          environmentId={environmentId}
          container={chosenContainer}
          draft={draft}
          onDraftChange={(text) => setDraft(worktreeId, chosenContainer, text)}
          onSaved={load.refetch}
          onClose={() => setEditing(false)}
        />
      ) : null}
    </section>
  )
}
