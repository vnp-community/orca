# TASK-TM-03: Add validateCwd() — Path Traversal Prevention in pty.spawn

**Task ID:** TASK-TM-03  
**Priority:** 🔴 HIGH  
**Bugs fixed:** TM-002  
**Estimated effort:** Medium  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/pty-handler.ts`

**Current code (L615 area):**
```typescript
const cwd = (params.cwd as string) || resolveDefaultCwd()
// ← No validation! Any path accepted including /etc, /root, /
```

**Problem:** An attacker (or buggy client) can set `cwd: '/etc'` or `cwd: '/../../../root'` and spawn a shell with that working directory. On a shared Dev Server, this allows directory traversal.

---

## Implementation

### Step 1: Add imports at the top of `pty-handler.ts`

Check if these are already imported; if not, add:
```typescript
import { existsSync } from 'node:fs'
import { resolve }    from 'node:path'
import { homedir }    from 'node:os'
```

### Step 2: Add `validateCwd()` private method to `PtyHandler` class

Add after `buildSpawnEnv()` method:

```typescript
/**
 * validateCwd — Resolves and validates a cwd path for PTY spawn.
 *
 * Allowed roots: home directory, /tmp, /var/tmp.
 * Rejects: absolute paths outside allowed roots, non-existent directories.
 *
 * Why: prevents directory traversal attacks via cwd parameter.
 * If cwd is outside allowed roots, falls back to home dir (permissive degradation).
 */
private validateCwd(rawCwd: string): string {
  if (!rawCwd) return homedir()

  const resolved = resolve(rawCwd)

  // Allow: home dir and its subdirs, /tmp, /var/tmp
  const allowedPrefixes = [
    homedir(),
    '/tmp',
    '/var/tmp',
  ]

  const isAllowed = allowedPrefixes.some(
    (p) => resolved === p || resolved.startsWith(p + '/')
  )

  if (!isAllowed) {
    process.stderr.write(
      `[pty-handler] validateCwd: rejected cwd="${resolved}" (outside allowed roots)\n`
    )
    return homedir()  // Degrade gracefully — don't reject the spawn entirely
  }

  if (!existsSync(resolved)) {
    process.stderr.write(
      `[pty-handler] validateCwd: cwd does not exist "${resolved}", falling back to home\n`
    )
    return homedir()
  }

  return resolved
}
```

### Step 3: Use `validateCwd()` in the `spawn()` method

Find where `cwd` is resolved in `spawn()`:

```typescript
// BEFORE (find this line):
const cwd = (params.cwd as string) || resolveDefaultCwd()

// AFTER:
const rawCwd = (params.cwd as string) || resolveDefaultCwd()
const cwd = this.validateCwd(rawCwd)
```

---

## Design Decision: Graceful Degradation vs. Hard Reject

This task uses **graceful degradation** (fall back to home dir) instead of throwing an error.

**Rationale:**
- The terminal pane is opened by the Orca UI — not a malicious actor in normal use
- A hard reject would break all terminal opens for paths outside home (e.g., `/workspace` on cloud VMs)
- Better UX: open terminal in home dir with a warning than fail completely

**If stricter behavior is preferred**, change the fallback to throw:
```typescript
throw Object.assign(
  new Error(`cwd not allowed: ${resolved}`),
  { code: -32003, data: { cwd: resolved } }
)
```

---

## Tests to Add

File: `src/relay/__tests__/pty-handler.test.ts`

```typescript
describe('validateCwd (via spawn)', () => {
  it('uses cwd when inside home directory', async () => {
    const result = await handler.spawn({ cwd: process.env.HOME }) as any
    expect(result.cwd).toBe(process.env.HOME)
  })

  it('uses cwd when inside /tmp', async () => {
    const result = await handler.spawn({ cwd: '/tmp' }) as any
    expect(result.cwd).toBe('/tmp')
  })

  it('falls back to home when cwd is outside allowed roots', async () => {
    const result = await handler.spawn({ cwd: '/etc/ssl' }) as any
    expect(result.cwd).toBe(homedir())  // graceful fallback
  })

  it('falls back to home when cwd does not exist', async () => {
    const result = await handler.spawn({ cwd: '/tmp/nonexistent-dir-xyz-abc' }) as any
    expect(result.cwd).toBe(homedir())
  })
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
**Implementation:** pty-handler.ts: validatePtyCwd(rawCwd) function tại L249. Reject ../, absolute paths ngoài home/work. Fallback home dir khi invalid.  
**Tests:** Verified: 'validatePtyCwd — TM-002: Validate the cwd parameter' tại L243.  
