import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type {
  WorkflowDefinition,
  WorkflowExecution,
  WorkflowExecutionStatus,
  StepStatus
} from '@shared/workflow-types'

export type WorkflowSlice = {
  templates: WorkflowDefinition[]
  executions: WorkflowExecution[]
  stepStatuses: Record<string, Record<string, StepStatus>> // execId → stepId → status
  streamingOutput: Record<string, string[]> // execId → lines[]
  workflowLoading: boolean

  setTemplates(templates: WorkflowDefinition[]): void
  addTemplate(template: WorkflowDefinition): void
  updateTemplate(id: string, patch: Partial<WorkflowDefinition>): void
  removeTemplate(id: string): void
  addExecution(execution: WorkflowExecution): void
  updateExecutionStatus(execId: string, status: WorkflowExecutionStatus): void
  setStepStatus(execId: string, stepId: string, status: StepStatus): void
  appendStreamLine(execId: string, line: string): void
  clearStreamLines(execId: string): void
  setWorkflowLoading(v: boolean): void
}

// Why every action returns a partial object instead of mutating `s` and
// returning nothing: this store has no immer middleware, so plain zustand's
// `set` treats a non-object return value (i.e. `undefined`, from a bare
// `set(s => { s.templates = t })`) as a full-state REPLACE — wiping the
// entire AppState to `undefined`. Same bug class fixed in task.ts's own
// doc comment (BUG-FE-TASKGRAPH-SETTINGS) — this slice had it too, just
// never live-triggered yet since WorkflowMonitor's fetches don't succeed
// on this deployment.
export const createWorkflowSlice: StateCreator<AppState, [], [], WorkflowSlice> = (set) => ({
  templates: [],
  executions: [],
  stepStatuses: {},
  streamingOutput: {},
  workflowLoading: false,

  setTemplates: (templates) => set(() => ({ templates })),
  addTemplate: (template) => set((s) => ({ templates: [...s.templates, template] })),
  updateTemplate: (id, patch) =>
    set((s) => ({
      templates: s.templates.map((t) => (t.id === id ? { ...t, ...patch } : t))
    })),
  removeTemplate: (id) => set((s) => ({ templates: s.templates.filter((t) => t.id !== id) })),
  addExecution: (execution) => set((s) => ({ executions: [...s.executions, execution] })),
  updateExecutionStatus: (execId, status) =>
    set((s) => ({
      executions: s.executions.map((e) => (e.id === execId ? { ...e, status } : e))
    })),
  setStepStatus: (execId, stepId, status) =>
    set((s) => ({
      stepStatuses: {
        ...s.stepStatuses,
        [execId]: { ...s.stepStatuses[execId], [stepId]: status }
      }
    })),
  appendStreamLine: (execId, line) =>
    set((s) => ({
      streamingOutput: {
        ...s.streamingOutput,
        [execId]: [...(s.streamingOutput[execId] ?? []), line]
      }
    })),
  clearStreamLines: (execId) =>
    set((s) => ({ streamingOutput: { ...s.streamingOutput, [execId]: [] } })),
  setWorkflowLoading: (v) => set(() => ({ workflowLoading: v }))
})
