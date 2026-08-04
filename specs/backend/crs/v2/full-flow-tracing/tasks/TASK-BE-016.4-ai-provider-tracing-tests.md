# TASK-BE-016.4: Test Vitest cho toàn bộ instrumentation CR-TRACE-016 (bao gồm test bảo mật bắt buộc)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-016](../solutions/SOL-BE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-016.1, TASK-BE-016.2, TASK-BE-016.3
**Status:** ✅ Done (2026-08-04) — Delivered 19 new tests (AIProviderService +5, ProviderResolver +6, ProviderHealthChecker +4, new `ai-provider-rpc.test.ts` +4) across 4 files, total ≥16 target met. Includes the mandatory security test asserting no `encryptedBlob`/`iv`/`apiKey` in any trace event field. All 62 tests across the 4 files pass (`pnpm test --run` for each). Transient note: 1 test briefly failed mid-session when an external `git reset --hard` on the shared working tree wiped `src/shared/trace/index.ts`'s `Tracer.start(fields, resume)` API (TASK-BE-000); it was restored by its owning task before this task closed and the full suite is green.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-016.1 → 016.3 trước khi viết test:

```bash
codegraph explore "AIProviderService.writeCredentialToDevServer"
codegraph explore "ProviderResolver.resolve"
codegraph explore "ProviderHealthChecker.runCheck"
```

## Mô tả

Viết test Vitest cho 4 module đã instrument ở TASK-BE-016.1 → 016.3: `AIProviderService`, `ProviderResolver`, `ProviderHealthChecker`, `ai-provider-rpc-handler`. Tổng mục tiêu ≥ 16 test case. **Bắt buộc** có 1 test bảo mật grep toàn bộ `TraceEvent.fields` để đảm bảo không leak credential.

## Test Plan (từ SOL-BE-TRACE-016 §3)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/ai-providers/__tests__/AIProviderService.test.ts` | `writeCredentialToDevServer() thành công → span.ok({ accountId, status: 'active' })` | mock trace sink |
| | `writeCredentialToDevServer() KHÔNG BAO GIỜ emit field encryptedBlob/iv trong bất kỳ event nào` | **security test bắt buộc** — grep toàn bộ `TraceEvent.fields` trong test, assert không có key `encryptedBlob`/`iv`/`apiKey` |
| | `writeCredentialToDevServer() với accountId không tồn tại → span.fail('ACCOUNT_NOT_FOUND')` | |
| | `writeCredentialToDevServer(traceId) → span.id === traceId` (resume) | |
| `src/main/ai-providers/__tests__/ProviderResolver.test.ts` | `resolve() hết quota → span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })` | |
| | `resolve() còn account nhưng không khớp scope → span.fail(..., { reason: 'no-scope-match' })` | |
| | `resolve() match ở pass 1 (modelHint) → step('scope-match', { usedModelHint: true })` | |
| | `resolve() match ở pass 2 (fallback, không modelHint) → step('scope-match', { usedModelHint: false })` | |
| `src/main/ai-providers/__tests__/ProviderHealthChecker.test.ts` | `runCheck() với N account → span.ok({ activeCount, quotaExceededCount, invalidCount, errorCount })` cộng đúng tổng N | |
| | `runCheck() 1 account throw exception → errorCount tăng, span vẫn ok() (không fail toàn cycle)` | |
| `src/main/ai-providers/__tests__/ai-provider-rpc.test.ts` | `aiProvider.writeCredential với params.traceId → forward đúng vào writeCredentialToDevServer()` | mock service, assert 4th arg |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| AIProviderService tracing (bao gồm security test) | ≥ 5 |
| ProviderResolver tracing | ≥ 5 |
| ProviderHealthChecker tracing | ≥ 4 |
| ai-provider-rpc-handler tracing | ≥ 2 |
| **Total** | **≥ 16** |

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/ai-providers/__tests__/AIProviderService.test.ts
pnpm test --run src/main/ai-providers/__tests__/ProviderResolver.test.ts
pnpm test --run src/main/ai-providers/__tests__/ProviderHealthChecker.test.ts
pnpm test --run src/main/ai-providers/__tests__/ai-provider-rpc.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Tổng ≥ 16 test case mới trên 4 file test liệt kê ở bảng trên, tất cả pass
- [ ] **Security-critical:** test grep tự động xác nhận không có bất kỳ trace event nào (start/step/ok/fail) trong toàn bộ CR-TRACE-016 chứa field `apiKey`, `encryptedBlob`, hoặc `iv` — không chỉ code review thủ công
- [ ] `ProviderResolver.test.ts` verify đúng 2 lý do fail phân biệt (`quota-or-inactive`/`no-scope-match`) và cả 2 pass (`usedModelHint: true/false`)
- [ ] `ProviderHealthChecker.test.ts` verify 4 counter cộng đúng tổng N account, và per-account error không fail toàn cycle
- [ ] `gitnexus_detect_changes()` xác nhận thay đổi chỉ giới hạn trong 4 file backend (`AIProviderService.ts`, `ProviderResolver.ts`, `ProviderHealthChecker.ts`, `ai-provider-rpc-handler.ts`) + `tracers.ts`, không lan sang `src/relay/ai-provider-handler.ts` (agent-side, ngoài phạm vi)
