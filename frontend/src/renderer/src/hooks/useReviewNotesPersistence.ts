/**
 * useReviewNotesPersistence.ts — FE-CV-TASK-060-05
 *
 * Persists review-note anchors and sent batches on the (base, head) ReviewState row through the
 * reading-progress slice's single save queue (patchReviewState), so reading progress and notes
 * never race on the same row. If a conflict resolution in the slice overwrote our pending notes,
 * the effect below merges them back (union, local wins) and patches again. Turn markers use the
 * separate worktree-level row (review-turn-marker-row.ts).
 *
 * @module hooks/useReviewNotesPersistence
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import type { ReviewTurnMarker } from '../../../shared/code-intel-types'
import { defaultReviewDataApi } from '../components/review-map/review-data-api-default'
import type { ReviewDataApi } from '../components/review-map/review-shell-data'
import {
  mergeReviewNotes,
  notesEqual,
  readNotes
} from '../components/review-map/notes/review-state-merge'
import { trimSentBatches } from '../components/review-map/notes/review-sent-batch'
import type { NotesPayload } from '../components/review-map/notes/review-sent-batch'
import {
  loadTurnMarkers,
  saveTurnMarkerRow
} from '../components/review-map/turns/review-turn-marker-row'

export type ReviewNotesSaveStatus = 'idle' | 'saving' | 'saved' | 'error'

export type UseReviewNotesPersistenceResult = {
  /** False until the (base, head) row is loaded; saves are refused (return false) until then. */
  ready: boolean
  notes: NotesPayload
  saveStatus: ReviewNotesSaveStatus
  /** Applies `update` to the freshest notes; false when the row is not loaded yet. */
  updateNotes: (update: (prev: NotesPayload) => NotesPayload) => boolean
  turnMarkers: ReviewTurnMarker[]
  markersStatus: ReviewNotesSaveStatus
  saveTurnMarkers: (markers: readonly ReviewTurnMarker[]) => Promise<boolean>
  retryMarkers: () => void
}

const EMPTY_NOTES: NotesPayload = { anchors: {}, sentBatches: [] }

export function useReviewNotesPersistence(
  worktreeId: string | null,
  opts: { enabled?: boolean; api?: ReviewDataApi } = {}
): UseReviewNotesPersistenceResult {
  const enabled = opts.enabled !== false && Boolean(worktreeId)
  const api = opts.api ?? defaultReviewDataApi
  const entry = useAppStore((s) => (worktreeId ? s.reviewProgressByWorktree[worktreeId] : undefined))
  const patchReviewState = useAppStore((s) => s.patchReviewState)

  const serverNotesRaw = entry?.serverState?.notes
  const ready = enabled && entry?.loadStatus === 'ready' && Boolean(entry.serverState)
  const serverNotesRef = useRef<NotesPayload>(EMPTY_NOTES)
  serverNotesRef.current = readNotes(serverNotesRaw)

  // What we last asked the slice to save; cleared once the server state contains it.
  const pendingRef = useRef<NotesPayload | null>(null)
  const retriedTooLargeRef = useRef(false)

  const push = useCallback(
    (next: NotesPayload, maxEntries?: number): void => {
      if (!worktreeId) {
        return
      }
      const trimmed = trimSentBatches(next, maxEntries ? { maxEntries } : {}).payload
      pendingRef.current = trimmed
      patchReviewState(worktreeId, { notes: trimmed })
    },
    [worktreeId, patchReviewState]
  )

  const updateNotes = useCallback(
    (update: (prev: NotesPayload) => NotesPayload): boolean => {
      if (!ready) {
        return false
      }
      const base = pendingRef.current ?? serverNotesRef.current
      retriedTooLargeRef.current = false
      push(update(base))
      return true
    },
    [ready, push]
  )

  // The slice writes serverState before the save lands, so only a settled ('saved') row tells us
  // whether a conflict round replaced our notes with the remote copy; then re-apply what is missing.
  useEffect(() => {
    const pending = pendingRef.current
    if (!pending || !ready || entry?.saveStatus !== 'saved') {
      return
    }
    const server = serverNotesRef.current
    if (notesEqual(server, pending)) {
      pendingRef.current = null
      return
    }
    const merged = mergeReviewNotes(pending, server)
    if (notesEqual(merged, server)) {
      pendingRef.current = null
    } else {
      push(merged)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- keyed on the server copy and save status
  }, [serverNotesRaw, ready, entry?.saveStatus])

  // PAYLOAD_TOO_LARGE: halve the entry budget and try once more.
  useEffect(() => {
    if (entry?.saveStatus === 'error' && entry.lastErrorKind === 'too-large' && pendingRef.current) {
      if (!retriedTooLargeRef.current) {
        retriedTooLargeRef.current = true
        push(pendingRef.current, 250)
      }
    }
  }, [entry?.saveStatus, entry?.lastErrorKind, push])

  const saveStatus: ReviewNotesSaveStatus = !entry
    ? 'idle'
    : entry.saveStatus === 'saving'
      ? 'saving'
      : entry.saveStatus === 'error'
        ? 'error'
        : entry.saveStatus === 'dirty'
          ? 'saving'
          : 'saved'

  // ---- worktree-level row: turn markers ----
  const [turnMarkers, setTurnMarkers] = useState<ReviewTurnMarker[]>([])
  const [markersStatus, setMarkersStatus] = useState<ReviewNotesSaveStatus>('idle')
  const lastMarkersRef = useRef<readonly ReviewTurnMarker[]>([])

  useEffect(() => {
    if (!enabled || !worktreeId) {
      setTurnMarkers([])
      return
    }
    let cancelled = false
    void loadTurnMarkers(api, worktreeId).then((res) => {
      if (!cancelled && res.ok) {
        setTurnMarkers(res.markers)
      }
    })
    return () => {
      cancelled = true
    }
  }, [enabled, worktreeId, api])

  const saveTurnMarkers = useCallback(
    async (markers: readonly ReviewTurnMarker[]): Promise<boolean> => {
      if (!enabled || !worktreeId) {
        return false
      }
      lastMarkersRef.current = markers
      setMarkersStatus('saving')
      const res = await saveTurnMarkerRow(api, worktreeId, markers)
      if (res.ok) {
        setTurnMarkers(res.markers)
        setMarkersStatus('saved')
        return true
      }
      setMarkersStatus('error')
      return false
    },
    [enabled, worktreeId, api]
  )

  const retryMarkers = useCallback(() => {
    if (lastMarkersRef.current.length > 0) {
      void saveTurnMarkers(lastMarkersRef.current)
    }
  }, [saveTurnMarkers])

  return {
    ready,
    notes: pendingRef.current ?? serverNotesRef.current,
    saveStatus,
    updateNotes,
    turnMarkers,
    markersStatus,
    saveTurnMarkers,
    retryMarkers
  }
}
