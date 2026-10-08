import type { LargeDiffRenderLimit } from './large-diff-render-limit'
import type { QualityAnnotationSource } from './quality-annotations/quality-annotation-eligibility'

// Why: review lenses jump to a symbol's line; nonce lets the same line be requested again.
export type DiffReviewReveal = {
  line: number
  side?: 'original' | 'modified'
  nonce: number
  onApplied?: (nonce: number) => void
}

export type DiffViewerProps = {
  modelKey: string
  originalModelKey?: string
  modifiedModelKey?: string
  originalContent: string
  modifiedContent: string
  language: string
  filePath: string
  relativePath: string
  sideBySide: boolean
  editable?: boolean
  // Why: optional because DiffViewer is also used by GitHubItemDialog for PR
  // review, where there is no local worktree to attach comments to.
  worktreeId?: string
  onAddLineComment?: (args: {
    lineNumber: number
    startLine?: number
    body: string
  }) => Promise<boolean>
  commentableLineNumbers?: readonly number[]
  addLineCommentLabel?: string
  addLineCommentPlaceholder?: string
  onContentChange?: (content: string) => void
  onSave?: (content: string) => void
  largeDiffRenderLimit?: LargeDiffRenderLimit
  // Why: main-process limited diffs intentionally blank text bodies before IPC;
  // the fallback must not treat that placeholder as a saveable draft.
  largeDiffSaveContentAvailable?: boolean
  reviewReveal?: DiffReviewReveal
  // Why: check annotations are drawn only when this side matches the worktree the run saw.
  diffSource?: QualityAnnotationSource
  compareHeadOid?: string | null
}
