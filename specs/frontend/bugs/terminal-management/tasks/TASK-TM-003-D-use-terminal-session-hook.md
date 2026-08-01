# TASK-TM-003-D: Renderer — `useTerminalSession` hook + auto save/restore

**Domain:** terminal-management  
**Solution Ref:** SOL-TM-003 Phần 4  
**Bug:** BUG-TM-003  
**Priority:** 🟡 P2  
**Estimated:** 40 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `useTerminalSession` hook để tự động save session khi tab bị đóng/unmount và restore khi tab được mở.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/hooks/useTerminalSession.ts`

---

## Các bước thực thi

```typescript
// src/renderer/src/hooks/useTerminalSession.ts
import { useEffect, useRef } from 'react'
import type { SerializeAddon } from '@xterm/addon-serialize'
import type { Terminal } from '@xterm/xterm'

interface UseTerminalSessionOptions {
  worktreeId: string
  terminalId: string
  term: Terminal | null
  serializeAddon: SerializeAddon | null
  cwd?: string
  title?: string
}

export function useTerminalSession({
  worktreeId,
  terminalId,
  term,
  serializeAddon,
  cwd,
  title,
}: UseTerminalSessionOptions) {

  // Restore on mount
  useEffect(() => {
    if (!term || !worktreeId || !terminalId) return

    async function restoreSession() {
      try {
        const session = await window.api.terminal.session.restore(worktreeId, terminalId)
        if (session?.snapshot && term) {
          term.write(session.snapshot)
          term.scrollToBottom()
        }
      } catch {
        // No session to restore — silent fail
      }
    }

    restoreSession()
  }, [worktreeId, terminalId, term])

  // Save on unmount
  useEffect(() => {
    return () => {
      if (!serializeAddon || !worktreeId || !terminalId) return
      try {
        const snapshot = serializeAddon.serialize()
        void window.api.terminal.session.save({
          worktreeId,
          terminalId,
          snapshot,
          cwd: cwd ?? '~',
          title: title ?? 'Terminal',
          cols: term?.cols ?? 80,
          rows: term?.rows ?? 24,
        })
      } catch {
        // Fire-and-forget — non-critical
      }
    }
  }, [])  // empty deps = only run on unmount
}
```

### Cập nhật `web-preload-api.ts` — thêm `terminal.session` namespace

```typescript
terminal: {
  session: {
    save:    (params) => window.api.terminal.session.save(params),
    restore: (worktreeId, terminalId) => window.api.terminal.session.restore(worktreeId, terminalId),
    delete:  (worktreeId, terminalId) => window.api.terminal.session.delete(worktreeId, terminalId),
  }
}
```

### Tích hợp vào `TerminalPane.tsx`

```typescript
import { useTerminalSession } from '@/hooks/useTerminalSession'

// Trong TerminalPane component:
useTerminalSession({
  worktreeId,
  terminalId: terminalId!,
  term: termRef.current,
  serializeAddon: serializeAddonRef.current,
  cwd,
  title,
})
```

---

## Verify

```bash
grep -n "useTerminalSession" \
  src/renderer/src/components/terminal-pane/TerminalPane.tsx
```

## Depends on
TASK-TM-003-C (IPC handlers)
