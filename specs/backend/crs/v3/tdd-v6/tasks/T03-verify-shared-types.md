# T03 — Verify + Fix Shared Types

**Phase:** 1 (Quick Win)  
**Effort:** ~15 min  
**Depends on:** —  
**Solution ref:** [05-tdd18-task-graph.md](../solutions/05-tdd18-task-graph.md), [03-tdd16-ai-provider-management.md](../solutions/03-tdd16-ai-provider-management.md)  
**TDD ref:** TDD-15, TDD-16, TDD-18

---

## Mục tiêu

Xác nhận 3 shared type files tồn tại với đúng types. Tạo mới nếu thiếu.

---

## Files Cần Kiểm Tra

### 1. `src/shared/task-types.ts`

**Kiểm tra:**
```bash
cat src/shared/task-types.ts
```

**Expected types (từ TDD-18 §2 và TaskService.ts usage):**

```typescript
export type TaskType = 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'
export type TaskStatus = 'backlog' | 'todo' | 'in_progress' | 'review' | 'blocked' | 'done'
export type TaskPriority = 'low' | 'medium' | 'high' | 'urgent'
export type TaskVisibility = 'private' | 'team' | 'company'

export interface OrcaTask {
  id: string
  title: string
  description?: string
  type: TaskType
  status: TaskStatus
  priority: TaskPriority
  parentId?: string
  projectId?: string
  assigneeId?: string
  reporterId: string
  aiContext?: string
  promptTemplate?: string
  labels?: string[]
  visibility: TaskVisibility
  progressPercent: number
  agentSessionId?: string
  dueDate?: Date
  estimatedHours?: number
  createdAt: Date
  updatedAt: Date
}

export interface TaskComment {
  id: string
  taskId: string
  userId: string
  content: string
  type: 'comment' | 'activity'
  createdAt: Date
}

export type TaskGrantLevel = 'view' | 'comment' | 'edit' | 'execute' | 'manage'
```

### 2. `src/shared/ai-provider-types.ts`

**Kiểm tra:**
```bash
cat src/shared/ai-provider-types.ts
```

**Expected types (từ TDD-16 §2):**

```typescript
export type AIProviderType =
  | 'anthropic' | 'openai' | 'gemini'
  | 'azure' | 'bedrock' | 'ollama' | 'vllm'

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

### 3. `src/shared/project-types.ts`

**Kiểm tra:**
```bash
cat src/shared/project-types.ts
```

**Expected types (từ TDD-15 §2):**

```typescript
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
  devServer: import('./dev-server-types').PersistedDevServer
  resolvedProfile: import('../main/profile/OrcaProfile').ResolvedProfile
}
```

---

## Procedure

1. Đọc từng file — nếu đúng types → skip
2. Nếu file thiếu → tạo mới với content ở trên
3. Nếu file có nhưng thiếu field → bổ sung field còn thiếu
4. Chạy `pnpm tsc --noEmit` để confirm không có lỗi

---

## Acceptance Criteria

- [x] `src/shared/task-types.ts` tồn tại và có đủ `OrcaTask`, `TaskComment`, `TaskGrantLevel` ✅
- [x] `src/shared/ai-provider-types.ts` tồn tại và có đủ `AIProviderAccount`, `AIProviderType`, `PROVIDER_ENV_KEYS` ✅
- [x] `src/shared/project-types.ts` tồn tại và có đủ `OrcaProject`, `ProjectMember`, `ProjectContext` ✅
- [x] `pnpm tsc --noEmit` → 0 errors ✅
