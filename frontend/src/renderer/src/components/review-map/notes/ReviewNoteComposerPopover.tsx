/**
 * ReviewNoteComposerPopover.tsx — FE-CV-TASK-060-03
 *
 * Writes a graph/finding note as an ordinary DiffComment (prefixed body) and stores the anchor in
 * ReviewState.notes. The popover shows the exact file:line the note will attach to and keeps the
 * draft when saving fails.
 *
 * @module components/review-map/notes/ReviewNoteComposerPopover
 */

import { useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Textarea } from '@/components/ui/textarea'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { useAppStore } from '@/store'
import { getScreenSubmitModifierLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import { useReviewNotesPersistence } from '../../../hooks/useReviewNotesPersistence'
import { buildGraphNoteBody, resolveGraphNodeCommentTarget } from './review-note-anchor'
import { tn } from './notes-i18n'

const SAVING_INDICATOR_DELAY_MS = 200

export function ReviewNoteComposerPopover({
  worktreeId,
  anchor,
  open,
  onOpenChange,
  onSaved,
  children
}: {
  worktreeId: string
  anchor: ReviewNoteAnchor
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved?: (commentId: string) => void
  children: React.ReactNode
}): React.JSX.Element {
  const addDiffComment = useAppStore((s) => s.addDiffComment)
  const { updateNotes } = useReviewNotesPersistence(worktreeId)
  const [text, setText] = useState('')
  const [saving, setSaving] = useState(false)
  const [showSpinner, setShowSpinner] = useState(false)
  const [failed, setFailed] = useState(false)
  const lockRef = useRef(false)
  const target = resolveGraphNodeCommentTarget(anchor)

  useEffect(() => {
    if (!saving) {
      setShowSpinner(false)
      return
    }
    const timer = setTimeout(() => setShowSpinner(true), SAVING_INDICATOR_DELAY_MS)
    return () => clearTimeout(timer)
  }, [saving])

  const save = async (): Promise<void> => {
    const body = text.trim()
    // Lock synchronously so a double Mod+Enter cannot create two notes.
    if (!target || !body || lockRef.current) {
      return
    }
    lockRef.current = true
    setSaving(true)
    setFailed(false)
    try {
      const comment = await addDiffComment({
        worktreeId,
        filePath: target.filePath,
        ...(target.startLine ? { startLine: target.startLine } : {}),
        lineNumber: target.lineNumber,
        body: buildGraphNoteBody(anchor, body),
        side: 'modified'
      })
      if (!comment) {
        setFailed(true)
        return
      }
      updateNotes((prev) => ({ ...prev, anchors: { ...prev.anchors, [comment.id]: anchor } }))
      setText('')
      onOpenChange(false)
      onSaved?.(comment.id)
    } finally {
      lockRef.current = false
      setSaving(false)
    }
  }

  const where = target
    ? target.lineNumber > 0
      ? `${target.filePath}:${target.lineNumber}`
      : target.filePath
    : null

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-80 space-y-2 p-3 text-xs"
        onKeyDown={(event) => {
          if (isScreenSubmitShortcut(event)) {
            event.preventDefault()
            void save()
          }
        }}
      >
        <div className="text-muted-foreground">
          {where
            ? tn('ReviewNote.attachesTo', 'Attaches to {{where}}', { where })
            : tn('ReviewNote.noFile', 'This item has no file to attach a note to.')}
        </div>
        <Textarea
          autoFocus
          value={text}
          disabled={saving}
          onChange={(e) => setText(e.target.value)}
          placeholder={tn('ReviewNote.placeholder', 'Write a note for the agent')}
          aria-label={tn('ReviewNote.label', 'Note')}
          className="min-h-20 text-xs"
        />
        {failed ? (
          <p role="alert" className="text-destructive">
            {tn('ReviewNote.saveFailed', 'Could not save the note. Your text is kept.')}
          </p>
        ) : null}
        <div className="flex items-center justify-between gap-2">
          <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} />
          <Button type="button" size="xs" disabled={saving || !text.trim() || !target} onClick={() => void save()}>
            {showSpinner ? <Loader2 className="animate-spin" aria-hidden /> : null}
            {tn('ReviewNote.save', 'Save note')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
