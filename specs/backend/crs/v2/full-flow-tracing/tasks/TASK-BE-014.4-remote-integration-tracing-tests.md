# TASK-BE-014.4: Test suite cho Remote Integration tracing (credential decrypt/store, preflight)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-014](../solutions/SOL-BE-TRACE-014-remote-integration.md) §3
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-014.2 + TASK-BE-014.3
**Status:** ✅ Done (2026-08-04) — 3 new test files, 21 tests total (7 + 8 + 6, all ≥ target minimums), all passing via `pnpm test --run`. Every credential/token test asserts the secret value never appears in any `TraceFields` (`JSON.stringify(event.fields)` must not contain the secret).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file mới — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-014.2/014.3 trước khi viết test:

```bash
codegraph explore "GitProviderCredentialService.getGitHubPAT"
codegraph explore "credentials.set"
codegraph explore "preflight.check"
```

## Mô tả

Tạo 3 file test Vitest mới xác nhận hành vi tracing của `GitProviderCredentialService` (TASK-BE-014.2), `credentials.set`/`credentials.revoke` và `preflight.check` (TASK-BE-014.3) — với trọng tâm là assertion bảo mật: không bao giờ có giá trị token/PAT thật lọt vào `TraceFields` ở bất kỳ event nào.

## File: `src/main/project/__tests__/GitProviderCredentialService-tracing.test.ts` [NEW]

Test cases cần cover:

- `getGitHubPAT() — found`: mock `store.getToken('bitbucket')` trả token string → assert `start({provider:'github', userId})`, `step('decrypt')`, `ok({provider:'github', found:true})`; assert KHÔNG có field nào trong bất kỳ event nào chứa giá trị token thật (so sánh toàn bộ `fields` object với token value, phải không match)
- `getGitHubPAT() — not found`: mock trả `null` → assert `ok({found:false})`
- `getGitHubPAT() — store throws`: mock `getToken` reject → assert `fail(err, {provider:'github'})` trước khi re-throw
- `getGitLabPAT() — found/not found/throws`: 3 test tương tự, `provider:'gitlab'`

Target: **≥ 6 test cases**.

## File: `src/main/runtime/rpc/methods/__tests__/credentials-tracing.test.ts` [NEW]

Test cases cần cover:

- `credentials.set — success`: mock `getWebCredentialStore().setToken` resolve → assert `start({service, userId})`, `step('encryptWrite')`, `ok({service})`; assert `params.token` KHÔNG xuất hiện trong bất kỳ field nào của bất kỳ event nào
- `credentials.set — not web credential mode`: mock `isWebCredentialMode()` false → assert `fail(err, {service})` được gọi trước khi throw
- `credentials.set — setToken throws`: assert `fail(err, {service})`
- `credentials.revoke — success/not-web-mode/throws`: 3 test tương tự
- `credentials.status/list — không tạo span`: gọi 2 method này, assert KHÔNG có event nào với flow `remoteIntegration:credentialStore` phát sinh

Target: **≥ 7 test cases**.

## File: `src/main/runtime/rpc/methods/__tests__/preflight-tracing.test.ts` [NEW]

Test cases cần cover:

- `preflight.check — local mode`: `params.devServerId` undefined → assert `step('localCheck')`, `ok({mode:'local'})`
- `preflight.check — remote mode success`: mock `ctx.devServerManager.getRelay()` trả relay mock, `relay.call` resolve → assert `step('relayDelegate', {devServerId})`, `ok({devServerId})`, và `relay.call` được gọi với params chứa `traceId: span.id`
- `preflight.check — relay not connected`: mock `getRelay()` trả `undefined` → assert `fail('relay-not-connected', {devServerId})` được gọi TRƯỚC khi throw (không chỉ dựa vào exception ở caller)
- `preflight.check — relay.call rejects`: mock `relay.call` reject với lỗi khác (không phải relay-not-connected) → assert `fail(err, {devServerId})` được gọi đúng 1 lần (không double-fail với nhánh relay-not-connected)
- `preflight.check — traceId resume`: gọi với `params.traceId='xyz'` → assert `span.id === 'xyz'`

Target: **≥ 5 test cases**.

## Verification

```bash
pnpm run typecheck:node
pnpm test --run src/main/project/__tests__/GitProviderCredentialService-tracing.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/credentials-tracing.test.ts
pnpm test --run src/main/runtime/rpc/methods/__tests__/preflight-tracing.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `GitProviderCredentialService-tracing.test.ts` có ≥ 6 test case, cover toàn bộ danh sách ở trên
- [ ] `credentials-tracing.test.ts` có ≥ 7 test case, cover toàn bộ danh sách ở trên
- [ ] `preflight-tracing.test.ts` có ≥ 5 test case, cover toàn bộ danh sách ở trên
- [ ] Mọi test liên quan tới credential/token có assertion tường minh rằng giá trị token/PAT thật KHÔNG xuất hiện trong bất kỳ `TraceFields` nào của `remoteIntegration:credentialDecrypt`/`remoteIntegration:credentialStore`
- [ ] Test `credentials.status/list — không tạo span` xác nhận không có tracer mới bị thêm ngoài phạm vi (regression check)
- [ ] Test `traceId resume` xác nhận `span.id === params.traceId` khi truyền vào (resume behavior từ TASK-BE-000)
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
