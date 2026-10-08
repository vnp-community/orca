/**
 * use-requirement-trace.ts — FE-CV-TASK-092-02
 *
 * Loads `quality.trace` for the requirements lens and exposes the write actions.
 * Quality flag off: no RPC, no event subscription. Trace text is task content, so it
 * lives in component memory only (never localStorage / IndexedDB).
 *
 * @module components/review-map/requirements/use-requirement-trace
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { getCodeIntelClient } from '../../../runtime/code-intel-client'
import { subscribeCodeIntelEvents } from '../../../lib/code-intel-event-bus'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { createRequirementEvidenceActions } from './requirement-evidence-actions'
import type { EvidenceRef, TraceActionResult } from './requirement-evidence-actions'
import { buildRequirementTraceViewModel, parseRequirementTrace } from './requirement-trace-view-model'
import type { RequirementTrace, RequirementTraceViewModel } from './requirement-trace-view-model'

const MAX_RETRY_MS = 90_000
const DEFAULT_RETRY_MS = 3000
const SILENT_KINDS = new Set(['disabled', 'unsupported', 'forbidden', 'quality-disabled', 'no-binding'])

export type RequirementTraceStatus = 'idle' | 'loading' | 'ready' | 'error' | 'disabled'

export type UseRequirementTraceResult = {
  status: RequirementTraceStatus
  view: RequirementTraceViewModel | null
  /** Index changed since the last load; shown as a hint, never auto-reloaded mid-interaction. */
  stale: boolean
  /** A write was refused (forbidden): the lens becomes read-only. */
  readOnly: boolean
  actionError: string | null
  showInferred: boolean
  setShowInferred: (value: boolean) => void
  confirm: (requirementKey: string, evidence: EvidenceRef) => Promise<void>
  reject: (requirementKey: string, evidence: EvidenceRef) => Promise<void>
  linkTask: (taskId: string) => Promise<void>
  unlinkTask: () => Promise<void>
  refetch: () => void
}

export function useRequirementTrace(args: {
  projectId: string | null | undefined
  worktreeId: string | null
  base?: string | null
}): UseRequirementTraceResult {
  const { projectId, worktreeId, base } = args
  const { quality } = useQualityFeatureFlags()
  const enabled = quality && Boolean(projectId) && Boolean(worktreeId)

  const [status, setStatus] = useState<RequirementTraceStatus>('idle')
  const [trace, setTrace] = useState<RequirementTrace | null>(null)
  const [stale, setStale] = useState(false)
  const [readOnly, setReadOnly] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [showInferred, setShowInferred] = useState(false)
  const [reloadTick, setReloadTick] = useState(0)

  useEffect(() => {
    if (!enabled || !worktreeId) {
      setStatus('idle')
      return
    }
    const ctrl = new AbortController()
    let retryTimer: ReturnType<typeof setTimeout> | null = null
    const startedAt = Date.now()
    setStatus('loading')

    const load = async (): Promise<void> => {
      try {
        const response = await getCodeIntelClient().call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_TRACE,
          { projectId, worktreeId, includeInferred: showInferred, ...(base ? { base } : {}) },
          {
            environmentId: getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId),
            signal: ctrl.signal
          }
        )
        if (ctrl.signal.aborted) {return}
        if (response.ok) {
          setTrace(parseRequirementTrace((response.result as { trace?: unknown } | null)?.trace))
          setStale(false)
          setStatus('ready')
        } else if (SILENT_KINDS.has(response.error.kind)) {
          setStatus('disabled')
        } else if (response.error.message?.includes('inProgress') && Date.now() - startedAt < MAX_RETRY_MS) {
          const wait = response.error.data?.retryAfterMs
          retryTimer = setTimeout(() => void load(), typeof wait === 'number' && wait > 0 ? wait : DEFAULT_RETRY_MS)
        } else {
          setStatus('error')
        }
      } catch {
        if (!ctrl.signal.aborted) {setStatus('error')}
      }
    }
    void load()

    const unsubscribe = subscribeCodeIntelEvents((event) => {
      if (event.worktreeId !== worktreeId) {return}
      if (event.event === 'qualityFinished' || event.event === 'gateChanged') {
        setReloadTick((n) => n + 1)
      } else if (event.event === 'changed') {
        setStale(true)
      }
    })
    return () => {
      ctrl.abort()
      unsubscribe()
      if (retryTimer) {clearTimeout(retryTimer)}
    }
  }, [enabled, worktreeId, projectId, base, showInferred, reloadTick])

  const actions = useMemo(() => {
    if (!enabled || !projectId || !worktreeId) {return null}
    return createRequirementEvidenceActions(
      { projectId, worktreeId },
      {
        call: async (method, params) =>
          getCodeIntelClient().call(worktreeId, method, params, {
            environmentId: getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId)
          })
      }
    )
  }, [enabled, projectId, worktreeId])

  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  const apply = useCallback(async (run: () => Promise<TraceActionResult> | undefined): Promise<void> => {
    const result = await run()
    if (!result || !mounted.current) {return}
    if (result.ok) {
      setTrace(result.trace)
      setActionError(null)
    } else if (!result.busy) {
      if (result.kind === 'forbidden') {setReadOnly(true)}
      setActionError(result.kind)
    }
  }, [])

  const view = useMemo(
    () => (status === 'ready' ? buildRequirementTraceViewModel(trace, { showInferred }) : null),
    [status, trace, showInferred]
  )

  return {
    status: enabled ? status : 'disabled',
    view,
    stale,
    readOnly,
    actionError,
    showInferred,
    setShowInferred,
    confirm: (key, evidence) => apply(() => actions?.confirm(key, evidence)),
    reject: (key, evidence) => apply(() => actions?.reject(key, evidence)),
    linkTask: (taskId) => apply(() => actions?.linkTask(taskId)),
    unlinkTask: () => apply(() => actions?.unlinkTask()),
    refetch: () => setReloadTick((n) => n + 1)
  }
}
