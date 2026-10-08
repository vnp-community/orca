import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'

// TASK-FE-ANNOTATE-005: best-effort bookkeeping after a successful send —
// a failure here must never surface as a "send failed" error since the
// prompt was already delivered by the time this runs. Only meaningful for
// the 'environment' target (no annotation-service bridge in desktop mode,
// same guard as useComposedAllNotesPrompt/persistRemote).
// Extracted from DiffNotesSendMenu (FE-CV-TASK-060-02) so the review-notes send menu shares it.
export function markAnnotationsSentBestEffort(annotationIds: readonly string[]): void {
  if (annotationIds.length === 0) {
    return
  }
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  if (target.kind === 'local') {
    return
  }
  void callRuntimeRpc(
    target,
    'annotation.markSent',
    { ids: annotationIds },
    { timeoutMs: 8000 }
  ).catch(() => {})
}
