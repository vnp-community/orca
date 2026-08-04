# TASK-BE-015.1: Đăng ký 4 tracer `profile:*` và instrument `profile-rpc-handler.ts` (BL-PRF-01)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-015](../solutions/SOL-BE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-015)
**Status:** ✅ Done (2026-08-04) — Added 4 `profile:*` tracers to `tracers.ts` (real code matched task doc exactly — `updateUser`/`updateCompany`/`updateDept`/`invalidate` all still at the same shape). Instrumented all 4 write handlers with `profile:updateLayer`, added `traceId?` to `UserIdParam`/`ProfileJsonParam`/`SetCompanyProfileParam`/`SetDeptProfileParam`. `profile.setUserDept` left untouched as required. typecheck clean (pre-existing unrelated `DeptIdParam` unused-var error confirmed present before this change too); 19/19 tests pass in `profile-rpc.test.ts`; detect_changes (staged) confirms LOW risk, only expected symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "profile.updateUser"
codegraph explore "profile.updateCompany"
codegraph explore "profile.updateDept"
codegraph explore "profile.invalidate"
```

Cả 4 là RPC handler đã tồn tại trong `src/main/profile/profile-rpc-handler.ts` (MODIFY case). Chạy:

```
gitnexus_impact({ target: "profile.updateUser", direction: "upstream" })
```

(Phần đăng ký 4 tracer mới vào `Tracers` là thay đổi additive-only — chỉ cần `codegraph explore "Tracers"` để tránh trùng tên.) Xác nhận `profile.setUserDept` KHÔNG bị đụng tới (ngoài phạm vi task này). Báo cáo blast radius trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 4 tracer mới (`profileUpdateLayerFlow`/`profileResolveFlow`/`profileProjectRouteFlow`/`profileAgentSpawnFlow`) trong `tracers.ts`, sau đó bọc 4 write-handler của `profile-rpc-handler.ts` (`updateUser`/`updateCompany`/`updateDept`/`invalidate`) bằng span `profile:updateLayer` — mỗi handler luôn `step('invalidateCache')` trước khi `ok()`. Các task TASK-BE-015.2 → TASK-BE-015.4 phụ thuộc vào 4 tracer được khai báo ở đây (chỉ `profileUpdateLayerFlow` có call site trong task này, 3 tracer còn lại chỉ khai báo tên).

## File: `src/shared/trace/tracers.ts` [MODIFY]

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow) unchanged...

  // ── Profile & Project (CR-TRACE-015) ────────────────────────────────────────
  /** BL-PRF-01: update company/dept/user profile + cache invalidate */
  profileUpdateLayerFlow:  createTracer('profile:updateLayer'),
  /** BL-PRF-02: 3-layer resolve (cache hit/miss + merge, không trace merge() nội bộ) */
  profileResolveFlow:      createTracer('profile:resolve'),
  /** BL-PRF-03: project create + dev-server relay routing (field `op` phân biệt) */
  profileProjectRouteFlow: createTracer('profile:projectRoute'),
  /** BL-PRF-04: profile-aware agent spawn orchestration */
  profileAgentSpawnFlow:   createTracer('profile:agentSpawnRoute'),
} as const
```

> Lưu ý: `profileResolveFlow`, `profileProjectRouteFlow`, `profileAgentSpawnFlow` chỉ được khai báo tên ở task này — call site thật thuộc TASK-BE-015.2, TASK-BE-015.3, TASK-BE-015.4.

## File: `src/main/profile/profile-rpc-handler.ts` [MODIFY]

Thêm import `Tracers`, thêm field `traceId` optional vào 4 schema ghi (không đổi schema đọc), rồi patch 4 handler `updateUser`/`updateCompany`/`updateDept`/`invalidate`.

```typescript
import { Tracers } from '../../shared/trace/tracers'

// ── Shared param schemas ─────────────────────────────────────────────────────
// Thêm traceId optional vào các schema ghi (không đổi schema đọc)
const SetCompanyProfileParam = z.object({
  companyId: z.string().min(1),
  profile: z.record(z.string(), z.unknown()),
  traceId: z.string().optional(),   // [NEW] CR-TRACE-000 §3.3 — WS RPC row
})
const SetDeptProfileParam = z.object({
  deptId: z.string().min(1),
  profile: z.record(z.string(), z.unknown()),
  traceId: z.string().optional(),   // [NEW]
})
const ProfileJsonParam = z.object({
  profile: z.record(z.string(), z.unknown()),
  traceId: z.string().optional(),   // [NEW]
})
const UserIdParam = z.object({
  userId: z.string().min(1).optional(),
  traceId: z.string().optional(),   // [NEW] — chỉ dùng cho profile.invalidate
})
```

