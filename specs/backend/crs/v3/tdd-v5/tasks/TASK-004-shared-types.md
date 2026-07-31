# TASK-004: Shared Type Files

**Phase:** 1 — Foundation  
**Solution ref:** [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md) §2, [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §2, [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §3  
**Prerequisite:** None  
**Status:** ✅ DONE — 2026-07-28

> **Kết quả:** 3 shared type files tạo thành công. Zero TypeScript errors.


---

## Mô tả

Tạo 3 shared type files trong `src/shared/`. Đây là pure types, không có logic — tạo trước các service files để TypeScript resolve imports đúng.

---

## 1. `src/shared/project-types.ts`

```typescript
import type { PersistedDevServer } from './dev-server-types'

export interface OrcaProject {
  id: string
  name: string
  description?: string
  devServerId: string
  repoPath: string
  defaultBranch: string
  visibility: 'private' | 'team' | 'company'
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

export interface ProjectMember {
  projectId: string
  userId: string
  role: 'owner' | 'member' | 'viewer'
  addedAt: Date
}

export interface ProjectContext {
  project: OrcaProject
  member: ProjectMember
  devServer: PersistedDevServer
  resolvedProfile: import('../main/profile/OrcaProfile').ResolvedProfile
}
```

---

## 2. `src/shared/ai-provider-types.ts`

```typescript
export type AIProviderType = 'anthropic' | 'openai' | 'gemini' | 'azure' | 'bedrock' | 'ollama' | 'vllm'
export type AIProviderScope = 'server' | 'project' | 'user'
export type AIProviderStatus = 'pending' | 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'

export interface AIProviderAccount {
  id: string
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeRefId?: string
  label: string
  model?: string
  baseUrl?: string
  status: AIProviderStatus
  lastHealthCheck?: Date
  quotaLimitDay: number
  quotaUsedToday?: number
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

export interface CredentialWriteRequest {
  accountId: string
  encryptedBlob: string
  iv: string
}

export const PROVIDER_ENV_KEYS: Record<AIProviderType, string[]> = {
  anthropic: ['ANTHROPIC_API_KEY'],
  openai:    ['OPENAI_API_KEY'],
  gemini:    ['GEMINI_API_KEY', 'GOOGLE_API_KEY'],
  azure:     ['AZURE_OPENAI_API_KEY', 'AZURE_OPENAI_ENDPOINT'],
  bedrock:   ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_DEFAULT_REGION'],
  ollama:    ['OLLAMA_BASE_URL'],
  vllm:      ['VLLM_BASE_URL', 'VLLM_API_KEY'],
}
```

---

## 3. `src/shared/task-types.ts`

```typescript
export type TaskType = 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'
export type TaskStatus = 'backlog' | 'todo' | 'in_progress' | 'review' | 'done' | 'blocked' | 'cancelled'
export type TaskPriority = 'critical' | 'high' | 'medium' | 'low'
export type TaskVisibility = 'private' | 'team' | 'company'
export type TaskPermission = 'view' | 'comment' | 'edit' | 'execute' | 'manage'

export interface OrcaTask {
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
  progressPercent: number
  aiContext?: string
  promptTemplate?: string
  dueDate?: Date
  createdAt: Date
  updatedAt: Date
}

export interface TaskGrant {
  id: string
  taskId: string
  scope: 'user' | 'team' | 'role' | 'everyone'
  scopeId?: string
  permission: TaskPermission
  applyTree: boolean
  grantedBy: string
  expiresAt?: Date
  createdAt: Date
}

export interface TaskComment {
  id: number
  taskId: string
  userId: string
  content: string
  type: 'comment' | 'activity'
  createdAt: Date
}

export interface TaskEdge {
  fromTaskId: string
  toTaskId: string
  edgeType: 'depends_on' | 'blocks' | 'relates_to' | 'duplicates'
}
```

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] `src/shared/project-types.ts` tạo thành công
- [x] `src/shared/ai-provider-types.ts` tạo thành công
- [x] `src/shared/task-types.ts` tạo thành công
- [x] Không TypeScript errors
- [x] `PROVIDER_ENV_KEYS` đúng 7 providers
