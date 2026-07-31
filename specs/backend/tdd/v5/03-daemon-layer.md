# TDD-03: PTY Daemon Layer

**Document:** TDD-03  
**Domain:** PTY Daemon — Out-of-process Terminal Engine  
**Source files:** `src/main/daemon/`  

---

## 1. Lý do tách Daemon thành process riêng

```
// src/main/daemon/daemon-init.ts (comment)
// "daemon init runs concurrently with window load"
// "owns the complete daemon lifecycle — init, out-of-process launch,
//  current+legacy adapter wiring, restart orchestration (the 7-step sequence
//  from docs/daemon-staleness-ux.md §Phase 1), and teardown on app quit"
```

**Lý do kỹ thuật:**
1. **Isolation:** Crash trong PTY không kill Electron app
2. **Restart:** Daemon có thể restart độc lập, giữ PTY sessions
3. **Upgrade:** New daemon version deploy mà không close terminal
4. **Long-running:** Terminal sessions sống qua nhiều Orca restarts

---

## 2. Daemon Process Architecture

```
Main Process (Electron)
  │
  │ fork('daemon-entry.js') + Unix socket auth token
  │
  ▼
Daemon Process (plain Node.js)
  │
  ├─ DaemonServer          — Unix socket server (NDJSON protocol)
  ├─ DaemonPtyProvider     — quản lý node-pty instances
  ├─ HistoryManager        — terminal scrollback persistence
  ├─ HeadlessEmulator      — off-screen terminal rendering
  └─ SessionManager        — terminal session lifecycle
```

---

## 3. Startup và Lifecycle

### 3.1 Fork daemon

```typescript
// src/main/daemon/daemon-init.ts
export async function initDaemonPtyProvider(): Promise<void> {
  spawner = new DaemonSpawner({
    entryPath: join(app.getPath('exe'), '../resources/app.asar.unpacked/out/main/daemon-entry.js'),
    // Hoặc dev: out/main/daemon-entry.js
  })

  // Spawn với auth token
  const handle = await spawner.launch()

  // Kết nối tới daemon socket
  adapter = new DaemonPtyAdapter(handle.socketPath, handle.authToken)
  await adapter.connect()

  // Đặt làm local PTY provider
  setLocalPtyProvider(adapter)
}
```

### 3.2 Daemon entry

```typescript
// src/main/daemon/daemon-entry.ts
async function main() {
  const server = new DaemonServer({
    socketPath: getDaemonSocketPath(),
    authToken: process.env['ORCA_DAEMON_TOKEN']!
  })
  await server.start()

  // Signal handlers
  process.on('SIGTERM', () => server.shutdown())
  process.on('SIGINT', () => server.shutdown())
}
```

### 3.3 Restart sequence (7-step)

```typescript
// src/main/daemon/daemon-init.ts
async function restartDaemon(): Promise<RestartDaemonResult> {
  // 1. Coalesce concurrent calls
  if (restartInFlight) return restartInFlight

  restartInFlight = (async () => {
    // 2. Capture current PTY state
    const currentPtys = adapter?.listActivePtys() ?? []

    // 3. Kill current adapter
    await disconnectDaemon()

    // 4. Kill stale daemon if still running
    await killStaleDaemon()

    // 5. Spawn new daemon
    const handle = await spawner!.launch()

    // 6. Connect new adapter
    const newAdapter = new DaemonPtyAdapter(handle.socketPath, handle.authToken)
    await newAdapter.connect()

    // 7. Rebind provider listeners
    replaceDaemonProvider(newAdapter)
    setLocalPtyProvider(newAdapter)

    return { restarted: true, restoredPtys: currentPtys.length }
  })()

  try {
    return await restartInFlight
  } finally {
    restartInFlight = null
  }
}
```

---

## 4. Protocol: Main Process ↔ Daemon

**Transport:** Unix domain socket  
**Format:** NDJSON (newline-delimited JSON)  
**Auth:** Bearer token (random hex string)

```typescript
// src/main/daemon/types.ts
const PROTOCOL_VERSION = 42  // bump when breaking changes

// Request từ Main Process:
type DaemonRequest = {
  id: string
  method: string    // 'pty.create' | 'pty.write' | 'pty.resize' | ...
  params: unknown
  token: string     // auth token
}

// Response từ Daemon:
type DaemonResponse =
  | { id: string; result: unknown }
  | { id: string; error: { code: string; message: string } }

// Stream events (không có id):
type DaemonEvent = {
  event: 'pty.data' | 'pty.exit' | 'session.update'
  payload: unknown
}
```

