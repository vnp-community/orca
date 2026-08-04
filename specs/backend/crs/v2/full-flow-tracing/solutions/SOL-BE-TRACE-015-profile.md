# SOL-BE-TRACE-015: Profile & Project — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**TDD Ref:** TDD-14 (User Profile Hierarchy, §3 ProfileResolver deep-merge — [14-profile-hierarchy.md](../../../../tdd/v5/14-profile-hierarchy.md)), TDD-15 (Project-Dev Server Binding, tham chiếu `ProjectService`/`ProjectServerRouter`)
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ thêm tracer calls vào code đã implement (`src/main/profile/`, `src/main/project/`), không đổi business logic, không đổi DB schema

---

## 1. Phân tích phạm vi (Backend-side only)

Bốn service backend trong CR-TRACE-015 **đã tồn tại và đã implement đầy đủ** theo TDD-14/TDD-15 (khác với giả định ban đầu của CR — lúc CR-TRACE-015 được viết, `AIProviderResolver` còn là interface-only; nay `AIProviderService`/`ProviderResolver` đã có, xem SOL-BE-TRACE-016). Đã verify trực tiếp bằng Read từng file — toàn bộ file:line trong CR-TRACE-015 khớp với code thực tế hiện tại (không lệch dòng), nên solution này dùng nguyên các vị trí đó.

| File | Hàm/method cần patch | Dòng thực tế đã verify | Gap |
|------|----------------------|-------------------------|-----|
| `src/shared/trace/tracers.ts` | thêm 4 tracer mới | — | ❌ Thiếu `profileUpdateLayerFlow`/`profileResolveFlow`/`profileProjectRouteFlow`/`profileAgentSpawnFlow` |
| `src/main/profile/profile-rpc-handler.ts` | `profile.updateCompany` (142), `profile.updateDept` (161), `profile.updateUser` (110), `profile.invalidate` (180) | ✅ khớp CR | ❌ Chưa có `Tracers.profileUpdateLayerFlow` bao quanh handler |
| `src/main/profile/ProfileResolver.ts` | `resolve()` (44), cache check (46-49), `merge()` (58-62), `invalidate()` (73) | ✅ khớp CR | ❌ Chưa có `Tracers.profileResolveFlow`; **không** thêm span cho `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()` (thuần in-memory — CR-TRACE-000 §5) |
| `src/main/project/ProjectService.ts` | `create()` (81), validate dev server (83) | ✅ khớp CR | ❌ Chưa có `Tracers.profileProjectRouteFlow` (field `op: 'create'`) |
| `src/main/project/ProjectServerRouter.ts` | `getRelayForProject()` (32), `getProjectContext()` (49) | ✅ khớp CR | ❌ `getRelayForProject()` chưa có tracer (field `op: 'getRelay'`); `getProjectContext()` **không** có tracer riêng — chỉ được đo gián tiếp qua `step('getProjectContext')` của `profile:agentSpawnRoute` khi gọi từ `ProfileAwareAgentSpawner` |
| `src/main/project/ProfileAwareAgentSpawner.ts` | `spawn()` (67), `AgentSpawnOptions` (29) | ✅ khớp CR | ✅ **Đã resolve 2026-08-02 (Known Conflicts, xem `tasks/00-index.md`)** — `spawn()` bản thân KHÔNG được solution này instrument nữa; span canonical duy nhất bọc `spawn()` là `Tracers.agentOrchSpawn` (SOL-BE-TRACE-002 §2.2). `Tracers.profileAgentSpawnFlow` chuyển sang bọc phần prep (`assertAccess`) tại `project-rpc-handler.ts`, **trước khi** gọi `spawn()` — xem §2.6/§2.7 bên dưới |
| `src/main/project/project-rpc-handler.ts` | `project.agentSpawn` handler — `assertAccess` (đã tồn tại, xem SOL-BE-TRACE-002 §2.3) | ✅ khớp CR | ❌ **[Mới, thay thế cho việc sửa `ProfileAwareAgentSpawner.ts`]** Chưa có `Tracers.profileAgentSpawnFlow` bọc `assertAccess`; chưa forward `routeSpan.id` làm `traceId` vào `agentSpawner.spawn()` |

