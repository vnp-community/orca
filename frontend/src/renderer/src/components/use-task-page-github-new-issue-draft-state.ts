import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import {
  isNewIssueDraftContentful,
  resolveVanishedNewIssueRepoReset
} from '@/components/task-page-new-issue-draft'
import type { GitHubAssignableUser, Repo } from '../../../shared/types'

// Why: split out of use-task-page-github-state.ts (TASK-BIGFILE-236) to stay
// under the 300-line file budget — this is the GitHub "new issue" draft
// slice (form fields + session-recovery persistence), used only while the
// new-issue dialog is open.
type UseTaskPageGitHubNewIssueDraftStateParams = {
  selectedRepos: readonly Repo[]
}

export function useTaskPageGitHubNewIssueDraftState({
  selectedRepos
}: UseTaskPageGitHubNewIssueDraftStateParams) {
  // Why: session-only draft slice backs recovery of an in-progress issue after
  // an accidental dismissal (outside click/Escape/Cancel) and across a Tasks
  // view unmount. Component `useState` stays the inputs' immediate source; the
  // store is the durable-across-remount backing. See task-page-new-issue-draft.
  const setNewIssueDraft = useAppStore((s) => s.setNewIssueDraft)
  const clearNewIssueDraft = useAppStore((s) => s.clearNewIssueDraft)

  const [newIssueOpen, setNewIssueOpen] = useState(false)
  const [newIssueTitle, setNewIssueTitle] = useState('')
  const [newIssueBody, setNewIssueBody] = useState('')
  const [newIssueLabels, setNewIssueLabels] = useState<string[]>([])
  const [newIssueAssignees, setNewIssueAssignees] = useState<GitHubAssignableUser[]>([])
  const [newIssueSubmitting, setNewIssueSubmitting] = useState(false)
  const [newIssueRepoId, setNewIssueRepoId] = useState<string | null>(null)

  // Why: repo-scoped labels/assignees can't cross repos. A reactive clear keyed
  // on the derived target id can't tell a restore apart from a user switch, so
  // it would wipe just-restored fields and corrupt the recovery draft via the
  // write-through below. Decompose by cause instead: this guard only handles the
  // "chosen repo vanished from the selection" case (removed/deselected). A
  // genuine user switch clears imperatively in the repo Select's handler; a
  // restore always seeds an in-selection repoId, so neither path fires here.
  useEffect(() => {
    const reset = resolveVanishedNewIssueRepoReset(
      newIssueRepoId,
      selectedRepos.map((r) => r.id)
    )
    if (!reset) {
      return
    }
    setNewIssueLabels([])
    setNewIssueAssignees([])
    setNewIssueRepoId(reset.repoId)
  }, [newIssueRepoId, selectedRepos])

  // Why: mirror the live fields into the session draft while the modal is open
  // so an accidental dismissal doesn't lose input. Content-gate the write so an
  // untouched open never pins a meaningless draft (repoId alone is not content),
  // and clear any stale draft once the form is emptied back out.
  useEffect(() => {
    if (!newIssueOpen) {
      return
    }
    if (
      isNewIssueDraftContentful({
        title: newIssueTitle,
        body: newIssueBody,
        labels: newIssueLabels,
        assignees: newIssueAssignees
      })
    ) {
      setNewIssueDraft({
        title: newIssueTitle,
        body: newIssueBody,
        labels: newIssueLabels,
        assignees: newIssueAssignees,
        repoId: newIssueRepoId
      })
    } else {
      clearNewIssueDraft()
    }
  }, [
    newIssueOpen,
    newIssueTitle,
    newIssueBody,
    newIssueLabels,
    newIssueAssignees,
    newIssueRepoId,
    setNewIssueDraft,
    clearNewIssueDraft
  ])

  return {
    newIssueOpen,
    setNewIssueOpen,
    newIssueTitle,
    setNewIssueTitle,
    newIssueBody,
    setNewIssueBody,
    newIssueLabels,
    setNewIssueLabels,
    newIssueAssignees,
    setNewIssueAssignees,
    newIssueSubmitting,
    setNewIssueSubmitting,
    newIssueRepoId,
    setNewIssueRepoId
  }
}
