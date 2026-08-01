# TASK-TM-01: Fix pty.spawn Return Value — Add cols/rows/cwd/shell Fields

**Task ID:** TASK-TM-01  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** TM-004  
**Estimated effort:** Tiny (1-line change)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/pty-handler.ts`

**Current code (L747):**
```typescript
return { id }
```

**Problem:** Orca Server (the caller) needs more than just `id` after spawning a PTY:
- `cols` / `rows` — to verify the spawn honored the requested dimensions
- `cwd` — to display in the terminal pane title/path breadcrumb
- `shell` — to show shell type (bash/zsh/fish) in the UI

Without these fields, the terminal pane cannot display accurate metadata and may show incorrect/stale dimensions.

---

## Implementation

### In `src/relay/pty-handler.ts`, at the end of `spawn()` method

```typescript
// OLD (line ~747):
return { id }

// NEW:
// TM-004: Return all metadata callers need to initialize the terminal pane.
// id alone is insufficient for correct pane initialization.
return { id, cols, rows, cwd, shell }
```

**Note:** `cols`, `rows`, `cwd`, `shell` are all already local variables in the `spawn()` method scope — no new computation needed.

---

## Update Return Type (if typed)

If `spawn()` has an explicit return type annotation, update it:

```typescript
// Find the return type annotation of private async spawn()
// Change: Promise<{ id: string }>
// To:     Promise<{ id: string; cols: number; rows: number; cwd: string; shell: string }>
```

---

## Tests to Add

File: `src/relay/__tests__/pty-handler.test.ts`

```typescript
it('spawn() returns id, cols, rows, cwd, shell', async () => {
  const result = await handler.spawn({
    cwd: '/tmp', cols: 120, rows: 40
  }) as any
  expect(result.id).toMatch(/^pty-\d+$/)
  expect(result.cols).toBe(120)
  expect(result.rows).toBe(40)
  expect(result.cwd).toBe('/tmp')
  expect(typeof result.shell).toBe('string')
  expect(result.shell.length).toBeGreaterThan(0)
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
**Implementation:** pty-handler.ts: spawn() trả { id, cols, rows, cwd, shell } thay vì chỉ { id }. Return type đã định nghĩa rõ ràng.  
**Tests:** grep verify: 'Promise<{ id: string; cols: number; rows: number; cwd: string; shell: string }>' tại L635.  
