# TASK-BE-002.4: Viết test cho tracing agent orchestration

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-002](../solutions/SOL-BE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-002.1 + TASK-BE-002.2 + TASK-BE-002.3
**Status:** ✅ Done (2026-08-04) — Added 20 new tests total (exceeds ≥16), spread across the ACTUAL existing file paths rather than the task doc's assumed ones (`tracers.test.ts` at `src/shared/trace/` not a `__tests__` subdir; `ProfileAwareAgentSpawner.test.ts` already existed, MODIFY not NEW; RPC handler test files are named `project-rpc.test.ts`/`task-rpc.test.ts`, not `project-rpc-handler.test.ts`/`task-rpc-handler.test.ts`): `tracers.test.ts` +2, `ProfileAwareAgentSpawner.test.ts` +6, `project-rpc.test.ts` +2, `TaskAgentExecutor.test.ts` +3 (including a static-source guard reading the .ts file to confirm no `createTracer`/`Tracers.*` usage), `task-rpc.test.ts` +2, `dev-server-relay-bridge.test.ts` +2 regression tests for the CR-TRACE-002 dual-field (`traceId` + `_trace.id`) `agent.exec` params not breaking bridge resume. All new tests pass (70 total in the affected files, 67 pass + 3 pre-existing unrelated `workdir`-field failures confirmed present before this task too). typecheck clean aside from the same pre-existing errors. detect_changes (staged) confirms LOW risk, test-only files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết/mở rộng test file — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-002.1 → 002.3 trước khi viết test:

```bash
codegraph explore "ProfileAwareAgentSpawner.spawn"
codegraph explore "TaskAgentExecutor.executeTask"
```

## Mô tả

Viết ≥ 16 test case (Vitest) bao phủ toàn bộ instrumentation đã thêm ở TASK-BE-002.1 → 002.3: tracer registration, thứ tự các bước trong `spawn()`, guard bảo mật (không log credential), resume qua 2 caller, và regression test đảm bảo tương thích với `DevServerRelayBridge` từ SOL-BE-TRACE-001.

## File: `src/shared/trace/__tests__/tracers.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'exports Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll with correct flow names'` | Verify convention CR-TRACE-000 §4, không trùng `agent:rpc` |

Target: ≥ 2 test.

## File: `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` [NEW]

| Test case | Mục tiêu |
|---|---|
| `'spawn() emits agentOrch:spawn span wrapping resolve-context → resolve-provider → relay-agent-exec steps in order'` | Mock `Tracers.agentOrchSpawn`, assert thứ tự `step()` calls |
| `'spawn() ok() contains sessionId only, never env/profileEnv/credentials'` | Guard bảo mật — assert field span không chứa key nhạy cảm |
| `'spawn() resumes span id from options.traceId when provided'` | Assert `start(fields, { id })` |
| `'spawn() sends both traceId (flat) and _trace.id (nested) to relay.call agent.exec'` | Assert cả 2 field cùng có mặt trong params gửi xuống relay |
| `'spawn() fail() propagates original error and projectId field on getProjectContext rejection'` | |
| `'spawn() fail() propagates on relay.call agent.exec rejection'` | |

Target: ≥ 6 test.

## File: `src/main/project/__tests__/project-rpc-handler.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'project.agentSpawn forwards traceId from params into agentSpawner.spawn()'` | Spy vào `agentSpawner.spawn`, assert `traceId` có trong argument |

Target: ≥ 2 test.

## File: `src/main/task/__tests__/TaskAgentExecutor.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'executeTask forwards traceId to agentSpawner.spawn() without creating its own span'` | Assert không có `createTracer`/`Tracers.*` nào được gọi trong `TaskAgentExecutor` |

Target: ≥ 2 test.

## File: `src/main/task/__tests__/task-rpc-handler.test.ts` [MODIFY]

| Test case | Mục tiêu |
|---|---|
| `'task.execute accepts optional traceId param and forwards to executor.executeTask'` | |

Target: ≥ 2 test.

## File: `src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts` [MODIFY — mở rộng, chung với SOL-BE-TRACE-001/TASK-BE-001.4]

| Test case | Mục tiêu |
|---|---|
| `'callWithTimeout resumes relay:agentCall span for agent.exec calls carrying flat traceId'` | Regression test đảm bảo field `_trace.id` (chỉ dùng phía Dev Server) không phá vỡ resume ở tầng bridge |

Target: ≥ 2 test.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
pnpm test --run src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts
pnpm test --run src/main/project/__tests__/project-rpc-handler.test.ts
pnpm test --run src/main/task/__tests__/TaskAgentExecutor.test.ts
pnpm test --run src/main/task/__tests__/task-rpc-handler.test.ts
pnpm test --run src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] ≥ 16 test case mới/mở rộng tổng cộng, đúng breakdown: `tracers.test.ts` ≥ 2, `ProfileAwareAgentSpawner.test.ts` ≥ 6, `project-rpc-handler.test.ts` ≥ 2, `TaskAgentExecutor.test.ts` ≥ 2, `task-rpc-handler.test.ts` ≥ 2, `dev-server-relay-bridge.test.ts` ≥ 2
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] KHÔNG có tracer/span mới nào được tạo cho `orchestration.send`/`.check`/`Coordinator` (TDD-08) — xác nhận đây là domain khác, ngoài phạm vi CR-TRACE-002
- [ ] KHÔNG có span mới được tạo cho mỗi `agent.output` PTY stream frame (BL-AG-05)
- [ ] Có test guard xác nhận `TaskAgentExecutor` không tự tạo tracer/span
