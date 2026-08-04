# SOL-FE-TRACE-015: Profile & Project — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**TDD Ref:** [TDD-FE-11](../../../../tdd/v5/11-profile-ui.md) (Profile Hierarchy UI, F33, ADR-007)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts` (`initBrowserTrace`), `src/shared/trace/index.ts`, TracePanel (`src/renderer/src/components/trace/TracePanel.tsx`); core API change `Tracer.start(fields?, resume?)` từ CR-TRACE-000 §3.1 (**chưa ship** — xem ghi chú ở §2.0)

---

## 0. Phạm vi (giới hạn theo TDD-FE-11)

CR-TRACE-015 (backend) bao phủ BL-PRF-01→04, nhưng TDD-FE-11 (Profile Hierarchy UI) chỉ implement UI cho BL-PRF-01 (update profile) và BL-PRF-02 (resolve/hiển thị effective settings). BL-PRF-03 (Project–Dev Server assignment) và BL-PRF-04 (Profile-aware agent spawn) là hành vi của **Project Workspace** (TDD-FE-12, ngoài phạm vi task này) — grep xác nhận không có lời gọi `project.create`/`project.agentSpawn` nào trong `src/renderer/src/components/profile/` hay `src/renderer/src/hooks/useProfile.ts`. Solution này vì vậy chỉ instrument BL-PRF-01/02; BL-PRF-03/04 được đánh dấu "ngoài phạm vi TDD-FE-11 — thuộc SOL-FE-TRACE-019 (Project Workspace, nếu có) hoặc solution phía Project Workspace UI, cần điều tra thêm khi triển khai".

---

## 1. Điểm khởi tạo trace trong Renderer

Toàn bộ RPC của luồng Profile UI đi qua 2 hàm trong `src/renderer/src/hooks/useProfile.ts` — không có call site RPC nào khác trong `components/profile/*.tsx` (`ProfileEditor.tsx`, `CompanyProfileAdmin.tsx`, `DeptProfileAdmin.tsx`, `ModelSelector.tsx`, `ProfileFieldRow.tsx`, `ProfileSourceBadge.tsx` đều chỉ đọc props/store, không tự gọi RPC).

| BL | Hành động user | Component kích hoạt | Hook thực thi RPC | RPC method | File:line hiện tại |
|----|-----------------|----------------------|--------------------|------------|----------------------|
| BL-PRF-02 (resolve) | Mount `ProfileEditor` (bất kỳ scope nào) | `ProfileEditor.tsx:17` (`useProfile()`) | `useProfile()` | `profile.getResolved` + `profile.getUser` (song song, `Promise.all`) | `src/renderer/src/hooks/useProfile.ts:23-26` |
| BL-PRF-01 (update user) | Click "Save Changes" trên tab "My Settings" | `ProfileEditor.tsx:81` (`onClick={() => saveProfile(scope, localProfile, scopeId)}`, `data-testid="save-profile-btn"`) | `useProfileActions().saveProfile('user', profile)` | `profile.updateUser` → refetch `profile.getResolved` | `src/renderer/src/hooks/useProfile.ts:53-59` |
| BL-PRF-01 (update company) | Click "Save Changes" trong `CompanyProfileAdmin` (render `<ProfileEditor scope="company">`) | `ProfileEditor.tsx:81` | `useProfileActions().saveProfile('company', profile)` | `profile.updateCompany` | `useProfile.ts:60-62` |
| BL-PRF-01 (update dept) | Click "Save Changes" trong `DeptProfileAdmin` (render `<ProfileEditor scope="dept" scopeId={deptId}>`) | `ProfileEditor.tsx:81` | `useProfileActions().saveProfile('dept', profile, deptId)` | `profile.updateDept` | `useProfile.ts:63-66` |

**Ghi chú quan trọng — TracePanel `isBackend` heuristic:** `TracePanel.tsx:42` phân loại event là backend nếu `flow.includes(':') && !flow.startsWith('ui:')`. Toàn bộ tracer BACKEND của CR-TRACE-015 dùng format `profile:updateLayer`/`profile:resolve` (namespace `domain:operation` theo CR-TRACE-000 §4) — nếu tracer FE dùng lại đúng tên này, event browser-side sẽ tự nhận nhầm badge "▲ srv". Vì vậy tracer phía renderer trong solution này dùng prefix riêng `ui:` (`ui:profile.update`, `ui:profile.resolve`) — vẫn cùng `id` (span) với tracer backend nhờ cơ chế `resume`, nhưng khác `flow` name để TracePanel hiển thị đúng badge "▼ cli" cho event do browser tự phát, tách biệt khỏi event backend nhận qua SSE dùng chung `id`.

