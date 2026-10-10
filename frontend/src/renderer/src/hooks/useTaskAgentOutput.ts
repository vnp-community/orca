import { useEffect, useMemo, useState } from 'react'
import {
  parseAgentOutput,
  type AgentActivityLine
} from '../components/task/agent-activity/agent-activity-lines'
import {
  watchTaskAgentOutput,
  type AgentOutputSnapshot,
  type AgentOutputStatus
} from './task-agent-output-store'

// Live output of the agent run behind a task, from `task.activity.subscribe` (see task-agent-output-store.ts).
// The backend keeps no history: a viewer that opens mid-run sees everything from the next frame on (frames
// are cumulative, at most one per 2 s), and nothing for a run that ended before this page loaded.

export type { AgentOutputStatus }

export type TaskAgentOutputState = {
  lines: AgentActivityLine[]
  olderHidden: boolean
  /** request_id of the run the lines belong to (a new run replaces the previous one). */
  runId: string | null
  status: AgentOutputStatus
  lastFrameAt: number | null
}

const IDLE_SNAPSHOT: AgentOutputSnapshot = {
  stdout: '',
  stderr: '',
  runId: null,
  receivedAt: null,
  status: 'connecting'
}

export function useTaskAgentOutput(taskId: string | null | undefined): TaskAgentOutputState {
  const [snapshot, setSnapshot] = useState<AgentOutputSnapshot>(IDLE_SNAPSHOT)

  useEffect(() => {
    if (!taskId) {
      setSnapshot(IDLE_SNAPSHOT)
      return
    }
    const handle = watchTaskAgentOutput(taskId, () => setSnapshot(handle.snapshot()))
    setSnapshot(handle.snapshot())
    return handle.release
  }, [taskId])

  return useMemo(() => {
    const parsed = parseAgentOutput(snapshot.stdout, snapshot.stderr)
    return {
      lines: parsed.lines,
      olderHidden: parsed.olderHidden,
      runId: snapshot.runId,
      status: snapshot.status,
      lastFrameAt: snapshot.receivedAt
    }
  }, [snapshot])
}
