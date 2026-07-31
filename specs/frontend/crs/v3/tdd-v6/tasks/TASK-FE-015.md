# TASK-FE-015: Verify TaskDetail — AI Decompose + Run Agent

**Task ID:** TASK-FE-015
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-005 (Section 5)
**Estimated effort:** 30 minutes
**Dependencies:** None

---

## Objective

Read `TaskDetail.tsx` (3215 bytes) and verify it has both action buttons required by TDD-FE-15:
1. "Decompose with AI" — opens `TaskAIDecompose` dialog
2. "Execute with Agent" — calls `tasks.runAgent` RPC

Also verify `TaskAIDecompose.tsx` uses the correct RPC methods.

---

## Step-by-Step Instructions

### Step 1: Read TaskDetail.tsx in full

```
Read file: src/renderer/src/components/task/TaskDetail.tsx
```

### Step 2: Check for required sections

**A. Task metadata grid** — must show: type, priority, status, assignee, reporter, estimated/actual hours

**B. Dependencies section** — must show:
```
← Blocked by: [task names]
→ Blocks: [task names]
```

**C. Action buttons** — must have BOTH:
```tsx
<Button onClick={handleDecompose}>🤖 Decompose with AI</Button>
<Button onClick={handleRunAgent}>▶ Execute with Agent</Button>
```

**D. Comments/Activity** — nice to have, not blocking

### Step 3: Read TaskAIDecompose.tsx

```
Read file: src/renderer/src/components/task/TaskAIDecompose.tsx
```

Verify:
- Calls `tasks.aiPlan(taskId)` (not `task.decomposeWithAI` — check exact method name)
- Shows suggestions list with title, type, estimatedHours
- "Apply All" calls `tasks.createSubtasks(taskId, approved[])`

### Step 4: Fix gaps in TaskDetail

#### Gap A — Missing "Execute with Agent" button:

If the button doesn't exist, add it:

```typescript
// In TaskDetail, add run agent handler:
const handleRunAgent = async () => {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  try {
    await callRuntimeRpc(target, 'tasks.runAgent', { taskId: task.id })
    toast.success(`Agent started for: ${task.title}`)
    // Optionally emit workspace event:
    // emit('agent.started', { taskId: task.id })
  } catch (err: any) {
    toast.error('Failed to start agent: ' + err.message)
  }
}

// In JSX, add button near Decompose button:
<Button variant="default" onClick={handleRunAgent} data-testid="run-agent-btn">
  ▶ Execute with Agent
</Button>
```

#### Gap B — TaskAIDecompose RPC method names:

Correct names per TDD-FE-15 addendum:
```typescript
// Decompose:
rpc: 'tasks.aiPlan'         // NOT 'task.decomposeWithAI'
params: { taskId }
// Apply:
rpc: 'tasks.createSubtasks' // NOT 'task.applyDecomposition'
params: { taskId, subtasks: approvedSuggestions }
```

Fix any mismatches in `TaskAIDecompose.tsx`.

#### Gap C — Dependencies display:

If dependencies are not shown, check if `task` object has `blockedBy` and `blocks` fields. If not available from the task object, fetch separately:

```typescript
// In TaskDetail, fetch dependencies:
const [deps, setDeps] = useState<{ blockedBy: OrcaTask[]; blocks: OrcaTask[] }>({
  blockedBy: [], blocks: []
})

useEffect(() => {
  if (!task?.id) return
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  callRuntimeRpc(target, 'tasks.getDependencies', { taskId: task.id })
    .then(d => setDeps(d as any))
    .catch(() => {})
}, [task?.id])
```

### Step 5: Verify TaskAIDecompose opens correctly from TaskDetail

In `TaskDetail`, the decompose flow should be:
```typescript
const [showDecompose, setShowDecompose] = useState(false)

const handleDecompose = () => setShowDecompose(true)

// In JSX:
{showDecompose && (
  <TaskAIDecompose
    taskId={task.id}
    onClose={() => setShowDecompose(false)}
  />
)}
```

### Step 6: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "TaskDetail|TaskAIDecompose" | head -15
```

---

## Acceptance Criteria

- [x] "Decompose with AI" button present in TaskDetail → opens TaskAIDecompose dialog
- [x] "Execute with Agent" button present → calls `tasks.runAgent` RPC
- [x] TaskAIDecompose uses `tasks.aiPlan` (not `task.decomposeWithAI`)
- [x] TaskAIDecompose "Apply All" uses `tasks.createSubtasks`
- [x] Dependencies section shows "blocked by" and "blocks" items
- [x] `data-testid="run-agent-btn"` on Execute button
- [x] No TypeScript errors

---

## Output

Report:
```
"Decompose with AI" button: ALREADY EXISTS (in TaskAIDecompose via subtasks tab)
"Execute with Agent" button: ADDED
tasks.aiPlan RPC: FIXED
tasks.createSubtasks RPC: FIXED
Dependencies section: IMPLEMENTED
TypeScript errors: 0
```
