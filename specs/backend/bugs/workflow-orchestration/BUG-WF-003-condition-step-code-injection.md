# BUG-WF-003 [BACKEND]: `StepExecutors.executeCondition()` dùng `new Function(...)` — Code Injection risk

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WF-001  
**Note:** StepExecutors.ts: new Function() → evaluateSafeCondition() sandbox  

## Mức độ: 🔴 HIGH (Security)

## Tóm tắt

`src/main/workflow/StepExecutors.ts:164-168`:
```typescript
private executeCondition(step: WorkflowStep, inputs: Record<string, unknown>): Promise<StepOutput> {
  try {
    const expression = step.config['expression'] as string
    // eslint-disable-next-line @typescript-eslint/no-implied-eval
    const fn = new Function('inputs', `return !!(${expression})`)  ← ⚠️ CODE INJECTION
    const result = fn(inputs) as boolean
```

`new Function(...)` là dynamic code evaluation. Nếu `expression` chứa malicious code, ví dụ:
```
expression: "require('child_process').execSync('rm -rf /')"
```

Điều này cho phép **RCE (Remote Code Execution)** trên Orca Server process nếu:
1. Malicious user có thể tạo workflow template với `condition` step
2. Expression không được sanitize

## Comment ESLint

```typescript
// eslint-disable-next-line @typescript-eslint/no-implied-eval
```

Đây là disable ESLint rule `no-implied-eval` — rule này đặc biệt để ngăn pattern nguy hiểm này. Developer đã disable nó một cách có ý thức nhưng không implement sandbox alternative.

## Ảnh hưởng

1. Workflow template creator (bất kỳ authenticated user nào) có thể inject code
2. Code chạy trong Orca Server process → có thể access file system, DB, relay connections
3. Nếu multi-tenant: user A có thể inject code ảnh hưởng user B data

## Fix đề xuất

Thay thế `new Function()` bằng safe expression evaluator:

Option 1 — Restricted sandbox library:
```typescript
import { evaluate } from 'safe-eval'  // or similar sandboxed eval
const result = evaluate(expression, { inputs })
```

Option 2 — Allowlist operators only (simple comparisons):
```typescript
// Only allow: inputs.key === value, inputs.key > 0, etc.
const SAFE_PATTERN = /^inputs\.\w+(\s*(===|!==|>=|<=|>|<)\s*[\w'".\-]+)*$/
if (!SAFE_PATTERN.test(expression.trim())) {
  throw new Error('UNSAFE_EXPRESSION: only simple comparisons allowed')
}
```

## Files liên quan

- `src/main/workflow/StepExecutors.ts:164-168`: vulnerable code
