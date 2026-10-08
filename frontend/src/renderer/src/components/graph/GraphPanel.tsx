/**
 * GraphPanel — FE-REQ-TASK-032-06
 *
 * Single owner of lens, view, selection, open groups and focus. Backend lenses
 * come from `impact.graph`; flow/plan/execution are built on the client.
 *
 * @module components/graph/GraphPanel
 */

import React, { Suspense, useCallback, useEffect, useMemo, useReducer, useRef } from 'react'
import { translate } from '@/i18n/i18n'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { useGraphLens } from '../../hooks/useGraphLens'
import { usePlanHeatmap } from '../../hooks/usePlanHeatmap'
import { usePlanTree } from '../../hooks/usePlanTree'
import { useRequestNarrowLayout } from '../../hooks/useRequestNarrowLayout'
import { useTaskDependencyEdges } from '../../hooks/useTaskDependencyEdges'
import { REQUEST_RPC_METHODS } from '../../../../shared/request-rpc-methods'
import { GRAPH_LENSES_CLIENT } from '../../../../shared/graph-types'
import { applyChangeView, hasChangeAxis } from './graph-before-after'
import { buildExecutionGraph, buildFlowGraph, buildPlanGraph } from './graph-client-lens-adapters'
import { GRAPH_LENSES } from './graph-lens-registry'
import {
  createGraphPanelState,
  graphPanelReducer,
  initialView,
  lensDisabledReason,
  shouldOpenSearchOnKey,
  type LensDisabledReason
} from './graph-panel-state'
import { useOpenDevServerSettings } from '../request/use-open-dev-server-settings'
import { GraphListView } from './GraphListView'
import { GraphNodeSheet } from './GraphNodeSheet'
import { GraphSearchPalette } from './GraphSearchPalette'
import { GraphEmptyState, GraphErrorState, GraphSkeleton, GraphStatusBanner } from './GraphStates'
import { GraphToolbar } from './GraphToolbar'
import type { OrcaRequest } from '../../../../shared/request-types'
import type {
  GraphLens,
  GraphNode,
  GraphPayload,
  GraphSubjectType
} from '../../../../shared/graph-types'

const GraphCanvas = React.lazy(() => import('./GraphCanvas'))

export type GraphPanelProps = {
  request: OrcaRequest
  subject: { type: GraphSubjectType; id: string; optionId?: string }
  lensInitial?: GraphLens
  /** false = known to have no assessment yet; undefined = unknown (chips stay enabled). */
  impactAssessed?: boolean
  /** Preselects a node, e.g. "View on graph" from an impact finding. */
  initialSelectedId?: string
  onNodeOpen?: (node: GraphNode) => void
  className?: string
}

