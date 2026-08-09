export type StepNumber = 0 | 1 | 2 | 3 | 4 | 5
export type StepId =
  | 'dev_server'
  | 'agent'
  | 'theme'
  | 'integrations'
  | 'windows_terminal'
  | 'notifications'

export const STEPS: readonly {
  id: StepId
  stepNumber: StepNumber
  valueKind: StepId
}[] = [
  { id: 'dev_server', stepNumber: 0, valueKind: 'dev_server' },
  { id: 'agent', stepNumber: 1, valueKind: 'agent' },
  { id: 'theme', stepNumber: 2, valueKind: 'theme' },
  { id: 'integrations', stepNumber: 3, valueKind: 'integrations' },
  { id: 'windows_terminal', stepNumber: 4, valueKind: 'windows_terminal' },
  { id: 'notifications', stepNumber: 5, valueKind: 'notifications' }
]

