# TASK-BE-003.1: Đăng ký tracer `terminal:*` và instrument RPC handler `terminal.create`/`.split`/`.resizeForClient`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-003](../solutions/SOL-BE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-003)
**Status:** ✅ Done (2026-08-04) — Drift: `Tracers.terminalCreate/Resize/Destroy` and `terminalReattach` (flow `terminal:reattach`, NOT `terminal:reconnect` as the doc names it) were already registered in `tracers.ts` by the concurrent agent-domain `pty-agent-bridge.ts` work — reused as-is, no tracers.ts edit made (additive-only rule: don't rename an entry you didn't create). `terminal.split`'s `span.ok()` logs `handle` (not `ptyId` — `RuntimeTerminalSplit` has no `ptyId` field, only `TerminalCreateParams`'s handler does). **Mid-task correction:** an external `git reset --hard HEAD` (not run by this agent — visible via repeated "reset: moving to HEAD" reflog entries, see 00-index.md addendum) wiped this file's first pass, and a `git pull` that landed concurrently added a NEW `ctx.userId` UNAUTHORIZED gate to `terminal.create` (`FIX TASK-TRM-006`) that did not exist in the version originally read — the re-applied edit keeps that gate and wraps it so an unauthorized attempt still emits `span.fail('UNAUTHORIZED: ...')` before throwing, span opened before the gate check. `pnpm tsc --noEmit` (via `typecheck:node`) has pre-existing unrelated failures in `src/renderer/src/store/*` (tsconfig file-list issue, confirmed via `git status` unmodified) — none reference `terminal.ts`. Test file created at `__tests__/terminal-tracing.test.ts` (not `__tests__/terminal.test.ts` as the doc named it — no pre-existing terminal RPC test file existed at either path; matches the `-tracing.test.ts` suffix convention already established by sibling CR-005/014 files in the same `__tests__/` dir) in TASK-BE-003.4, 8 tests, all pass. `gitnexus_detect_changes` confirmed only `TerminalSplit`/`TERMINAL_METHODS` symbols touched in this file on the first pass (pre-reset); re-verified via `git status`/`git diff` after re-applying (no gitnexus re-run to avoid another giant repo-wide diff dump).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "terminal.create"
codegraph explore "terminal.split"
codegraph explore "terminal.resizeForClient"
```

Cả 3 là RPC handler đã tồn tại (MODIFY case). Chạy:

```
gitnexus_impact({ target: "terminal.create", direction: "upstream" })
gitnexus_impact({ target: "terminal.resizeForClient", direction: "upstream" })
```

(Phần đăng ký tracer mới vào object `Tracers` đã tồn tại là thay đổi additive-only, không cần impact riêng — chỉ cần `codegraph explore "Tracers"` để tránh trùng tên entry.) Báo cáo blast radius trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 4 tracer `terminal:create|resize|destroy|reconnect` trong `tracers.ts`, thêm field `traceId?` vào schema `terminal.create`, và bọc 3 RPC handler `terminal.create`/`terminal.split`/`terminal.resizeForClient` (`src/main/runtime/rpc/methods/terminal.ts`). `terminal.split` dùng lại `Tracers.terminalCreate` (KHÔNG tạo tracer `terminal:split` riêng — về bản chất split là một lần create PTY, chỉ khác context UI).

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các entry từ SOL-BE-TRACE-001/002: `worktree:*`, `agentOrch:*`, cùng entry gốc):

```typescript
export const Tracers = {
  // ...existing entries (worktree:*, agentOrch:*, devServer:*, ...) unchanged...

  // ─── CR-TRACE-003: Terminal Management (BL-TM-01→04) ───────────────────────
  /** terminal.create + terminal.split — BL-TM-01/02 */
  terminalCreate:    createTracer('terminal:create'),
  /** terminal.resizeForClient — BL-TM-02 */
  terminalResize:    createTracer('terminal:resize'),
  /** scrollback save (migrateWorkspaceSessionTerminalScrollbackSnapshots write path) — BL-TM-03 */
  terminalDestroy:   createTracer('terminal:destroy'),
  /** scrollback restore (readTerminalScrollbackSnapshotSync) — BL-TM-03 */
  terminalReconnect: createTracer('terminal:reconnect'),
} as const
```

## File: `src/main/runtime/rpc/methods/terminal.ts` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// TerminalCreateParams — thêm field:
const TerminalCreateParams = z.object({
  // ...existing fields (worktree, command, env, launchConfig, ...) unchanged...
  traceId: OptionalString, // [NEW CR-TRACE-003]
})

// ...

defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, ctx) => {
    if (!ctx.userId) {
      throw new Error('UNAUTHORIZED: terminal.create requires an authenticated session. Please log in.')
    }
    const span = Tracers.terminalCreate.start(
      { worktree: params.worktree ?? '' },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const { runtime } = ctx
      const terminal = await runtime.createTerminal(params.worktree, {
        // ...existing options (command, env, launchConfig, title, focus, ...) unchanged...
      })
      // Why: createTerminal() tự chọn Local/SSH provider nội bộ (resolveExecutionHost);
      // providerType không lộ ra qua kết quả trả về hôm nay — dùng field terminal.ptyId
      // prefix hoặc runtime.resolveExecutionHost(params.worktree) để suy ra nếu cần chi tiết
      // hơn khi implement thật (xem TASK-BE-003.2 cho instrumentation trực tiếp tại provider).
      span.ok({ ptyId: terminal.ptyId })
      return { terminal }
    } catch (err) {
      span.fail(err, { worktree: params.worktree ?? '' })
      throw err
    }
  }
}),

defineMethod({
  name: 'terminal.split',
  params: TerminalSplit,
  handler: async (params, { runtime }) => {
    const span = Tracers.terminalCreate.start({ terminal: params.terminal, direction: params.direction ?? '' })
    try {
      const split = await runtime.splitTerminal(params.terminal, {
        direction: params.direction,
        command: params.command,
        env: params.env,
        telemetrySource: params.telemetrySource
      })
      span.ok({ ptyId: split.ptyId })
      return { split }
    } catch (err) {
      span.fail(err, { terminal: params.terminal })
      throw err
    }
  }
}),

// ...

defineMethod({
  name: 'terminal.resizeForClient',
  params: TerminalResizeForClient,
  handler: async (params, { runtime }) => {
    const span = Tracers.terminalResize.start({ terminal: params.terminal, mode: params.mode })
    const leaf = runtime.resolveLiveLeafForHandle(params.terminal)
    if (!leaf?.ptyId) {
      span.fail('no_connected_pty', { terminal: params.terminal })
      throw new Error('no_connected_pty')
    }
    try {
      const result = await runtime.resizeForClient(
        leaf.ptyId,
        params.mode,
        params.clientId,
        params.mode === 'mobile-fit' ? params.cols : undefined,
        params.mode === 'mobile-fit' ? params.rows : undefined
      )
      span.ok({ ptyId: leaf.ptyId })
      return {
        terminal: {
          handle: params.terminal,
          ...result
        }
      }
    } catch (err) {
      span.fail(err, { terminal: params.terminal })
      throw err
    }
  }
})
```

**Lưu ý quan trọng:** `terminal.split` dùng lại `Tracers.terminalCreate` — KHÔNG tạo tracer `terminal:split` riêng (đúng nguyên tắc CR-TRACE-003 §3).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/runtime/rpc/methods/__tests__/terminal.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.terminalCreate`, `terminalResize`, `terminalDestroy`, `terminalReconnect` tồn tại trong `tracers.ts` đúng flow name `terminal:create|resize|destroy|reconnect`
- [ ] Handler `terminal.create` và `terminal.split` (`terminal.ts`) đều dùng `Tracers.terminalCreate` — không có tracer `terminal:split` riêng
- [ ] `terminal.create` resume span id từ `params.traceId` khi có
- [ ] Handler `terminal.resizeForClient` phát span `terminal:resize` riêng biệt, `span.fail('no_connected_pty', ...)` được gọi trước khi throw khi không tìm thấy leaf/ptyId
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
