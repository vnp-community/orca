// Shared types cho Workflow (TDD-FE-14)

export type WorkflowStepType = 'agent' | 'shell' | 'notify' | 'approval'
export type WorkflowScope    = 'personal' | 'project' | 'company'

export type AgentStepConfig = {
  type:         'agent'
  prompt:       string
  model?:       string
  worktreePath: string
}

export type ShellStepConfig = {
  type:    'shell'
  command: string
  args?:   string[]
  cwd?:    string
}

export type NotifyStepConfig = {
  type:    'notify'
  message: string
  channel: 'slack' | 'email' | 'webhook'
  target:  string
}

export type WorkflowStep = {
  id:              string
  type:            WorkflowStepType
  name:            string
  serverSpec:      string
  config:          AgentStepConfig | ShellStepConfig | NotifyStepConfig
  dependsOn:       string[]
  continueOnError: boolean
  timeout:         number
}

export type WorkflowDefinition = {
  id:          string
  name:        string
  templateId?: string
  scope:       WorkflowScope
  scopeRefId?: string
  steps:       WorkflowStep[]
}

export type WorkflowExecutionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
export type StepStatus             = 'pending' | 'running' | 'completed' | 'failed' | 'skipped'

export type WorkflowExecution = {
  id:          string
  templateId:  string
  status:      WorkflowExecutionStatus
  startedAt:   number
  endedAt?:    number
  triggeredBy: string
  definition:  WorkflowDefinition
}
