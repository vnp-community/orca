/**
 * useC4Override.ts — FE-CV-TASK-055-06
 *
 * Load/save of one container's c4.yaml override. "Saved" is only reported after c4.save
 * resolves ok; the save lock is a ref so a double click cannot send two requests.
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { getCodeIntelClient } from '../runtime/code-intel-client'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { invalidateCodeIntelViewCache } from './useCodeIntelViewLoad'
import { C4_ARCHITECTURE_METHOD } from './useC4Architecture'

export type C4OverrideRecord = {
  document: string
  /** 0 = no record yet (create). */
  version: number
  updatedBy: string | null
  updatedAt: string | null
  seedSource: string | null
}

export type C4SaveFailure =
  | { kind: 'conflict'; currentVersion: number | null }
  | { kind: 'invalid'; field: string | null; reason: string | null }
  | { kind: 'too-large'; limit: number | null }
  | { kind: 'forbidden' }
  | { kind: 'offline' }
  | { kind: 'other'; message: string }

export type C4SaveOutcome =
  | { ok: true; version: number; warnings: { code: string; message: string }[] }
  | { ok: false; failure: C4SaveFailure }

export type UseC4OverrideResult = {
  status: 'loading' | 'ready' | 'error'
  record: C4OverrideRecord | null
  loadError: string | null
  readOnly: boolean
  saving: boolean
  save: (document: string) => Promise<C4SaveOutcome>
  /** Re-reads the server record (used for "load latest" and before an overwrite). */
  reload: () => Promise<C4OverrideRecord | null>
  /** Fetches the latest version, then saves over it. Callers must have asked for confirmation. */
  overwrite: (document: string) => Promise<C4SaveOutcome>
}

function toNumber(v: unknown, fallback: number): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : fallback
}

export function parseC4OverrideRecord(raw: unknown): C4OverrideRecord {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  return {
    document: typeof r.document === 'string' ? r.document : '',
    version: toNumber(r.version, 0),
    updatedBy: typeof r.updatedBy === 'string' && r.updatedBy ? r.updatedBy : null,
    updatedAt: typeof r.updatedAt === 'string' && r.updatedAt ? r.updatedAt : null,
    seedSource: typeof r.seedSource === 'string' && r.seedSource ? r.seedSource : null
  }
}

export function classifyC4SaveError(error: CodeIntelRpcError): C4SaveFailure {
  const data = error.data ?? {}
  switch (error.kind) {
    case 'conflict':
      return {
        kind: 'conflict',
        currentVersion: typeof data.currentVersion === 'number' ? data.currentVersion : null
      }
    case 'forbidden':
      return { kind: 'forbidden' }
    case 'offline':
    case 'timeout':
      return { kind: 'offline' }
    case 'too-large':
      return { kind: 'too-large', limit: typeof data.limit === 'number' ? data.limit : null }
    case 'validation':
      if (error.code === 'CODEINTEL_PAYLOAD_TOO_LARGE') {
        return { kind: 'too-large', limit: typeof data.limit === 'number' ? data.limit : null }
      }
      return {
        kind: 'invalid',
        field: typeof data.field === 'string' ? data.field : null,
        reason: typeof data.reason === 'string' ? data.reason : error.message || null
      }
    default:
      return { kind: 'other', message: error.message }
  }
}

export function useC4Override(args: {
  worktreeId: string
  environmentId: string | null
  container: string
}): UseC4OverrideResult {
  const { worktreeId, environmentId, container } = args
  const [status, setStatus] = useState<UseC4OverrideResult['status']>('loading')
  const [record, setRecord] = useState<C4OverrideRecord | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [readOnly, setReadOnly] = useState(false)
  const [saving, setSaving] = useState(false)
  const lock = useRef(false)
  const versionRef = useRef(0)

  const fetchRecord = useCallback(
    async (signal?: AbortSignal): Promise<C4OverrideRecord | { error: string }> => {
      const res = await getCodeIntelClient().call(
        worktreeId,
        CODE_INTEL_RPC_METHODS.C4_GET,
        { container },
        { environmentId, signal }
      )
      if (!res.ok) {
        // No record yet is a normal first-edit state, not an error.
        if (res.error.kind === 'not-found') {
          return { document: '', version: 0, updatedBy: null, updatedAt: null, seedSource: null }
        }
        if (res.error.kind === 'forbidden') {
          setReadOnly(true)
        }
        return { error: res.error.message || res.error.kind }
      }
      return parseC4OverrideRecord(res.result)
    },
    [worktreeId, environmentId, container]
  )

  useEffect(() => {
    const ctrl = new AbortController()
    setStatus('loading')
    setReadOnly(false)
    void (async () => {
      try {
        const out = await fetchRecord(ctrl.signal)
        if (ctrl.signal.aborted) {
          return
        }
        if ('error' in out) {
          setLoadError(out.error)
          setStatus('error')
        } else {
          versionRef.current = out.version
          setRecord(out)
          setLoadError(null)
          setStatus('ready')
        }
      } catch (err) {
        if (!ctrl.signal.aborted) {
          setLoadError(err instanceof Error ? err.message : 'failed')
          setStatus('error')
        }
      }
    })()
    return () => ctrl.abort()
  }, [fetchRecord])

  const doSave = useCallback(
    async (document: string, expectedVersion: number): Promise<C4SaveOutcome> => {
      if (lock.current) {
        return { ok: false, failure: { kind: 'other', message: 'save in progress' } }
      }
      lock.current = true
      setSaving(true)
      try {
        const res = await getCodeIntelClient().call<{
          version?: unknown
          warnings?: unknown
        }>(worktreeId, CODE_INTEL_RPC_METHODS.C4_SAVE, { container, document, expectedVersion }, { environmentId })
        if (!res.ok) {
          const failure = classifyC4SaveError(res.error)
          if (failure.kind === 'forbidden') {
            setReadOnly(true)
          }
          return { ok: false, failure }
        }
        const version = toNumber(res.result?.version, expectedVersion + 1)
        const warnings = Array.isArray(res.result?.warnings)
          ? (res.result.warnings as { code?: unknown; message?: unknown }[]).map((w) => ({
              code: String(w?.code ?? ''),
              message: String(w?.message ?? '')
            }))
          : []
        versionRef.current = version
        setRecord((prev) => ({
          document,
          version,
          updatedBy: prev?.updatedBy ?? null,
          updatedAt: new Date().toISOString(),
          seedSource: prev?.seedSource ?? null
        }))
        // Why: the diagram is built server-side from the override, so cached views are stale.
        invalidateCodeIntelViewCache(worktreeId, C4_ARCHITECTURE_METHOD)
        return { ok: true, version, warnings }
      } catch (err) {
        return { ok: false, failure: { kind: 'other', message: err instanceof Error ? err.message : 'failed' } }
      } finally {
        lock.current = false
        setSaving(false)
      }
    },
    [worktreeId, environmentId, container]
  )

  const save = useCallback((document: string) => doSave(document, versionRef.current), [doSave])

  const reload = useCallback(async () => {
    const out = await fetchRecord()
    if ('error' in out) {
      return null
    }
    versionRef.current = out.version
    setRecord(out)
    return out
  }, [fetchRecord])

  const overwrite = useCallback(
    async (document: string): Promise<C4SaveOutcome> => {
      const latest = await fetchRecord()
      if ('error' in latest) {
        return { ok: false, failure: { kind: 'other', message: latest.error } }
      }
      return doSave(document, latest.version)
    },
    [fetchRecord, doSave]
  )

  return { status, record, loadError, readOnly, saving, save, reload, overwrite }
}
