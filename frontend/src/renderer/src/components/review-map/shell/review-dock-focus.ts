/**
 * review-dock-focus.ts — FE-CV-TASK-059-05
 *
 * Lets controls outside the bottom dock (e.g. the "N findings" summary chip) open a dock panel
 * without lifting the dock's persisted open/panel state into the workspace.
 *
 * @module components/review-map/shell/review-dock-focus
 */

type Listener = (worktreeId: string, panelId: string) => void

const listeners = new Set<Listener>()

export function requestReviewDockPanel(worktreeId: string, panelId: string): void {
  for (const listener of listeners) {
    listener(worktreeId, panelId)
  }
}

export function subscribeReviewDockFocus(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}
