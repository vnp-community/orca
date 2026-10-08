/**
 * ReviewSendPreviewDialog.tsx — FE-CV-TASK-060-04
 *
 * Read-only view of the prompt that "All unsent notes" will send, with a size warning.
 *
 * @module components/review-map/notes/ReviewSendPreviewDialog
 */

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { SEND_PREVIEW_WARN_COUNT } from './review-sent-batch'
import { tn } from './notes-i18n'

export function ReviewSendPreviewDialog({
  open,
  onOpenChange,
  prompt,
  noteCount
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  prompt: string
  noteCount: number
}): React.JSX.Element {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{tn('ReviewNote.preview.title', 'Prompt preview')}</DialogTitle>
          <DialogDescription>
            {tn('ReviewNote.preview.description', '{{count}} notes will be sent as written below.', {
              count: noteCount
            })}
          </DialogDescription>
        </DialogHeader>
        {noteCount > SEND_PREVIEW_WARN_COUNT ? (
          <p role="alert" className="text-xs text-destructive">
            {tn('ReviewNote.preview.tooMany', 'This is a large batch. Consider sending a smaller scope first.')}
          </p>
        ) : null}
        <pre
          aria-label={tn('ReviewNote.preview.title', 'Prompt preview')}
          className="max-h-96 overflow-auto whitespace-pre-wrap rounded-md border bg-muted p-2 text-xs"
        >
          {prompt}
        </pre>
      </DialogContent>
    </Dialog>
  )
}
