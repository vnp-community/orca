import type { WorkflowDefinition, WorkflowExecution, WorkflowExecutionStatus, StepStatus } from '@shared/workflow-types'

export type WorkflowSlice = {
  templates:       WorkflowDefinition[]
  executions:      WorkflowExecution[]
  stepStatuses:    Record<string, Record<string, StepStatus>>  // execId → stepId → status
  streamingOutput: Record<string, string[]>                    // execId → lines[]
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

export function createWorkflowSlice(set): WorkflowSlice {
  return {
    templates:       [],
    executions:      [],
    stepStatuses:    {},
    streamingOutput: {},
    workflowLoading: false,

    setTemplates: (t)   => set(s => { s.templates = t }),
    addTemplate:  (t)   => set(s => { s.templates.push(t) }),
    updateTemplate: (id, patch) => set(s => {
      const idx = s.templates.findIndex((t: WorkflowDefinition) => t.id === id)
      if (idx !== -1) Object.assign(s.templates[idx], patch)
    }),
    removeTemplate: (id) => set(s => { s.templates = s.templates.filter((t: WorkflowDefinition) => t.id !== id) }),
    addExecution:  (e)   => set(s => { s.executions.push(e) }),
    updateExecutionStatus: (execId, status) => set(s => {
      const e = s.executions.find((ex: WorkflowExecution) => ex.id === execId)
      if (e) e.status = status
    }),
    setStepStatus: (execId, stepId, status) => set(s => {
      if (!s.stepStatuses[execId]) s.stepStatuses[execId] = {}
      s.stepStatuses[execId][stepId] = status
    }),
    appendStreamLine: (execId, line) => set(s => {
      if (!s.streamingOutput[execId]) s.streamingOutput[execId] = []
      s.streamingOutput[execId].push(line)
    }),
    clearStreamLines: (execId) => set(s => { s.streamingOutput[execId] = [] }),
    setWorkflowLoading: (v) => set(s => { s.workflowLoading = v }),
  }
}