**Ngoài phạm vi (agent-side, do companion effort xử lý):** bất kỳ xử lý nào xảy ra sau khi `relay.call('agent.exec', ...)` rời khỏi `ProfileAwareAgentSpawner.spawn()` — tức toàn bộ phần Dev Server nhận `agent.exec` và spawn tiến trình agent thực tế (`src/relay/agent-*.ts`). Backend chỉ chịu trách nhiệm đến điểm `relay:agentCall` (đã có tracer sẵn tại `dev-server-relay-bridge.ts:21`) nhận `traceId` và resume.

**Phối hợp với SOL-BE-TRACE-002 và SOL-BE-TRACE-018 (đã resolve 2026-08-02):** Ban đầu solution này tự bọc toàn bộ `ProfileAwareAgentSpawner.spawn()` bằng `Tracers.profileAgentSpawnFlow` — xung đột trực tiếp với SOL-BE-TRACE-002 (bọc cùng hàm bằng `Tracers.agentOrchSpawn`). Theo quyết định resolve tại `tasks/00-index.md` (2026-08-02): `agentOrch:spawn` (SOL-BE-TRACE-002) là span canonical duy nhất cho `spawn()` — điểm hội tụ thật sự "spawn 1 AI agent". `profile:agentSpawnRoute` KHÔNG còn bọc `spawn()` độc lập; thay vào đó nó bọc riêng phần chuẩn bị/routing theo profile domain xảy ra TRƯỚC khi gọi `spawn()` — cụ thể là `projectService.assertAccess(...)` tại `project.agentSpawn` RPC handler (`project-rpc-handler.ts`, đã tồn tại sẵn trong code, xem SOL-BE-TRACE-002 §2.3) — rồi forward `routeSpan.id` vào `agentSpawner.spawn({ ..., traceId: routeSpan.id })` để `agentOrch:spawn` **resume** đúng id đó (CR-TRACE-000 §3.1), thay vì 2 span độc lập chạy song song cho cùng 1 lần spawn. `AgentSpawnOptions.traceId` do SOL-BE-TRACE-002 §2.2 sở hữu (không phải solution này thêm). Với luồng Task Graph (`task.execute` → `TaskAgentExecutor`), `taskGraph:execute` (SOL-BE-TRACE-018) forward thẳng vào `agentOrch:spawn`, KHÔNG đi qua `profile:agentSpawnRoute` (span đó chỉ tồn tại ở nhánh `project.agentSpawn` RPC trực tiếp).

**Core API dependency:** toàn bộ code dưới đây dùng `resume?: { id: string }` tại `Tracer.start()` — đây là thay đổi CR-TRACE-000 §3.1 áp dụng lên `src/shared/trace/index.ts`, **chưa tồn tại trong code hiện tại** (đã verify: `Tracer.start(fields?: TraceFields): TraceSpan` — không có tham số `resume`). Solution này giả định core API change đó đã ship trước (Phase 0 theo CR-TRACE-000 §6) — không nằm trong phạm vi patch của SOL-BE-TRACE-015.

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 4 tracer

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

### 2.2 `src/main/profile/profile-rpc-handler.ts` — BL-PRF-01

Patch cả 3 write-handler (`updateCompany`/`updateDept`/`updateUser`) và `invalidate`. Thêm import `Tracers`, thêm optional `traceId` vào param schema theo CR-TRACE-000 §3.3 hàng "WebSocket RPC".

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

> **Lưu ý:** `profile.setUserDept` (dòng 190-202) cũng gọi `profileResolver.invalidate(params.userId)` — không được liệt kê trong BL-PRF-01 của CR (nó thuộc BL-PRF-03 assignment, không phải profile update). Solution này **không** thêm tracer cho nó để giữ đúng ranh giới sub-flow của CR; nếu cần quan sát, đề xuất bổ sung ở CR follow-up riêng, không lẫn vào `profile:updateLayer`.

### 2.3 `src/main/profile/ProfileResolver.ts` — BL-PRF-02

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

### 2.4 `src/main/project/ProjectService.ts` — BL-PRF-03 (phần `create()`)

