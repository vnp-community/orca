import { useEffect } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useAppStore } from '../../../store'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import type { GitCommit } from '../../../store/slices/git-panel'

export function GitHistory() {
  const { project } = useWorkspace()
  const gitHistory  = useAppStore(s => s.gitHistory)

  useEffect(() => {
    if (!project) return
    callRuntimeRpc('git.getLog', { projectId: project.id, limit: 50 })
      .then(commits => {
        useAppStore.getState().setGitHistory(commits as GitCommit[])
      })
  }, [project])

  return (
    <div className="git-history p-2" data-testid="git-history">
      {gitHistory.map(commit => (
        <div key={commit.hash} className="py-2 border-b last:border-0">
          <div className="flex items-baseline gap-2">
            <code className="text-xs text-muted-foreground font-mono">{commit.shortHash}</code>
            <span className="text-sm truncate">{commit.message}</span>
          </div>
          <div className="text-xs text-muted-foreground mt-0.5">
            {commit.author} · {new Date(commit.date).toLocaleDateString()}
          </div>
        </div>
      ))}
      {gitHistory.length === 0 && (
        <div className="text-sm text-muted-foreground py-4 text-center">No commits yet</div>
      )}
    </div>
  )
}