export function GraphPanel({
  request,
  subject,
  lensInitial,
  impactAssessed,
  initialSelectedId,
  onNodeOpen,
  className
}: GraphPanelProps): React.JSX.Element {
  const narrow = useRequestNarrowLayout()
  const openDevServerSettings = useOpenDevServerSettings()
  const [state, dispatch] = useReducer(graphPanelReducer, undefined, () =>
    createGraphPanelState(
      lensInitial ?? (impactAssessed ? 'impact' : 'flow'),
      narrow ? 'list' : 'graph',
      initialSelectedId ?? null
    )
  )
  const isClientLens = (GRAPH_LENSES_CLIENT as readonly string[]).includes(state.lens)

  const planTree = usePlanTree(request)
  const tasks = useMemo(
    () =>
      planTree.tree
        ? [
            ...planTree.tree.phases,
            ...Object.values(planTree.tree.tasksByPhase).flat(),
            ...planTree.tree.flatTasks
          ]
        : [],
    [planTree.tree]
  )
  const deps = useTaskDependencyEdges(tasks)
  const heat = usePlanHeatmap(request.planTaskId, state.lens === 'plan')

  const clientPayload = useMemo<GraphPayload | null>(() => {
    if (state.lens === 'flow') {
      return buildFlowGraph(request, (k) => translate(k, k.split('.').pop() ?? k))
    }
    if (state.lens === 'plan') {
      return buildPlanGraph(planTree.tree, deps.edges, heat.heatmap)
    }
    if (state.lens === 'execution') {
      return buildExecutionGraph(planTree.tree, deps.edges)
    }
    return null
  }, [state.lens, request, planTree.tree, deps.edges, heat.heatmap])

  const backend = useGraphLens({
    request,
    lens: state.lens,
    subjectType: subject.type,
    subjectId: subject.id,
    enabled: !isClientLens
  })

  const payload = isClientLens ? clientPayload : backend.payload
  const status = isClientLens ? 'ready' : backend.status
  const backendUnsupported = backend.error?.kind === 'unsupported'

  // Why: the list is the default for truncated payloads (also after a lens switch) until the user picks a view.
  const userChoseView = useRef(false)
  useEffect(() => {
    if (userChoseView.current || !payload || status !== 'ready') {
      return
    }
    if (initialView({ truncated: payload.truncated, narrow: false }) === 'list') {
      dispatch({ type: 'view', view: 'list' })
    }
  }, [payload, status])

  useEffect(() => {
    const onKey = (e: KeyboardEvent): void => {
      if (shouldOpenSearchOnKey(e)) {
        e.preventDefault()
        dispatch({ type: 'search', open: true })
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  const disabledReasons = useMemo(() => {
    const out: Partial<Record<GraphLens, LensDisabledReason>> = {}
    for (const l of GRAPH_LENSES) {
      const reason = lensDisabledReason(l.id, {
        impactAssessed,
        backendUnsupported,
        hasPlan: Boolean(request.planTaskId),
        executing:
          request.status === 'executing' ||
          request.status === 'completed' ||
          (planTree.tree?.phases.length ?? 0) > 0
      })
      if (reason) {
        out[l.id] = reason
      }
    }
    return out
  }, [impactAssessed, backendUnsupported, request.planTaskId, request.status, planTree.tree])

  const runAssessment = useCallback(async () => {
    await callRequestRpc(REQUEST_RPC_METHODS.IMPACT_REQUEST, {
      subjectType: subject.type,
      subjectId: subject.id
    })
    backend.refetch()
  }, [subject.type, subject.id, backend])

  const viewPayload = useMemo(
    () => (payload ? applyChangeView(payload, state.changeView) : null),
    [payload, state.changeView]
  )
  const loading = status === 'loading'

  let body: React.ReactNode
  if (!payload || (status === 'idle' && !isClientLens)) {
    body =
      backend.showSkeleton || loading ? (
        <GraphSkeleton />
      ) : (
        <GraphEmptyState
          kind="noAssessment"
          onRunAssessment={
            backendUnsupported
              ? undefined
              : () => {
                  void runAssessment()
                }
          }
        />
      )
  } else if (payload.nodes.length === 0) {
    body = <GraphEmptyState kind="empty" />
  } else if (state.view === 'list') {
    body = (
      <GraphListView
        payload={viewPayload ?? payload}
        selectedId={state.selectedId}
        onSelect={(id) => dispatch({ type: 'select', id })}
        onOpen={(id) => dispatch({ type: 'sheet', id })}
      />
    )
  } else {
    body = (
      <Suspense fallback={<GraphSkeleton />}>
        <GraphCanvas
          payload={payload}
          selectedId={state.selectedId}
          onSelect={(id) => dispatch({ type: 'select', id })}
          onOpenNode={(id) => dispatch({ type: 'sheet', id })}
          openGroups={state.openGroups}
          onToggleGroup={(id) => dispatch({ type: 'toggleGroup', id })}
          changeView={state.changeView}
          focusId={state.focusId}
          onFocus={(id) => dispatch({ type: 'focus', id })}
          fitViewSignal={state.fitViewSignal}
        />
      </Suspense>
    )
  }

  return (
    <div className={className ?? 'flex h-full min-h-[360px] flex-col'} data-testid="graph-panel">
      <GraphToolbar
        lens={state.lens}
        view={state.view}
        changeView={state.changeView}
        showChangeToggle={payload !== null && hasChangeAxis(payload)}
        disabledReasons={disabledReasons}
        disabled={loading && !payload}
        onLens={(lens) => dispatch({ type: 'lens', lens })}
        onView={(view) => {
          userChoseView.current = true
          dispatch({ type: 'view', view })
        }}
        onChangeView={(changeView) => dispatch({ type: 'changeView', changeView })}
        onSearch={() => dispatch({ type: 'search', open: true })}
      />
      {payload ? <GraphStatusBanner payload={payload} /> : null}
      {backend.error && backend.status === 'error' ? (
        <GraphErrorState
          error={backend.error}
          onRetry={backend.refetch}
          hasStalePayload={payload !== null}
          onConnectDevServer={openDevServerSettings}
        />
      ) : null}
      <div className={loading && payload ? 'min-h-0 flex-1 opacity-60' : 'min-h-0 flex-1'}>
        {body}
      </div>
      {payload ? (
        <>
          <GraphSearchPalette
            open={state.searchOpen}
            nodes={payload.nodes}
            onOpenChange={(open) => dispatch({ type: 'search', open })}
            onPick={(node) => dispatch({ type: 'searchPick', node })}
          />
          <GraphNodeSheet
            payload={payload}
            nodeId={state.sheetNodeId}
            onClose={() => dispatch({ type: 'sheet', id: null })}
            onOpenNode={onNodeOpen}
          />
        </>
      ) : null}
    </div>
  )
}