### Method list

| Method | Direction | Params |
|--------|-----------|--------|
| `pty.create` | Main → Daemon | `{ shell, cwd, env, cols, rows }` |
| `pty.write` | Main → Daemon | `{ ptyId, data: Uint8Array }` |
| `pty.resize` | Main → Daemon | `{ ptyId, cols, rows }` |
| `pty.kill` | Main → Daemon | `{ ptyId, signal? }` |
| `session.list` | Main → Daemon | `{}` |
| `session.reattach` | Main → Daemon | `{ sessionId }` |
| `history.read` | Main → Daemon | `{ ptyId, offset, length }` |
| `pty.data` (event) | Daemon → Main | `{ ptyId, data: Uint8Array }` |
| `pty.exit` (event) | Daemon → Main | `{ ptyId, exitCode, signal }` |

---

## 5. PTY Subprocess (`daemon/pty-subprocess.ts`)

```typescript
// src/main/daemon/pty-subprocess.ts (~46K)
class PtySubprocess {
  private pty: IPty         // node-pty instance
  private emulator: HeadlessEmulator  // terminal state machine

  async create(opts: PtyCreateOptions): Promise<string> {
    this.pty = spawn(opts.shell, [], {
      cwd: opts.cwd,
      env: opts.env,
      cols: opts.cols ?? 80,
      rows: opts.rows ?? 24
    })

    // Pipe pty output → emulator → history
    this.pty.onData(data => {
      this.emulator.write(data)
      this.historyManager.append(this.ptyId, data)
      this.emit('data', { ptyId: this.ptyId, data })
    })

    this.pty.onExit(({ exitCode, signal }) => {
      this.emit('exit', { ptyId: this.ptyId, exitCode, signal })
    })
  }
}
```

---

## 6. HeadlessEmulator (`daemon/headless-emulator.ts`)

Orca có **custom terminal emulator** được implement hoàn toàn trong code:

```typescript
// src/main/daemon/headless-emulator.ts (~19K)
class HeadlessEmulator {
  // Xterm-js compatible terminal emulation
  // Không cần DOM — thuần JS state machine
  private buffer: CellBuffer    // terminal grid
  private cursor: Cursor
  private scrollbackBuffer: ScrollbackBuffer
  private osc7Uri: string       // current working directory

  write(data: string | Uint8Array): void {
    // VT100/ANSI/xterm sequences parsing
    // OSC (Operating System Command) handling
    // CSI (Control Sequence Introducer) handling
    // Update cell buffer
  }

  snapshot(): TerminalSnapshot {
    return {
      lines: this.buffer.toLines(),
      cursor: this.cursor,
      scrollback: this.scrollbackBuffer.tail(1000)
    }
  }
}
```

---

## 7. History Manager (`daemon/history-manager.ts`)

```typescript
// src/main/daemon/history-manager.ts (~14K)
class HistoryManager {
  // Ring buffer: giới hạn scrollback memory
  // Persist tới disk: ~/.config/orca/terminal-history/
  // Compress với LZ4: tiết kiệm disk
  // Indexed: O(1) seek theo line number

  append(ptyId: string, data: string): void
  readRange(ptyId: string, from: number, to: number): string[]
  snapshot(ptyId: string): TerminalHistorySnapshot
}
```

---

## 8. Degraded Mode

Nếu daemon crash và không restart được:

```typescript
// src/main/daemon/degraded-daemon-pty-provider.ts
class DegradedDaemonPtyProvider {
  // Fallback: dùng DegradedPtyProvider
  // Không có: history, snapshot, session persistence
  // Có: basic PTY create/write/resize/kill
  // Hiển thị error banner trong UI
}
```

---

## 9. Daemon Staleness Detection

```typescript
// src/main/daemon/daemon-health.ts (~20K)
// Kiểm tra daemon có cần restart không:

isDaemonStaleForCurrentBundle(daemonInfo: DaemonLaunchIdentity): boolean {
  // So sánh: bundle version của daemon vs hiện tại
  // Nếu Orca được update → daemon version cũ → stale
  // → trigger restart sequence
}

checkDaemonHealth(socketPath: string): Promise<DaemonHealthResult> {
  // Kết nối tới daemon socket
  // Gửi ping request
  // Nếu timeout → unhealthy
}
```
