# TASK-BE-003.3: Instrument scrollback save (`migrateWorkspaceSessionTerminalScrollbackSnapshots`) và restore (Electron IPC)

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-003](../solutions/SOL-BE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-003.1
**Status:** ✅ Done (2026-08-04) — Confirmed drift matches doc: real `migrateWorkspaceSessionTerminalScrollbackSnapshots()` batch/no-gzip/no-relay/no-DB-table shape as described, and the `session:read-terminal-scrollback-sync` handler's real body is a one-line ternary (`event.returnValue = typeof args?.ref === 'string' ? store.readTerminalScrollbackSnapshot(args.ref) : null`), simpler than the doc's multi-step sample — restructured into the guard + span form while preserving identical null-handling semantics. Further drift: doc's sample used `Tracers.terminalReconnect`, which does not exist — reused the already-registered `Tracers.terminalReattach` (flow `terminal:reattach`, registered by the concurrent agent-domain `pty-agent-bridge.ts` work) as the closest existing "reconnect" concept per the additive-only/no-near-duplicate-entry rule; documented the substitution inline as a `Why` comment. No tracers.ts edit made. `pnpm tsc --noEmit` clean for both files. Neither file had a dedicated test file at first; created `src/main/__tests__/terminal-scrollback-snapshots.test.ts` (4 tests) and `src/main/ipc/__tests__/session.test.ts` (4 tests) in TASK-BE-003.4. Ran `persistence.test.ts` (366 tests, exercises `migrateWorkspaceSessionTerminalScrollbackSnapshots` via `Store.setWorkspaceSession`/`load`) and `ipc/register-core-handlers.test.ts` (2 tests, exercises `registerSessionHandlers`) — all pass. **Mid-task correction:** an external `git reset --hard HEAD` not run by this agent wiped both files' first instrumentation pass (reflog shows repeated "reset: moving to HEAD"); re-applied identically after confirming via fresh `Read` both files matched their original pre-edit content exactly.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "migrateWorkspaceSessionTerminalScrollbackSnapshots"
```

Symbol đã tồn tại (MODIFY case) — hàm này chạy trên MỌI lần `Store.setWorkspaceSession()/load()`. Chạy:

```
gitnexus_impact({ target: "migrateWorkspaceSessionTerminalScrollbackSnapshots", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận guard `hasPendingBuffers` không bị phá vỡ (tránh tạo span trên mọi lần gọi, over-instrumentation). Với handler IPC `session:read-terminal-scrollback-sync` (`src/main/ipc/session.ts`), cũng chạy `codegraph explore "session:read-terminal-scrollback-sync"` trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

**Quan trọng — kiến trúc thật khác với giả định ban đầu của CR-TRACE-003, PHẢI theo đúng thiết kế dưới đây, không theo mô tả gốc của CR:** CR-TRACE-003 §4 BL-TM-03 giả định 1 span bao "write-snapshot-sync" + "provider-destroy" trên 1 PTY duy nhất, đi qua `relay.call('pty.scrollback')` → gzip → DB `orca_terminal_scrollback_snapshots`. Thực tế đã xác nhận qua grep toàn bộ `src/`: **không có bảng DB đó, không có gzip, không có relay call nào**. Scrollback save chạy qua hàm **batch** `migrateWorkspaceSessionTerminalScrollbackSnapshots()` (`src/main/terminal-scrollback-snapshots.ts:173`), xử lý **N PTY cùng lúc** (toàn bộ session), được gọi từ `Store.setWorkspaceSession()`/`Store.load()` (`persistence.ts`), trigger bởi **Electron IPC** (`ipcMain.handle('session:set', ...)`, KHÔNG phải per-terminal-destroy hook, và KHÔNG gọi `provider.destroy()` ở đây. Đây là kênh Electron `ipcMain`/`contextBridge` (renderer ↔ main, cùng process) — không nằm trong 6 dòng transport của CR-TRACE-000 §3.3, xử lý như in-process (không cần wire `traceId`, chỉ 1 process tham gia).

## File: `src/main/terminal-scrollback-snapshots.ts` [MODIFY]

```typescript
import { Tracers } from '../shared/trace'
// Lưu ý: file này nằm dưới src/main/ (không phải rpc/methods), import trực tiếp
// từ '../shared/trace/tracers' theo path tương đối đúng vị trí file.

export function migrateWorkspaceSessionTerminalScrollbackSnapshots(
  session: WorkspaceSessionState,
  storage?: TerminalScrollbackSnapshotStorage
): { session: WorkspaceSessionState; changed: boolean } {
  // Why (CR-TRACE-003): guard trước khi start span — hàm này được gọi mỗi lần
  // Store.setWorkspaceSession()/load() chạy, phần lớn lời gọi không có buffer nào
  // cần ghi (đã migrate từ trước). Chỉ tạo span khi thực sự có sync I/O sắp chạy,
  // tránh vi phạm nguyên tắc chống over-instrumentation (CR-TRACE-000 §5).
  const hasPendingBuffers = Object.values(session.terminalLayoutsByTabId ?? {}).some(
    (layout) => layout.buffersByLeafId && Object.keys(layout.buffersByLeafId).length > 0
  )
  if (!hasPendingBuffers) {
    // ...existing early-return / no-op path unchanged (return { session, changed: false })...
  }

  const span = Tracers.terminalDestroy.start({ step: 'migrate-scrollback-snapshots' })
  let bytesWritten = 0
  let terminalLayoutsByTabId: WorkspaceSessionState['terminalLayoutsByTabId'] | null = null
  try {
    for (const [tabId, layout] of Object.entries(session.terminalLayoutsByTabId ?? {})) {
      const buffers = layout.buffersByLeafId
      if (!buffers || Object.keys(buffers).length === 0) {
        continue
      }
      const refs = { ...layout.scrollbackRefsByLeafId }
      const remainingBuffers: Record<string, string> = {}
      let layoutChanged = false
      for (const [leafId, buffer] of Object.entries(buffers)) {
        span.step('write-snapshot-sync', { tabId, leafId, bufferBytes: buffer.length })
        const ref = writeTerminalScrollbackSnapshotSync({ tabId, leafId, buffer, storage })
        if (ref) {
          bytesWritten += buffer.length
          refs[leafId] = ref
        }
        // ...existing refs/remainingBuffers/layoutChanged bookkeeping unchanged...
      }
      // ...existing terminalLayoutsByTabId assembly unchanged...
    }
    span.ok({ bytesWritten })
    // ...existing return { session: {...}, changed } unchanged...
  } catch (err) {
    span.fail(err, { bytesWritten })
    throw err
  }
}
```

**Ràng buộc bắt buộc:** KHÔNG thêm bước "provider-destroy" vào span này — hàm này không destroy PTY (đó là một luồng runtime riêng, `runtime.closeTerminal()`/`stopTerminalsForWorktree()`, hiện chưa được CR-TRACE-003 hay task này instrument vì không nằm trong danh sách file mà CR liệt kê). Chỉ trace phần ghi scrollback thật sự tồn tại trong code.

## File: `src/main/ipc/session.ts` [MODIFY]

Instrument restore path — lời gọi **đồng bộ, cùng process** (`event.returnValue`), không có `traceId` wire nào để propagate:

```typescript
import { Tracers } from '../../shared/trace/tracers'

ipcMain.on(
  'session:read-terminal-scrollback-sync',
  (event, args: { ref?: unknown } | undefined) => {
    const ref = typeof args?.ref === 'string' ? args.ref : null
    if (!ref) {
      event.returnValue = null
      return
    }
    const span = Tracers.terminalReconnect.start({ ref })
    try {
      span.step('read-snapshot-sync', { ref })
      const buffer = store.readTerminalScrollbackSnapshot(ref)
      span.ok({ ref, restoredBytes: buffer?.length ?? 0 })
      event.returnValue = buffer
    } catch (err) {
      span.fail(err, { ref })
      event.returnValue = null
    }
  }
)
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/__tests__/terminal-scrollback-snapshots.test.ts
pnpm test --run src/main/ipc/__tests__/session.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `migrateWorkspaceSessionTerminalScrollbackSnapshots()` chỉ tạo span khi thực sự có `buffersByLeafId` cần ghi (guard chống over-instrumentation — hàm này được gọi trên mọi `setWorkspaceSession()`)
- [ ] `span.step('write-snapshot-sync', { bufferBytes })` bao quanh mỗi lời gọi `writeTerminalScrollbackSnapshotSync()` (sync I/O có khả năng block event loop — đúng tiêu chí CR-TRACE-000 §5)
- [ ] Span không chứa bước "provider-destroy" giả định — chỉ trace đúng phần ghi scrollback thật có trong code
- [ ] `session:read-terminal-scrollback-sync` (Electron IPC, `ipc/session.ts`) phát span `terminal:reconnect` với `restoredBytes`, kể cả khi `buffer` là `null` (`restoredBytes: 0`)
- [ ] Không start span khi `ref` thiếu/invalid trong `session:read-terminal-scrollback-sync`
- [ ] Không có wire field `traceId` nào được thêm vào kênh Electron IPC này (đây là in-process, không phải cross-boundary theo CR-TRACE-000 §3.3)