## 2.0. Core API chưa sẵn sàng — `Tracer.start()` hiện chỉ nhận `fields`

Đọc `src/shared/trace/index.ts:39-48` xác nhận `Tracer.start(fields?: TraceFields): TraceSpan` — **chưa có tham số `resume`** như CR-TRACE-000 §3.1 mô tả. Code dưới đây viết theo API đích (`start(fields, resume)`) vì CR-TRACE-000 liệt đây là thay đổi bắt buộc trước khi bất kỳ CR-TRACE-0XX nào triển khai; solution này không tự ý vá `index.ts` (thuộc phạm vi CR-TRACE-000, có thể do một PR nền tảng chung xử lý), chỉ giả định API đã có khi PR áp dụng.

## 2. Full Implementation

### 2.1. Thêm tracer phía browser vào `src/shared/trace/tracers.ts`

`tracers.ts` là registry isomorphic (dùng chung Node + browser) — theo đúng cách các CR-TRACE-0XX backend đã thêm entry của họ vào cùng file này, solution này bổ sung 2 entry mới với prefix `ui:`:

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow)...
  // ...existing backend entries from CR-TRACE-015 (profileUpdateLayerFlow, profileResolveFlow, ...)...

  /** Browser-initiated: user click "Save Changes" trong ProfileEditor (BL-PRF-01) */
  uiProfileUpdateFlow:  createTracer('ui:profile.update'),
  /** Browser-initiated: mount ProfileEditor → fetch resolved + user profile (BL-PRF-02) */
  uiProfileResolveFlow: createTracer('ui:profile.resolve'),
} as const
```

### 2.2. `useProfile.ts` — instrument read hook (BL-PRF-02)

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
    // Đây là root span của cả 2 lời gọi song song bên dưới — dùng chung 1 id vì
    // về nghiệp vụ đây là MỘT thao tác "load profile" duy nhất, không phải 2 flow riêng.
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

// --- Write hook (BL-PRF-01) ---

export function useProfileActions() {
  const saveProfile = useCallback(
    async (
      scope: 'user' | 'dept' | 'company',
      profile: OrcaProfile,
      scopeId?: string
    ) => {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // BL-PRF-01: 1 span bao phủ toàn bộ save + refetch resolved (nếu scope='user'),
      // field `scope` phân biệt 3 nhánh — đối xứng với cách backend `profile:updateLayer`
      // dùng 1 tracer cho cả 3 scope (CR-TRACE-015 §3).
      const span = Tracers.uiProfileUpdateFlow.start({ scope, targetId: scopeId })
      try {
        if (scope === 'user') {
          await callRuntimeRpc(target, 'profile.updateUser', { profile, traceId: span.id })
          const resolved = await callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', {})
          const store = useAppStore.getState() as any
          store.setResolved(resolved)
          store.setUserProfile(profile)
          toast.success('Profile saved')
        } else if (scope === 'company') {
          await callRuntimeRpc(target, 'profile.updateCompany', { profile, traceId: span.id })
          toast.success('Company profile updated')
        } else if (scope === 'dept' && scopeId) {
          await callRuntimeRpc(target, 'profile.updateDept', { deptId: scopeId, profile, traceId: span.id })
          toast.success('Department profile updated')
        }
        span.ok({ scope })
      } catch (err: any) {
        span.fail(err, { scope })
        toast.error(err?.message ?? 'Failed to save profile')
        throw err
      }
    },
    []
  )

  return { saveProfile }
}
```