```typescript
import { Tracers } from '../../shared/trace/tracers'

async create(params: CreateProjectParams): Promise<OrcaProject> {
  const span = Tracers.profileProjectRouteFlow.start({
    op: 'create', devServerId: params.devServerId, memberCount: 1, // creator luôn là owner đầu tiên
  })

  const server = this.devServerManager.get(params.devServerId)
  span.step('validateDevServer', { devServerId: params.devServerId })
  if (!server) {
    span.fail('DEV_SERVER_NOT_FOUND', { devServerId: params.devServerId })
    throw new Error(`DEV_SERVER_NOT_FOUND: devServerId '${params.devServerId}' does not exist`)
  }

  const id = randomUUID()
  const now = Date.now()
  const defaultBranch = params.defaultBranch ?? 'main'
  const visibility = params.visibility ?? 'team'

  await this.pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_v5_projects
         (id, name, description, dev_server_id, repo_path, default_branch, visibility, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [id, params.name, params.description ?? null, params.devServerId, params.repoPath,
       defaultBranch, visibility, params.createdBy, now, now]
    )
  )
  await this.addMember(id, params.createdBy, 'owner')

  span.ok({ op: 'create', projectId: id, devServerId: params.devServerId })
  return {
    id, name: params.name, description: params.description, devServerId: params.devServerId,
    repoPath: params.repoPath, defaultBranch, visibility, createdBy: params.createdBy,
    createdAt: new Date(now), updatedAt: new Date(now),
  }
}
```

### 2.5 `src/main/project/ProjectServerRouter.ts` — BL-PRF-03 (phần `getRelayForProject()`) + BL-PRF-04 support

```typescript
import { Tracers } from '../../shared/trace/tracers'

async getRelayForProject(projectId: string, userId: string): Promise<DevServerRelayBridge> {
  const span = Tracers.profileProjectRouteFlow.start({ op: 'getRelay', projectId })
  try {
    await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) { span.fail('PROJECT_NOT_FOUND', { op: 'getRelay', projectId }); throw new Error('PROJECT_NOT_FOUND') }
    const server = this.devServerManager.get(project.devServerId)
    if (!server) { span.fail('DEV_SERVER_NOT_FOUND', { op: 'getRelay', projectId }); throw new Error('DEV_SERVER_NOT_FOUND') }
    const relay = this.relayPool.getOrConnect(project.devServerId, server)
    span.ok({ op: 'getRelay', projectId, devServerId: project.devServerId })
    return relay
  } catch (err) {
    span.fail(err, { op: 'getRelay', projectId })
    throw err
  }
}

// getProjectContext() — KHÔNG thêm tracer riêng ở đây (xem §1 bảng gap): hàm này chỉ được
// gọi từ ProfileAwareAgentSpawner.spawn(), nơi đã có step('getProjectContext') bao quanh nó
// trong span profile:agentSpawnRoute. Thêm span thứ 2 ở đây sẽ tạo 2 lớp instrumentation
// chồng nhau cho cùng 1 lời gọi function nội bộ (không băng qua boundary) — vi phạm
// nguyên tắc "step() chỉ khi băng qua boundary hoặc là điểm rẽ nhánh quan trọng" (CR-TRACE-000 §5).
async getProjectContext(
  projectId: string, userId: string, profileResolver: ProfileResolver
): Promise<ProjectContext> {
  const member = await this.projectService.assertAccess(projectId, userId)
  const project = await this.projectService.get(projectId)
  if (!project) throw new Error('PROJECT_NOT_FOUND')
  const devServer = this.devServerManager.get(project.devServerId)
  if (!devServer) throw new Error('DEV_SERVER_NOT_FOUND')
  const resolvedProfile = await profileResolver.resolve(userId) // tự có span profile:resolve riêng
  return { project, member, devServer, resolvedProfile }
}
```

### 2.6 `src/main/project/ProfileAwareAgentSpawner.ts` — BL-PRF-04 (✅ resolved 2026-08-02, xem `tasks/00-index.md`)

