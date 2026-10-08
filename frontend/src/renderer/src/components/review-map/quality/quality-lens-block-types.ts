/**
 * quality-lens-block-types.ts — FE-CV-TASK-087-07
 *
 * Props every collapsible block of the `quality` lens receives (coverage, trend, hotspot,
 * dependency). Blocks mount lazily, only when opened, and never call RPC while closed.
 *
 * @module components/review-map/quality/quality-lens-block-types
 */

export type QualityBlockProps = {
  worktreeId: string
  /** Opens the diff of `path` at an optional 1-based line (Review's diff opener). */
  onOpenDiff: (path: string, line?: number) => void
  /** Switches the Review workspace to another lens (e.g. 'structure'); absent when not possible. */
  onOpenLens?: (lensId: string) => void
}
