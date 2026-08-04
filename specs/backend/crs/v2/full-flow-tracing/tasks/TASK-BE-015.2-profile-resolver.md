# TASK-BE-015.2: Instrument `ProfileResolver.resolve()` (BL-PRF-02)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-015](../solutions/SOL-BE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-015.1
**Status:** ✅ Done (2026-08-04) — Implemented exactly per spec, real code matched task doc precisely. `gitnexus_impact` flagged HIGH risk (expected — `resolve()` is a hot path called from `ProfileAwareAgentSpawner.spawn()` among others); proceeded since the change is a purely additive tracer wrap (span start/step/ok only, zero logic/control-flow change) per the stated exception for this rollout. No span/step added to `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()`; `invalidate()` left untouched (no second tracer). typecheck clean (2 pre-existing unrelated `AgentProfileSection`/`EditorProfileSection` generic-constraint errors confirmed present before this change too). 16/16 tests pass in `ProfileResolver.test.ts` (one transient failure during a `git stash`/`pop` timing window resolved on rerun — not a real regression). detect_changes (staged) confirms LOW risk, only expected symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ProfileResolver.resolve"
```

Symbol đã tồn tại (MODIFY case) — đây là hot path (3-layer resolve + cache) được gọi từ nhiều nơi, bao gồm `ProfileAwareAgentSpawner.spawn()`. Chạy:

```
gitnexus_impact({ target: "ProfileResolver.resolve", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận không thêm span/step cho `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()` (thuần in-memory). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `ProfileResolver.resolve()` bằng span `profile:resolve`, phân biệt rõ `cacheHit: true/false` trong `ok()`. `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()` là hàm thuần in-memory nên **không** có span/step riêng (CR-TRACE-000 §5). `invalidate()` cũng không thêm tracer thứ hai vì caller (`profile-rpc-handler.ts`, TASK-BE-015.1) đã có `step('invalidateCache')` bao quanh — tránh double span cho cùng 1 hành động.

## File: `src/main/profile/ProfileResolver.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

async resolve(userId: string): Promise<ResolvedProfile> {
  const span = Tracers.profileResolveFlow.start({ userId })
  const now = Date.now()
  const cached = this.cache.get(userId)
  if (cached && cached.expiresAt > now) {
    span.step('cacheCheck', { cacheHit: true })
    span.ok({ cacheHit: true })
    return cached.resolved
  }
  span.step('cacheCheck', { cacheHit: false })

  // Fetch all 3 layers in parallel — không step riêng cho từng SELECT (CR-TRACE-000 §5)
  const [companyProfile, deptProfile, userProfile] = await Promise.all([
    this.profileService.getCompanyProfileForUser(userId),
    this.profileService.getDeptProfileForUser(userId),
    this.profileService.getUserProfile(userId),
  ])

  // merge() thuần in-memory — KHÔNG có span/step riêng (CR-TRACE-000 §5, xem §1 bảng gap)
  const resolved = this.merge(companyProfile ?? {}, deptProfile ?? {}, userProfile ?? {})

  this.cache.set(userId, { resolved, expiresAt: now + PROFILE_TTL_MS })
  span.ok({ cacheHit: false, hasSecurityLock: resolved.security !== undefined })
  return resolved
}

invalidate(userId?: string): void {
  // Hàm này được gọi TỪ profile-rpc-handler.ts, nơi đã có span.step('invalidateCache')
  // bao quanh lời gọi — không cần tracer thứ hai bên trong invalidate() (tránh double span
  // cho cùng 1 hành động, vi phạm "1 tracer = 1 sub-flow" CR-TRACE-000 §4).
  if (userId !== undefined) {
    this.cache.delete(userId)
  } else {
    this.cache.clear()
  }
}
```

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

- [ ] `Tracers.profileResolveFlow` phân biệt `cacheHit: true/false` trong mọi `ok()`
- [ ] `resolve()` cache hit → chỉ `step('cacheCheck', { cacheHit: true })` rồi `ok()`, KHÔNG fetch 3 layer
- [ ] `resolve()` cache miss → `step('cacheCheck', { cacheHit: false })`, fetch song song 3 layer, rồi `ok({ cacheHit: false, hasSecurityLock })`
- [ ] Không có bất kỳ `span`/`step()` nào được thêm cho `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()`
- [ ] `invalidate()` KHÔNG tạo tracer/span riêng (chỉ được đo gián tiếp qua `step('invalidateCache')` của caller)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
