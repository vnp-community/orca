import type { OrcaTask } from '../../../../shared/task-types'

export type TaskDependencyProgress = {
  total: number
  done: OrcaTask[]
  pending: OrcaTask[]
}

// Why: "Blocked by" is only true for dependencies that are not finished; a status the gateway did not
// send counts as unfinished so the label never claims "all done" without evidence.
export function summarizeTaskDependencies(deps: readonly OrcaTask[]): TaskDependencyProgress {
  const done = deps.filter((d) => d.status === 'done')
  const pending = deps.filter((d) => d.status !== 'done')
  return { total: deps.length, done, pending }
}
