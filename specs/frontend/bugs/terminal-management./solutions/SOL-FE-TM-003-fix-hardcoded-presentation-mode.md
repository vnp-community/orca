# SOL-FE-TM-003: Fix Hardcoded `presentation: 'background'` — Respect Caller Intent

## Bug Reference
- **Bug:** BUG-FE-TM-003
- **Mức độ:** LOW
- **File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` Lines 640-658
- **TDD Reference:** TDD-FE-04 §3 PTY Transport Layer — `terminal.create` params

---

## Root Cause

```typescript
// Lines 653-657 — hardcoded, bỏ qua caller intent
const created = await callRuntime<{ terminal: RuntimeTerminalCreate }>('terminal.create', {
  worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
  // ...
  focus: false,                // ← luôn false
  presentation: 'background', // ← luôn background
  ...(activate === true ? { activate: true } : {})
})
```

Comment trong code giải thích: "transport đã mount renderer pane → không cần remote reveal".  
**Logic này đúng trong Electron mode** nhưng sai trong Web Server mode (headless) khi:
- `presentation: 'focused'` nên trigger `notifier.revealTerminalSession()` trên mobile/other clients
- `publishPtyBackedMobileSessionTerminal` với `selectIfNoActiveTab` không được set đúng

---

## Phân tích kỹ

Có 2 scenarios:

| Mode | Behavior đúng | Hiện tại |
|------|---------------|---------|
| **Electron Desktop** | `presentation: 'background'` là đúng — renderer tự manage tabs locally | ✅ Correct |
| **Web Server (headless)** | `presentation` nên follow caller intent — backend cần reveal trên other clients | ❌ Wrong |

---

## Giải pháp

### Option A: Expose `presentation` như optional transport option

```typescript
// Trong ConnectOptions type (hoặc tương đương):
interface ConnectOptions {
  worktreeId: string
  cols?: number
  rows?: number
  activate?: boolean
  // NEW: allow callers to override presentation mode
  presentation?: 'background' | 'focused'
  focus?: boolean
}

// Trong connect():
const effectivePresentation = options.presentation ?? 'background'
const effectiveFocus = options.focus ?? false

const created = await callRuntime<{ terminal: RuntimeTerminalCreate }>('terminal.create', {
  worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
  // ...
  focus: effectiveFocus,
  presentation: effectivePresentation,
  ...(activate === true ? { activate: true } : {})
})
```

**Diff:**
```diff
  const created = await callRuntime<{ terminal: RuntimeTerminalCreate }>('terminal.create', {
    worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
    // ...
-   focus: false,
-   presentation: 'background',
+   focus: options.focus ?? false,
+   presentation: options.presentation ?? 'background',
    ...(activate === true ? { activate: true } : {})
  })
```

### Option B: Mode-aware presentation (Electron vs Web)

```typescript
// Detect runtime mode:
import { isElectronMode } from '@/lib/platform-detect'

const effectivePresentation = isElectronMode()
  ? 'background'             // Electron: renderer manages locally
  : (options.presentation ?? 'background')  // Web: respect caller intent
```

> **Recommended: Option A** — đơn giản hơn, backward compatible (default = 'background' giữ nguyên behavior cũ)

---

## Callers cần update

Sau khi fix transport, các callers muốn `focused` terminal cần truyền explicitly:

```typescript
// TerminalPane.tsx hoặc WorktreeCard.tsx — khi user mở terminal focused:
const transport = createRemoteRuntimePtyTransport({
  worktreeId,
  presentation: isUserInitiatedFocused ? 'focused' : 'background',
  focus: isUserInitiatedFocused,
})
```

---

## Files cần sửa

| File | Lines | Change |
|------|-------|--------|
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | 640-658 | Replace hardcoded `focus/presentation` với `options.focus/options.presentation` |
| `ConnectOptions` type | — | Thêm `presentation?` và `focus?` optional fields |

---

## Verification

```bash
# Grep xem caller nào pass presentation:
grep -rn "presentation.*focused" src/renderer/src/

# Test Web Server mode:
# 1. Open terminal with 'focused' intent (ví dụ từ mobile trigger)
# 2. Verify: backend notifier.revealTerminalSession() được called
# 3. Verify: Electron mode vẫn dùng 'background' by default
```

---

## Liên quan

- **BL-TM-01**: Response Path `notifier.revealTerminalSession()` ✅ enabled khi presentation='focused'
- **BR-TM**: presentation mode configurable
- **TDD-FE-04**: §3 PTY Transport Layer
- **BUG-FE-TM-001**: Timeout (independent fix)
- **BUG-FE-TM-004**: Viewport defaults (independent fix)

---

> **Note về severity:** Bug này là LOW severity vì trong Electron mode hiện tại, behavior 'background' là correct. Chỉ ảnh hưởng khi Web Server mode + mobile companion integration. Fix trước khi launch Mobile Companion feature.
