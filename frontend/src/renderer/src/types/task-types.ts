// task-types.ts — Shared types for Task Graph (TDD-FE-15)

export type TaskType     = 'epic' | 'story' | 'task' | 'bug' | 'chore'
export type TaskStatus   = 'todo' | 'in_progress' | 'done' | 'cancelled'
export type TaskPriority = 'critical' | 'high' | 'medium' | 'low'

export type OrcaTask = {
  id:           string
  projectId:    string
  parentId:     string | null
  type:         TaskType
  title:        string
  description?: string
  status:       TaskStatus
  priority:     TaskPriority
  assigneeId?:  string
  dependsOn:    string[]
  agentPrompt?: string
  progress:     number        // 0–100
  createdAt:    number
  updatedAt:    number
}
