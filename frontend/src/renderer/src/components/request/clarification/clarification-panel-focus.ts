/**
 * Moves the user to the Clarification panel once it has rendered, e.g. right after "Ask back"
 * flips the Request to `awaiting_information` (the panel mounts a refetch later).
 *
 * @module components/request/clarification/clarification-panel-focus
 */

import { CLARIFICATION_PANEL_DOM_ID } from './ClarificationPanel'

const RETRY_MS = 250

export function focusClarificationPanelWhenReady(triesLeft = 20): void {
  const panel = document.getElementById(CLARIFICATION_PANEL_DOM_ID)
  if (panel) {
    panel.scrollIntoView({ block: 'start' })
    panel.focus({ preventScroll: true })
    return
  }
  if (triesLeft > 0) {
    setTimeout(() => focusClarificationPanelWhenReady(triesLeft - 1), RETRY_MS)
  }
}
