# TASK-FE-021 through TASK-FE-026: Test Tasks — Consolidated Reference

**Task IDs:** TASK-FE-021, TASK-FE-022, TASK-FE-023, TASK-FE-024, TASK-FE-025, TASK-FE-026
**Phase:** 3 — Tests
**Priority:** P2
**Estimated effort:** 60-90 minutes each

> This file is a consolidated reference for the remaining 6 test tasks.
> Each section below describes one task. Create individual task files as needed.

---

# TASK-FE-021: Write Tests — Workspace + Project Module (25+ tests)

**Dependencies:** TASK-FE-003, TASK-FE-004
**Solution Ref:** SOL-FE-V6-002

## Test Files

### `components/project/__tests__/ProjectSwitcher.test.tsx` (5 tests)
- renders current project name from context
- opens dropdown with full projects list
- calls `switchProject(id)` when project item clicked
- shows loading spinner (isInitializing=true)
- search input filters project list by name

### `components/project/__tests__/ProjectSettings.test.tsx` (4 tests)
- renders dialog with "General" and "Members" tabs
- project name appears in dialog title
- closes when Escape pressed (onClose called)
- Members tab renders MemberManager

### `components/project/__tests__/MemberManager.test.tsx` (5 tests)
- fetches members via `projects.listMembers` on mount
- renders member rows with displayName + email
- role Select calls `projects.updateMemberRole` on change
- remove button calls `projects.removeMember`
- shows loading state while fetching

### `components/workspace/__tests__/WorkspaceLayout.test.tsx` (6 tests)
- renders `NoProjectSelected` when project=null
- renders `WorkspaceSkeletonLoader` when isInitializing=true
- renders `OfflineBanner` when isOffline=true
- "git" tab active → GitPanel renders
- "tasks" tab click → shows task content
- "Show Terminal" toggle shows terminal panel

### `context/__tests__/WorkspaceContext.test.tsx` (8 tests)
- `switchProject()` calls multiple RPCs and sets project state
- `switchProject()` sets isOffline=true on DEV_SERVER_UNREACHABLE error
- `refreshGitStatus()` calls git.status and updates gitStatus
- `refreshFileTree()` calls workspace.listFiles and updates fileTree
- `emit(event, data)` + `on(event, handler)` delivers data to handler
- `on()` returns cleanup function that unsubscribes handler
- `agent.complete` event registered listener is called
- `isInitializing` is false after successful `switchProject`

## Run command
```bash
npx vitest run --reporter=verbose \
  src/renderer/src/components/project/ \
  src/renderer/src/components/workspace/ \
  src/renderer/src/context/
```

---

# TASK-FE-022: Write Tests — AI Provider Module (25+ tests)

**Dependencies:** TASK-FE-008, TASK-FE-009
**Solution Ref:** SOL-FE-V6-003

## Test Files

### `components/ai-provider/__tests__/ProviderList.test.tsx` (5 tests)
- renders account rows with provider name, label, server
- filter by `devServerId` shows only matching accounts
- filter by `scope='project'` shows only project-scope accounts
- filter by `status='invalid'` shows only invalid accounts
- "Add Account" button click opens ProviderForm

### `components/ai-provider/__tests__/ProviderForm.test.tsx` (5 tests)
- renders in create mode (no account prop) — "Create Account" title
- renders in edit mode (account prop) — "Edit Account" title
- Save in create mode calls `aiProvider.create`
- Save in edit mode calls `aiProvider.update`
- when encryptedCredential is set: also calls `aiProvider.writeCredential`

### `components/ai-provider/__tests__/CredentialInput.test.tsx` (6 tests) [CRITICAL]
- input has `type="password"`
- input has `autoComplete="new-password"`
- value < 10 chars: isEncrypted state stays false
- value >= 10 chars: `crypto.subtle.encrypt` is called
- after encryption: raw value state is reset to `''`
- after encryption: shows lock/encrypted icon
- Ollama provider: CredentialInput returns null (not rendered)

### `components/ai-provider/__tests__/HealthStatusBadge.test.tsx` (3 tests)
- status='healthy' → green CheckCircle
- status='invalid' → red XCircle
- status='quota_exceeded' → orange AlertTriangle

### `hooks/__tests__/useAIProviders.test.ts` (6 tests)
- fetches accounts on mount via `aiProvider.list`
- filters by devServerId in returned accounts
- `testConnection` ok → account status set to 'healthy'
- `testConnection` fail → account status set to 'invalid'
- `refresh()` re-fetches and updates accounts
- empty devServerId filter → returns all accounts

