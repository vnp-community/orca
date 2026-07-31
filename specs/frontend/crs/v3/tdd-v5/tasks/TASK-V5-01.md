# TASK-V5-01: Shared Types & Store Registration Setup

**Order:** 1 (no prerequisites)  
**Solution Ref:** SOL-FE-V5-01, SOL-FE-V5-02, SOL-FE-V5-03, SOL-FE-V5-04, SOL-FE-V5-05, SOL-FE-V5-06  
**Est. effort:** ~30 min  

---

## Mô tả

Tạo shared type definitions dùng chung cho tất cả v5.0 features và chuẩn bị store `index.ts` để nhận các slices mới.

---

## Files Cần Tạo

### 1. `src/shared/profile-types.ts`

```typescript
// Shared types cho Profile Hierarchy (TDD-FE-11)

export type ProfileSource = 'company' | 'dept' | 'user' | 'concat'

export type McpServerConfig = {
  name:    string
  command: string
  args?:   string[]
  env?:    Record<string, string>
}

export type OrcaProfile = {
  agent?: {
    preferredModel?:     string
    trustPreset?:        'strict' | 'standard' | 'relaxed' | 'custom'
    customInstructions?: string
  }
  mcp?: {
    servers?: McpServerConfig[]
  }
  shell?: {
    defaultShell?:  string
    pathAdditions?: string[]
    envVars?:       Record<string, string>
  }
  security?: {
    approvedModels?: string[]   // glob patterns, e.g. 'claude-*'
    disallowedCmds?: string[]
  }
}

export type ResolvedProfile = OrcaProfile & {
  _sources: Record<string, ProfileSource>  // e.g. 'agent.preferredModel' → 'dept'
}

export type Department = {
  id:       string
  name:     string
  parentId: string | null
}
```

### 2. `src/shared/workspace-types.ts`

```typescript
// Shared types cho Workspace + File Explorer (TDD-FE-12, 17)

export type OrcaProject = {
  id:            string
  name:          string
  description?:  string
  repoPath:      string
  defaultBranch: string
  devServerId:   string
  visibility:    'private' | 'team' | 'public'
  createdAt:     number
  updatedAt:     number
}

export type ProjectMember = {
  userId: string
  email:  string
  name:   string
  role:   'owner' | 'member' | 'viewer'
}

export type FileNode = {
  name:       string
  path:       string          // relative to project root
  type:       'file' | 'directory'
  size?:      number          // bytes (files only)
  children?:  FileNode[]      // lazy loaded
  isLoading?: boolean
}

export type GitStatus = {
  branch:         string
  aheadBy:        number
  behindBy:       number
  hasUncommitted: boolean
  staged:         number
  unstaged:       number
}
```

### 3. `src/shared/ai-provider-types.ts`

```typescript
// Shared types cho AI Provider (TDD-FE-13)

export type AIProviderType =
  | 'anthropic' | 'openai' | 'gemini'
  | 'azure'     | 'bedrock'
  | 'ollama'    | 'vllm'

export type AIProviderScope = 'server' | 'project' | 'user'

export type AIProviderStatus =
  | 'active' | 'pending' | 'invalid'
  | 'quota_exceeded' | 'unreachable'

export type AIProviderAccount = {
  id:            string
  provider:      AIProviderType
  label:         string
  model:         string
  baseUrl?:      string         // Ollama / vLLM
  scope:         AIProviderScope
  scopeRefId:    string
  devServerId:   string
  status:        AIProviderStatus
  quotaLimitDay: number         // 0 = unlimited
  createdAt:     number
}

export type AIProviderUsage = {
  accountId: string
  tokens:    number
  requests:  number
  date:      string             // YYYY-MM-DD
}
```

### 4. `src/shared/workflow-types.ts`

```typescript
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
```

### 5. `src/shared/task-types.ts`

```typescript
// Shared types cho Task Graph (TDD-FE-15)

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
```

---

## Files Cần Sửa

### `src/renderer/src/store/index.ts`

Thêm import + registration cho 6 slices mới (chưa có implementation, chỉ chuẩn bị placeholder):

```typescript
// THÊM vào cuối phần imports (sau các import slice hiện có):
// import { createProfileSlice }    from './slices/profile'     // TASK-V5-04
// import { createWorkspaceSlice }  from './slices/workspace'   // TASK-V5-02
// import { createAIProviderSlice } from './slices/ai-provider' // TASK-V5-06
// import { createWorkflowSlice }   from './slices/workflow'    // TASK-V5-17
// import { createTaskSlice }       from './slices/task'        // TASK-V5-14
// import { createGitPanelSlice }   from './slices/git-panel'   // TASK-V5-11

// KHÔNG uncomment ngay — các task sau sẽ uncomment khi tạo slice
// Comment này chỉ để ghi nhận planned registrations
```

> **Lưu ý:** TASK-V5-01 chỉ tạo shared types. KHÔNG thêm imports thực tế vào store/index.ts vì slices chưa tồn tại.

---

## Acceptance Criteria

- [x] 5 files `src/shared/*.ts` được tạo không có lỗi TypeScript
- [x] Không có circular dependency với các file hiện có
- [x] Export tất cả types từ mỗi file (named exports, không default)
- [x] Không import từ `src/renderer` hay `src/main` (shared = platform-agnostic)

---

## Test Cases

_Không có test cho file types thuần — TypeScript compiler là test._
