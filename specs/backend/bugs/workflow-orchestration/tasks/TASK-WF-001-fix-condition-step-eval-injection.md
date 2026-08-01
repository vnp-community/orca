# TASK-WF-001: Implement WorkflowOrchestrator eval() injection fix (BUG-WF-003)

**Priority:** 🔴 CRITICAL SECURITY — Code injection via condition step  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-WF-003  
**Depends on:** WorkflowOrchestrator đã được implement (SOL-V5-004 ✅ DONE)  
**Solution ref:** [SOLUTION-workflow-orchestration.md](../solutions/SOLUTION-workflow-orchestration.md)

---

## Mục tiêu

Xóa bỏ `eval()` hoặc `new Function()` trong condition step executor. Thay bằng sandboxed expression evaluator chỉ support operators đơn giản.

## Bước 1 — Tìm eval() trong StepExecutors

```bash
grep -rn "eval\|new Function\|Function(" src/main/workflow/ | head -10
```

## Bước 2 — File cần sửa

```
src/main/workflow/StepExecutors.ts
```

## Thay đổi cụ thể

### Thay thế condition evaluator (tìm `'condition'` step type handler):

**TRƯỚC (unsafe — nếu dùng eval/Function):**
```typescript
case 'condition': {
  const { expression } = step.config
  // eslint-disable-next-line no-eval
  const result = eval(expression)  // ← CODE INJECTION!
  // ...
}
```

**SAU (sandboxed — chỉ support comparisons đơn giản):**
```typescript
case 'condition': {
  const { expression } = step.config
  const context = { inputs, ...step.env }

  // Safe expression evaluator — NO eval/Function
  const result = evaluateCondition(expression, context)
  return { status: result ? 'completed' : 'skipped', output: { result } }
}

// Thêm safe evaluator function:
function evaluateCondition(
  expression: string,
  context:    Record<string, unknown>
): boolean {
  // Support patterns:
  // "${var} == 'value'"    → string equality
  // "${var} != 'value'"    → string inequality
  // "${var} > 0"           → numeric comparison
  // "${var} == true"       → boolean check
  // "true" / "false"       → literal

  const interpolated = expression.replace(
    /\$\{([^}]+)\}/g,
    (_, key: string) => {
      const val = context[key.trim()]
      return val === undefined ? '' : String(val)
    }
  )

  // Parse safe comparison patterns only:
  const eqMatch    = interpolated.match(/^(.+?)\s*==\s*(.+)$/)
  const neqMatch   = interpolated.match(/^(.+?)\s*!=\s*(.+)$/)
  const gtMatch    = interpolated.match(/^(.+?)\s*>\s*(.+)$/)
  const ltMatch    = interpolated.match(/^(.+?)\s*<\s*(.+)$/)
  const gteMatch   = interpolated.match(/^(.+?)\s*>=\s*(.+)$/)
  const lteMatch   = interpolated.match(/^(.+?)\s*<=\s*(.+)$/)

  const normalize = (s: string): unknown => {
    const t = s.trim().replace(/^['"]|['"]$/g, '')
    if (t === 'true')  return true
    if (t === 'false') return false
    const n = Number(t)
    return isNaN(n) ? t : n
  }

  if (eqMatch)  return normalize(eqMatch[1]!)  === normalize(eqMatch[2]!)
  if (neqMatch) return normalize(neqMatch[1]!) !== normalize(neqMatch[2]!)
  if (gtMatch)  return Number(normalize(gtMatch[1]!))  > Number(normalize(gtMatch[2]!))
  if (ltMatch)  return Number(normalize(ltMatch[1]!))  < Number(normalize(ltMatch[2]!))
  if (gteMatch) return Number(normalize(gteMatch[1]!)) >= Number(normalize(gteMatch[2]!))
  if (lteMatch) return Number(normalize(lteMatch[1]!)) <= Number(normalize(lteMatch[2]!))

  // Literal boolean
  if (interpolated.trim() === 'true')  return true
  if (interpolated.trim() === 'false') return false

  // Unknown expression → log warning, return false (fail-safe)
  console.warn(`[WorkflowOrchestrator] Unsupported condition expression: "${expression}"`)
  return false
}
```

## Verification

```bash
pnpm tsc --noEmit

# Verify không còn eval:
grep -n "eval\|new Function" src/main/workflow/StepExecutors.ts
# Expected: no results

# Test expressions:
# "${status} == 'done'" → true khi inputs.status === 'done'
# "${count} > 5" → true khi inputs.count > 5
# Arbitrary code "process.exit(1)" → unsupported, return false
```
