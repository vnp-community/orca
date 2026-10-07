/**
 * review-scope-model.ts — FE-CV-TASK-051-01
 *
 * Pure scope model for the Review workspace.
 * Determines what is "in scope" for a given lens selection.
 *
 * @module components/review-map/review-scope-model
 */

export type ReviewScope =
  | { type: 'whole' }
  | { type: 'file'; path: string }
  | { type: 'symbol'; symbolId: string }

/**
 * Build the review scope from user selection.
 * Falls back to whole-review scope when selection is invalid.
 */
export function buildReviewScope(selection: {
  selectedPath?: string | null
  selectedSymbolId?: string | null
}): ReviewScope {
  if (selection.selectedSymbolId) {
    return { type: 'symbol', symbolId: selection.selectedSymbolId }
  }
  if (selection.selectedPath && selection.selectedPath !== '*') {
    return { type: 'file', path: selection.selectedPath }
  }
  return { type: 'whole' }
}

/**
 * Get a stable string key for a review scope (for use in cache keys).
 */
export function getReviewScopeKey(scope: ReviewScope): string {
  switch (scope.type) {
    case 'whole': return 'whole'
    case 'file': return `file:${scope.path}`
    case 'symbol': return `symbol:${scope.symbolId}`
  }
}

/**
 * Determine if a given file path is within scope.
 */
export function isPathInScope(scope: ReviewScope, path: string): boolean {
  switch (scope.type) {
    case 'whole': return true
    case 'file': return scope.path === path
    case 'symbol': return true // Symbol may span multiple files
  }
}
