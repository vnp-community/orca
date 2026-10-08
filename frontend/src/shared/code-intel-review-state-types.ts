/**
 * code-intel-review-state-types.ts — CONTRACT-codeintel-ui-api §4.6 review state and notes
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

// ---------------------------------------------------------------------------
// §4.6  Review state
// ---------------------------------------------------------------------------

export type ReadingProgress = {
  version: 1
  entries: Record<string, { state: 'seen' | 'unseen'; at: number }>
  lastFocusedKey: string | null
}

export type ReviewNoteLens =
  | 'impact'
  | 'architecture'
  | 'dataflow'
  | 'erd'
  | 'storage'
  | 'structure'
  | 'contract'
  | 'quality'
  | 'requirements'

export type ReviewNoteAnchor =
  | { kind: 'diff-line'; filePath: string; startLine?: number; lineNumber: number }
  | {
      kind: 'graph-node'
      lens: ReviewNoteLens
      nodeKey: string
      filePath: string
      startLine?: number
      endLine?: number
      label: string
    }
  | { kind: 'finding'; findingKey: string; filePath: string; startLine?: number; label: string }

export type ReviewSentBatch = {
  batchId: string
  sentAt: number
  turnId: string | null
  targetPaneKey: string | null
  agentType: string | null
  notes: {
    commentId: string
    anchor: ReviewNoteAnchor
    filePath: string
    startLine?: number
    lineNumber: number
    body: string
    fileIdentityAtSend?: string
  }[]
}

export type ReviewTurnMarker = {
  /** `${paneKey}:${doneAt}` */
  turnId: string
  worktreeId: string
  paneKey: string
  agentType: string | null
  startedAt: number | null
  endedAt: number
  interrupted?: boolean
  baseOid: string | null
  headOid: string | null
  files: { p: string; o?: string; h: string }[]
  symbolKeys?: string[]
  overlayAvailable: boolean
}

/** version 0 means "no record yet". Writes pass expectedVersion = the version read. */
export type ReviewState = {
  baseCommit: string
  headCommit: string
  readingProgress: ReadingProgress
  notes: { anchors: Record<string, ReviewNoteAnchor>; sentBatches: ReviewSentBatch[] }
  turnMarkers: ReviewTurnMarker[]
  status: 'open' | 'reviewed'
  updatedBy?: string
  updatedAt?: string
  version: number
}
