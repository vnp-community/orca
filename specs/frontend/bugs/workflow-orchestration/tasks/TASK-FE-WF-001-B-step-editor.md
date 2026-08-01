# TASK-FE-WF-001-B: `StepEditor` component — step config form (BL-WF-01)

**Domain:** workflow-orchestration  
**Solution Ref:** SOL-FE-WF-001B §Component 1  
**Bug:** BUG-FE-WF-001  
**Priority:** 🟠 P1  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Already implemented in codebase

---

## Mục tiêu

Tạo `StepEditor` component — form edit đầy đủ cho một workflow step (type, name, server, config, dependsOn, timeout).

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/workflow/step-editor.tsx`

---

## Các bước thực thi

Tạo file với nội dung đầy đủ từ SOL-FE-WF-001B §Component 1:

### Props

```typescript
interface StepEditorProps {
  step: WorkflowStep
  allSteps: WorkflowStep[]  // for dependsOn checkboxes (exclude self)
  onUpdate: (patch: Partial<WorkflowStep>) => void
  onDelete: () => void
}
```

### Sections

1. **Header:** "Edit Step" + Delete (Trash2) button
2. **Name:** `<Input>` text
3. **Type selector:** `<Select>` — agent | shell | notify (with default config on change)
4. **Server spec:** `<Input>` — `"project:current"` or server ID
5. **Type-specific config:**
   - `agent`: prompt `<Textarea>` + worktreePath `<Input>`
   - `shell`: command `<Textarea font-mono>` + workdir `<Input>`
   - `notify`: channel `<Input>` + message `<Textarea>`
6. **Depends On:** Checkboxes từ `potentialDeps` (allSteps.filter(s => s.id !== step.id))
7. **Options:** continueOnError `<Checkbox>` + timeoutMinutes `<Input type="number">`

---

## Verify

```bash
grep -n "StepEditor\|onUpdate\|potentialDeps" \
  src/renderer/src/components/workflow/step-editor.tsx
```

## Depends on
TASK-FE-WF-001-A (WorkflowStep type)

## Blocking
TASK-FE-WF-001-D (WorkflowBuilder)
