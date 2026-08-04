# TASK-BE-003.4: Viết test cho tracing terminal management

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-003](../solutions/SOL-BE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-003.1 + TASK-BE-003.2 + TASK-BE-003.3
**Status:** ✅ Done (2026-08-04) — 24 test cases across 7 files (target ≥18): `tracers.test.ts` +2 (flat file, no `__tests__` subfolder — matches its existing convention), `terminal-tracing.test.ts` 8 new (created at `src/main/runtime/rpc/methods/__tests__/terminal-tracing.test.ts`, not `__tests__/terminal.test.ts` as the doc named it — no pre-existing terminal RPC test file existed; matched the `-tracing.test.ts` suffix convention already used by sibling CR-005/014 files `git-remote-tracing.test.ts`/`credentials-tracing.test.ts`/`preflight-tracing.test.ts` in the same directory, and included 1 bonus test for the newly-discovered `ctx.userId` UNAUTHORIZED gate), `local-pty-provider.test.ts` +2 (extended the existing flat file directly, per the TASK-BE-001.4 precedent on `worktree.test.ts` — did NOT create a duplicate `__tests__/local-pty-provider.test.ts`), `ssh-pty-provider.test.ts` +3 (same — extended in place), `terminal-scrollback-snapshots.test.ts` 4 new (created — no prior test file existed), `session.test.ts` 4 new (created — no prior test file existed), OSC133 guard 1 (grep-based, created at `src/shared/__tests__/terminal-osc133-command-finished.test.ts`). All 24 new + all pre-existing tests in the 7 touched files pass (verified via full combined run: 578 tests across 17 files, 0 failures). `pnpm tsc --noEmit` clean for every touched file. Adapted the scrollback-write span.fail() test to a throwing-Proxy buffer instead of mocking `writeTerminalScrollbackSnapshotSync` (that function never actually throws — it catches fs errors internally and returns null — so its own failure path is unreachable from the caller; the Proxy exercises the surrounding write-loop's span.fail()/rethrow instead, which is the real reachable failure surface).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết/mở rộng test file — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-003.1 → 003.3 trước khi viết test:

```bash
codegraph explore "terminal.create"
codegraph explore "LocalPtyProvider.spawn"
codegraph explore "SshPtyProvider.spawn"
codegraph explore "migrateWorkspaceSessionTerminalScrollbackSnapshots"
```

## Mô tả

Viết ≥ 18 test case (Vitest) bao phủ toàn bộ instrumentation đã thêm ở TASK-BE-003.1 → 003.3: tracer registration, RPC handler spans, provider-level spans (local/ssh), guard chống over-instrumentation trên scrollback save, restore path, và guard xác nhận OSC 133 (BL-TM-04) vẫn KHÔNG bị instrument.

## File: `src/shared/trace/__tests__/tracers.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'exports Tracers.terminalCreate/Resize/Destroy/Reconnect with correct flow names'` | Convention CR-TRACE-000 §4 |

Target: ≥ 2 test.

## File: `src/main/runtime/rpc/methods/__tests__/terminal.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'terminal.create emits terminalCreate span with ok() containing ptyId'` | |
| `'terminal.create resumes span id from params.traceId'` | |
| `'terminal.split reuses Tracers.terminalCreate, not a separate tracer'` | Guard chống tạo `terminal:split` riêng |
| `'terminal.resizeForClient emits terminalResize span distinct from terminalCreate'` | |
| `'terminal.resizeForClient span.fail() called on no_connected_pty before throwing'` | |

Target: ≥ 5 test.

## File: `src/main/providers/__tests__/local-pty-provider.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'spawn() emits terminalCreate span with providerType=local'` | |
| `'spawn() span.fail() on underlying pty spawn error, providerType field present'` | |

Target: ≥ 2 test.

## File: `src/main/providers/__tests__/ssh-pty-provider.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'spawn() emits terminalCreate span with providerType=ssh'` | |
| `'spawn() span does not attempt to propagate traceId into remote shell (no wire field added)'` | Guard xác nhận đúng CR-TRACE-000 §3.3 dòng SSH exec |

Target: ≥ 2 test.

## File: `src/main/__tests__/terminal-scrollback-snapshots.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'migrateWorkspaceSessionTerminalScrollbackSnapshots skips span entirely when no buffersByLeafId pending'` | Guard chống over-instrumentation |
| `'migrateWorkspaceSessionTerminalScrollbackSnapshots emits terminalDestroy span with step write-snapshot-sync per leaf, ok() bytesWritten aggregated'` | |
| `'migrateWorkspaceSessionTerminalScrollbackSnapshots span.fail() on writeTerminalScrollbackSnapshotSync throw'` | |

Target: ≥ 3 test.

## File: `src/main/ipc/__tests__/session.test.ts` [NEW hoặc MODIFY nếu đã tồn tại]

| Test case | Mục tiêu |
|---|---|
| `'session:read-terminal-scrollback-sync emits terminalReconnect span with restoredBytes'` | |
| `'session:read-terminal-scrollback-sync span.ok() with restoredBytes=0 when ref not found (buffer null)'` | |
| `'session:read-terminal-scrollback-sync does not start a span when ref is missing/invalid'` | |

Target: ≥ 3 test.

## File: `src/shared/__tests__/terminal-osc133-command-finished.test.ts` [MODIFY — regression guard, không phải test mới cho tính năng]

| Test case | Mục tiêu |
|---|---|
| `'no Tracers.* call anywhere in osc133 scanning path'` | Xác nhận BL-TM-04 vẫn không bị instrument (grep-based guard test) |

Target: 1 test.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/terminal.test.ts
pnpm test --run src/main/providers/__tests__/local-pty-provider.test.ts
pnpm test --run src/main/providers/__tests__/ssh-pty-provider.test.ts
pnpm test --run src/main/__tests__/terminal-scrollback-snapshots.test.ts
pnpm test --run src/main/ipc/__tests__/session.test.ts
pnpm test --run src/shared/__tests__/terminal-osc133-command-finished.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] ≥ 18 test case mới/mở rộng tổng cộng trên 7 file, đúng breakdown: `tracers.test.ts` ≥ 2, `terminal.test.ts` ≥ 5, `local-pty-provider.test.ts` ≥ 2, `ssh-pty-provider.test.ts` ≥ 2, `terminal-scrollback-snapshots.test.ts` ≥ 3, `session.test.ts` ≥ 3, OSC133 guard = 1
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] KHÔNG có span/tracer nào được tạo cho việc scan OSC 133 trên mỗi PTY chunk (BL-TM-04) — verify bằng test guard dựa trên grep
- [ ] Có test guard xác nhận `migrateWorkspaceSessionTerminalScrollbackSnapshots` bỏ qua hoàn toàn việc tạo span khi không có buffer cần ghi
