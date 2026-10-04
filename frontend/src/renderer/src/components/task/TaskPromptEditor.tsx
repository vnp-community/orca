import { useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { useTask } from '../../hooks/useTask'
import { useTaskComments } from '../../hooks/useTaskComments'
import { Button } from '../ui/button'
import { Loader2 } from 'lucide-react'
import { GenerateAgentPromptButton } from './GenerateAgentPromptButton'
import { Tracers } from '../../../../shared/trace/tracers'
import type { OrcaTask } from '../../../../shared/task-types'
import {
  derivePhase,
  withPhase,
  buildSpecPrompt,
  buildImplementPrompt,
  PHASE_LABEL_SPEC_PENDING,
  PHASE_LABEL_CODE_PENDING
} from '../../lib/task-spec-build-loop'

// BL-TG-05: the Generate Spec / Implement Spec presets below only fill this
// editor's existing textarea + set a pending phase label consumed by the
// existing runWithAgent() flow — they never auto-run and never bypass the
// user's own review of the prompt before it's sent. See
// docs/logic/task-graph/BL-TG-05-spec-approve-build-loop.md for the full
// design this implements.
export function TaskPromptEditor({ task }: { task: OrcaTask }) {
  const [prompt, setPrompt] = useState(task.promptTemplate ?? '')
  const [isRunning, setIsRunning] = useState(false)
  const [pendingPhaseLabel, setPendingPhaseLabel] = useState<string | null>(null)
  const [isActing, setIsActing] = useState(false)
  const [reviewComment, setReviewComment] = useState('')
  const { project } = useWorkspace()
  const { updateTask } = useTask(task.id)
  const { addComment } = useTaskComments(task.id)

  const phase = derivePhase(task)
  const isReview = task.status === 'review'
  // BUG-027 follow-up: task.execute now dispatches async (returns before
  // the agent finishes), so local `isRunning` (reset in runWithAgent's
  // `finally`, right after that now-fast RPC resolves) is no longer a
  // reliable "is the agent still working" signal on its own — it flips
  // back to false in milliseconds while the actual dispatch keeps running
  // in the background. `task` here is expected to be the caller's
  // freshest known copy (TaskDetail passes its polled task once one
  // exists), so task.status === 'in_progress' covers the gap once polling
  // catches up; isRunning alone still covers the instant right after a
  // click, before that.
  const isTaskRunning = isRunning || task.status === 'in_progress'

  const runWithAgent = async () => {
    setIsRunning(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.uiTaskGraphExecuteFlow.start({
      taskId: task.id,
      entryPoint: 'prompt-editor',
      promptLength: prompt.length
    })
    try {
      // BL-TG-05: a preset (Generate Spec/Implement Spec) sets a pending
      // phase label that must land BEFORE the agent starts, not after — a
      // plain custom-prompt run (no preset clicked) leaves labels untouched.
      if (pendingPhaseLabel) {
        await updateTask({ labels: withPhase(task.labels, pendingPhaseLabel) })
        setPendingPhaseLabel(null)
      }
      // BACKLOG-016: task.execute now accepts `prompt` — an empty value
      // (user never edited the textarea) falls back to the executor's own
      // default (SimpleExecutor.buildExecutePrompt, built from the task's
      // title), same behavior as before this field existed.
      //
      // Why no worktreePath: task.execute's real handler never reads it —
      // SimpleExecutor resolves the task's own worktree server-side. Sending
      // `currentWorktree!.path` crashed whenever no worktree was selected in
      // the sidebar (unrelated to this task, and not required on the Tasks
      // tab) — found live.
      await callRuntimeRpc(target, 'task.execute', {
        taskId: task.id,
        projectId: project!.id,
        traceId: span.id,
        prompt: prompt.trim() || undefined
      })
      span.ok({ taskId: task.id })
    } catch (err) {
      span.fail(err, { taskId: task.id })
      throw err
    } finally {
      setIsRunning(false)
    }
  }

  const handleGenerateSpec = () => {
    setPrompt(buildSpecPrompt(task))
    setPendingPhaseLabel(PHASE_LABEL_SPEC_PENDING)
  }

  const handleImplementSpec = () => {
    setPrompt(buildImplementPrompt(task))
    setPendingPhaseLabel(PHASE_LABEL_CODE_PENDING)
  }

  const handleApproveSpec = async () => {
    setIsActing(true)
    try {
      await updateTask({ labels: withPhase(task.labels, 'phase:spec-approved') })
    } finally {
      setIsActing(false)
    }
  }

  const handleApproveAndMarkDone = async () => {
    setIsActing(true)
    try {
      await updateTask({ status: 'done', labels: withPhase(task.labels, null) })
    } finally {
      setIsActing(false)
    }
  }

  const handleRequestChanges = async () => {
    if (!reviewComment.trim()) {
      return
    }
    setIsActing(true)
    try {
      await addComment(reviewComment.trim())
      setReviewComment('')
    } finally {
      setIsActing(false)
    }
  }

  return (
    <div className="task-prompt-editor space-y-3 mt-3" data-testid="task-prompt-editor">
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={handleGenerateSpec}
          disabled={isTaskRunning}
          data-testid="generate-spec-btn"
        >
          📝 {phase === 'not-started' ? 'Generate Spec' : 'Regenerate Spec'}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={handleImplementSpec}
          disabled={isTaskRunning || (phase !== 'spec-approved' && phase !== 'code-pending')}
          data-testid="implement-spec-btn"
        >
          🔨 {phase === 'code-pending' ? 'Re-run Implement Spec' : 'Implement Spec'}
        </Button>
      </div>

      <textarea
        className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        placeholder="Describe what the agent should do for this task..."
        rows={4}
      />
      <div className="flex items-center gap-2">
        <GenerateAgentPromptButton
          taskId={task.id}
          currentPrompt={prompt}
          onGenerated={setPrompt}
        />
        <Button onClick={runWithAgent} disabled={isTaskRunning} data-testid="run-agent-btn">
          {isTaskRunning ? (
            <>
              <Loader2 size={12} className="animate-spin mr-1" />
              Running...
            </>
          ) : (
            '▶ Run with Agent'
          )}
        </Button>
      </div>

      {isReview && phase === 'spec-pending' && (
        <div
          className="rounded-md border border-input p-3 space-y-2"
          data-testid="spec-review-panel"
        >
          <p className="text-xs text-muted-foreground">
            Agent finished generating a spec. Review it in the Git tab of this task&apos;s worktree,
            then approve or request changes.
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={handleApproveSpec}
              disabled={isActing}
              data-testid="approve-spec-btn"
            >
              ✅ Approve Spec
            </Button>
          </div>
          <textarea
            className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={reviewComment}
            onChange={(e) => setReviewComment(e.target.value)}
            placeholder="What needs to change?"
            rows={2}
          />
          <Button
            size="sm"
            variant="destructive"
            onClick={handleRequestChanges}
            disabled={isActing || !reviewComment.trim()}
            data-testid="request-changes-btn"
          >
            Request Changes
          </Button>
        </div>
      )}

      {isReview && phase === 'code-pending' && (
        <div
          className="rounded-md border border-input p-3 space-y-2"
          data-testid="code-review-panel"
        >
          <p className="text-xs text-muted-foreground">
            Agent finished implementing the spec. Review the diff in the Git tab of this task&apos;s
            worktree, then approve or request changes.
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={handleApproveAndMarkDone}
              disabled={isActing}
              data-testid="approve-code-btn"
            >
              ✅ Approve &amp; Mark Done
            </Button>
          </div>
          <textarea
            className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={reviewComment}
            onChange={(e) => setReviewComment(e.target.value)}
            placeholder="What needs to change?"
            rows={2}
          />
          <Button
            size="sm"
            variant="destructive"
            onClick={handleRequestChanges}
            disabled={isActing || !reviewComment.trim()}
            data-testid="request-changes-code-btn"
          >
            Request Changes
          </Button>
        </div>
      )}
    </div>
  )
}
