# TASK-FE-WF-001-C: `DAGPreview` + `StepStatusBadge` components (BL-WF-01, BL-WF-02)

**Domain:** workflow-orchestration  
**Solution Ref:** SOL-FE-WF-001B §Component 2 & 3  
**Bug:** BUG-FE-WF-001  
**Priority:** 🟠 P1  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Already implemented in codebase

---

## Mục tiêu

1. `DAGPreview` — React Flow visualization read-only cho workflow steps
2. `StepStatusBadge` — icon+label badge cho execution step status

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/workflow/dag-preview.tsx`
- **TẠO MỚI:** `src/renderer/src/components/workflow/step-status-badge.tsx`

---

## Bước 1: `dag-preview.tsx`

### Dependency check

```bash
grep "@xyflow/react" package.json
# Nếu không có: npm install @xyflow/react
```

### Implementation

Props: `steps: WorkflowStep[]`, `selectedStepId: string | null`

**`buildDAGLayout(steps)`** — pure function:
1. Topological sort → `waveMap: Map<stepId, waveNumber>` (DFS từ leaves)
2. Group by wave → positions: `x = wave * 180, y = idx * 70`
3. Build `edges` từ `step.dependsOn`

**ReactFlow render:**
- `nodesDraggable={false}`, `nodesConnectable={false}`, `elementsSelectable={false}`
- Selected node: `border: 2px solid #3b82f6, background: #dbeafe`
- Empty state: "Add steps to see DAG preview"

## Bước 2: `step-status-badge.tsx`

```typescript
type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped'

// STATUS_CONFIG map:
// pending   → Clock icon, muted text
// running   → Loader2 icon (animate-spin), blue text
// completed → CheckCircle icon, green text
// failed    → XCircle icon, red text
// skipped   → AlertTriangle icon, yellow text

// Props: status, compact? (icon only mode)
```

---

## Verify

```bash
grep -n "DAGPreview\|buildDAGLayout" \
  src/renderer/src/components/workflow/dag-preview.tsx

grep -n "StepStatusBadge\|STATUS_CONFIG" \
  src/renderer/src/components/workflow/step-status-badge.tsx
```

## Test

```typescript
// dag-preview.test.tsx
// - linear deps → wave 0 and wave 1 nodes
// - parallel (no deps) → all in wave 0
// - edges created for each dependency
// - empty steps → shows "Add steps" message
```

## Depends on
TASK-FE-WF-001-A (types)

## Blocking
TASK-FE-WF-001-D (WorkflowBuilder), TASK-FE-WF-001-E (ExecutionMonitor)
