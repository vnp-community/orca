import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../ui/dialog'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import type { MergeStrategy } from '../../hooks/useTaskBatchMerge'

type TaskMergeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  taskCount: number
  merging: boolean
  onMerge: (baseBranch: string, strategy: MergeStrategy) => Promise<void>
}

// BL-TG-06 §D: minimal params worktree.merge actually needs (baseBranch +
// strategy) — no diff preview here, the Git tab (already fixed, BUG-021)
// is where a lead reviews a task's changes before merging, same as the
// single-task approve flow (BL-TG-05) already relies on that tab rather
// than duplicating a diff viewer.
export function TaskMergeDialog({
  open,
  onOpenChange,
  taskCount,
  merging,
  onMerge
}: TaskMergeDialogProps) {
  const [baseBranch, setBaseBranch] = useState('main')
  const [strategy, setStrategy] = useState<MergeStrategy>('merge')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            Merge {taskCount} worktree{taskCount === 1 ? '' : 's'}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground" htmlFor="merge-base-branch">
              Base branch
            </label>
            <Input
              id="merge-base-branch"
              value={baseBranch}
              onChange={(e) => setBaseBranch(e.target.value)}
              placeholder="main"
              data-testid="merge-base-branch-input"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground" htmlFor="merge-strategy">
              Strategy
            </label>
            <select
              id="merge-strategy"
              value={strategy}
              onChange={(e) => setStrategy(e.target.value as MergeStrategy)}
              className="w-full text-sm border rounded px-2 py-1"
              data-testid="merge-strategy-select"
            >
              <option value="merge">merge</option>
              <option value="squash">squash</option>
              <option value="rebase">rebase</option>
            </select>
          </div>
          <p className="text-xs text-muted-foreground">
            Merges run one at a time and stop at the first conflict or failure — a task&apos;s own
            worktree is removed automatically once its merge succeeds.
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={merging}>
            Cancel
          </Button>
          <Button
            onClick={() => void onMerge(baseBranch.trim() || 'main', strategy)}
            disabled={merging || !baseBranch.trim()}
            data-testid="merge-submit"
          >
            {merging ? 'Merging…' : `Merge into ${baseBranch.trim() || 'main'}`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
