import { useState } from 'react'
import { useTask } from '../../hooks/useTask'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { Loader2 } from 'lucide-react'
import type { OrcaTask } from '../../../../shared/task-types'

export function TaskAIDecompose({ parentTask }: { parentTask: OrcaTask }) {
  const { aiDecompose, acceptSubtasks } = useTask(parentTask.id)
  const [instruction, setInstruction] = useState('')
  const [isDecomposing, setIsDecomposing] = useState(false)
  const [proposedSubtasks, setProposedSubtasks] = useState<Partial<OrcaTask>[]>([])

  const decompose = async () => {
    setIsDecomposing(true)
    try {
      const subtasks = await aiDecompose(instruction || undefined)
      setProposedSubtasks(subtasks)
    } finally {
      setIsDecomposing(false)
    }
  }

  const accept = async () => {
    // projectId is optional on OrcaTask in general, but a rendered task always has one.
    await acceptSubtasks(proposedSubtasks, parentTask.projectId!)
    setProposedSubtasks([])
  }

  return (
    <div className="task-ai-decompose space-y-3 mt-3" data-testid="task-ai-decompose">
      <Input
        value={instruction}
        onChange={(e) => setInstruction(e.target.value)}
        placeholder="Optional: decompose instructions..."
      />
      <Button onClick={decompose} disabled={isDecomposing} data-testid="decompose-btn">
        {isDecomposing ? (
          <>
            <Loader2 size={12} className="animate-spin mr-1" /> Decomposing...
          </>
        ) : (
          '🤖 Decompose with AI'
        )}
      </Button>

      {proposedSubtasks.length > 0 && (
        <div className="proposed-subtasks space-y-1" data-testid="proposed-subtasks">
          {proposedSubtasks.map((st, i) => (
            <div key={i} className="flex items-center gap-2 px-2 py-1 rounded bg-muted/50 text-sm">
              <span className="text-xs font-mono text-muted-foreground">{st.type}</span>
              <span>{st.title}</span>
            </div>
          ))}
          <div className="flex gap-2 pt-2">
            <Button size="sm" onClick={accept} data-testid="accept-subtasks-btn">
              Accept All
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setProposedSubtasks([])}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