## Run command
```bash
npx vitest run --reporter=verbose src/renderer/src/components/ai-provider/ src/renderer/src/hooks/__tests__/useAIProviders.test.ts
```

---

# TASK-FE-023: Write Tests — Git UI (30+ tests)

**Dependencies:** TASK-FE-010, TASK-FE-011, TASK-FE-012
**Solution Ref:** SOL-FE-V6-006

## Test Files

### `components/workspace/git/__tests__/GitPanel.test.tsx` (6 tests)
- renders 4 tabs: changes, history, branches, pullrequests
- shows branch name from gitStatus.branch
- `data-testid="sync-button"` visible
- clicking Sync calls `git.push` RPC
- isPushing=true shows Loader2 on sync button
- switching to "pullrequests" tab renders PullRequestList

### `components/workspace/git/__tests__/DiffViewer.test.tsx` (5 tests)
- shows Skeleton while isLoading=true
- fetches HEAD content via `git.exec(['show', 'HEAD:path'])`
- fetches modified content via `fs.readFile`
- Monaco DiffEditor rendered with original + modified
- `.ts` extension detected as `typescript` language

### `components/workspace/git/__tests__/CommitForm.test.tsx` (6 tests)
- empty message → clicking Commit shows error (not committed)
- non-empty message → Commit calls `git.commit` RPC
- after commit: message field is cleared
- AI assist button calls RPC to generate message
- generated message populates the textarea
- `onCommitted` prop callback fires after successful commit

### `components/workspace/git/__tests__/PullRequestList.test.tsx` (5 tests)
- fetches PRs on mount via `git.pr.list`
- renders PR items with title and number
- empty list → shows empty state icon + text
- external link has `target="_blank"` and correct `href`
- refresh button re-fetches without showing loading skeleton

### `hooks/__tests__/useGit.test.ts` (6 tests)
- `stageFile(path)` calls `git.add` and then `refreshGitStatus`
- `unstageFile(path)` calls `git.restore` with staged=true
- `stageAll()` calls `git.add` with `files=['.']`
- `unstageAll()` calls `git.restore` with `files=['.']` and staged=true
- `getDiff(path)` calls `git.diff` and returns string
- `commit(msg)` calls `git.commit` with message and author info

## Run command
```bash
npx vitest run --reporter=verbose src/renderer/src/components/workspace/git/ src/renderer/src/hooks/__tests__/useGit.test.ts
```

---

# TASK-FE-024: Write Tests — Task Graph (30+ tests)

**Dependencies:** TASK-FE-013, TASK-FE-014, TASK-FE-015
**Solution Ref:** SOL-FE-V6-005

## Test Files

### `components/task/__tests__/TaskCard.test.tsx` (6 tests)
- renders task title and type badge
- shows expand chevron when task has children
- no chevron when task has no children
- `status='done'` → title has line-through class
- keyboard Enter on focused card → calls onSelect
- hover group reveals action area

### `components/task/__tests__/TaskTreeView.test.tsx` (5 tests)
- renders only root tasks (parentId=null/undefined)
- does NOT render children of a collapsed node
- expand node → children become visible
- nested 3-level tree: indent increases by depth * 20px
- selecting task calls `setActiveTask(task.id)`

### `components/task/__tests__/TaskDetail.test.tsx` (5 tests)
- renders task metadata: type, priority, status, assignee
- "Decompose with AI" button calls TaskAIDecompose flow
- "Execute with Agent" button (`data-testid="run-agent-btn"`) calls `tasks.runAgent`
- null task → nothing rendered / empty state
- dependencies section renders blocked-by list

### `components/task/__tests__/TaskAIDecompose.test.tsx` (6 tests)
- "Decompose" button calls `tasks.aiPlan` RPC
- shows loading indicator while waiting
- shows suggestions list after resolve
- each suggestion shows title, type, estimated hours
- "Apply All" button calls `tasks.createSubtasks`
- "Regenerate" resets suggestions and allows retry

### `components/task/__tests__/TaskStatusBadge.test.tsx` (4 tests)
- status='in_progress' → correct color class (blue)
- status='done' → green CheckCircle2
- status='blocked' → red OctagonX
- status='todo' → blue/grey CircleDot

### `hooks/__tests__/useTasks.test.ts` (5 tests)
- fetches tasks via `tasks.list(projectId)` on mount
- filterStatus='done' → filteredTasks contains only done tasks
- searchQuery filters tasks by title (case-insensitive)
- `toggleExpanded(id)` adds id to expandedNodes Set
- `toggleExpanded(id)` again removes id (toggle behavior)

