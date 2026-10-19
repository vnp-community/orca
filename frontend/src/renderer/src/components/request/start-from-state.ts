/**
 * start-from-state.ts — FE-REQ-TASK-040-03
 *
 * State of the "Start from" field of the create dialog: the repository default (nothing is sent) or a ref the user picked.
 *
 * @module components/request/start-from-state
 */

export type StartFromState = {
  /** The picked ref, or null for the repository default; only a picked ref is ever sent as `baseRef`. */
  ref: string | null
  /** The host could not be asked for branches: the field offers typing a name instead. */
  hostOffline: boolean
}

export type StartFromAction =
  | { type: 'select'; ref: string }
  | { type: 'useDefault' }
  | { type: 'projectChanged' }
  | { type: 'hostOffline'; offline: boolean }

export function initialStartFrom(): StartFromState {
  return { ref: null, hostOffline: false }
}

export function startFromReducer(state: StartFromState, action: StartFromAction): StartFromState {
  switch (action.type) {
    case 'select': {
      const ref = action.ref.trim()
      // An empty pick means "no choice": it must not turn into a pinned empty ref.
      return { ...state, ref: ref === '' ? null : ref }
    }
    case 'useDefault':
      return { ...state, ref: null }
    case 'projectChanged':
      // Why: a branch of the previous project's repo says nothing about the new one.
      return { ref: null, hostOffline: false }
    case 'hostOffline':
      return state.hostOffline === action.offline
        ? state
        : { ...state, hostOffline: action.offline }
  }
}

/** The `baseRef` to send: only a ref the user picked that differs from the default; undefined means send nothing. */
export function baseRefToSend(state: StartFromState): string | undefined {
  return state.ref ?? undefined
}
