/**
 * review-turn-recorder-bus.ts — FE-CV-TASK-060-07
 *
 * Link between the App-level turn recorder (runs even while the Review tab is closed) and the
 * Review workspace: the workspace publishes overlay symbol keys, and reads back saved markers
 * and the save-failure state. A module store, so no provider has to wrap the App.
 *
 * @module components/review-map/turns/review-turn-recorder-bus
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'

export type TurnSaveFailure = { worktreeId: string; retry: () => void } | null

type SavedListener = (worktreeId: string, markers: ReviewTurnMarker[]) => void

const symbolKeyProviders = new Map<string, () => readonly string[] | null>()
const savedListeners = new Set<SavedListener>()
const failureListeners = new Set<() => void>()
let failure: TurnSaveFailure = null

/** The workspace registers its loaded overlay; returns the unregister function. */
export function registerTurnSymbolKeys(worktreeId: string, provider: () => readonly string[] | null): () => void {
  symbolKeyProviders.set(worktreeId, provider)
  return () => {
    if (symbolKeyProviders.get(worktreeId) === provider) {
      symbolKeyProviders.delete(worktreeId)
    }
  }
}

export function getRegisteredTurnSymbolKeys(worktreeId: string): readonly string[] | null {
  return symbolKeyProviders.get(worktreeId)?.() ?? null
}

export function publishTurnSaved(worktreeId: string, markers: ReviewTurnMarker[]): void {
  for (const listener of savedListeners) {
    listener(worktreeId, markers)
  }
}

export function subscribeTurnSaved(listener: SavedListener): () => void {
  savedListeners.add(listener)
  return () => {
    savedListeners.delete(listener)
  }
}

export function setTurnSaveFailure(next: TurnSaveFailure): void {
  failure = next
  for (const listener of failureListeners) {
    listener()
  }
}

export function getTurnSaveFailure(): TurnSaveFailure {
  return failure
}

export function subscribeTurnSaveFailure(listener: () => void): () => void {
  failureListeners.add(listener)
  return () => {
    failureListeners.delete(listener)
  }
}
