/**
 * ReviewSentBatchList.tsx — FE-CV-TASK-060-04
 *
 * History of batches handed to an agent. Each note carries a *hint* whether its file changed
 * since sending; it is an estimate from coarse fingerprints and never means "addressed".
 *
 * @module components/review-map/notes/ReviewSentBatchList
 */

import type { ReviewSentBatch } from '../../../../../shared/code-intel-types'
import { noteProgressHint } from '../turns/turn-compare-model'
import type { NoteProgressHint } from '../turns/turn-compare-model'
import { tn } from './notes-i18n'

function hintLabel(hint: NoteProgressHint): string | null {
  switch (hint) {
    case 'file_changed':
      return tn('ReviewSent.hint.changed', 'File changed since sending (estimate)')
    case 'file_unchanged':
      return tn('ReviewSent.hint.unchanged', 'No change seen in this file (estimate)')
    default:
      return null
  }
}

export function ReviewSentBatchList({
  batches,
  currentIdentityByPath,
  identityListTruncated = false
}: {
  batches: readonly ReviewSentBatch[]
  currentIdentityByPath: Readonly<Record<string, string>>
  identityListTruncated?: boolean
}): React.JSX.Element {
  if (batches.length === 0) {
    return <p className="p-3 text-xs text-muted-foreground">{tn('ReviewSent.empty', 'Nothing sent yet.')}</p>
  }
  const ordered = [...batches].sort((a, b) => b.sentAt - a.sentAt)
  return (
    <section aria-label={tn('ReviewSent.title', 'Sent in earlier turns')} className="text-xs">
      <h3 className="px-2 py-1 text-[11px] font-medium text-muted-foreground">
        {tn('ReviewSent.title', 'Sent in earlier turns')}
      </h3>
      <ul>
        {ordered.map((batch) => (
          <li key={batch.batchId} className="border-b px-2 py-1.5">
            <div className="text-[11px] text-muted-foreground">
              {new Date(batch.sentAt).toLocaleString()}
              {batch.agentType ? ` · ${batch.agentType}` : ''} ·{' '}
              {tn('ReviewSent.count', '{{count}} notes', { count: batch.notes.length })}
            </div>
            <ul className="mt-1 space-y-0.5">
              {batch.notes.map((note) => {
                const hint = hintLabel(
                  noteProgressHint(note, currentIdentityByPath, identityListTruncated)
                )
                return (
                  <li key={note.commentId}>
                    <span className="break-words">{note.body}</span>
                    <span className="ml-1 text-muted-foreground">
                      {note.filePath}
                      {note.lineNumber > 0 ? `:${note.lineNumber}` : ''}
                    </span>
                    {hint ? <span className="ml-1 text-[11px] text-muted-foreground">{hint}</span> : null}
                  </li>
                )
              })}
            </ul>
          </li>
        ))}
      </ul>
    </section>
  )
}
