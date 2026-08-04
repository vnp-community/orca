# TASK-AG-001.2: Add worktree tracing tests to agent-git-handler.test.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-001](../solutions/SOL-AG-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Precondition:** Phase 0 + [TASK-AG-001.1](./TASK-AG-001.1-agent-git-handler-worktree-tracing.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — adapted to this file's real convention (no spawn mocking anywhere in the file; tests run real git commands against `mkdtemp` temp dirs). Added a real temp-git-repo setup (`git init` + initial commit) in `beforeEach` so the "ok() on success" and "forwards id to nested agent:git span" cases exercise a genuine `git worktree add/remove` rather than a mocked one. 63/63 tests pass (58 pre-existing + 5 new).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này viết test cho các symbol đã tồn tại — chạy `codegraph explore` để hiểu implementation thật trước khi viết assertion:

```bash
codegraph explore "handleGitWorktreeAdd"
codegraph explore "handleGitWorktreeRemove"
```

Đây là symbol MODIFY (đã tồn tại, được TASK-AG-001.1 vừa thêm tracer) — chạy thêm impact analysis để biết còn caller nào khác có thể bị ảnh hưởng bởi hành vi mới:

```
gitnexus_impact({ target: "handleGitWorktreeAdd", direction: "upstream" })
gitnexus_impact({ target: "handleGitWorktreeRemove", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-git-handler.test.ts` [MODIFY]

File test đã tồn tại — thêm các case sau, dùng `registerTraceSink` từ `../../shared/trace` để bắt `TraceEvent[]` phát ra (pattern isomorphic test đã mô tả ở TDD-AG-01 §7).

```typescript
// src/relay/__tests__/agent-git-handler.test.ts (thêm)
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import { handleGitWorktreeAdd, handleGitWorktreeRemove } from '../agent-git-handler'

describe('agent-git-handler — worktree tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('handleGitWorktreeAdd emits worktree:create span with ok() on success', async () => {
    // ... mock spawn('git', ['worktree','add',...]) → exit 0 ...
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'feature/x' }, config, log)
    const okEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'ok')
    expect(okEvent).toBeDefined()
  })

  it('handleGitWorktreeAdd emits fail() when path/branch missing', async () => {
    await handleGitWorktreeAdd(1, {}, config, log)
    const failEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'fail')
    expect(failEvent).toBeDefined()
  })

  it('resumes span id from params._trace.id when present', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x', _trace: { id: 'abc123' } }, config, log)
    const startEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(startEvent?.id).toBe('abc123')
  })

  it('generates a new span id when params._trace is absent (backward-compat)', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x' }, config, log)
    const startEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(startEvent?.id).toBeDefined()
    expect(startEvent?.id).not.toBe('abc123')
  })

  it('handleGitWorktreeRemove emits worktree:delete span, forwards id to nested agent:git span', async () => {
    await handleGitWorktreeRemove(1, { path: '/tmp/wt1', _trace: { id: 'xyz789' } }, config, log)
    const deleteStart = events.find(e => e.flow === 'worktree:delete' && e.level === 'start')
    const gitStart     = events.find(e => e.flow === 'agent:git' && e.level === 'start')
    expect(deleteStart?.id).toBe('xyz789')
    expect(gitStart?.id).toBe('xyz789')   // nối tiếp id xuống agent:git qua _trace forward
  })
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-git-handler.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] Tất cả 5 test case trên có mặt trong `agent-git-handler.test.ts`
- [ ] `mock spawn('git', ...)` được thiết lập đúng theo pattern đã có sẵn trong file test (không tạo mock framework mới)
- [ ] Test "resumes span id" và "generates a new span id" cùng pass — xác nhận cả 2 nhánh có/không `_trace`
- [ ] Test "forwards id to nested agent:git span" xác nhận `id` của `agent:git` span TRÙNG với `worktree:delete` span (cross-span id propagation, không phải chỉ trùng ngẫu nhiên)
- [ ] `pnpm vitest run src/relay/__tests__/agent-git-handler.test.ts` pass toàn bộ (test cũ + mới)