**Solution này KHÔNG patch `spawn()`.** Bản instrument ban đầu ở đây (bọc trọn thân `spawn()` bằng `Tracers.profileAgentSpawnFlow`) xung đột trực tiếp với SOL-BE-TRACE-002 §2.2 (`Tracers.agentOrchSpawn` bọc cùng hàm). Quyết định resolve: `agentOrch:spawn` là span canonical duy nhất cho `spawn()` — SOL-BE-TRACE-015 không tạo span thứ 2 bọc cùng thân hàm. `AgentSpawnOptions.traceId` cũng do SOL-BE-TRACE-002 §2.2 sở hữu, không phải solution này thêm.

Thay vào đó, `Tracers.profileAgentSpawnFlow` (`profile:agentSpawnRoute`) chuyển xuống bọc phần chuẩn bị/routing theo profile domain xảy ra **trước khi** gọi `spawn()` — xem §2.7. `getProjectContext()`/`resolveProvider()`/`relayExec` (3 step nguyên bản của §2.6 cũ) vẫn chạy y nguyên, chỉ là chúng nằm bên trong `agentOrch:spawn` (SOL-BE-TRACE-002 §2.2 dùng tên step `resolve-context`/`resolve-provider`/`relay-agent-exec`) thay vì `profile:agentSpawnRoute`.

### 2.7 `src/main/project/project-rpc-handler.ts` — `profile:agentSpawnRoute` bọc prep TRƯỚC khi gọi `spawn()`

```typescript
import { Tracers } from '../../shared/trace/tracers'

// project-rpc-handler.ts — 'project.agentSpawn' handler
// profile:agentSpawnRoute bọc riêng phần access-check theo profile/project domain
// (assertAccess) xảy ra TRƯỚC khi delegate vào spawn() — KHÔNG bọc lại spawn() (đã
// thuộc agentOrch:spawn, SOL-BE-TRACE-002 §2.2). Forward routeSpan.id làm traceId để
// agentOrch:spawn RESUME đúng id đó (CR-TRACE-000 §3.1) thay vì tạo span độc lập.
defineMethod({
  name: 'project.agentSpawn',
  params: AgentSpawnParam, // traceId optional field — đã thêm bởi SOL-BE-TRACE-002 §2.3
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')
    if (!agentSpawner) throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')

    const routeSpan = Tracers.profileAgentSpawnFlow.start(
      { projectId: params.projectId, userId },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      await projectService.assertAccess(params.projectId, userId)
      routeSpan.ok({ projectId: params.projectId })
    } catch (err) {
      routeSpan.fail(err, { projectId: params.projectId })
      throw err
    }

    // agentOrch:spawn (SOL-BE-TRACE-002) resume đúng id của routeSpan — 1 chuỗi liên tục
    // profile:agentSpawnRoute → agentOrch:spawn → relay:agentCall trên TracePanel.
    return agentSpawner.spawn({ ...params, userId, traceId: routeSpan.id })
  }
}),
```

> **Vì sao không còn "nhánh mặc định luôn tạo span mới":** trước đây `spawn()` (khi bọc bằng `profileAgentSpawnFlow`) luôn mở span mới trừ khi `options.traceId` được set. Nay `spawn()` chỉ có 1 span canonical (`agentOrch:spawn`), và `project-rpc-handler.ts` LUÔN mở `routeSpan` trước, LUÔN forward `routeSpan.id` — nên `agentOrch:spawn` LUÔN resume từ `profile:agentSpawnRoute` trên nhánh `project.agentSpawn` RPC (khác với nhánh Task Graph, nơi `taskGraph:execute` resume thẳng vào `agentOrch:spawn`, không qua `profile:agentSpawnRoute` — xem SOL-BE-TRACE-018 §1).

---

