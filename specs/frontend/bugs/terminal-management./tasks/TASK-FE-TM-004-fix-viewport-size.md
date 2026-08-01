# TASK-FE-TM-004: Fix default viewport size `0×0` → `80×24` (TM-004)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-004  
**Bug:** BUG-FE-TM-004  
**Priority:** 🟠 P1  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Already implemented

> **Verified:** `remote-runtime-pty-transport.ts:669-672` đã có `cols: options.cols ?? 80, rows: options.rows ?? 24`. Fallback đã đúng.

---

## Mục tiêu

Đảm bảo `terminal.create` luôn gửi cols/rows ≥ 1 (fallback 80×24), không gửi 0×0 khi container chưa render.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

---

## Cách tìm đúng đoạn cần sửa

```bash
grep -n "cols\|rows\|viewport\|dimensions" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts | head -20
```

## Các bước thực thi

### Option A: Trong transport factory (Recommended)

```typescript
const DEFAULT_COLS = 80
const DEFAULT_ROWS = 24

// Thêm helper:
function safeViewport(cols: number, rows: number) {
  return {
    cols: Math.max(cols || DEFAULT_COLS, 1),
    rows: Math.max(rows || DEFAULT_ROWS, 1),
  }
}
```

### Trong terminal.create call:

```typescript
const { cols, rows } = safeViewport(
  term?.cols ?? 0,
  term?.rows ?? 0
)

await callRuntime('terminal.create', {
  worktreeId,
  cols,
  rows,
})
```

### Option B: Trong TerminalPane trước khi init transport

```typescript
// Đảm bảo container đã render trước khi create transport
if (!containerRef.current?.clientWidth) {
  await new Promise(r => setTimeout(r, 100))  // wait one frame
}
```

---

## Verify

```bash
grep -n "safeViewport\|DEFAULT_COLS\|Math.max.*cols" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Test

```typescript
// Vitest unit test:
// safeViewport(0, 0)   → { cols: 80, rows: 24 }
// safeViewport(120, 0) → { cols: 120, rows: 24 }
// safeViewport(0, 40)  → { cols: 80, rows: 40 }
// safeViewport(120, 40)→ { cols: 120, rows: 40 }
```

## Depends on
Không có (độc lập)
