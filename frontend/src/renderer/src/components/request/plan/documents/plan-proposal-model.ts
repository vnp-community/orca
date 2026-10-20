/**
 * plan-proposal-model.ts — CR-REQ-045
 *
 * The native Plan proposal as the reader sees it: phases and tasks numbered from 1, dependencies as the numbers of the tasks
 * they wait for. The wire carries 0-based indices the AI wrote, so a bad one (out of range, itself, repeated) is dropped here
 * instead of breaking the page. Pure.
 *
 * @module components/request/plan/documents/plan-proposal-model
 */

import { translate } from '@/i18n/i18n'
import type {
  PlanProposalDocument,
  PlanTaskProposal
} from '../../../../../../shared/request-plan-revision'

export type OutlineTask = {
  /** 1-based within its phase (or within the plan when it has no phase). */
  number: number
  title: string
  description: string
  taskType: string
  estimatedHours?: number
  irreversible: boolean
  labels: string[]
  /** Numbers of the tasks of the same list this one waits for, ascending. */
  dependsOn: number[]
  promptTemplate: string
}

export type OutlinePhase = {
  number: number
  title: string
  description: string
  /** Numbers of the phases this one waits for, ascending. */
  dependsOn: number[]
  tasks: OutlineTask[]
}

export type ProposalOutline = {
  title: string
  summary: string
  notes: string
  phases: OutlinePhase[]
  /** Tasks directly under the plan, when it has no phase. */
  tasks: OutlineTask[]
  taskCount: number
}

/** 1-based numbers of the valid targets: in range, not the item itself, each once. */
function dependencyNumbers(indices: readonly number[], size: number, selfIndex: number): number[] {
  const valid = new Set<number>()
  for (const index of indices) {
    if (Number.isInteger(index) && index >= 0 && index < size && index !== selfIndex) {
      valid.add(index + 1)
    }
  }
  return [...valid].sort((a, b) => a - b)
}

function outlineTasks(tasks: readonly PlanTaskProposal[]): OutlineTask[] {
  return tasks.map((task, index) => ({
    number: index + 1,
    title: task.title.trim() === '' ? `#${index + 1}` : task.title.trim(),
    description: task.description,
    taskType: task.taskType,
    ...(task.estimatedHours !== undefined ? { estimatedHours: task.estimatedHours } : {}),
    irreversible: task.irreversible,
    labels: task.labels,
    dependsOn: dependencyNumbers(task.dependsOnIndices, tasks.length, index),
    promptTemplate: task.promptTemplate
  }))
}

export function toProposalOutline(proposal: PlanProposalDocument): ProposalOutline {
  const phases = proposal.phases.map((phase, index) => ({
    number: index + 1,
    title: phase.title.trim() === '' ? `#${index + 1}` : phase.title.trim(),
    description: phase.description,
    dependsOn: dependencyNumbers(phase.dependsOnPhaseIndices, proposal.phases.length, index),
    tasks: outlineTasks(phase.tasks)
  }))
  const tasks = outlineTasks(proposal.tasks)
  return {
    title: proposal.title,
    summary: proposal.summary,
    notes: proposal.notes,
    phases,
    tasks,
    taskCount: tasks.length + phases.reduce((sum, phase) => sum + phase.tasks.length, 0)
  }
}

/** `2 h`, `1.5 h`; a missing or non-positive estimate has no label. */
export function formatEstimatedHours(hours: number | undefined): string | null {
  if (hours === undefined || !Number.isFinite(hours) || hours <= 0) {
    return null
  }
  const text = new Intl.NumberFormat('en', { maximumFractionDigits: 1 }).format(hours)
  return translate('auto.components.request.plan.documents.proposal.hours', '{{hours}} h', {
    hours: text
  })
}

/** `#2, #3`: how a list of task numbers reads in "Depends on …". */
export function formatDependencies(numbers: readonly number[]): string {
  return numbers.map((n) => `#${n}`).join(', ')
}
