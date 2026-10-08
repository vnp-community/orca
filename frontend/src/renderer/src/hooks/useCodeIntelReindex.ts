/**
 * useCodeIntelReindex.ts — FE-CV-TASK-050-14
 *
 * Starts and follows a reindex job. `start()` locks synchronously (two clicks = one call),
 * REINDEX_IN_PROGRESS attaches to the running job, REINDEX_COOLDOWN yields `cooldownUntil`.
 * Progress comes from push events with a `reindexStatus` poll every 2 s as fallback.
 * No toasts: callers render `error`.
 *
 * @module hooks/useCodeIntelReindex
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { subscribeCodeIntelEvents } from '../lib/code-intel-event-bus'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import type { ReindexJob, ReindexMode } from '../../../shared/code-intel-types'
import { defaultCodeIntelCall } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError } from './useCodeIntelQuery'

export const REINDEX_POLL_MS = 2_000

export type ReindexSnapshot = {
  jobId: string
  status: ReindexJob['status']
  /** null = unknown (kept as null, never coerced to 0) */
  percent: number | null
  stage: string
}

export type UseCodeIntelReindexResult = {
  start: (mode?: ReindexMode) => Promise<void>
  isStarting: boolean
  job: ReindexSnapshot | null
  isRunning: boolean
  cooldownUntil: number | null
  error: CodeIntelQueryError | null
}

function toSnapshot(job: Partial<ReindexJob> & { jobId: string }): ReindexSnapshot {
  return {
    jobId: job.jobId,
    status: job.status ?? 'unknown',
    percent: typeof job.percent === 'number' ? job.percent : null,
    stage: job.stage ?? ''
  }
}

function isActive(job: ReindexSnapshot | null): boolean {
  return job !== null && (job.status === 'queued' || job.status === 'running')
}

export function useCodeIntelReindex(
  worktreeId: string | null,
  environmentId: string | null,
  options: {
    callFn?: CodeIntelCallFn
    /** Refreshes the index status after a job succeeds */
    onSucceeded?: () => void | Promise<void>
    now?: () => number
  } = {}
): UseCodeIntelReindexResult {
  const { callFn = defaultCodeIntelCall, onSucceeded, now = Date.now } = options
  const [isStarting, setIsStarting] = useState(false)
  const [job, setJob] = useState<ReindexSnapshot | null>(null)
  const [cooldownUntil, setCooldownUntil] = useState<number | null>(null)
  const [error, setError] = useState<CodeIntelQueryError | null>(null)
  const startLockRef = useRef(false)
  const callRef = useRef(callFn)
  callRef.current = callFn
  const succeededRef = useRef(onSucceeded)
  succeededRef.current = onSucceeded
  const nowRef = useRef(now)
  nowRef.current = now

  const call = useCallback(
    (method: string, params: Record<string, unknown>) =>
      callRef.current(worktreeId ?? '', method, params, new AbortController().signal, environmentId),
    [worktreeId, environmentId]
  )

  const start = useCallback(
    async (mode: ReindexMode = 'incremental') => {
      if (!worktreeId || startLockRef.current) {return}
      startLockRef.current = true
      setIsStarting(true)
      setError(null)
      try {
        const outcome = await call(CODE_INTEL_RPC_METHODS.REINDEX, { mode })
        if (outcome.ok) {
          const raw = outcome.result as Partial<ReindexJob> | null
          if (raw && typeof raw.jobId === 'string') {setJob(toSnapshot({ ...raw, jobId: raw.jobId }))}
          return
        }
        const err = outcome.error
        if (err.kind === 'reindex-in-progress' && typeof err.data?.jobId === 'string') {
          // Another job is already running: follow it instead of reporting an error.
          setJob(toSnapshot({ jobId: err.data.jobId, status: 'running', stage: String(err.data.stage ?? '') }))
        } else if (err.kind === 'rate-limited' && typeof err.data?.retryAfterSeconds === 'number') {
          setCooldownUntil(nowRef.current() + err.data.retryAfterSeconds * 1000)
        } else {
          setError(err)
        }
      } finally {
        startLockRef.current = false
        setIsStarting(false)
      }
    },
    [worktreeId, call]
  )

  // Push progress: percent null stays null.
  useEffect(() => {
    if (!worktreeId) {return}
    return subscribeCodeIntelEvents((event) => {
      if (event.event !== 'reindexProgress' || event.worktreeId !== worktreeId) {return}
      setJob((prev) => (prev && isActive(prev) ? { ...prev, percent: event.percent } : prev))
    })
  }, [worktreeId])

  // Fallback: poll the job while it is active.
  const activeJobId = isActive(job) ? job!.jobId : null
  useEffect(() => {
    if (!activeJobId) {return}
    let cancelled = false
    const timer = setInterval(() => {
      void call(CODE_INTEL_RPC_METHODS.REINDEX_STATUS, { jobId: activeJobId }).then(async (outcome) => {
        if (cancelled) {return}
        if (!outcome.ok) {
          setError(outcome.error)
          return
        }
        const raw = outcome.result as Partial<ReindexJob> | null
        if (!raw || typeof raw.jobId !== 'string') {return}
        const next = toSnapshot({ ...raw, jobId: raw.jobId })
        setJob(next)
        if (next.status === 'succeeded') {
          useAppStore.getState().triggerCodeIntelResync()
          await succeededRef.current?.()
        }
      })
    }, REINDEX_POLL_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [activeJobId, call])

  return { start, isStarting, job, isRunning: isActive(job), cooldownUntil, error }
}
