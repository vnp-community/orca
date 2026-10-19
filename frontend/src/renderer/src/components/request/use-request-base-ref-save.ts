/**
 * use-request-base-ref-save.ts — FE-REQ-TASK-040-04
 *
 * Saves a Request's starting branch with `request.setBaseRef` and turns every outcome into a state the Overview row can show:
 * stored (with the "could not check" notice), refused (under the row), locked (neutral toast, then the row drops its edit
 * button) or stale (reload, then ask). The typed branch is never lost: a refusal leaves everything as it was. The state is
 * per Request: callers key the row by Request id.
 *
 * @module components/request/use-request-base-ref-save
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../shared/request-rpc-methods'
import { parseSetBaseRefResult, verifyBaseRefEcho } from '../../../../shared/request-base-ref'
import { parseRequest } from '../../../../shared/request-wire-parsers'
import { classifyBaseRefWriteError, type BaseRefWriteFailure } from './base-ref-write-error'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestBaseRefRow.'

export type BaseRefSaveState = {
  saving: boolean
  /** Shown under the row; locked and stale answers are handled by toast and `conflict` instead. */
  failure: BaseRefWriteFailure | null
  /** Someone changed the Request first: `mine` is what this user asked for, shown next to what is stored now. */
  conflict: { mine: string } | null
  /** The viewer is not allowed to change the branch: the edit button goes away. */
  forbidden: boolean
  /** The ref the host could not confirm after the last save. */
  unverifiedRef: string | null
}

const IDLE: BaseRefSaveState = {
  saving: false,
  failure: null,
  conflict: null,
  forbidden: false,
  unverifiedRef: null
}

/** The more recent of what the pane passed and what the store knows (a save lands in the store before the pane refetches). */
export function useFreshestRequest(request: OrcaRequest): OrcaRequest {
  const stored = useAppStore((s) => s.requestsById[request.id])
  return stored && (stored.version ?? 0) > (request.version ?? 0) ? stored : request
}

async function reloadRequest(id: string): Promise<OrcaRequest | null> {
  const result = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.GET, { id })
  if (!result.ok) {
    return null
  }
  const raw = (result.value as { request?: unknown } | null)?.request ?? result.value
  const fresh = parseRequest(raw)
  useAppStore.getState().upsertRequests([fresh])
  return fresh
}

export function useRequestBaseRefSave(request: OrcaRequest): {
  state: BaseRefSaveState
  save: (baseRef: string) => Promise<void>
  applyConflict: () => Promise<void>
  dismiss: () => void
} {
  const [state, setState] = useState<BaseRefSaveState>(IDLE)
  const latest = useRef(request)
  useEffect(() => {
    latest.current = request
  }, [request])

  const save = useCallback(async (baseRef: string): Promise<void> => {
    const current = latest.current
    const version = current.version
    setState((s) => ({ ...s, saving: true, failure: null, conflict: null }))
    const result = await callRequestRpc<unknown>(
      REQUEST_RPC_METHODS.SET_BASE_REF,
      { id: current.id, baseRef, ...(version !== undefined ? { expectedVersion: version } : {}) },
      // Why: without expectedVersion a repeated call is not guarded against a concurrent edit.
      { idempotent: version !== undefined }
    )
    if (!result.ok) {
      const failure = classifyBaseRefWriteError(result.error)
      if (failure.kind === 'locked') {
        toast(failure.message)
        await reloadRequest(current.id)
        setState((s) => ({ ...s, saving: false }))
      } else if (failure.kind === 'conflict') {
        const fresh = await reloadRequest(current.id)
        const changed = fresh !== null && fresh.baseRef !== baseRef
        setState((s) => ({
          ...s,
          saving: false,
          conflict: changed ? { mine: baseRef } : null,
          failure: fresh === null ? failure : null
        }))
      } else {
        setState((s) => ({
          ...s,
          saving: false,
          failure,
          forbidden: s.forbidden || failure.kind === 'forbidden'
        }))
      }
      return
    }
    const written = parseSetBaseRefResult(result.value)
    if (verifyBaseRefEcho(baseRef, written.request) === 'mismatch') {
      // A gateway that ignored the field answers without it; showing success here would be a lie.
      setState((s) => ({
        ...s,
        saving: false,
        failure: {
          kind: 'other',
          message: translate(`${T}notApplied`, "This server didn't apply the starting branch.")
        }
      }))
      return
    }
    // Only the base-ref fields are merged: the answer is a slimmer view than request.get and must not blank other fields.
    useAppStore.getState().upsertRequests([
      {
        ...current,
        baseRef: written.request.baseRef ?? baseRef,
        baseRefLocked: written.request.baseRefLocked ?? current.baseRefLocked,
        baseRefLockReason: written.request.baseRefLockReason ?? current.baseRefLockReason,
        version: written.request.version ?? current.version,
        updatedAt: written.request.updatedAt || current.updatedAt
      }
    ])
    toast.success(translate(`${T}saved`, 'Starting branch updated'))
    setState((s) => ({
      ...s,
      saving: false,
      unverifiedRef: written.baseRefUnverified ? baseRef : null
    }))
  }, [])

  const applyConflict = useCallback(async (): Promise<void> => {
    const mine = state.conflict?.mine
    if (mine === undefined) {
      return
    }
    // The reload already refreshed the store, so the retry carries the new version.
    await save(mine)
  }, [save, state.conflict])

  const dismiss = useCallback(() => {
    setState((s) => ({ ...s, failure: null, conflict: null }))
  }, [])

  return { state, save, applyConflict, dismiss }
}
