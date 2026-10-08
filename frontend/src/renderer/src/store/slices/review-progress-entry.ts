/** Shape of one worktree's reading-progress state (see review-progress.ts). */

import type {
  ReadingProgress,
  ReviewStateView
} from '../../components/review-map/review-wire-types'

export type ReviewProgressSaveStatus = 'saved' | 'dirty' | 'saving' | 'error'

export type ReviewProgressEntry = {
  baseCommit: string
  headCommit: string
  loadStatus: 'loading' | 'ready' | 'error'
  loadErrorKind: string | null
  serverState: ReviewStateView | null
  progress: ReadingProgress
  saveStatus: ReviewProgressSaveStatus
  lastErrorKind: string | null
  /** forbidden on save: marks stay local and are not retried. */
  localOnly: boolean
  /** Payload still too large after pruning tombstones: part of it is not saved. */
  oversize: boolean
}
