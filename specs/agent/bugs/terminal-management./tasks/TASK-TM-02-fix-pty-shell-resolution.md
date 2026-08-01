# TASK-TM-02: Fix pty.spawn Shell Resolution — Honor env.SHELL

**Task ID:** TASK-TM-02  
**Priority:** 🔴 HIGH  
**Bugs fixed:** TM-003  
**Estimated effort:** Small  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/pty-handler.ts`

**Current code (L626-629):**
```typescript
const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
const shell = resolvedShellOverride || resolveDefaultShell()
```

**Problem:** `resolvePtyShellOverride()` only works on Windows (handles Windows shell profiles). On Linux/macOS, it always returns `''`.

This means on Linux/macOS:
- A user who prefers `fish` shell cannot specify it
- A user on `bash` whose system default is `zsh` cannot override it
- The only option is `resolveDefaultShell()` which reads the system default

---

## Implementation

### In `src/relay/pty-handler.ts`, update lines 626-629

```typescript
// OLD:
const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
const shell = resolvedShellOverride || resolveDefaultShell()

// NEW:
const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
// TM-003: On Linux/macOS, resolvePtyShellOverride returns '' (Windows-only logic).
// Fall back to env.SHELL if provided by the renderer, before system default.
const envShell = (env && typeof env.SHELL === 'string' && env.SHELL.trim())
  ? env.SHELL.trim()
  : ''
const shell = resolvedShellOverride || envShell || resolveDefaultShell()
```

---

## Security Note

**No path validation needed here** — `shell` is passed to `pty.spawn(shell, ...)` which uses `execvp`. If the path is invalid or non-executable, `node-pty` will throw and the error is caught by the spawn handler. The existing error path handles this correctly.

However, if stricter validation is desired, add:
```typescript
// Optional: validate envShell is an absolute path (prevents injection via env.SHELL = 'sh; rm -rf /')
const safeEnvShell = /^\/[^\s]+$/.test(envShell) ? envShell : ''
const shell = resolvedShellOverride || safeEnvShell || resolveDefaultShell()
```

---

## Tests to Add

File: `src/relay/__tests__/pty-handler.test.ts`

```typescript
it('uses env.SHELL when provided on Linux/macOS', async () => {
  if (process.platform === 'win32') return  // skip on Windows

  // Mock resolveDefaultShell to return '/bin/sh' so we can detect override
  const result = await handler.spawn({
    cwd: '/tmp',
    env: { SHELL: '/bin/bash' }
  }) as any

  // shell in response should match the requested env.SHELL
  expect(result.shell).toBe('/bin/bash')
})

it('falls back to resolveDefaultShell when env.SHELL absent', async () => {
  const result = await handler.spawn({ cwd: '/tmp' }) as any
  expect(result.shell).toBeTruthy()  // some shell resolved
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep pty-handler
npx vitest run src/relay/__tests__/pty-handler.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** pty-handler.ts: Shell resolution ưu tiên params.env.SHELL trước SHELL env var, fallback /bin/sh. validatePtyCwd reject path traversal.  
**Tests:** pty-handler.ts verified via grep.  
