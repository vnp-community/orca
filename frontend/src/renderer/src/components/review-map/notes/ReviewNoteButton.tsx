/**
 * ReviewNoteButton.tsx — FE-CV-TASK-060-03
 *
 * "Note" action for a graph node, finding or contract change. Disabled with a visible reason when
 * the node has no file (cluster, service, repo, secret), because a DiffComment must live in a file.
 * Optional `n` hotkey for the node that is currently selected.
 *
 * @module components/review-map/notes/ReviewNoteButton
 */

import { useEffect, useState } from 'react'
import { MessageSquarePlus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { isEditableTarget } from '@/lib/editable-target'
import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import { resolveGraphNodeCommentTarget } from './review-note-anchor'
import { ReviewNoteComposerPopover } from './ReviewNoteComposerPopover'
import { tn } from './notes-i18n'

export function ReviewNoteButton({
  worktreeId,
  anchor,
  hotkey = false,
  onSaved
}: {
  worktreeId: string
  anchor: ReviewNoteAnchor
  /** Enable the `n` shortcut (only for the node that is currently selected). */
  hotkey?: boolean
  onSaved?: (commentId: string) => void
}): React.JSX.Element {
  const [open, setOpen] = useState(false)
  const target = resolveGraphNodeCommentTarget(anchor)

  useEffect(() => {
    if (!hotkey || !target) {
      return
    }
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key !== 'n' || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) {
        return
      }
      if (isEditableTarget(event.target)) {
        return
      }
      event.preventDefault()
      setOpen(true)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [hotkey, target])

  if (!target) {
    return (
      <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
        <Button type="button" variant="ghost" size="xs" disabled aria-disabled="true">
          <MessageSquarePlus aria-hidden />
          {tn('ReviewNote.button', 'Note')}
        </Button>
        {tn('ReviewNote.noFileReason', 'No file to attach to')}
      </span>
    )
  }

  return (
    <ReviewNoteComposerPopover
      worktreeId={worktreeId}
      anchor={anchor}
      open={open}
      onOpenChange={setOpen}
      onSaved={onSaved}
    >
      <Button type="button" variant="ghost" size="xs">
        <MessageSquarePlus aria-hidden />
        {tn('ReviewNote.button', 'Note')}
      </Button>
    </ReviewNoteComposerPopover>
  )
}
