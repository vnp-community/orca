/**
 * code-intel-settings-visibility.ts — FE-CV-TASK-073-06
 *
 * Whether the Settings "Code intelligence" section is offered. Kept apart from the card so the
 * navigation metadata can import it without pulling in the RPC client.
 *
 * @module lib/code-intel-settings-visibility
 */

/** Hidden only when the backend is known not to offer code intelligence. */
export function selectCodeIntelSettingsVisible(state: {
  codeIntelSupportState?: { state: string } | null
}): boolean {
  return state.codeIntelSupportState?.state !== 'unsupported'
}
