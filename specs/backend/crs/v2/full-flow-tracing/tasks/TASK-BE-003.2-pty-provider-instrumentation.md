# TASK-BE-003.2: Instrument `LocalPtyProvider.spawn()` / `SshPtyProvider.spawn()`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-003](../solutions/SOL-BE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-003.1
**Status:** ✅ Done (2026-08-04) — Drift: reused already-registered `Tracers.terminalCreate` (`terminal:create`), no tracers.ts edit needed. Both `spawn()` methods are large (LocalPtyProvider.spawn() ~500 lines with 2 return points; SshPtyProvider.spawn() has a nested try/catch for the sessionId-reattach path) — wrapped the FULL method body in an outer try/catch rather than the doc's simplified 1-return-point sample, with `span.ok()` at every real return point (local: reattach-return + final-return; ssh: attach-return + spawn-return) and a single `span.fail()` at the outer catch only (an earlier draft double-fired `span.fail()` from both the inner SSH-reattach catch and the outer catch for the same span id — fixed by letting the inner catch only transform/rethrow the error, not call span.fail() itself). Ran `oxfmt --write` on `local-pty-provider.ts` only (not repo-wide, to avoid touching concurrent agents' files) to reindent the body inside the new try block. `PtySpawnOptions`/`PtySpawnResult` shapes untouched — confirmed no `traceId` field added, no wire field added to the SSH `pty.attach`/`pty.spawn` RPC payloads. Verification test paths in the doc (`__tests__/local-pty-provider.test.ts`, `__tests__/ssh-pty-provider.test.ts`) don't exist — actual files are `src/main/providers/local-pty-provider.test.ts` and `ssh-pty-provider.test.ts` (no `__tests__` subfolder); extended those directly in TASK-BE-003.4 (5 new tests total) rather than creating new files, matching the precedent set by TASK-BE-001.4 on `worktree.test.ts`. **Mid-task correction:** an external `git reset --hard HEAD` not run by this agent wiped both files' first instrumentation pass (visible via reflog "reset: moving to HEAD" entries) — re-applied identically after confirming via fresh `Read` that HEAD's `spawn()` bodies were otherwise unchanged. `pnpm tsc --noEmit` clean for both files; final combined run (local-pty-provider.test.ts 60, ssh-pty-provider.test.ts 43, provider-dispatch.test.ts 6) all pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "LocalPtyProvider.spawn"
codegraph explore "SshPtyProvider.spawn"
```

Cả 2 là method đã tồn tại, cùng implement interface `IPtyProvider` (MODIFY case). Chạy:

```
gitnexus_impact({ target: "LocalPtyProvider.spawn", direction: "upstream" })
gitnexus_impact({ target: "SshPtyProvider.spawn", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — đặc biệt xác nhận `PtySpawnOptions` (interface dùng chung) không bị đổi shape. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

`OrcaRuntimeService.createTerminal()` (26K dòng, nhiều nhánh `shouldCreateInBackground`/renderer-backed) không có một điểm "chọn provider" đơn giản để chèn `span.step()` an toàn mà không đọc toàn bộ hàm — vì vậy solution này instrument **trực tiếp bên trong từng provider's `spawn()`**, đã xác nhận qua Read trực tiếp source là cả `LocalPtyProvider` và `SshPtyProvider` cùng implement interface `IPtyProvider` với cùng tên method `spawn()` (CR-TRACE-003 §4 BL-TM-01 gốc ghi "chưa xác định tên hàm chính xác" — nay đã xác nhận).

**Thiết kế quan trọng cần tuân thủ đúng:** 2 span `provider-spawn` này **KHÔNG `resume`** từ span `terminal:create` ở RPC layer (TASK-BE-003.1) vì `PtySpawnOptions` không mang `traceId` xuyên qua `createTerminal()` — mở rộng `PtySpawnOptions` để thêm field đó có blast radius lớn (dùng chung bởi daemon adapter, agent-teams launch, background terminal spawn, ...), nằm ngoài phạm vi "additive-only, không đổi business logic" của CR-TRACE-003. 2 span xuất hiện **độc lập** trong TracePanel, join bằng field `ptyId` chung — đây là gap được flag rõ ràng, không phải thiếu sót cần "sửa thêm" trong task này.

## File: `src/main/providers/local-pty-provider.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

export class LocalPtyProvider implements IPtyProvider {
  // ...existing fields unchanged...

  async spawn(args: PtySpawnOptions): Promise<PtySpawnResult> {
    // Why: span không nhận traceId ở đây — PtySpawnOptions không có field trace
    // (interface dùng chung cho cả 2 provider, thay đổi shape sẽ ảnh hưởng rộng).
    // Thay vào đó dùng span rời (không resume) chỉ để đo latency + phân biệt provider,
    // join với span RPC cha (terminal:create) qua field ptyId ở phía log/TracePanel.
    const span = Tracers.terminalCreate.start({ providerType: 'local', step: 'provider-spawn' })
    try {
      // ...existing reattach / allocatePtyId / spawn logic unchanged...
      const result = /* existing return value */
      span.ok({ providerType: 'local', ptyId: result.id })
      return result
    } catch (err) {
      span.fail(err, { providerType: 'local' })
      throw err
    }
  }
}
```

## File: `src/main/providers/ssh-pty-provider.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

export class SshPtyProvider implements IPtyProvider {
  // ...existing fields unchanged...

  async spawn(opts: PtySpawnOptions): Promise<PtySpawnResult> {
    // Why: theo CR-TRACE-000 §3.3 dòng cuối — traceId KHÔNG lan vào remote shell.
    // Span này chỉ đo phía Main (mux.request('pty.attach'/'pty.create', ...) round-trip
    // qua SshChannelMultiplexer), không kỳ vọng resume tiếp bên trong remote host.
    const span = Tracers.terminalCreate.start({ providerType: 'ssh', step: 'provider-spawn' })
    try {
      // ...existing reattach-via-pty.attach / spawn-via-pty.create logic unchanged...
      const result = /* existing return value */
      span.ok({ providerType: 'ssh', ptyId: result.id })
      return result
    } catch (err) {
      span.fail(err, { providerType: 'ssh' })
      throw err
    }
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/providers/__tests__/local-pty-provider.test.ts
pnpm test --run src/main/providers/__tests__/ssh-pty-provider.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `LocalPtyProvider.spawn()` và `SshPtyProvider.spawn()` đều phát span `terminal:create` (dùng lại `Tracers.terminalCreate`, không tạo tracer riêng) với field `providerType` phân biệt (`'local'`/`'ssh'`)
- [ ] Cả 2 span KHÔNG `resume` từ span `terminal:create` ở RPC layer — đây là 2 span độc lập, không truyền `traceId` xuyên `PtySpawnOptions`
- [ ] `SshPtyProvider.spawn()` KHÔNG thêm bất kỳ field wire nào để propagate `traceId` vào remote shell (đúng CR-TRACE-000 §3.3 — traceId không lan vào SSH exec)
- [ ] `span.fail()` chứa field `providerType` tương ứng khi lỗi
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
