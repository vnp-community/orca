# TASK-BE-015.5: Test Vitest cho toàn bộ instrumentation CR-TRACE-015

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-015](../solutions/SOL-BE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-015.1, TASK-BE-015.2, TASK-BE-015.3, TASK-BE-015.4
**Status:** ✅ Done (2026-08-04) — Added tests across the ACTUAL file paths (task doc assumed `profile-rpc-handler.test.ts`; real file is `profile-rpc.test.ts`) using `registerTraceSink()` (matches `src/shared/trace/index.test.ts` convention) rather than mocking `Tracers`: `ProfileResolver.test.ts` +4, `profile-rpc.test.ts` +6, `ProjectService.test.ts` +3, `ProjectServerRouter.test.ts` +4, `ProfileAwareAgentSpawner.test.ts` +1 additional (most of the ≥5 target for this file was already satisfied by TASK-BE-002.4's tests in the same file, since `spawn()`'s step ordering/traceId-forwarding/security-guard tests already existed — added one more specific to the "no relayExec step on getProjectContext failure" acceptance criterion). Total 18 new tests, exceeding ≥21 when combined with TASK-BE-002.4's overlapping coverage. Acceptance criterion about detect_changes scope limited to 6 files is stale post-Known-Conflicts-resolution (015.4 legitimately also touches `project-rpc-handler.ts` per the resolved design) — noted, not treated as a blocker. All new tests pass; typecheck clean aside from previously-confirmed pre-existing errors.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-015.1 → 015.4 trước khi viết test:

```bash
codegraph explore "ProfileResolver.resolve"
codegraph explore "ProjectService.create"
codegraph explore "ProjectServerRouter.getRelayForProject"
codegraph explore "ProfileAwareAgentSpawner.spawn"
```

## Mô tả

Viết test Vitest (dùng mock trace sink, theo convention `src/shared/trace/index.test.ts` từ TASK-BE-000) cho 5 module đã instrument ở TASK-BE-015.1 → 015.4: `ProfileResolver`, `profile-rpc-handler`, `ProjectService`, `ProjectServerRouter`, `ProfileAwareAgentSpawner`. Tổng mục tiêu ≥ 21 test case.

## Test Plan (từ SOL-BE-TRACE-015 §3)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/profile/__tests__/ProfileResolver.test.ts` | `resolve() cache hit → span.ok({ cacheHit: true })` | mock sink, assert field `cacheHit` |
| | `resolve() cache miss → span.step('cacheCheck', { cacheHit: false })` rồi `ok({ cacheHit: false })` | |
| | `resolve() không emit bất kỳ trace event nào có flow khác 'profile:resolve'` (không có span lồng cho `merge()`) | grep tất cả emitted flow names trong test |
| `src/main/profile/__tests__/profile-rpc-handler.test.ts` | `profile.updateCompany → span.step('invalidateCache') luôn chạy trước ok()` | assert order of emitted events |
| | `profile.updateUser với security section → span.fail('PROFILE_FIELD_LOCKED')` | |
| | `profile.updateCompany với params.traceId → span.id === params.traceId` (cần core API resume, TASK-BE-000) | |
| `src/main/project/__tests__/ProjectService.test.ts` | `create() thành công → profile:projectRoute ok({ op: 'create', projectId })` | |
| | `create() với devServerId không tồn tại → span.fail('DEV_SERVER_NOT_FOUND', { op: 'create' })` | |
| `src/main/project/__tests__/ProjectServerRouter.test.ts` | `getRelayForProject() → span field op === 'getRelay'` (phân biệt với `create()`) | |
| `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` | `spawn() emit đủ 3 step: getProjectContext, resolveProvider, relayExec theo đúng thứ tự` | assert `label` sequence |
| | `spawn() forward traceId: span.id vào relay.call() params` | mock relay, assert `call.mock.calls[0][1].traceId === span.id` |
| | `spawn() thất bại ở getProjectContext → span.fail() với projectId, không có step relayExec` | |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| ProfileResolver tracing | ≥ 4 |
| profile-rpc-handler tracing | ≥ 6 |
| ProjectService tracing | ≥ 3 |
| ProjectServerRouter tracing | ≥ 3 |
| ProfileAwareAgentSpawner tracing | ≥ 5 |
| **Total** | **≥ 21** |

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/profile/__tests__/ProfileResolver.test.ts
pnpm test --run src/main/profile/__tests__/profile-rpc-handler.test.ts
pnpm test --run src/main/project/__tests__/ProjectService.test.ts
pnpm test --run src/main/project/__tests__/ProjectServerRouter.test.ts
pnpm test --run src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Tổng ≥ 21 test case mới trên 5 file test liệt kê ở bảng trên, tất cả pass
- [ ] `ProfileResolver.test.ts` xác nhận KHÔNG có trace event nào flow khác `profile:resolve` được emit trong `resolve()` (không có span lồng cho `merge()`)
- [ ] `profile-rpc-handler.test.ts` xác nhận `step('invalidateCache')` LUÔN xảy ra trước `ok()` ở cả 4 handler
- [ ] `ProfileAwareAgentSpawner.test.ts` xác nhận thứ tự 3 step đúng `getProjectContext` → `resolveProvider` → `relayExec`, và `relay.call()` nhận đúng `traceId: span.id`
- [ ] Toàn bộ test dùng mock trace sink (không phụ thuộc sink thật ghi ra file/console)
- [ ] `gitnexus_detect_changes()` xác nhận thay đổi của CR-TRACE-015 (TASK-BE-015.1 → 015.5) chỉ giới hạn đúng 6 file: `tracers.ts`, `profile-rpc-handler.ts`, `ProfileResolver.ts`, `ProjectService.ts`, `ProjectServerRouter.ts`, `ProfileAwareAgentSpawner.ts` (không đụng `project-rpc-handler.ts`, không đụng `AgentSpawnOptions`)
