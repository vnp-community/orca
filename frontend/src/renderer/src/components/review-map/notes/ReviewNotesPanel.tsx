/**
 * ReviewNotesPanel.tsx — FE-CV-TASK-060-03
 *
 * Review-map notes grouped by lens with edit / delete / jump-to-anchor and the batch send menu.
 * Editing a sent note returns it to the send queue (updateDiffComment clears `sentAt`).
 *
 * @module components/review-map/notes/ReviewNotesPanel
 */

import { useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useAppStore } from '@/store'
import { useReviewNotesPersistence } from '../../../hooks/useReviewNotesPersistence'
import { buildGraphNoteBody, parseGraphNoteBody } from './review-note-anchor'
import { jumpToNoteAnchor } from './review-note-navigation'
import { anchoredNotes, groupNotesByLens } from './review-note-selectors'
import type { AnchoredNote } from './review-note-selectors'
import { ReviewNotesSendMenu } from './ReviewNotesSendMenu'
import { tn } from './notes-i18n'

function lensLabel(lens: string): string {
  return lens === 'findings' ? tn('ReviewNote.lens.findings', 'Findings') : lens
}

function NoteRow({
  note,
  onJump,
  onSave,
  onDelete
}: {
  note: AnchoredNote
  onJump: () => void
  onSave: (body: string) => Promise<boolean>
  onDelete: () => void
}): React.JSX.Element {
  const [editing, setEditing] = useState(false)
  const parsed = parseGraphNoteBody(note.comment.body)
  const shown = parsed?.text ?? note.comment.body
  const [draft, setDraft] = useState(shown)
  const [failed, setFailed] = useState(false)
  const where =
    note.comment.lineNumber > 0 ? `${note.comment.filePath}:${note.comment.lineNumber}` : note.comment.filePath
  return (
    <li className="space-y-1 border-b px-2 py-1.5 text-xs" data-comment-id={note.comment.id}>
      <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
        <span className="font-medium text-foreground">{note.anchor.label}</span>
        <span className="truncate" title={where}>{where}</span>
        {note.comment.sentAt ? <span>{tn('ReviewNote.sent', 'Sent')}</span> : null}
      </div>
      {editing ? (
        <div className="space-y-1">
          <Textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            aria-label={tn('ReviewNote.label', 'Note')}
            className="min-h-16 text-xs"
          />
          {failed ? <p role="alert" className="text-destructive">{tn('ReviewNote.saveFailed', 'Could not save the note. Your text is kept.')}</p> : null}
          <div className="flex gap-1">
            <Button
              type="button"
              size="xs"
              onClick={async () => {
                const ok = await onSave(buildGraphNoteBody(note.anchor, draft))
                setFailed(!ok)
                if (ok) {
                  setEditing(false)
                }
              }}
            >
              {tn('ReviewNote.save', 'Save note')}
            </Button>
            <Button type="button" size="xs" variant="ghost" onClick={() => setEditing(false)}>
              {tn('ReviewNote.cancel', 'Cancel')}
            </Button>
          </div>
        </div>
      ) : (
        <p className="whitespace-pre-wrap break-words">{shown}</p>
      )}
      {!editing ? (
        <div className="flex gap-1">
          <Button type="button" variant="ghost" size="xs" onClick={onJump}>
            {tn('ReviewNote.jump', 'Go to')}
          </Button>
          <Button type="button" variant="ghost" size="xs" onClick={() => setEditing(true)}>
            {tn('ReviewNote.edit', 'Edit')}
          </Button>
          <Button type="button" variant="ghost" size="xs" onClick={onDelete}>
            {tn('ReviewNote.delete', 'Delete')}
          </Button>
        </div>
      ) : null}
    </li>
  )
}

export function ReviewNotesPanel({
  worktreeId,
  onOpenDiff
}: {
  worktreeId: string
  onOpenDiff: (path: string, line?: number) => void
}): React.JSX.Element {
  const comments = useAppStore((s) => s.getDiffComments(worktreeId))
  const updateDiffComment = useAppStore((s) => s.updateDiffComment)
  const deleteDiffComment = useAppStore((s) => s.deleteDiffComment)
  const setReviewLens = useAppStore((s) => s.setReviewLens)
  const selectReviewSymbol = useAppStore((s) => s.selectReviewSymbol)
  const selectErdTable = useAppStore((s) => s.selectErdTable)
  const selectStorageNode = useAppStore((s) => s.selectStorageNode)
  const setReviewDataFlowId = useAppStore((s) => s.setReviewDataFlowId)
  const { notes, saveStatus } = useReviewNotesPersistence(worktreeId)

  const groups = useMemo(() => groupNotesByLens(anchoredNotes(comments, notes.anchors)), [comments, notes.anchors])
  const total = groups.reduce((n, g) => n + g.notes.length, 0)

  return (
    <section aria-label={tn('ReviewNote.panel', 'Review notes')} className="flex min-h-0 flex-col text-xs">
      <header className="flex items-center justify-between gap-2 border-b px-2 py-1.5">
        <span className="font-medium">{tn('ReviewNote.panel', 'Review notes')} ({total})</span>
        <ReviewNotesSendMenu worktreeId={worktreeId} />
      </header>
      {saveStatus === 'error' ? (
        <p role="status" className="px-2 py-1 text-destructive">
          {tn('ReviewNote.persistFailed', 'Could not save the note history. Notes are kept locally.')}
        </p>
      ) : null}
      {total === 0 ? (
        <p className="p-3 text-muted-foreground">
          {tn('ReviewNote.empty', 'No notes yet. Use Note on a node or a finding.')}
        </p>
      ) : (
        groups.map((group) => (
          <div key={group.lens}>
            <h3 className="bg-muted/40 px-2 py-1 text-[11px] font-medium text-muted-foreground">
              {lensLabel(group.lens)} ({group.notes.length})
            </h3>
            <ul>
              {group.notes.map((note) => (
                <NoteRow
                  key={note.comment.id}
                  note={note}
                  onJump={() =>
                    jumpToNoteAnchor(worktreeId, note.anchor, {
                      setReviewLens,
                      selectReviewSymbol,
                      selectErdTable,
                      selectStorageNode,
                      setReviewDataFlowId,
                      openDiff: onOpenDiff
                    })
                  }
                  onSave={(body) => updateDiffComment(worktreeId, note.comment.id, body)}
                  onDelete={() => void deleteDiffComment(worktreeId, note.comment.id)}
                />
              ))}
            </ul>
          </div>
        ))
      )}
    </section>
  )
}
