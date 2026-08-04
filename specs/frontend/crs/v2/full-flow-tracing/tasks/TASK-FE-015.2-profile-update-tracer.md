# TASK-FE-015.2: Instrument `useProfileActions().saveProfile()` write hook (BL-PRF-01, update)

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-015 §1, §2.2 (phần ghi)](../solutions/SOL-FE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-015.1 (cùng file `useProfile.ts`, làm sau để tránh conflict)
**Status:** ✅ Done (2026-08-04) — implemented as spec'd; `uiProfileUpdateFlow` tracer already existed in `tracers.ts` (re-added after a shared-file reset during this session, additive-only). `pnpm tsc --noEmit` clean; `useProfile.test.ts` 12/12 passing (4 new write-hook cases, 12 total combined with TASK-FE-015.1), `ProfileEditor.test.tsx` 5/5 passing.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useProfileActions"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useProfileActions", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

3 nhánh scope đều gọi cùng entry point UI ("Save Changes" button, `ProfileEditor.tsx:81`, `data-testid="save-profile-btn"`) nhưng RPC method khác nhau theo scope: `profile.updateUser`/`profile.updateCompany`/`profile.updateDept`. Một span duy nhất bao phủ cả 3 nhánh, field `scope` phân biệt — đối xứng với cách backend `profile:updateLayer` dùng 1 tracer cho cả 3 scope.

## File: `src/renderer/src/hooks/useProfile.ts` [MODIFY] — chỉ phần ghi (`useProfileActions()`)

```typescript
// --- Write hook (BL-PRF-01) ---

export function useProfileActions() {
  const saveProfile = useCallback(
    async (scope: 'user' | 'dept' | 'company', profile: OrcaProfile, scopeId?: string) => {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // BL-PRF-01: 1 span bao phủ toàn bộ save + refetch resolved (nếu scope='user'),
      // field `scope` phân biệt 3 nhánh.
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

**Vì sao không `traceId` trên lời gọi `profile.getResolved` refetch sau save:** đây là thao tác refetch riêng biệt (đọc lại cache đã invalidate), không phải một phần của span `ui:profile.update` — theo nguyên tắc "1 tracer = 1 sub-flow" (CR-TRACE-000 §4). Để tránh tạo 1 span con lồng bên trong `saveProfile` gây rối TracePanel, refetch này không-traced (single RPC call, latency thấp) — nhất quán với nguyên tắc chống over-instrumentation (CR-TRACE-000 §5).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useProfile.test.ts
pnpm test --run src/renderer/src/components/profile/__tests__/ProfileEditor.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useProfileActions().saveProfile()` tạo `ui:profile.update` span với field `scope` đúng 1 trong `'user'|'dept'|'company'`, `targetId` set khi có `scopeId`
- [ ] RPC `profile.updateUser`/`updateCompany`/`updateDept` đều nhận `traceId: span.id` trong params tương ứng với nhánh scope đang chạy
- [ ] `span.fail()` được gọi kèm field `scope` trước khi lỗi được re-throw cho `toast.error` xử lý (không nuốt lỗi)
- [ ] Lời gọi refetch `profile.getResolved` sau khi save `scope='user'` KHÔNG nhận `traceId` (không phải một phần của span `ui:profile.update`)
- [ ] Click "Save Changes" (`data-testid="save-profile-btn"`) trong `ProfileEditor.tsx:81` → `saveProfile` được gọi đúng `scope`/`scopeId` (test hiện có, không cần sửa vì tracer nằm trong hook, không phải component)
- [ ] Test suite `useProfile.test.ts` đạt tối thiểu 4 test case mới cho phần ghi (`start({ scope: 'user' })`; forward `traceId` vào `profile.updateCompany`; field `targetId` khi `scope='dept'`; `fail(err, { scope })` trước re-throw) — tổng cộng ≥ 7 test case mới trong file kết hợp với TASK-FE-015.1
