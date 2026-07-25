# TDD-FE-04: Terminal Subsystem

**Document:** TDD-FE-04  
**Domain:** Terminal — xterm.js, PTY Transport, Pane Layout, TerminalPane  
**Source files:** `src/renderer/src/components/terminal-pane/` (~358 files!)

---

## 1. Tổng quan

Terminal là **tính năng cốt lõi** của Orca. Đây là subsystem phức tạp nhất:

```
TerminalPane.tsx (~127K)
  │
  ├─ xterm.js (Terminal object) — rendering, emulation
  │   ├─ WebGL renderer (primary)
  │   └─ Canvas renderer (fallback)
  │
  ├─ PtyTransport — kết nối PTY data
  │   ├─ LocalPtyTransport (Electron IPC)
  │   └─ RemoteRuntimePtyTransport (WebSocket)
  │
  ├─ PaneManager — split/resize/collapse layout
  │
  └─ AgentCompletionCoordinator — detect khi agent done
```

---

## 2. xterm.js Integration

```typescript
// src/renderer/src/components/terminal-pane/TerminalPane.tsx
import { Terminal } from '@xterm/xterm'
import { WebglAddon } from '@xterm/addon-webgl'
import { CanvasAddon } from '@xterm/addon-canvas'
import { SearchAddon } from '@xterm/addon-search'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SerializeAddon } from '@xterm/addon-serialize'

function createTerminal(options: TerminalOptions): Terminal {
  const term = new Terminal({
    allowProposedApi: true,
    cols: 80,
    rows: 24,
    // Font:
    fontFamily: settings.terminalFontFamily ?? 'Menlo, monospace',
    fontSize: settings.terminalFontSize ?? 13,
    lineHeight: 1.2,
    // Colors: sẽ được override bởi theme
    theme: resolveTerminalTheme(settings, systemPrefersDark),
    // Feature flags:
    minimumContrastRatio: 4.5,  // WCAG AA
    allowTransparency: false
  })

  // Addons:
  const webgl = new WebglAddon()
  term.loadAddon(webgl)

  const search = new SearchAddon()
  term.loadAddon(search)

  const serialize = new SerializeAddon()
  term.loadAddon(serialize)

  return term
}
```

---

## 3. PTY Transport Layer

### 3.1 Transport types

```typescript
// src/renderer/src/components/terminal-pane/pty-transport-types.ts
interface PtyTransport {
  // Write input tới shell:
  write(data: Uint8Array): void

  // Resize terminal:
  resize(cols: number, rows: number): void

  // Kill terminal:
  kill(signal?: string): void

  // Subscribe terminal output:
  subscribe(onData: (data: Uint8Array) => void): Unsubscribe

  // Lifecycle:
  dispose(): void
}

type LocalPtyTransport = PtyTransport & {
  kind: 'local'
  ptyId: string
}

type RemoteRuntimePtyTransport = PtyTransport & {
  kind: 'remote'
  ptyId: string
  environmentId: string
}
```

### 3.2 PTY Transport Factory (`pty-transport.ts`)

~38KB — Khởi tạo transport phù hợp:

```typescript
// src/renderer/src/components/terminal-pane/pty-transport.ts

async function createPtyTransport(args: {
  worktreeId: string
  environmentId: string | null
  command?: string
  cwd?: string
  env?: Record<string, string>
  cols: number
  rows: number
}): Promise<PtyTransport> {
  if (!args.environmentId) {
    // Local: Electron IPC
    const ptyId = await window.api.pty.create({
      shell: resolveShell(settings),
      cwd: args.cwd ?? worktreePath,
      env: args.env,
      cols: args.cols,
      rows: args.rows
    })
    return createLocalPtyTransport(ptyId)
  } else {
    // Remote: WebSocket RPC
    const result = await callRuntimeRpc(
      { kind: 'environment', environmentId: args.environmentId },
      'terminal.create',
      { worktree: worktreeSelector, command: args.command, ... }
    )
    return createRemoteRuntimePtyTransport(result.ptyId, args.environmentId)
  }
}
```

### 3.3 PTY Dispatcher (`pty-dispatcher.ts`)

~15KB — Phân phối PTY data tới đúng xterm.js instance:

```typescript
// src/renderer/src/components/terminal-pane/pty-dispatcher.ts
// Registry: ptyId → xterm.js write callback

registerPtyDataHandler(ptyId: string, handler: (data: Uint8Array) => void)
unregisterPtyDataHandler(ptyId: string)

// Khi nhận data từ IPC event 'pty:data':
dispatchPtyData(ptyId, data)
// → lookup handler → call handler → xterm.js.write(data)

// Eager buffer: buffer data cho PTY trước khi terminal ready
getEagerPtyBufferHandle(ptyId)
```

### 3.4 PTY Connection (`pty-connection.ts`)

~325KB — **File lớn nhất toàn bộ project**!  
Quản lý full lifecycle của một PTY connection:

```typescript
// src/renderer/src/components/terminal-pane/pty-connection.ts
class PtyConnection {
  // State machine:
  // created → connecting → connected → disconnected → reconnecting | disposed

  // Lifecycle hooks:
  onConnected()
  onData(data: Uint8Array)       // → xterm.js.write()
  onExit(exitCode, signal)       // → show exit banner
  onReconnect()                  // → SSH auto-reconnect
  onDispose()                    // → cleanup

  // Features:
  // - Replay guard (prevent duplicate output on reconnect)
  // - Session snapshot (restore scrollback on reattach)
  // - SSH reconnect overlay
  // - IME input handling
  // - Paste coordination
}
```

---

## 4. Pane Layout

### 4.1 PaneManager (`lib/pane-manager/`)