## 3. Test Plan (Vitest)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/profile/__tests__/ProfileResolver.test.ts` | `resolve() cache hit → span.ok({ cacheHit: true })` | mock sink, assert field `cacheHit` |
| | `resolve() cache miss → span.step('cacheCheck', { cacheHit: false })` rồi `ok({ cacheHit: false })` | |
| | `resolve() không emit bất kỳ trace event nào có flow khác 'profile:resolve'` (không có span lồng cho `merge()`) | grep tất cả emitted flow names trong test |
| `src/main/profile/__tests__/profile-rpc-handler.test.ts` | `profile.updateCompany → span.step('invalidateCache') luôn chạy trước ok()` | assert order of emitted events |
| | `profile.updateUser với security section → span.fail('PROFILE_FIELD_LOCKED')` | |
| | `profile.updateCompany với params.traceId → span.id === params.traceId` (cần core API resume) | |
| `src/main/project/__tests__/ProjectService.test.ts` | `create() thành công → profile:projectRoute ok({ op: 'create', projectId })` | |
| | `create() với devServerId không tồn tại → span.fail('DEV_SERVER_NOT_FOUND', { op: 'create' })` | |
| `src/main/project/__tests__/ProjectServerRouter.test.ts` | `getRelayForProject() → span field op === 'getRelay'` (phân biệt với `create()`) | |
| `src/main/project/__tests__/project-rpc-handler.test.ts` | `project.agentSpawn → routeSpan (profile:agentSpawnRoute) mở trước, ok() khi assertAccess pass` | assert order: `profile:agentSpawnRoute` start trước `agentOrch:spawn` start |
| | `project.agentSpawn forward routeSpan.id làm traceId vào agentSpawner.spawn()` | mock `agentSpawner.spawn`, assert `options.traceId === routeSpan.id` |
| | `project.agentSpawn assertAccess reject → routeSpan.fail(), KHÔNG gọi agentSpawner.spawn()` | |
| `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` | (không có test case riêng cho `profile:agentSpawnRoute` trong file này nữa — spawn() chỉ phát `agentOrch:spawn`, xem SOL-BE-TRACE-002 §3) | — |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| ProfileResolver tracing | ≥ 4 |
| profile-rpc-handler tracing | ≥ 6 |
| ProjectService tracing | ≥ 3 |
| ProjectServerRouter tracing | ≥ 3 |
| project-rpc-handler tracing (`profile:agentSpawnRoute` prep, thay cho ProfileAwareAgentSpawner tracing cũ) | ≥ 5 |
| **Total** | **≥ 21** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.profileResolveFlow` phân biệt `cacheHit: true/false` trong mọi `ok()`
- [ ] `Tracers.profileUpdateLayerFlow` luôn có `step('invalidateCache')` trước `ok()` ở cả `updateCompany`/`updateDept`/`updateUser`/`invalidate`
- [ ] `Tracers.profileProjectRouteFlow` dùng field `op` (`'create'` | `'getRelay'`) để phân biệt 2 call site trong cùng 1 tracer
- [ ] `Tracers.profileAgentSpawnFlow` (`profile:agentSpawnRoute`) bọc riêng phần `assertAccess` prep tại `project-rpc-handler.ts`'s `project.agentSpawn` handler — **KHÔNG bọc `ProfileAwareAgentSpawner.spawn()`** (đó là `agentOrch:spawn`, SOL-BE-TRACE-002, span canonical duy nhất — xem Known Conflicts resolution `tasks/00-index.md`)
- [ ] Không có bất kỳ `span`/`step()` nào được thêm cho `ProfileResolver.merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()`/`mergeEnvVars()` hay `ProjectServerRouter.getProjectContext()` khi được gọi nội bộ từ `spawn()`
- [ ] `project.agentSpawn` forward `routeSpan.id` làm `traceId` vào `agentSpawner.spawn()` để `agentOrch:spawn` resume đúng id đó
- [ ] Không tracer nào trong solution này trùng tên với `relay:agentCall`/`agent:rpc`/`devServer:*`/`agentWs:lifecycle`/`ipc:devServerProxy` đã tồn tại
- [ ] `ProfileAwareAgentSpawner.ts`/`AgentSpawnOptions` KHÔNG bị sửa trong solution này (thân `spawn()` và field `traceId?: string` đều thuộc SOL-BE-TRACE-002 §2.2) — verify bằng `gitnexus_detect_changes()` chỉ ra đúng 7 file trong bảng §1 (trong đó `project-rpc-handler.ts` được sửa thay cho `ProfileAwareAgentSpawner.ts`)
