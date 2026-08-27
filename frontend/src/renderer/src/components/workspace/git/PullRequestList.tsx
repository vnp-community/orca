// NEW: src/renderer/src/components/workspace/git/PullRequestList.tsx
import { useWorkspace } from '../../../context/WorkspaceContext'
import { GitPullRequest } from 'lucide-react'

// Why: this used to call 'git.pr.list', which has never existed as an RPC
// method (backend/src/main/runtime/rpc/methods/git.ts has no git.pr.* group —
// hosted-review listing is 'hostedReview.forBranch', which needs a `repo`
// selector resolved from the runtime's Repo/connection model. OrcaProject
// (the Project Workspace / F38 multi-user model this panel belongs to) has no
// such selector today, so wiring this up for real is separate, scoped
// backend/design work — not a param/method-name fix like the rest of this
// panel. Left as an honest "not available" state instead of a broken RPC call
// (same crash class as the GitPanel.tsx push bug this investigation started from).
export function PullRequestList() {
  const { project } = useWorkspace()

  if (!project) {
    return (
      <div className="p-3 text-xs text-muted-foreground" data-testid="pr-no-project">
        No project selected
      </div>
    )
  }

  return (
    <div className="pr-list" data-testid="pr-list">
      <div
        className="flex flex-col items-center py-8 gap-2 px-3 text-center"
        data-testid="pr-unavailable"
      >
        <GitPullRequest size={24} className="text-muted-foreground opacity-30" />
        <p className="text-sm text-muted-foreground">
          Pull requests are not available in this workspace yet
        </p>
        <p className="text-xs text-muted-foreground">
          Viewing hosted pull requests from a Project Workspace is not supported yet — this is a
          known gap, not a bug.
        </p>
      </div>
    </div>
  )
}
