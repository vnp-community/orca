/**
 * review-note-anchor.ts — FE-CV-TASK-060-01
 *
 * A review-map note is an ordinary DiffComment (it must live in a file) plus an anchor kept in
 * ReviewState.notes.anchors and a text prefix. The prefix is read by the receiving agent, so its
 * format is a soft contract: change it deliberately.
 *
 * @module components/review-map/notes/review-note-anchor
 */

import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import { maskSensitiveText } from '../sensitive-text-masking'

export const NOTE_LABEL_MAX = 120
export const NOTE_PREFIX_RE = /^\[Review map · ([^·\]]+) · ([^\]]*)\] ?/

export type GraphNoteCommentTarget = {
  filePath: string
  startLine?: number
  /** 0 means a file-level note. */
  lineNumber: number
}

export function isGraphAnchor(
  anchor: ReviewNoteAnchor | undefined | null
): anchor is Extract<ReviewNoteAnchor, { kind: 'graph-node' | 'finding' }> {
  return anchor?.kind === 'graph-node' || anchor?.kind === 'finding'
}

/** Lens id used to group notes in the panel; finding anchors belong to the structure findings list. */
export function anchorLensId(anchor: ReviewNoteAnchor): string {
  if (anchor.kind === 'graph-node') {
    return anchor.lens
  }
  return anchor.kind === 'finding' ? 'findings' : 'diff'
}

/** Where the DiffComment for this anchor goes; null when the node has no file (cluster, service...). */
export function resolveGraphNodeCommentTarget(anchor: ReviewNoteAnchor): GraphNoteCommentTarget | null {
  if (!anchor.filePath) {
    return null
  }
  if (anchor.kind === 'diff-line') {
    return {
      filePath: anchor.filePath,
      ...(anchor.startLine !== undefined ? { startLine: anchor.startLine } : {}),
      lineNumber: anchor.lineNumber
    }
  }
  if (anchor.kind === 'finding') {
    return {
      filePath: anchor.filePath,
      ...(anchor.startLine ? { startLine: anchor.startLine } : {}),
      lineNumber: anchor.startLine ?? 0
    }
  }
  const startLine = anchor.startLine
  return {
    filePath: anchor.filePath,
    ...(startLine ? { startLine } : {}),
    lineNumber: anchor.endLine ?? startLine ?? 0
  }
}

function safeLabel(label: string): string {
  const masked = maskSensitiveText(label).text
  const flat = masked.replace(/[\r\n\]·]+/g, ' ').replace(/\s+/g, ' ').trim()
  return flat.length > NOTE_LABEL_MAX ? `${flat.slice(0, NOTE_LABEL_MAX - 1)}…` : flat
}

export function buildGraphNoteBody(anchor: ReviewNoteAnchor, userText: string): string {
  if (!isGraphAnchor(anchor)) {
    return userText
  }
  return `[Review map · ${anchorLensId(anchor)} · ${safeLabel(anchor.label)}] ${userText}`
}

/** Splits a stored body back into the prefix parts and the user's text (for display only). */
export function parseGraphNoteBody(body: string): { lens: string; label: string; text: string } | null {
  const match = NOTE_PREFIX_RE.exec(body)
  if (!match) {
    return null
  }
  return { lens: match[1], label: match[2], text: body.slice(match[0].length) }
}
