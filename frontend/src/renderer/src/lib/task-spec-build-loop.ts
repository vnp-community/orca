// BL-TG-05: Task -> Agent-generated spec -> human approval -> Agent-generated
// code -> Task, closed loop. Phase state lives entirely in Task.Labels (no
// schema change) — see docs/logic/task-graph/BL-TG-05-spec-approve-build-loop.md
// for the full design and why each phase label is set where it is.
import type { OrcaTask } from '../../../shared/task-types'

export type SpecBuildPhase =
  | 'not-started'
  | 'spec-pending'
  | 'spec-approved'
  | 'code-pending'
  | 'done'

export const PHASE_LABEL_SPEC_PENDING = 'phase:spec-pending'
export const PHASE_LABEL_SPEC_APPROVED = 'phase:spec-approved'
export const PHASE_LABEL_CODE_PENDING = 'phase:code-pending'

const PHASE_LABELS = [PHASE_LABEL_SPEC_PENDING, PHASE_LABEL_SPEC_APPROVED, PHASE_LABEL_CODE_PENDING]

export function derivePhase(task: Pick<OrcaTask, 'status' | 'labels'>): SpecBuildPhase {
  if (task.labels.includes(PHASE_LABEL_CODE_PENDING)) {
    return 'code-pending'
  }
  if (task.labels.includes(PHASE_LABEL_SPEC_APPROVED)) {
    return 'spec-approved'
  }
  if (task.labels.includes(PHASE_LABEL_SPEC_PENDING)) {
    return 'spec-pending'
  }
  if (task.status === 'done') {
    return 'done'
  }
  return 'not-started'
}

/**
 * Replaces whichever phase:* label is currently set (if any) with `phase`,
 * or clears it entirely when `phase` is null — never touches any other
 * label the user set independently.
 */
export function withPhase(currentLabels: string[], phase: string | null): string[] {
  const kept = currentLabels.filter((l) => !PHASE_LABELS.includes(l))
  return phase ? [...kept, phase] : kept
}

function specFilePath(task: Pick<OrcaTask, 'taskNumber'>): string {
  return `specs/generated/TASK-${task.taskNumber ?? 0}-spec.md`
}

export function buildSpecPrompt(
  task: Pick<OrcaTask, 'taskNumber' | 'title' | 'description'>
): string {
  const path = specFilePath(task)
  return `Write a detailed technical spec for this task. Do NOT write implementation code yet.

Requirements:
1. Read the existing code in this worktree relevant to the task below before writing the spec.
2. Save the spec as a new Markdown file at exactly this path: ${path}
3. The spec must cover: problem statement, proposed approach, affected files, edge cases, and a test plan.
4. Do not modify any other file in this worktree in this step.
5. Commit the new spec file with message: "docs(task-${task.taskNumber ?? 0}): add spec for ${task.title}"

Task: ${task.title}
${task.description ?? ''}`
}

export function buildImplementPrompt(task: Pick<OrcaTask, 'taskNumber' | 'title'>): string {
  const path = specFilePath(task)
  return `Implement exactly the spec at ${path} in this worktree.

Requirements:
1. Read that spec file first — it is the source of truth for this change.
2. Implement what it describes. If you must deviate, say so explicitly in a code comment, don't implement silently differently.
3. Add or update tests per the spec's test plan.
4. Commit your changes with message: "feat(task-${task.taskNumber ?? 0}): ${task.title}"

Task: ${task.title}`
}
