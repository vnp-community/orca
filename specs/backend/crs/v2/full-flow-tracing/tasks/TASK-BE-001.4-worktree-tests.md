# TASK-BE-001.4: Viết test cho tracing worktree management

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-001](../solutions/SOL-BE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-001.1 + TASK-BE-001.2 + TASK-BE-001.3
**Status:** ✅ Done (2026-08-04) — Added 22 new tests total (exceeds ≥20): `src/shared/trace/tracers.test.ts` (new file, 4 tests), `worktree.test.ts` (+7), `git-remote.test.ts` (+6), `dev-server-relay-bridge.test.ts` (+5). Followed existing convention of using real `registerTraceSink()` to capture emitted `TraceEvent`s rather than mocking `Tracers` — matches `src/shared/trace/index.test.ts`'s style. One drift: the `AGENT_NOT_CONNECTED` fail-branch inside `callWithTimeout()` is unreachable via the public `call()` wrapper when session is null from the start (that wrapper throws its own error first) — it's only reachable via the reconnect-wait-timeout path. Tested it by invoking the private `callWithTimeout` method directly instead of simulating the full 20s reconnect timing, with a comment explaining why. All 66 tests across the 4 files pass; typecheck clean; detect_changes (staged) confirms LOW risk, test-only symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết/mở rộng test file (Vitest) — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Trước khi viết test, khám phá lại các symbol đã được instrument ở TASK-BE-001.1 → 001.3 để bám đúng behavior thật:

```bash
codegraph explore "Tracers.worktreeCreate"
codegraph explore "worktree.create"
codegraph explore "git.worktree.add"
codegraph explore "DevServerRelayBridge.callWithTimeout"
```

## Mô tả

Viết ≥ 20 test case (Vitest) bao phủ toàn bộ instrumentation đã thêm ở TASK-BE-001.1 → 001.3: đăng ký tracer đúng tên, resume span từ `traceId`, propagate `traceId` vào `relay.call`, và guard chống gắn nhầm tracer vào `git.diff`.

## File: `src/shared/trace/__tests__/tracers.test.ts` [MODIFY hoặc NEW nếu chưa tồn tại]

Test case cần có:

| Test case | Mục tiêu |
|---|---|
| `'exports Tracers.worktreeCreate with flow name worktree:create'` | Verify tên tracer đúng convention CR-TRACE-000 §4 |
| `'exports worktreeFanOut/worktreeCompare/worktreeMerge as reserved tracers'` | Không throw khi `.start()` được gọi dù chưa có call site thật |

Target: ≥ 3 test.

## File: `src/main/runtime/rpc/methods/__tests__/worktree.test.ts` [MODIFY hoặc NEW nếu chưa tồn tại]

Test case cần có (mock `Tracers.worktreeCreate`/`Tracers.worktreeDelete`):

| Test case | Mục tiêu |
|---|---|
| `'worktree.create emits worktreeCreate span with ok() on success'` | Assert `start`/`ok` được gọi với field `worktreeId`/`path` |
| `'worktree.create resumes span id from params.traceId when provided'` | Assert `start(fields, { id: 'abc123' })` khi `params.traceId = 'abc123'` |
| `'worktree.create span.fail() called on createManagedWorktree rejection, provenance released'` | Assert `fail()` + `releaseAutomationWorkspaceProvenanceRequest` đều chạy |
| `'worktree.rm emits worktreeDelete span, does not resume from a prior worktree.create span'` | Assert `worktree.rm` tạo `id` mới độc lập |

Target: ≥ 6 test.

## File: `src/main/runtime/rpc/methods/__tests__/git-remote.test.ts` [MODIFY hoặc NEW nếu chưa tồn tại]

Test case cần có (mock `relay.call`):

| Test case | Mục tiêu |
|---|---|
| `'git.worktree.add emits worktreeCreate span and forwards traceId into relay.call'` | Assert `params.traceId === span.id` |
| `'git.worktree.remove emits worktreeDelete span and forwards traceId'` | tương tự |
| `'git.diff does NOT create any worktree:* span'` | Guard chống regression — xác nhận method dùng chung không bị gắn nhầm tracer |

Target: ≥ 6 test.

## File: `src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts` [MODIFY]

Test case cần có:

| Test case | Mục tiêu |
|---|---|
| `'callWithTimeout resumes relay:agentCall span id from params.traceId'` | Assert `relayCallTracer.start` nhận đúng `resume.id` |
| `'callWithTimeout starts a new relay:agentCall span id when params.traceId is absent'` | Backward-compat — không resume khi thiếu field |
| `'callWithTimeout emits fail() with AGENT_NOT_CONNECTED before throwing, still resumes traceId'` | Cả nhánh lỗi cũng phải resume đúng |

Target: ≥ 5 test.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/worktree.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/git-remote.test.ts
pnpm test --run src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] ≥ 20 test case mới/mở rộng tổng cộng trên 4 file, đúng breakdown: `tracers.test.ts` ≥ 3, `worktree.test.ts` ≥ 6, `git-remote.test.ts` ≥ 6, `dev-server-relay-bridge.test.ts` ≥ 5
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] Có test guard xác nhận `git.diff` không tạo span `worktree:compare`
- [ ] Có test xác nhận `worktree.rm` không resume từ span `worktree.create` trước đó (2 span độc lập)
- [ ] Có test xác nhận backward-compat: params không có `traceId` vẫn tạo span (id ngẫu nhiên) không lỗi
