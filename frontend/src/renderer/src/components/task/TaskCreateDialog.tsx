import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../ui/dialog'
import { Input } from '../ui/input'
import { Button } from '../ui/button'

type TaskCreateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  // parentId when opened from the "+ subtask" context on a specific TaskCard; undefined = root task
  parentId?: string
  onCreate: (title: string, parentId?: string) => Promise<void>
}

export function TaskCreateDialog({
  open,
  onOpenChange,
  parentId,
  onCreate
}: TaskCreateDialogProps) {
  const [title, setTitle] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async () => {
    if (!title.trim()) {
      return
    }
    setSubmitting(true)
    try {
      await onCreate(title.trim(), parentId)
      setTitle('')
      onOpenChange(false)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{parentId ? 'New Subtask' : 'New Task'}</DialogTitle>
        </DialogHeader>
        {/* Title only — backend-go's CreateTaskRequest doesn't accept type/priority/
            description (BUG-TASKV1-001); adding those fields here would be a UX lie. */}
        <Input
          autoFocus
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Task title..."
          onKeyDown={(e) => e.key === 'Enter' && submit()}
          data-testid="new-task-title-input"
        />
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={submit}
            disabled={submitting || !title.trim()}
            data-testid="new-task-submit"
          >
            {submitting ? 'Creating…' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
