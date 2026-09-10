/**
 * Task Types — v5.0 (TDD-18)
 *
 * Shared types for the task graph management system.
 * Tasks form a tree (parent_id) with an additional DAG of dependency edges.
 *
 * @module shared/task-types
 */

/** Task classification */
export type TaskType = 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'

/** Task lifecycle status */
export type TaskStatus =
  | 'backlog'
  | 'todo'
  | 'in_progress'
  | 'review'
  | 'done'
  | 'blocked'
  | 'cancelled'

/** Task priority levels */
export type TaskPriority = 'critical' | 'high' | 'medium' | 'low'

/** Visibility scopes for tasks */
export type TaskVisibility = 'private' | 'team' | 'company'

/**
 * Permission levels for task grants.
 * Hierarchy (highest → lowest): manage > execute > edit > comment > view
 */
export type TaskPermission = 'view' | 'comment' | 'edit' | 'execute' | 'manage'

/** Edge types between tasks (dependency DAG) */
export type TaskEdgeType = 'depends_on' | 'blocks' | 'relates_to' | 'duplicates'

/** A task entity */
export type OrcaTask = {
  id: string
  projectId?: string
  parentId?: string
  title: string
  description?: string
  type: TaskType
  status: TaskStatus
  priority: TaskPriority
  labels: string[]
  visibility: TaskVisibility
  reporterId?: string
  assigneeId?: string
  estimatedHours?: number
  /** 0–100; auto-calculated from children when parent node */
  progressPercent: number
  /** AI context / additional instructions for agent execution */
  aiContext?: string
  /** Template string with ${task.*} interpolation for agent prompt */
  promptTemplate?: string
  /** Set khi user "Attach Workflow Template" (FE-TASK-002). Backend hiện CHƯA
   * lưu/trả field này (CR-FLOW-TASK-002 chưa triển khai) — luôn `undefined` sau
   * khi load lại task cho tới khi backend field tồn tại. */
  workflowTemplateId?: string
  dueDate?: Date
  createdAt: Date
  updatedAt: Date
}

/** Parameters for creating a new task */
export type CreateTaskParams = {
  projectId?: string
  parentId?: string
  title: string
  description?: string
  type?: TaskType
  priority?: TaskPriority
  labels?: string[]
  visibility?: TaskVisibility
  reporterId?: string
  assigneeId?: string
  estimatedHours?: number
  aiContext?: string
  promptTemplate?: string
  dueDate?: Date
}

/** A permission grant on a task (optionally propagates to descendants) */
export type TaskGrant = {
  id: string
  taskId: string
  /** Who the grant applies to */
  scope: 'user' | 'team' | 'role' | 'everyone'
  /** userId / teamId / role name; null when scope='everyone' */
  scopeId?: string
  permission: TaskPermission
  /** If true, grant propagates to all descendants via BFS */
  applyTree: boolean
  grantedBy: string
  expiresAt?: Date
  createdAt: Date
}

/** A comment or activity event on a task */
export type TaskComment = {
  id: number
  taskId: string
  userId: string
  content: string
  type: 'comment' | 'activity'
  createdAt: Date
}

/** A directed dependency edge between two tasks */
export type TaskEdge = {
  fromTaskId: string
  toTaskId: string
  edgeType: TaskEdgeType
  createdAt?: Date
}

/** Ordered permission levels for hierarchy comparison */
export const TASK_PERMISSION_ORDER: Readonly<Record<TaskPermission, number>> = {
  view: 1,
  comment: 2,
  edit: 3,
  execute: 4,
  manage: 5
} as const

/** Progress weights by status (leaf node calculation) */
export const TASK_STATUS_PROGRESS: Readonly<Record<TaskStatus, number>> = {
  backlog: 0,
  todo: 0,
  in_progress: 40,
  review: 80,
  done: 100,
  blocked: 0,
  cancelled: 0
} as const

// Grantee-kind scale thật của backend-go's `GrantLevel` (task.proto:92-99) — KHÁC
// TaskPermission (action scale) ở trên. TaskPermission tài liệu hoá ý định thiết kế
// ban đầu (BL-TG-03) chưa từng được backend-go hiện thực; dùng TaskGrantLevel khi gọi
// task.grant/resolvePermission, KHÔNG dùng TaskPermission cho việc đó (BUG-TASKV1-003).
export type TaskGrantLevel = 'owner' | 'admin' | 'user' | 'team' | 'company'

/** Alias for TaskGrantLevel — matches backend taskv1.GrantLevel (BE-SOL-003). Kept for
 * TaskGrantModal/useTaskGrants call sites that predate the TaskGrantLevel fix above. */
export type RealGrantLevel = TaskGrantLevel
