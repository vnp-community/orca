# TASK-FE-017: Integrate DAGPreview into WorkflowBuilder

**Task ID:** TASK-FE-017
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 2 — New Components
**Priority:** P2
**Solution Ref:** SOL-FE-V6-004 (Section 4)
**Estimated effort:** 25 minutes
**Dependencies:** TASK-FE-016 (DAGPreview.tsx must exist)

---

## Objective

Modify `WorkflowBuilder.tsx` to add a DAGPreview panel that shows the current workflow's step graph. The panel should toggle on/off.

---

## Step-by-Step Instructions

### Step 1: Verify DAGPreview.tsx exists

```bash
ls /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/workflow/DAGPreview.tsx && echo "EXISTS" || echo "NOT FOUND — run TASK-FE-016 first"
```

### Step 2: Read WorkflowBuilder.tsx in full

```
Read file: src/renderer/src/components/workflow/WorkflowBuilder.tsx
```

Understand:
- How the template/steps state is managed
- What state variable holds steps
- If there is a `selectedStepId` state
- Current layout structure

### Step 3: Add DAGPreview to WorkflowBuilder

Add these changes to `WorkflowBuilder.tsx`:

**A. Add import (lazy loaded to avoid heavy initial bundle):**
```typescript
import { lazy, Suspense } from 'react'
const DAGPreview = lazy(() => import('./DAGPreview').then(m => ({ default: m.DAGPreview })))
```

**B. Add state for DAG panel toggle:**
```typescript
const [showDag, setShowDag] = useState(false)
```

**C. Add toggle button in the header/toolbar:**
```typescript
// In the WorkflowBuilder header section:
<button
  onClick={() => setShowDag(v => !v)}
  className={`text-xs px-2 py-1 border rounded ${showDag ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}
  data-testid="toggle-dag-preview"
>
  {showDag ? 'Hide DAG' : 'Show DAG'}
</button>
```

**D. Render DAGPreview in a side panel:**
```typescript
// Wrap existing content + DAG panel in flex row:
<div className="flex flex-1 overflow-hidden">
  {/* Existing left panel (StepList + StepEditor) */}
  <div className="flex-1 overflow-auto">
    {/* existing WorkflowBuilder content */}
  </div>
  
  {/* DAG Preview panel */}
  {showDag && (
    <div className="dag-panel w-72 border-l overflow-hidden shrink-0">
      <div className="px-2 py-1 border-b text-xs text-muted-foreground bg-muted/30">
        DAG Preview
      </div>
      <Suspense fallback={<div className="p-3 text-xs text-muted-foreground">Loading DAG...</div>}>
        <DAGPreview
          steps={localTemplate?.steps ?? []}
          selectedStepId={selectedStepId}
        />
      </Suspense>
    </div>
  )}
</div>
```

**Note:** Adjust `localTemplate?.steps` and `selectedStepId` to match the actual variable names found in WorkflowBuilder.

### Step 4: Preserve existing functionality

Ensure the modifications do NOT break:
- Step add/remove/edit
- Save workflow
- Run workflow
- ExecutionMonitor display

Review the diff of changes before applying.

### Step 5: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "WorkflowBuilder|DAGPreview" | head -10
```

---

## Acceptance Criteria

- [x] WorkflowBuilder has a "Show DAG" / "Hide DAG" toggle button
- [x] `data-testid="toggle-dag-preview"` on toggle button
- [x] DAGPreview panel appears when toggle is on
- [x] DAGPreview receives the current template's steps
- [x] DAGPreview receives the selected step ID for highlighting
- [x] DAGPreview is lazy loaded
- [x] Existing step add/remove/edit still works
- [x] No TypeScript errors

---

## Output

Report:
```
WorkflowBuilder modified: YES
DAG toggle button: ADDED
Steps variable name used: template.steps
selectedStepId variable name: selectedStepId
TypeScript errors: 0
```
