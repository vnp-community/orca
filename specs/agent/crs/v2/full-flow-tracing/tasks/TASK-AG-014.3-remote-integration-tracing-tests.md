# TASK-AG-014.3: Add remote-integration tracing tests (external-api-connector, fs-agent-extensions)

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-014](../solutions/SOL-AG-TRACE-014-remote-integration.md)
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Precondition:** Phase 0 + [TASK-AG-014.1](./TASK-AG-014.1-external-api-connector-auth-status-spans.md) + [TASK-AG-014.2](./TASK-AG-014.2-fs-agent-extensions-preflight-tracer.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — both target test files already existed (MODIFY, not NEW) and were extended with tracing-specific `describe` blocks matching each file's existing conventions (`MOCK_CONFIG`/`MOCK_LOG` in `external-api-connector.test.ts`; `makeConfig()` + `beforeEach`/`afterEach` tmpdir fixture in `fs-agent-extensions.test.ts`). Drift: sandbox has real `gh` (unauthenticated) but no `glab` binary — rather than assume a fixed ok/fail outcome for `handleGitHubAuthStatus` (task doc assumed exit 0), tests assert on whichever terminal event (`ok`|`fail`) actually fires and check its `cli` field; `handleGitLabAuthStatus` deterministically hits the `fail` path since `glab` is absent (spawn ENOENT), matching the doc's fail-case sample as-is. For `handlePreflightCheck`'s ok-path test, used `services: []` (vacuous all-ok, `checkedCount: 0`) instead of `['ripgrep']` since `checkRgAvailable` is mocked to always resolve `false` in this file, which would make an ripgrep-only ok assertion always fail. All 3 new `external-api-connector.test.ts` describe blocks (6 tests) and 3 new `fs-agent-extensions.test.ts` tests added — 88/88 tests pass total across both files. typecheck:node clean for both source files; pre-existing unrelated `TS6133 'join'` unused-import error remains in `external-api-connector.test.ts` (present before this task, not introduced by it).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleGitHubAuthStatus"
codegraph explore "handleGitLabAuthStatus"
codegraph explore "handlePreflightCheck"
```

Đây đều là symbol MODIFY (đã tồn tại, vừa được TASK-AG-014.1/014.2 thêm tracer) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handleGitHubAuthStatus", direction: "upstream" })
gitnexus_impact({ target: "handleGitLabAuthStatus", direction: "upstream" })
gitnexus_impact({ target: "handlePreflightCheck", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/external-api-connector.test.ts` [MODIFY]

Thêm sau `describe('buildGlabEnv', ...)` hiện có:

```typescript
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'
import { handleGitHubAuthStatus, handleGitLabAuthStatus } from '../external-api-connector'

describe('handleGitHubAuthStatus — agent:ext-api tracing', () => {
  it('span.ok({cli:"gh", authenticated:true}) khi gh auth status exit 0', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    // mock execFileCaptured indirectly via spawn — xem setup có sẵn trong file test hiện tại
    const res = await handleGitHubAuthStatus(1, { userId: 'u1' }, config, log) as { result: { ok: boolean } }
    unregister()
    const ok = events.find(e => e.flow === 'agent:ext-api' && e.level === 'ok')
    expect(ok?.fields.cli).toBe('gh')
  })

  it('KHÔNG có field nào trong agent:ext-api chứa nội dung stdout/stderr của gh auth status', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitHubAuthStatus(1, { userId: 'u1' }, config, log)
    unregister()
    const fields = events.filter(e => e.flow === 'agent:ext-api').flatMap(e => Object.keys(e.fields))
    expect(fields).not.toContain('stdout')
    expect(fields).not.toContain('stderr')
  })
})

describe('handleGitLabAuthStatus — agent:ext-api tracing', () => {
  it('span.fail(..., {cli:"glab", exitCode}) khi glab auth status exit != 0', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitLabAuthStatus(1, { userId: 'u1' }, config, log)
    unregister()
    const fail = events.find(e => e.flow === 'agent:ext-api' && e.level === 'fail')
    expect(fail?.fields.cli).toBe('glab')
  })
})
```

## File: `src/relay/__tests__/fs-agent-extensions.test.ts` [NEW hoặc MODIFY nếu đã tồn tại]

```typescript
import { describe, it, expect, vi } from 'vitest'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'
import { handlePreflightCheck } from '../fs-agent-extensions'

describe('handlePreflightCheck — agent:preflight tracing', () => {
  it('span.ok({checkedCount}) khi tất cả services khả dụng', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['ripgrep'] }, config)
    unregister()
    expect(events.some(e => e.flow === 'agent:preflight' && e.level === 'ok')).toBe(true)
  })

  it('span.fail("unavailable: ...") khi có service không cài đặt', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['not-a-real-binary-xyz'] }, config)
    unregister()
    const fail = events.find(e => e.flow === 'agent:preflight' && e.level === 'fail')
    expect(fail?.fields.failedCount).toBe(1)
  })
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/external-api-connector.test.ts src/relay/__tests__/fs-agent-extensions.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `external-api-connector.test.ts` có thêm ≥ 3 test case theo trên
- [ ] `fs-agent-extensions.test.ts` có ≥ 2 test case theo trên (file mới nếu chưa tồn tại — kiểm tra `ls src/relay/__tests__/` trước khi quyết định NEW vs MODIFY)
- [ ] Test xác nhận không có `stdout`/`stderr` key nào trong fields của `agent:ext-api`
- [ ] `pnpm vitest run src/relay/__tests__/external-api-connector.test.ts src/relay/__tests__/fs-agent-extensions.test.ts` pass