```typescript
// ── profile.updateUser (dòng 110-127 hiện tại) ────────────────────────────────
defineMethod({
  name: 'profile.updateUser',
  params: ProfileJsonParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')

    const span = Tracers.profileUpdateLayerFlow.start(
      { scope: 'user', targetId: userId },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const profile = params.profile as OrcaProfile
      if ('security' in profile && profile.security !== undefined) {
        span.fail('PROFILE_FIELD_LOCKED', { scope: 'user' })
        throw new Error('PROFILE_FIELD_LOCKED: security section is company-admin only')
      }

      await profileService.setUserProfile(userId, profile)
      span.step('invalidateCache', { scope: 'user', affectedUserId: userId })
      profileResolver.invalidate(userId)
      span.ok({ scope: 'user', targetId: userId })
      return { success: true }
    } catch (err) {
      span.fail(err, { scope: 'user' })
      throw err
    }
  }
}),

// ── profile.updateCompany (dòng 142-157 hiện tại) ─────────────────────────────
defineMethod({
  name: 'profile.updateCompany',
  params: SetCompanyProfileParam,
  handler: async (params, ctx) => {
    requireAdmin(ctx)
    const userId = ctx.userId ?? 'unknown'
    const span = Tracers.profileUpdateLayerFlow.start(
      { scope: 'company', targetId: params.companyId },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      await profileService.setCompanyProfile(params.companyId, params.profile as OrcaProfile, userId)
      // Company thay đổi → invalidate toàn bộ cache (mọi user), không chỉ 1 userId
      span.step('invalidateCache', { scope: 'company' })
      profileResolver.invalidate()
      span.ok({ scope: 'company', targetId: params.companyId })
      return { success: true }
    } catch (err) {
      span.fail(err, { scope: 'company' })
      throw err
    }
  }
}),

// ── profile.updateDept (dòng 161-176 hiện tại) ────────────────────────────────
defineMethod({
  name: 'profile.updateDept',
  params: SetDeptProfileParam,
  handler: async (params, ctx) => {
    requireAdmin(ctx)
    const userId = ctx.userId ?? 'unknown'
    const span = Tracers.profileUpdateLayerFlow.start(
      { scope: 'dept', targetId: params.deptId },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      await profileService.setDeptProfile(params.deptId, params.profile as OrcaProfile, userId)
      span.step('invalidateCache', { scope: 'dept' })
      profileResolver.invalidate() // an toàn: invalidate toàn bộ vì không track dept membership trong cache key
      span.ok({ scope: 'dept', targetId: params.deptId })
      return { success: true }
    } catch (err) {
      span.fail(err, { scope: 'dept' })
      throw err
    }
  }
}),

// ── profile.invalidate (dòng 180-188 hiện tại) — admin manual invalidate ──────
defineMethod({
  name: 'profile.invalidate',
  params: UserIdParam,
  handler: async (params, ctx) => {
    requireAdmin(ctx)
    const span = Tracers.profileUpdateLayerFlow.start(
      { scope: 'manual', targetId: params.userId ?? 'all' },
      params.traceId ? { id: params.traceId } : undefined
    )
    span.step('invalidateCache', { scope: 'manual', affectedUserId: params.userId })
    profileResolver.invalidate(params.userId)
    span.ok({ scope: 'manual', targetId: params.userId ?? 'all' })
    return { success: true, cleared: params.userId ?? 'all' }
  }
}),
```

> **Lưu ý:** `profile.setUserDept` (dòng 190-202) cũng gọi `profileResolver.invalidate(params.userId)` — không được liệt kê trong BL-PRF-01 (nó thuộc BL-PRF-03 assignment, không phải profile update). **Không** thêm tracer cho nó ở task này để giữ đúng ranh giới sub-flow; nếu cần quan sát, đề xuất bổ sung ở CR follow-up riêng, không lẫn vào `profile:updateLayer`.

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] 4 tracer `profileUpdateLayerFlow`/`profileResolveFlow`/`profileProjectRouteFlow`/`profileAgentSpawnFlow` tồn tại trong `tracers.ts` với đúng flow name `profile:updateLayer`/`profile:resolve`/`profile:projectRoute`/`profile:agentSpawnRoute`
- [ ] `Tracers.profileUpdateLayerFlow` luôn có `step('invalidateCache')` trước `ok()` ở cả 4 handler `updateUser`/`updateCompany`/`updateDept`/`invalidate`
- [ ] `profile.updateUser` với `profile.security !== undefined` → `span.fail('PROFILE_FIELD_LOCKED')` trước khi throw
- [ ] 4 schema ghi (`SetCompanyProfileParam`/`SetDeptProfileParam`/`ProfileJsonParam`/`UserIdParam`) có field `traceId?: string` optional, backward-compatible
- [ ] `profile.setUserDept` KHÔNG bị patch trong task này
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
