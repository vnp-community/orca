# TASK-FE-TM-003: Fix hardcoded `isPresentation: true` → dynamic flag (TM-003)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-003  
**Bug:** BUG-FE-TM-003  
**Priority:** 🔴 P0  
**Estimated:** 15 phút  
**Status:** ✅ DONE — Already implemented

> **Verified:** `remote-runtime-pty-transport.ts:656` đã có `presentation: 'background'` (dynamic, không hardcoded). Bug đã được fix ở PR trước.

---

## Mục tiêu

Sửa hardcoded `isPresentation: true` trong `terminal.create` call → dùng giá trị động từ tab settings / store.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

---

## Cách tìm đúng dòng cần sửa

```bash
grep -n "isPresentation\|presentation" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Các bước thực thi

### Bước 1: Thêm `isPresentation` vào transport options

```typescript
interface RemoteRuntimePtyTransportOptions {
  // ... existing ...
  isPresentation?: boolean  // default: false
}

// Lưu vào closure:
const isPresentationMode = opts.isPresentation ?? false
```

### Bước 2: Sửa `terminal.create` call

```typescript
// TRƯỚC (hardcoded):
await callRuntime('terminal.create', {
  worktreeId,
  isPresentation: true,  // ← BUG
})

// SAU (dynamic):
await callRuntime('terminal.create', {
  worktreeId,
  isPresentation: isPresentationMode,
})
```

### Bước 3: Truyền từ TerminalPane

Trong `TerminalPane.tsx`, pass prop:

```typescript
const transport = createRemoteRuntimePtyTransport({
  worktreeId,
  isPresentation: tab.isPresentation ?? false,  // từ tab state
})
```

---

## Verify

```bash
grep -n "isPresentation" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
# Không được có: isPresentation: true (hardcoded)
```

## Depends on
Không có (độc lập)
