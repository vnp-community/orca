# TASK-FE-015.1: Instrument `useProfile()` read hook (BL-PRF-02, resolve)

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-015 §0, §2.2 (phần đọc)](../solutions/SOL-FE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiProfileResolveFlow` đã đăng ký ở TASK-FE-001) + external TASK-BE-000 (core API `Tracer.start(fields?, resume?)` — xem ghi chú §2.0 dưới)
**Status:** ✅ Done (2026-08-04) — implemented as spec'd; `uiProfileResolveFlow` tracer already existed in `tracers.ts` (re-added after a shared-file reset during this session, additive-only). Core API `Tracer.start(fields?, resume?)` in `src/shared/trace/index.ts` already supports `resume` (drift from spec note, no blocker). `pnpm tsc --noEmit` clean; `useProfile.test.ts` 12/12 passing (3 new read-hook cases).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useProfile"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useProfile", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

**Phạm vi giới hạn theo TDD-FE-11:** CR-TRACE-015 (backend) bao phủ BL-PRF-01→04, nhưng TDD-FE-11 chỉ implement UI cho BL-PRF-01 (update) và BL-PRF-02 (resolve). BL-PRF-03/04 (Project–Dev Server assignment, Profile-aware agent spawn) là hành vi của Project Workspace (TDD-FE-12, ngoài phạm vi task này) — đã grep xác nhận không có lời gọi `project.create`/`project.agentSpawn` trong `components/profile/` hay `useProfile.ts`.

**Core API chưa sẵn sàng:** `src/shared/trace/index.ts` hiện chỉ có `Tracer.start(fields?: TraceFields): TraceSpan` — **chưa có tham số `resume`**. Code dưới đây viết theo API đích (`start(fields, resume)` khi cần) vì CR-TRACE-000 liệt đây là thay đổi bắt buộc trước khi triển khai bất kỳ CR-TRACE-0XX nào (xem external TASK-BE-000). Task này KHÔNG tự vá `index.ts`.

**Toàn bộ RPC của luồng Profile UI đi qua `useProfile.ts`** — không có call site RPC nào khác trong `components/profile/*.tsx`.

**Lưu ý TracePanel `isBackend` heuristic:** tracer BACKEND của CR-TRACE-015 dùng `profile:resolve`/`profile:updateLayer` (namespace `domain:operation`) — nếu FE dùng lại đúng tên, event browser-side sẽ tự nhận nhầm badge "▲ srv". Solution dùng prefix `ui:` (`ui:profile.resolve`) — đã đăng ký trong TASK-FE-001.

## File: `src/renderer/src/hooks/useProfile.ts` [MODIFY] — chỉ phần đọc (`useProfile()`)

```typescript
// src/renderer/src/hooks/useProfile.ts
import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
import type { OrcaProfile, ResolvedProfile } from '../types/profile-types'
import { toast } from 'sonner'

export function useProfile() {
  const { resolvedProfile, userProfile, profileIsLoading } = useAppStore(s => ({
    resolvedProfile:  (s as any).resolvedProfile as ResolvedProfile | null,
    userProfile:      (s as any).userProfile as OrcaProfile | null,
    profileIsLoading: (s as any).profileIsLoading as boolean,
  }))

  useEffect(() => {
    const store = useAppStore.getState() as any
    store.setProfileLoading(true)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BL-PRF-02: browser tạo traceId TRƯỚC khi gọi RPC (CR-TRACE-000 §3.3 hàng 1).
    // Đây là root span của cả 2 lời gọi song song bên dưới — dùng chung 1 id vì về
    // nghiệp vụ đây là MỘT thao tác "load profile" duy nhất, không phải 2 flow riêng.
    const span = Tracers.uiProfileResolveFlow.start()

    Promise.all([
      callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', { traceId: span.id }),
      callRuntimeRpc<OrcaProfile>(target, 'profile.getUser', { traceId: span.id }),
    ])
      .then(([resolved, user]) => {
        store.setResolved(resolved)
        store.setUserProfile(user)
        span.ok({ hasSecurityLock: resolved?.security !== undefined })
      })
      .catch(err => {
        console.error('[useProfile] fetch failed:', err)
        span.fail(err)
      })
      .finally(() => {
        store.setProfileLoading(false)
      })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return { resolvedProfile, userProfile, profileIsLoading }
}
```

> Việc thêm `traceId: z.string().optional()` vào Zod params schema của `profile.getResolved`/`profile.getUser` (`src/main/profile/profile-rpc-handler.ts`) là thay đổi **phía backend**, thuộc companion solution BE — nếu schema dùng `.strict()`, gửi `traceId` sẽ bị reject 400 cho tới khi backend solution ship.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useProfile.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useProfile()` (read hook) tạo `ui:profile.resolve` span khi mount, `span.ok({ hasSecurityLock })` khi cả 2 RPC thành công, `span.fail(err)` khi có lỗi
- [ ] `profile.getResolved` và `profile.getUser` đều nhận `traceId: span.id` trong params (cùng 1 id, vì là 1 thao tác "load" duy nhất)
- [ ] Tracer flow name dùng prefix `ui:` (không phải `profile:`) để TracePanel `isBackend` heuristic (`TracePanel.tsx:42`) hiển thị đúng badge "▼ cli"
- [ ] Không thêm tracer/span cho BL-PRF-03/04 trong task này — các sub-flow đó ngoài phạm vi TDD-FE-11
- [ ] Test suite `useProfile.test.ts` đạt tối thiểu 3 test case mới cho phần đọc (start khi mount + ok với hasSecurityLock; fail khi 1 trong 2 RPC reject; traceId trong params của cả 2 RPC), không phá vỡ test hiện có
