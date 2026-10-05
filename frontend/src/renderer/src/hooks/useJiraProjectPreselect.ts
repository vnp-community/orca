import { useEffect, useRef } from 'react'
import type { Repo } from '../../../shared/types'
import { getLinkedWorkItemProvider } from '../lib/linked-work-item-provider'
import { matchProjectForJiraIssue } from '../lib/jira-project-matching'
import type { LinkedWorkItemSummary } from '../lib/new-workspace'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'
import type { OrcaProject } from '../types/workspace-types'

// Why: opening the composer from a Jira issue should land on the Orca project
// mapped to the issue's key prefix; best-effort and one-shot so the user's later
// manual project choice is never overridden.
export function useJiraProjectPreselect({
  linkedWorkItem,
  skip,
  eligibleRepos,
  setRepoId
}: {
  linkedWorkItem: LinkedWorkItemSummary | null | undefined
  skip: boolean
  eligibleRepos: readonly Pick<Repo, 'id' | 'projectId'>[]
  setRepoId: (id: string) => void
}): void {
  const doneRef = useRef(false)
  const reposRef = useRef(eligibleRepos)
  reposRef.current = eligibleRepos
  const setRepoIdRef = useRef(setRepoId)
  setRepoIdRef.current = setRepoId
  const key = linkedWorkItem?.jiraIdentifier
  const siteId = linkedWorkItem?.jiraSiteId
  const isJira = linkedWorkItem ? getLinkedWorkItemProvider(linkedWorkItem) === 'jira' : false

  useEffect(() => {
    if (doneRef.current || skip || !isJira || !key) {
      return
    }
    doneRef.current = true
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    void callRuntimeRpc<OrcaProject[]>(target, 'project.list', {})
      .then((list) => {
        if (!Array.isArray(list)) {
          return
        }
        const project = matchProjectForJiraIssue(list, key, siteId)
        const repo = project ? reposRef.current.find((r) => r.projectId === project.id) : undefined
        if (repo) {
          setRepoIdRef.current(repo.id)
        }
      })
      .catch(() => {})
  }, [isJira, key, siteId, skip])
}