## Run command
```bash
npx vitest run --reporter=verbose src/renderer/src/components/task/ src/renderer/src/hooks/__tests__/useTasks.test.ts
```

---

# TASK-FE-025: Write Tests — Workflow Module (25+ tests)

**Dependencies:** TASK-FE-016, TASK-FE-017
**Solution Ref:** SOL-FE-V6-004

## Test Files

### `components/workflow/__tests__/WorkflowBuilder.test.tsx` (6 tests)
- renders steps from template
- "Add Step" button adds step with default config
- removing a step also removes it from other steps' `dependsOn`
- updating a step field updates local template state
- "Save" button calls `workflow.template.update` or `create` RPC
- "Show DAG" toggle shows/hides the DAGPreview panel

### `components/workflow/__tests__/DAGPreview.test.tsx` (5 tests)
- empty steps array → shows empty state (`data-testid="dag-preview-empty"`)
- linear deps: 2 steps with A→B dependency → 2 waves
- parallel steps (no deps): all in wave 0, positioned at x=0
- dependency creates an edge between the two nodes
- selectedStepId → that node has highlighted border style

### `components/workflow/__tests__/ExecutionMonitor.test.tsx` (5 tests)
- renders step list from execution object
- step status='running' → shows spinner icon
- step status='done' → shows green checkmark
- step status='failed' → shows red X
- "Cancel" button calls appropriate cancel RPC

### `hooks/__tests__/useWorkflow.test.ts` (5 tests)
- `saveTemplate` without templateId → calls `workflow.template.create`
- `saveTemplate` with templateId → calls `workflow.template.update`
- `runWorkflow(templateId)` calls `workflow.execute`
- `runWorkflow` adds returned execution to store
- `updateTemplate(id, patch)` merges patch into existing template

## Run command
```bash
npx vitest run --reporter=verbose src/renderer/src/components/workflow/ src/renderer/src/hooks/__tests__/useWorkflow.test.ts
```

---

# TASK-FE-026: Write Tests — File Explorer (30+ tests)

**Dependencies:** TASK-FE-018, TASK-FE-019
**Solution Ref:** SOL-FE-V6-007

## Test Files

### `components/workspace/__tests__/ExplorerPanel.test.tsx` (7 tests)
- renders file tree from fileTree context
- clicking directory toggles expanded state
- expanding dir calls `refreshFileTree(path)` for lazy load
- selecting file shows FileViewer
- `agent.complete` event → `refreshFileTree()` called
- `files.changed` event → `refreshFileTree(parentDir)` called for each file
- search icon click → shows FileSearchPanel

### `components/workspace/__tests__/FileTreeNode.test.tsx` (8 tests)
- directory node: shows ChevronRight when collapsed
- directory node: shows ChevronDown when expanded
- file node: shows file icon, no chevron
- selected file → has `bg-accent` class
- gitStatus='M' → yellow dot/M indicator
- gitStatus='A' → green A indicator
- gitStatus='D' → red D indicator + text strikethrough
- keyboard Enter on node → calls toggleDir or selectFile

### `components/workspace/__tests__/FileViewer.test.tsx` (5 tests)
- shows Skeleton while isLoading=true
- fetches file content via `fs.readFile` on filePath change
- Monaco Editor rendered with file content
- `filePath="component.tsx"` → language='typescript'
- FILE_TOO_LARGE error → shows error message, not editor

### `components/workspace/__tests__/FileSearchPanel.test.tsx` (5 tests)
- query < 2 chars → no search triggered
- query >= 2 chars after 300ms debounce → `fs.grep` called
- shows results with file path and line number
- no results after search → shows "No results" text
- clicking result → calls `onSelect(path)`

## Run command
```bash
npx vitest run --reporter=verbose src/renderer/src/components/workspace/
```

---

## General Test Guidelines (All Test Tasks)

```typescript
// Standard mock setup for callRuntimeRpc:
vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' }),
}))

// Standard mock for useAppStore:
vi.mock('@/store', () => ({
  useAppStore: vi.fn(selector => selector(mockStoreState)),
}))

// Standard mock for useWorkspace:
vi.mock('@/context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: mockProject,
    gitStatus: mockGitStatus,
    isOffline: false,
    isInitializing: false,
    switchProject: vi.fn(),
    refreshGitStatus: vi.fn(),
    refreshFileTree: vi.fn(),
    emit: vi.fn(),
    on: vi.fn().mockReturnValue(() => {}),
    fileTree: [],
    currentWorktree: null,
  }),
}))
```

**Read an existing test file first to copy the exact mock pattern used in THIS project.**
