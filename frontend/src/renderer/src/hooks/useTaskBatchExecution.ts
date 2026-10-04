import { useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useWorkspace } from '../context/WorkspaceContext'
import { Tracers } from '../../../shared/trace/tracers'
import { withPhase } from '../lib/task-spec-build-loop'
import type { OrcaTask } from '../../../shared/task-types'

// BUG-027 bite 2's own lesson, reused here rather than re-derived: a user
// firing many task.execute dispatches at once against the same dev server
// connection can crash it. 3 is the pre-existing, already-tested value this
// hook shipped with before BL-TG-06 — not re-tuned here (no new experimental
// data since), just kept.
const MAX_CONCURRENCY = 3

export type TaskBatchDispatchOptions = {
  // promptBuilder/phaseLabel (BL-TG-06): when set, each task's prompt is
  // built individually (BL-TG-05's buildSpecPrompt/buildImplementPrompt need
  // per-task title/description/taskNumber, not a single shared string), and
  // phaseLabel is written via task.update BEFORE task.execute for that task
  // — same ordering TaskPromptEditor.tsx's single-task runWithAgent already
  // establishes (labels must land before dispatch, not after). Omitted
  // entirely (undefined) preserves this hook's original behavior: a plain
  // task.execute with no explicit prompt (SimpleExecutor's own default) and
  // no label write — the already-shipped "just run these tasks" action,
  // unrelated to the spec/approve/build loop.
  promptBuilder?: (task: OrcaTask) => string
  phaseLabel?: string | null
}

export function useTaskBatchExecution() {
  const [running, setRunning] = useState(false)
  const [results, setResults] = useState<Map<string, 'ok' | 'error'>>(new Map())
  const { project } = useWorkspace()

  // BUG-025's own lesson, reused here: task.execute's real handler
  // (channels_automation_task.go) never reads worktreePath at all —
  // SimpleExecutor resolves the task's own worktree server-side. This hook
  // used to require a sidebar-selected currentWorktree and send its path,
  // which is dead-on-arrival AND (found live, BL-TG-06) silently no-op'd
  // every batch run whenever the Tasks tab had no worktree selected in the
  // sidebar — a real, common state that tab has no worktree requirement of
  // its own. Only `project` (for projectId) is actually needed.
  const runSelected = async (
    tasks: OrcaTask[],
    options?: TaskBatchDispatchOptions
  ): Promise<Map<string, 'ok' | 'error'>> => {
    if (!project) {
      return new Map()
    }
    setRunning(true)
    setResults(new Map())
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const queue = [...tasks]
    const runResults = new Map<string, 'ok' | 'error'>()

    const runOne = async (task: OrcaTask) => {
      const span = Tracers.uiTaskGraphExecuteFlow.start({
        taskId: task.id,
        entryPoint: 'batch-run'
      })
      try {
        // Label write BEFORE dispatch — same ordering constraint
        // TaskPromptEditor.tsx's single-task flow already established
        // (BL-TG-05): a phase label staged after the agent already started
        // would race the agent's own run.
        if (options?.phaseLabel !== undefined) {
          await callRuntimeRpc(target, 'task.update', {
            id: task.id,
            labels: withPhase(task.labels, options.phaseLabel)
          })
        }
        await callRuntimeRpc(target, 'task.execute', {
          taskId: task.id,
          projectId: project.id,
          traceId: span.id,
          prompt: options?.promptBuilder?.(task)
        })
        span.ok({ taskId: task.id })
        runResults.set(task.id, 'ok')
        setResults((prev) => new Map(prev).set(task.id, 'ok'))
      } catch (err) {
        span.fail(err, { taskId: task.id })
        runResults.set(task.id, 'error')
        setResults((prev) => new Map(prev).set(task.id, 'error'))
      }
    }
    // Simple bounded pool, no extra library.
    const workers = Array.from({ length: MAX_CONCURRENCY }, async () => {
      while (queue.length > 0) {
        const task = queue.shift()
        if (task) {
          await runOne(task)
        }
      }
    })
    await Promise.all(workers)
    setRunning(false)
    return runResults
  }

  return { running, results, runSelected }
}
