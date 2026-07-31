import { useState, useEffect } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useAppStore } from '../../../store'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Badge } from '../../ui/badge'
import { GitBranch } from 'lucide-react'

// Lists branches, current branch indicator, create/checkout/delete actions

export function BranchManager() {
  const { project } = useWorkspace()
  const branches    = useAppStore(s => s.branches)
  const [newBranch, setNewBranch] = useState('')

  useEffect(() => {
    if (!project) return
    callRuntimeRpc('git.listBranches', { projectId: project.id })
      .then(bs => useAppStore.getState().setBranches(bs as any[]))
  }, [project])

  const checkout = async (branch: string) => {
    await callRuntimeRpc('git.checkout', { projectId: project!.id, branch })
    // Refresh branches
    const bs = await callRuntimeRpc('git.listBranches', { projectId: project!.id })
    useAppStore.getState().setBranches(bs as any[])
  }

  const create = async () => {
    if (!newBranch.trim()) return
    await callRuntimeRpc('git.createBranch', { projectId: project!.id, name: newBranch.trim() })
    setNewBranch('')
    checkout(newBranch.trim())
  }

  return (
    <div className="branch-manager p-2 space-y-3" data-testid="branch-manager">
      {/* Create branch */}
      <div className="flex gap-2">
        <Input value={newBranch} onChange={e => setNewBranch(e.target.value)} placeholder="New branch name..." className="text-sm" />
        <Button size="sm" onClick={create} disabled={!newBranch.trim()}>Create</Button>
      </div>

      {/* Branch list */}
      <div className="space-y-0.5">
        {branches.map(b => (
          <div
            key={b.name}
            className={`flex items-center gap-2 px-2 py-1.5 rounded text-sm ${b.isCurrent ? 'bg-accent' : 'hover:bg-accent/50'}`}
            data-testid={`branch-${b.name}`}
          >
            <GitBranch size={12} className="text-muted-foreground shrink-0" />
            <span className="flex-1 truncate">{b.name}</span>
            {b.isCurrent && <Badge className="text-xs">current</Badge>}
            {!b.isCurrent && (
              <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={() => checkout(b.name)}>
                Checkout
              </Button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
