# TASK-BE-005.4: Test suite cho Code Review tracing (LOCAL + REMOTE)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-005](../solutions/SOL-BE-TRACE-005-code-review.md) §3
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-005.2 + TASK-BE-005.3
**Status:** ✅ Done (2026-08-04) — Created `git-remote-tracing.test.ts` (10 tests, target ≥9) and `git-local-tracing.test.ts` (9 tests, target ≥4), using the `registerTraceSink`/`TraceEvent` capture pattern already established in `git-remote.test.ts` rather than mocking `Tracers` directly. Both files assert the renamed tracer flow strings from TASK-BE-005.1 (`codeReview:diff`/`codeReview:aiCommitMessage`/`codeReview:createPr`) and include the `codeReviewAnnotate`/`codeReviewFeedback` no-call-site regression check. All 19 new tests pass; existing `git.test.ts` + `git-remote.test.ts` + `tracers.test.ts` (56 tests combined) re-run clean, no regression. `pnpm run typecheck:node` clean for all touched files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file mới — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các handler đã instrument ở TASK-BE-005.2/005.3 trước khi viết test:

```bash
codegraph explore "git.diff"
codegraph explore "git.generateCommitMessage"
codegraph explore "git.pr.create"
```

## Mô tả

Tạo 2 file test Vitest mới xác nhận hành vi tracing của các handler đã patch ở TASK-BE-005.2 (LOCAL) và TASK-BE-005.3 (REMOTE), bao gồm cả regression check rằng `codeReviewAnnotateFlow`/`codeReviewFeedbackFlow` (từ TASK-BE-005.1) không có call site nào.

## File: `src/main/runtime/rpc/methods/__tests__/git-remote-tracing.test.ts` [NEW]

Test cases cần cover (mock `router.getRelayForProject` + mock `relay.call`):

- `git.diff (remote)`: gọi handler với mock `relay.call` trả `{stdout, stderr, exitCode:0}` → assert `Tracers.codeReviewDiffFlow` nhận đúng 1 `start()` với `mode:'remote'`, 1 `step('routeRelay')`, 1 `ok()` với `fileCount`
- `git.diff (remote) — relay.call throws`: assert `span.fail()` được gọi với `mode:'remote'` trước khi exception propagate
- `git.diff (remote) — traceId forwarded`: gọi handler với `params.traceId = 'abc123'` → assert `span.id === 'abc123'` (resume, không random mới)
- `git.diff (remote) — relay.call receives traceId`: assert `relay.call` được gọi với `params.traceId === span.id`
- `git.generateCommitMessage (remote) — happy path`: assert 2 `step()` riêng biệt (`diffStaged`, `aiComplete`) theo đúng thứ tự, mỗi step có field đúng theo bảng gap analysis của SOL-BE-TRACE-005 §1.2
- `git.generateCommitMessage (remote) — GIT_NO_STAGED_CHANGES`: mock diff rỗng → assert `span.fail('GIT_NO_STAGED_CHANGES', ...)` được gọi TRƯỚC `ai.complete` (không gọi AI khi không có staged diff)
- `git.generateCommitMessage (remote) — GIT_AI_EMPTY_RESPONSE`: mock AI trả rỗng → assert `span.fail('GIT_AI_EMPTY_RESPONSE', ...)`
- `git.pr.create (remote) — success`: mock `shell.exec` trả `exitCode:0` → assert `span.ok()` với `prUrl`, `exitCode:0`
- `git.pr.create (remote) — gh CLI fails`: mock `shell.exec` trả `exitCode:1` → assert `span.fail()` được gọi với `exitCode:1` (KHÔNG chỉ dựa vào exception vì `relay.call('shell.exec')` không tự throw khi exitCode != 0)

Target: **≥ 9 test cases**.

## File: `src/main/runtime/rpc/methods/__tests__/git-local-tracing.test.ts` [NEW]

Test cases cần cover (mock `runtime.getRuntimeGitDiff`/`runtime.generateRuntimeCommitMessage`):

- `git.diff (local)`: mock `runtime.getRuntimeGitDiff` → assert `Tracers.codeReviewDiffFlow.start()` với `mode:'local'`, `ok()` cùng `mode`
- `git.diff (local) — runtime throws`: assert `span.fail(err, {mode:'local'})` trước khi re-throw
- `git.generateCommitMessage (local) — success.true`: mock `runtime.generateRuntimeCommitMessage` trả `{success:true, message:'...'}` → assert `ok()` với `messageChars`
- `git.generateCommitMessage (local) — success.false`: mock trả `{success:false, error:'...'}` → assert `fail()` được gọi, KHÔNG throw (vì handler return result thay vì throw trong nhánh này)

Target: **≥ 4 test cases**.

## Xác nhận không phá tracer hiện có (regression)

Thêm assertion (trong 1 trong 2 file trên, hoặc file `tracers.test.ts` riêng nếu tồn tại): `Tracers.codeReviewAnnotateFlow` và `Tracers.codeReviewFeedbackFlow` tồn tại (được export) nhưng KHÔNG có bất kỳ call site nào trong `git.ts`/`git-remote.ts` — có thể verify bằng grep-based assertion hoặc review thủ công trong CI checklist, không cần test runtime.

## Verification

```bash
pnpm run typecheck:node
pnpm test --run src/main/runtime/rpc/methods/__tests__/git-remote-tracing.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/git-local-tracing.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `git-remote-tracing.test.ts` có ≥ 9 test case, cover toàn bộ danh sách ở trên
- [ ] `git-local-tracing.test.ts` có ≥ 4 test case, cover toàn bộ danh sách ở trên
- [ ] Assertion về `codeReviewAnnotateFlow`/`codeReviewFeedbackFlow` không có call site nào được thêm vào
- [ ] Test `traceId forwarded` xác nhận `span.id === params.traceId` khi truyền vào (resume behavior từ TASK-BE-000)
- [ ] Test `relay.call receives traceId` xác nhận `traceId: span.id` xuất hiện trong params gửi tới `relay.call`
- [ ] Không có assertion nào kiểm tra giá trị token/credential thật trong `TraceFields` (vì các handler này không xử lý credential — không áp dụng, nhưng test không được vô tình log secret nào nếu mock data chứa placeholder)
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
