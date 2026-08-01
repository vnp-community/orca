# SOL-FE-TM-004: Fix Default Viewport từ `cols: 80, rows: 24` → `cols: 120, rows: 40`

## Bug Reference
- **Bug:** BUG-FE-TM-004
- **Mức độ:** LOW
- **File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` Lines 669-672, 712-715, 230-233
- **TDD Reference:** TDD-FE-04 §2 xterm.js Integration, TDD-FE-04 §3.2 PTY Transport Factory

---

## Root Cause

Default viewport xuất hiện ở **3 vị trí** trong file, tất cả đều hardcode `cols: 80, rows: 24`:

```typescript
// Lines 669-672 (connect)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }

// Lines 712-715 (attach)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }

// Lines 230-233 (attachHostSessionMirror)
desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
```

HLD yêu cầu: `cols: 120, rows: 40` (theo terminal-create-flow.md §Bước 3).

---

## Phân tích

Trong **thực tế**, Browser gọi `resize()` ngay sau khi PTY spawn để sync kích thước pane thực tế. Nhưng nếu:
- Race condition giữa `connect()` và first `resize()`
- `resize()` call bị miss (timing issue, component unmount, etc.)
- PTY sẽ chạy với default viewport → layout issues với TUI apps (vim, tmux, htop)

---

## Giải pháp

### Fix: Thay đổi 3 locations — align với HLD defaults

**File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

#### Tạo constants ở top của file:

```typescript
// === HLD-aligned PTY defaults (BUG-FE-TM-004 fix) ===
// Theo terminal-create-flow.md §Bước 3: ptyController.spawn({ cols: 120, rows: 40 })
const DEFAULT_PTY_COLS = 120
const DEFAULT_PTY_ROWS = 40

// Minimum size (BR-TM-06)
const MIN_PTY_COLS = 80
const MIN_PTY_ROWS = 10
```

#### Update 3 locations:

**Location 1 — Lines 669-672 (connect):**
```diff
- desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
+ desiredViewport = {
+   cols: Math.max(options.cols ?? DEFAULT_PTY_COLS, MIN_PTY_COLS),
+   rows: Math.max(options.rows ?? DEFAULT_PTY_ROWS, MIN_PTY_ROWS),
+ }
```

**Location 2 — Lines 712-715 (attach):**
```diff
- desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
+ desiredViewport = {
+   cols: Math.max(options.cols ?? DEFAULT_PTY_COLS, MIN_PTY_COLS),
+   rows: Math.max(options.rows ?? DEFAULT_PTY_ROWS, MIN_PTY_ROWS),
+ }
```

**Location 3 — Lines 230-233 (attachHostSessionMirror):**
```diff
- desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
+ desiredViewport = {
+   cols: Math.max(options.cols ?? DEFAULT_PTY_COLS, MIN_PTY_COLS),
+   rows: Math.max(options.rows ?? DEFAULT_PTY_ROWS, MIN_PTY_ROWS),
+ }
```

---

### Cải tiến: Lấy viewport từ pane container size

Thay vì dùng hardcoded default, cách tốt hơn là đo actual pane size ngay khi khởi tạo:

```typescript
// Trong TerminalPane.tsx, truyền actual container dimensions khi gọi connect():
const paneRef = useRef<HTMLDivElement>(null)

const getActualViewport = () => {
  if (!paneRef.current) return { cols: DEFAULT_PTY_COLS, rows: DEFAULT_PTY_ROWS }
  
  const { width, height } = paneRef.current.getBoundingClientRect()
  // xterm.js character dimensions (approximation)
  const charWidth = fontSize * 0.6    // ~7.8px at 13px
  const charHeight = fontSize * lineHeight  // ~15.6px at 13px

  return {
    cols: Math.max(Math.floor(width / charWidth), MIN_PTY_COLS),
    rows: Math.max(Math.floor(height / charHeight), MIN_PTY_ROWS),
  }
}

// Sử dụng khi connect:
const viewport = getActualViewport()
transport.connect({
  worktreeId,
  cols: viewport.cols,
  rows: viewport.rows,
})
```

> **Recommended:** Implement `getActualViewport()` approach khi có thời gian. Short-term fix: update constants.

---

## Minimal One-Line Fix

Nếu muốn fix tối thiểu nhất:

```typescript
// Thêm constants ngay trên 3 locations và update chúng:
const DEFAULT_PTY_COLS = 120  // HLD default (was: 80)
const DEFAULT_PTY_ROWS = 40   // HLD default (was: 24)

// Và thay:
// cols: options.cols ?? 80 → cols: options.cols ?? DEFAULT_PTY_COLS
// rows: options.rows ?? 24 → rows: options.rows ?? DEFAULT_PTY_ROWS
```

---

## Files cần sửa

| File | Lines | Change |
|------|-------|--------|
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | 669-672 | Update default cols/rows |
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | 712-715 | Update default cols/rows |
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | 230-233 | Update default cols/rows |

---

## Note về xterm.js Initial Size

```typescript
// TDD-FE-04 §2 — createTerminal() cũng dùng cols: 80, rows: 24:
function createTerminal(options: TerminalOptions): Terminal {
  const term = new Terminal({
    cols: 80,   // ← cần align với DEFAULT_PTY_COLS
    rows: 24,   // ← cần align với DEFAULT_PTY_ROWS
    ...
  })
}
```

**Cần sửa cả `createTerminal()`** để initial xterm.js size match với PTY:
```diff
  const term = new Terminal({
-   cols: 80,
-   rows: 24,
+   cols: DEFAULT_PTY_COLS,  // 120
+   rows: DEFAULT_PTY_ROWS,  // 40
    ...
  })
```

---

## Verification

```bash
# Grep xem còn 80/24 hardcoded không:
grep -n "cols.*80\|rows.*24" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts

# Test: Mở terminal, không resize pane
# Verify: vim/tmux/htop render đúng 120 cols (không bị 80-col wrapping)

# Test: Minimum size enforcement:
# Resize pane rất nhỏ → PTY không xuống dưới 80x10
```

---

## Liên quan

- **BR-TM-02**: Resize propagation — viewport sync giữa Browser và PTY ✅ improved defaults
- **BR-TM-06**: Minimum size 80 cols × 10 rows ✅ enforced với Math.max
- **TDD-FE-04**: §2 createTerminal initial size, §3.2 PTY Transport Factory
