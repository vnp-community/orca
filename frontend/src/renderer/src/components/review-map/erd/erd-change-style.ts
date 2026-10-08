/**
 * erd-change-style.ts — FE-CV-TASK-057-05
 *
 * Token classes for change states. Git-decoration tokens are used only because the
 * states mirror git add/modify/delete; each state also carries a symbol and a text label.
 */

import type { ErdChangeKind } from './erd-column-changes'

export const ERD_CHANGE_TEXT_CLASS: Record<ErdChangeKind, string> = {
  added: 'text-[color:var(--git-decoration-added)]',
  modified: 'text-[color:var(--git-decoration-modified)]',
  removed: 'text-[color:var(--git-decoration-deleted)]'
}

export const ERD_CHANGE_BORDER_CLASS: Record<ErdChangeKind, string> = {
  added: 'border-[color:var(--git-decoration-added)]',
  modified: 'border-[color:var(--git-decoration-modified)]',
  removed: 'border-[color:var(--git-decoration-deleted)] border-dashed'
}

export const ERD_CHANGE_LABEL: Record<ErdChangeKind, { key: string; fallback: string }> = {
  added: { key: 'auto.components.reviewMap.ErdTableNode.change.added', fallback: 'Added' },
  modified: { key: 'auto.components.reviewMap.ErdTableNode.change.modified', fallback: 'Changed' },
  removed: { key: 'auto.components.reviewMap.ErdTableNode.change.removed', fallback: 'Removed' }
}
