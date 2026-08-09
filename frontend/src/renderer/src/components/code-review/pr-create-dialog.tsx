// src/renderer/src/components/code-review/pr-create-dialog.tsx
// BL-CR-05: Dialog wrapper around the existing PullRequestForm
// Triggered from CodeReviewPanel "PR" button to open a PR in GitHub/GitLab

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { GitPullRequest } from 'lucide-react'
import { PullRequestForm } from '../workspace/git/PullRequestForm'

type PrCreateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The project the PR is created against */
  projectId: string
  /** Optional source branch override; defaults to active branch in PullRequestForm */
  sourceBranch?: string
}

export function PrCreateDialog({
  open,
  onOpenChange,
  projectId,
  sourceBranch,
}: PrCreateDialogProps) {
  function handleSuccess(prUrl: string) {
    onOpenChange(false)
    // Open PR in browser
    window.open(prUrl, '_blank', 'noopener,noreferrer')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <GitPullRequest size={16} />
            Create Pull Request
          </DialogTitle>
        </DialogHeader>

        <PullRequestForm
          projectId={projectId}
          sourceBranch={sourceBranch}
          onSuccess={handleSuccess}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  )
}
