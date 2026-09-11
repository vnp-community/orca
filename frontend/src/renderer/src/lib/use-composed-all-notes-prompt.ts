// TASK-FE-ANNOTATE-004: enrich the "all unsent notes" send-to-agent prompt
// with backend-go's annotation.composeReviewPrompt (±2-line code context,
// BR-CR-11) when available, falling back to the existing client-side
// formatDiffComments() when the RPC isn't reachable/fails/is slow. See
// SOL-FE-ANNOTATE-002 — this deliberately does NOT touch
// sendPromptWithGuardedPasteAndEnter/sendNotesToActiveAgentSession, only
// the source of the `prompt` string those already-working functions send.
import { useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { formatDiffComments } from '@/lib/diff-comments-format'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import type { DiffComment } from '../../../shared/types'

// Why: debounce so a burst of comment adds (each re-triggering this effect
// via the notesKey dependency) doesn't fire one annotation.composeReviewPrompt
// call per keystroke-adjacent edit — the send menu only needs a
// reasonably-fresh prompt by the time the user actually opens it, not one
// recomputed on every state change.
const COMPOSE_DEBOUNCE_MS = 500

type ComposedPrompt = { prompt: string; annotationIds: string[] }

export function useComposedAllNotesPrompt(
  worktreeId: string,
  worktreeName: string,
  unsentNotes: readonly DiffComment[]
): ComposedPrompt {
  const settings = useAppStore((s) => s.settings)
  const fallbackPrompt = useMemo(() => formatDiffComments(unsentNotes), [unsentNotes])
  const fallbackIds = useMemo(() => unsentNotes.map((c) => c.id), [unsentNotes])
  const [composed, setComposed] = useState<ComposedPrompt | null>(null)
  // Why: identity key, not the array itself — unsentNotes gets a fresh
  // array identity on every store update even when its contents haven't
  // changed (Zustand's default equality), which would otherwise re-fire
  // this effect (and the debounce timer) on unrelated state changes.
  const notesKey = useMemo(
    () => unsentNotes.map((c) => `${c.id}:${c.body}`).join('|'),
    [unsentNotes]
  )

  useEffect(() => {
    // Why: a stale composed prompt from a previous, now-changed buffer must
    // not linger — better to show the always-correct client-side fallback
    // immediately than a composed prompt for a comment set that no longer
    // matches what's about to be sent.
    setComposed(null)
    if (unsentNotes.length === 0) {
      return
    }
    const target = getActiveRuntimeTarget(settings)
    // Why: only the 'environment' (web/multi-user) target has an
    // annotation-service bridge — see SOL-FE-ANNOTATE-001 §0. Calling this
    // RPC from desktop/local would just be a guaranteed-to-fail round trip.
    if (target.kind === 'local') {
      return
    }
    let cancelled = false
    const timer = setTimeout(() => {
      void callRuntimeRpc<ComposedPrompt>(
        target,
        'annotation.composeReviewPrompt',
        { worktreeId, worktreeName },
        { timeoutMs: 8000 }
      )
        .then((result) => {
          if (!cancelled && result?.prompt) {
            setComposed(result)
          }
        })
        .catch(() => {
          // Why: silent — the fallback prompt already covers the UX; a
          // toast here would fire on every debounce tick while
          // annotation-service is degraded, which is noise, not signal
          // (CR-ANNOTATE-002's acceptance criterion: a compose failure must
          // not block sending).
        })
    }, COMPOSE_DEBOUNCE_MS)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- Why: notesKey is the intentional identity proxy for unsentNotes (see above); including unsentNotes itself would defeat it.
  }, [notesKey, worktreeId, worktreeName, settings, unsentNotes.length])

  return composed ?? { prompt: fallbackPrompt, annotationIds: fallbackIds }
}