**Vì sao không `traceId` trên lời gọi `profile.getResolved` refetch sau save (dòng "const resolved = ...")**: đây là một thao tác refetch riêng biệt (đọc lại cache đã invalidate), không phải một phần của span `ui:profile.update` — theo đúng nguyên tắc "1 tracer = 1 sub-flow" (CR-TRACE-000 §4), refetch này thuộc về BL-PRF-02 (`ui:profile.resolve`) nếu muốn trace riêng, nhưng để tránh tạo 1 span con lồng bên trong `saveProfile` gây rối TracePanel, solution này để nó không-traced (single RPC call, latency thấp, không băng qua nhiều bước) — nhất quán với nguyên tắc "chống over-instrumentation" (CR-TRACE-000 §5).

### 2.3. Zod schema phía backend (out-of-scope, chỉ ghi chú)

`profile.updateUser`/`updateCompany`/`updateDept`/`getResolved` cần thêm field `traceId: z.string().optional()` vào Zod params schema trong `src/main/profile/profile-rpc-handler.ts` để không bị `defineMethod` reject field lạ — đây là thay đổi **phía backend**, thuộc companion solution BE cho CR-TRACE-015, không sửa ở đây. Nếu backend schema dùng `.strict()` thay vì mặc định strip-unknown, gửi `traceId` sẽ bị reject 400 cho tới khi backend solution ship — điểm phối hợp cần đồng bộ giữa 2 team.

## 3. Test Plan (Vitest)

Mở rộng các file test hiện có (không tạo file mới) — theo pattern `vi.mock('../runtime/runtime-rpc-client')` đã dùng trong `src/renderer/src/hooks/__tests__/` (xem `useAIProviders.test.ts` làm ví dụ mocking).

| File | Test case mới |
|------|----------------|
| `src/renderer/src/hooks/__tests__/useProfile.test.ts` | `useProfile()` gọi `Tracers.uiProfileResolveFlow.start()` khi mount, và `span.ok({ hasSecurityLock })` sau khi `Promise.all` resolve |
| | `useProfile()` gọi `span.fail(err)` khi 1 trong 2 RPC reject |
| | `profile.getResolved`/`profile.getUser` được gọi kèm `traceId: span.id` trong params |
| | `useProfileActions().saveProfile('user', ...)` gọi `Tracers.uiProfileUpdateFlow.start({ scope: 'user' })` |
| | `saveProfile('company', ...)` forward `traceId: span.id` vào `profile.updateCompany` params |
| | `saveProfile('dept', profile, deptId)` set field `targetId: deptId` trên span |
| | `saveProfile()` throw → `span.fail(err, { scope })` được gọi trước khi re-throw |
| `src/renderer/src/components/profile/__tests__/ProfileEditor.test.tsx` | Click "Save Changes" (`data-testid="save-profile-btn"`) → `saveProfile` được gọi đúng `scope`/`scopeId` (test hiện có, không cần sửa vì tracer nằm trong hook, không phải component) |

**Mục tiêu:** +7 test case trong `useProfile.test.ts` (không phá vỡ test hiện có).

## 4. Acceptance Criteria

- [ ] `useProfile()` (read hook) tạo `ui:profile.resolve` span khi mount, `span.ok({ hasSecurityLock })` khi cả 2 RPC thành công, `span.fail(err)` khi có lỗi
- [ ] `profile.getResolved` và `profile.getUser` trong `useProfile()` đều nhận `traceId: span.id` trong params (cùng 1 id, vì là 1 thao tác "load" duy nhất)
- [ ] `useProfileActions().saveProfile()` tạo `ui:profile.update` span với field `scope` đúng 1 trong `'user'|'dept'|'company'`, `targetId` set khi có `scopeId`
- [ ] RPC `profile.updateUser`/`updateCompany`/`updateDept` đều nhận `traceId: span.id` trong params tương ứng với nhánh scope đang chạy
- [ ] `span.fail()` được gọi kèm field `scope` trước khi lỗi được re-throw cho `toast.error` xử lý (không nuốt lỗi)
- [ ] Tracer flow name dùng prefix `ui:` (không phải `profile:`) để TracePanel `isBackend` heuristic (`TracePanel.tsx:42`) hiển thị đúng badge "▼ cli"
- [ ] Không thêm tracer/span cho BL-PRF-03/04 trong solution này — các sub-flow đó ngoài phạm vi TDD-FE-11
- [ ] Test suite `useProfile.test.ts` đạt tối thiểu 7 test case mới, không phá vỡ test hiện có