```typescript
// src/renderer/src/lib/pane-manager/pane-manager.ts
class PaneManager {
  // Manages split-pane layout cho một terminal tab
  // Supports: horizontal split, vertical split, collapse

  split(paneId: number, direction: 'horizontal' | 'vertical'): number  // new paneId
  close(paneId: number): void
  resize(paneId: number, size: number): void
  serialize(): TerminalPaneLayoutNode    // persist layout
  restore(layout: TerminalPaneLayoutNode): void
}
```

### 4.2 Layout Serialization (`layout-serialization.ts`)

```typescript
// src/renderer/src/components/terminal-pane/layout-serialization.ts
// ~15K — serialize/deserialize pane tree

function serializePaneTree(manager: PaneManager): TerminalPaneLayoutNode
function collectLeafIdsInOrder(layout: TerminalPaneLayoutNode): number[]
function normalizeTerminalLayoutSnapshot(snapshot: TerminalLayoutSnapshot): TerminalLayoutSnapshot
```

### 4.3 Terminal Hidden View Parking

```typescript
// terminal-hidden-view-parking.ts (~11K)
// Khi terminal tab không active → "park" nó (ẩn DOM nhưng giữ alive)
// Tránh xterm.js không cần re-render khi tab trở lại visible
// Kỹ thuật: visibility CSS + requestAnimationFrame gate
```

---

## 5. IME (Input Method Editor) Support

Toàn bộ IME handling được implement riêng vì xterm.js IME support kém:

```typescript
// terminal-ime-input-source.ts     — detect IME state
// terminal-ime-composition-tracker.ts — track composition
// terminal-ime-candidate-key-release-guard.ts — guard candidate key
// terminal-ime-linux-candidate-state.ts — Linux-specific
// terminal-ime-native-text-candidates.ts — macOS native IME
// terminal-ime-native-text-forwarder.ts — forward IME text
```

---

## 6. Terminal Link Handling

```typescript
// terminal-link-handlers.ts (~12K)
// Xử lý click vào links trong terminal:

// 1. URL links (http/https):
//    - Click → system browser hoặc Browser pane
//    - Hard-wrapped URL reconstruction (multi-line URLs)

// 2. File links (file:// hoặc relative paths):
//    - Click → open trong Editor tab
//    - OSC 7 CWD tracking để resolve relative paths
//    - Path exists check trước khi render link

// 3. GitHub PR links:
//    - Click → open PullRequestPage
//    - OSC link ranges extraction

// 4. Orchestration task links:
//    - Click → navigate to orchestration task
```

---

## 7. Agent Completion Coordinator

```typescript
// src/renderer/src/components/terminal-pane/agent-completion-coordinator.ts
// ~33K — detect khi AI agent hoàn thành task

class AgentCompletionCoordinator {
  // Monitors terminal output
  // Detects "agent done" signals:
  // - Shell prompt return after agent exits
  // - Specific exit patterns (Claude: "Human: ", Codex: "$PROMPT")
  // - Timeout-based completion (no output for N seconds)

  onAgentCompleted(callback: (result: AgentCompletionResult) => void): void
  reset(): void    // khi agent mới bắt đầu

  // Completion evidence:
  // DEFINITIVE: explicit exit marker
  // PROBABILISTIC: prompt return + no recent output
  // NO_EVIDENCE: can't confirm
}
```

---

## 8. Keyboard Handling

```typescript
// src/renderer/src/components/terminal-pane/keyboard-handlers.ts
// ~23K — custom keyboard handlers

// Global shortcuts trong terminal:
// Cmd+C  → copy selection (bypass xterm default)
// Cmd+V  → paste (with large text warning)
// Cmd+F  → terminal search
// Cmd+T  → new tab
// Cmd+W  → close tab
// Ctrl+C → send SIGINT

// xterm.js bypass policy:
// Quyết định key nào bypass xterm vs native behavior
// xterm-bypass-policy.ts (~10K)
```

---

## 9. Terminal Appearance (`terminal-appearance.ts`)

```typescript
// ~16K — theme + font resolution

function resolveTerminalTheme(
  settings: GlobalSettings,
  systemPrefersDark: boolean
): ITheme {
  // Merge: user custom theme + default light/dark theme
  // Return xterm.js ITheme object (16 ANSI colors + background/foreground/...)
}

function resolveTerminalFontSettings(settings: GlobalSettings): {
  fontFamily: string
  fontSize: number
  lineHeight: number
}
```

---

## 10. Terminal Paste System

```typescript
// Paste pipeline:
// 1. Clipboard read (xterm bypass)
// 2. Content size check (> 10KB → warning dialog)
// 3. Multi-line check → bracketed paste
// 4. Encoding (Uint8Array)
// 5. Chunking (large pastes → multiple writes)
// 6. Write qua PtyTransport

// Files:
// terminal-paste-coordinator.ts  — orchestrate pipeline
// terminal-paste-runtime.ts      — platform-specific
// terminal-bracketed-paste.ts    — bracketed paste mode
// terminal-paste-chunks.ts       — chunking logic
```

---

## 11. OSC 7 (Working Directory Tracking)

```typescript
// OSC 7: shell emits current directory as URI
// "file:///Users/user/project"

// parse-osc7.ts — parse OSC 7 sequences from xterm.js
// terminal-worktree-path-link.ts — highlight worktree paths in output
// Used by:
//   - "split terminal with inherited cwd"
//   - file link click → correct relative path
//   - runtime-graph sync (update activeTerminalCwd)
```

---

## 12. WebGL Recovery

```typescript
// terminal-webgl-atlas-recovery.ts
// xterm.js WebGL renderer có thể lose context (GPU crash)
// Recovery: detect loss → reinitialize WebGL → restore terminal state

// terminal-webgl-diagnostics-breadcrumbs.ts
// Sentry breadcrumbs khi WebGL events xảy ra
```
