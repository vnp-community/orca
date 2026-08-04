# TASK-FE-003.2: Instrument `resize()`/`claimViewport()` (chỉ nhánh claim) + scrollback save trong `disconnect()`

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-003 §2.3, §2.5](../solutions/SOL-FE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-003.1 (dùng tracer `terminalResize`/`terminalDestroy` đã khai báo)
**Status:** ✅ Done (2026-08-03) — Used `Tracers.uiTerminalResizeFlow`/`uiTerminalDestroyFlow` (see TASK-FE-003.1 note on the ui: collision resolution) instead of the task doc's `terminalResize`/`terminalDestroy` names. Instrumented `resize()` (claim-only branch) and `claimViewport()` with `ui:terminal.resize`, and the `disconnect()` scrollback-save fire-and-forget path with `ui:terminal.destroy` (route: 'scrollback-save'). Confirmed real RPC method is `terminal.updateViewport`, not `terminal.resizeForClient` (backend CR-TRACE-003 drift, noted per task instructions, not fixed). Added 6 new tests to `remote-runtime-pty-transport.test.ts` (unclaimed resize → no span, claimed resize/claimViewport → span+ok, disconnect without tabId → no scrollback span, disconnect with save success → ok+bufferBytes, disconnect with save rejection → fail without throw). `pnpm tsc --noEmit` clean, 60/60 tests pass (concurrent edits to this file and tracers.ts during the session did not clobber this work, re-verified after each).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "createRemoteRuntimePtyTransport"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "createRemoteRuntimePtyTransport", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa — cụ thể các method `resize`/`claimViewport`/`disconnect` trong closure này. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

**BL-TM-02 (resize):** CR-TRACE-003 (backend) chỉ đích danh RPC `terminal.resizeForClient`, nhưng method đó **không có call site nào** trong renderer. Đường resize thật gọi `terminal.updateViewport` (`sendViewportUpdate()`, `remote-runtime-pty-transport.ts:408-429`) — **lệch tên method so với backend CR**, cần đồng bộ lại với companion backend CR trước khi backend thêm tracer cho method sai. Resize thường (kéo pane) đi qua `viewportBatcher` (flush mỗi 33ms) — **KHÔNG instrument** (chống over-instrumentation). Chỉ nhánh `claim === true` (giành quyền viewport) được span, vì đây là điểm rẽ nhánh quan trọng.

**Scrollback save:** fire-and-forget trong `disconnect()`, khác với backend's `writeTerminalScrollbackSnapshotSync` (đồng bộ) — đây là async, qua `window.api.terminalSessions.save` (Electron contextBridge IPC, tag `BUG-FE-TM-003` trong preload).

## File: `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` [MODIFY]

```typescript
resize(cols: number, rows: number, meta): boolean {
  if (!connected || !handle) {
    return false
  }
  rememberViewport(cols, rows)
  if (meta?.claim) {
    // Why: claim-viewport là một điểm rẽ nhánh quan trọng — đáng span. Resize
    // thường (không claim) đi qua viewportBatcher ở tần suất cao trong lúc kéo
    // pane — KHÔNG instrument, đúng nguyên tắc chống over-instrumentation.
    const span = Tracers.terminalResize.start({ worktreeId: worktreeId ?? '', cols, rows, claim: true })
    viewportBatcher.clear()
    sendViewportUpdate(cols, rows, true)
    span.ok({ cols, rows })
    return true
  }
  viewportBatcher.queue(cols, rows)
  return true
},

claimViewport(cols: number, rows: number): boolean {
  if (!connected || !handle) {
    return false
  }
  rememberViewport(cols, rows)
  viewportBatcher.clear()
  const span = Tracers.terminalResize.start({ worktreeId: worktreeId ?? '', cols, rows, claim: true })
  sendViewportUpdate(cols, rows, true)
  span.ok({ cols, rows })
  return true
},
```

> `sendViewportUpdate()` tự nó là fire-and-forget (`.catch(() => {})`) — không await được kết quả thật của `terminal.updateViewport`. `span.ok()` ở đây chỉ xác nhận "đã gửi request", không xác nhận "server đã áp dụng" — hạn chế có sẵn của code hiện tại, không claim `verified: true` ở bất kỳ đâu.

```typescript
disconnect() {
  inputBatcher.flush()
  inputBatcher.clear()
  viewportBatcher.flush()
  outputProcessor.clearAccumulatedState()
  if (!connected && !handle) {
    return
  }
  connected = false
  clearPendingViewportClaim()
  const id = remotePtyId
  if (id && worktreeId && tabId) {
    const stream = multiplexedStream
    if (stream) {
      // Why: không mở span riêng cho save — fire-and-forget best-effort, nhưng
      // vẫn đáng 1 span nhẹ vì đây là sync-adjacent I/O có thể chậm nếu buffer lớn.
      const span = Tracers.terminalDestroy.start({ ptyId: id, route: 'scrollback-save', tabId })
      stream.serializeBuffer?.({ scrollbackRows: 1000 })
        .then(snap => {
          if (snap && worktreeId && tabId) {
            void window.api.terminalSessions?.save?.({
              worktreeId, tabId, leafId: leafId ?? undefined,
              snapshotData: snap.data, snapshotCols: snap.cols, snapshotRows: snap.rows,
            }).then(() => span.ok({ ptyId: id, bufferBytes: snap.data.length }))
              .catch((err) => span.fail(err, { ptyId: id }))
          } else {
            span.ok({ ptyId: id, skipped: true })
          }
        })
        .catch((err) => span.fail(err, { ptyId: id }))
    }
  }
  closeMultiplexedStream()
  handle = null
  remotePtyId = null
  storedCallbacks.onDisconnect?.()
  if (id) {
    onPtyExit?.(id)
  }
}
```

> `route: 'scrollback-save'` dùng cùng tracer `terminalDestroy` nhưng field `route` khác 2 route xử lý ở TASK-FE-003.3 — cho phép TracePanel lọc riêng "chỉ xem các lần save scrollback".

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `resize()`/`claimViewport()` chỉ mở span `ui:terminal.resize` cho nhánh `claim === true` — resize thường qua `viewportBatcher` (33ms) KHÔNG có span, xác nhận bằng code review + test đếm số lần `Tracers.terminalResize.start` được gọi khi giả lập một chuỗi resize kéo pane
- [ ] `claimViewport()` luôn mở span `ui:terminal.resize`
- [ ] Scrollback save trong `disconnect()` có span riêng route `'scrollback-save'`, field `bufferBytes` khi có, không throw nếu `window.api.terminalSessions.save` reject (giữ nguyên hành vi "Non-fatal" hiện tại)
- [ ] `disconnect()` không có `worktreeId`/`tabId` → không mở span save (nhánh skip)
- [ ] Ghi rõ trong PR/commit rằng RPC method resize thật là `terminal.updateViewport`, KHÔNG phải `terminal.resizeForClient` như CR-TRACE-003 (backend) đặt tên ban đầu — cần đồng bộ lại với companion backend CR
- [ ] Test suite đạt ≥ 4 test case mới: claim mở span/resize thường không mở span, `disconnect()` mở span save + `ok()` khi thành công, không mở span khi thiếu `worktreeId`/`tabId`
