# CR-TRACE-015 — Profile & Project Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-015 |
| **Tên** | Profile & Project Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P3 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/profile.md`, `src/main/profile/ProfileService.ts`, `src/main/profile/ProfileResolver.ts`, `src/main/profile/profile-rpc-handler.ts`, `src/main/project/ProjectService.ts`, `src/main/project/ProjectServerRouter.ts`, `src/main/project/project-rpc-handler.ts`, `src/main/project/ProfileAwareAgentSpawner.ts` |

---

## 1. Vấn đề

Cả 4 lớp business logic (`ProfileService`, `ProfileResolver`, `ProjectService`/`ProjectServerRouter`, `ProfileAwareAgentSpawner`) đều **hoàn toàn không có tracer nào** (`grep "createTracer"` trong cả 4 file cho kết quả rỗng — xác nhận qua đọc trực tiếp). Đây là luồng nền tảng chạy trước MỌI lần spawn agent, nên khi agent spawn chậm hoặc dùng sai model/env, không có cách phân biệt:

- **BL-PRF-01 (Update Profile)**: `ProfileService.setCompanyProfile()`/`setDeptProfile()`/`setUserProfile()` (`src/main/profile/ProfileService.ts`) UPDATE SQLite rồi phải invalidate cache của `ProfileResolver` — nếu invalidate không chạy hoặc chạy sai `userId`, user sẽ thấy profile cũ tối đa 60s (TTL) mà không có log nào chỉ ra "đã update DB nhưng chưa invalidate cache" hay ngược lại.
- **BL-PRF-02 (3-layer merge)**: `ProfileResolver.resolve()` (`src/main/profile/ProfileResolver.ts:44`) có cache 60s TTL — khi user báo "profile không cập nhật dù đã sửa trong Settings", không biết đang do cache hit (dùng bản cũ, đúng thiết kế, chỉ cần đợi TTL) hay do bug ở tầng merge (`merge()`, dòng 83). Không phân biệt được 2 case này nếu không có metric cache-hit/cache-miss.
- **BL-PRF-03 (Project-Dev Server Assignment)**: `ProjectService.create()` (`src/main/project/ProjectService.ts:81`) validate `devServerId` rồi INSERT `orca_projects` + N dòng `orca_project_members` — nếu tạo project chậm, không biết đang chậm ở bước validate dev server (in-process) hay ở INSERT hàng loạt members.
- **BL-PRF-04 (Profile-Aware Agent Spawn)**: `ProfileAwareAgentSpawner.spawn()` (`src/main/project/ProfileAwareAgentSpawner.ts:67`) là điểm hội tụ của **3 dependency khác nhau** trong cùng 1 lần gọi — `ProjectServerRouter.getProjectContext()` (DB + access check), `ProfileResolver.resolve()` (cache/DB), `AIProviderResolver.resolveForProject()` (chưa có implementation cụ thể — interface only), rồi mới `relay.call('agent.exec', ...)` băng qua network tới Dev Server. Đây chính xác là kiểu "nhiều bước tuần tự xuyên nhiều subsystem" mà F40 sinh ra để giải quyết, nhưng hiện tại spawn chậm/fail chỉ trả về 1 exception duy nhất không có breakdown theo bước.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thành phần thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------------------|-------------------------------|-------|-----------|-------------------------------|
| Admin/Lead Browser | Renderer, gọi qua WebSocket RPC | UI | WebSocket RPC | WebSocket RPC row |
| Orca Web Server (REST `/api/profiles`, `/api/projects`) | RPC methods thực tế: `profile.getResolved`/`profile.updateUser`/`profile.updateCompany`/`profile.updateDept`/`profile.invalidate` (`src/main/profile/profile-rpc-handler.ts`), `project.create`/`project.update`/`project.agentSpawn` (`src/main/project/project-rpc-handler.ts`) — KHÔNG phải REST `/api/profiles` như flow doc mô tả, mà là WS RPC method (đúng transport chung của Orca) | Backend | WebSocket RPC | WebSocket RPC row |
| CompanyService | Gộp vào `ProfileService` (`setCompanyProfile()`, `createCompany()`, dòng 22/52) — không tồn tại lớp `CompanyService` riêng | Business Logic | in-process | — |
| ProfileResolver | `ProfileResolver` (`src/main/profile/ProfileResolver.ts`), cache TTL 60s (đúng như flow doc) | Business Logic | in-process | — |
| ProjectService | `ProjectService` (`src/main/project/ProjectService.ts`) | Business Logic | in-process → SQLite | — |
| ProjectServerRouter | `ProjectServerRouter` (`src/main/project/ProjectServerRouter.ts`) — method thực tế là `getRelayForProject()`/`getProjectContext()`/`getProject()`, KHÔNG có `getServer()`/`updateRouting()` như flow doc mô tả | Business Logic | in-process → SQLite + relay pool lookup | — |
| ProfileAwareAgentSpawner | `ProfileAwareAgentSpawner` (`src/main/project/ProfileAwareAgentSpawner.ts`) | Business Logic | in-process, rồi `relay.call('agent.exec', ...)` | `relay.call()` row |
| Server Database | SQLite (`orca_company`/`orca_departments`/`orca_projects`/`orca_project_members` — tên bảng suy ra từ comment/query trong `ProjectService.ts`, chưa đọc schema `.sql` trực tiếp) | Persistence | in-process | — |
| (bổ sung) AIProviderResolver | Chỉ là **interface** khai báo inline trong `ProfileAwareAgentSpawner.ts:21` (`resolveForProject()`), implementation cụ thể (`AIProviderService`, theo comment "TASK-021"/Phase 4) **chưa tồn tại trong code** — thuộc phạm vi CR-TRACE-016 (`ai-providers.md`), không phải CR này | Business Logic | — | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  profileUpdateLayerFlow:   createTracer('profile:updateLayer'),   // BL-PRF-01: update company/dept/user profile + cache invalidate
  profileResolveFlow:       createTracer('profile:resolve'),       // BL-PRF-02: 3-layer resolve (cache hit/miss + merge)
  profileProjectRouteFlow:  createTracer('profile:projectRoute'),  // BL-PRF-03: project create + dev-server assignment/routing
  profileAgentSpawnFlow:    createTracer('profile:agentSpawnRoute'), // BL-PRF-04: profile-aware agent spawn orchestration
}
```

**Không thêm tracer riêng cho "mergeLayer"** dù CR-TRACE-000 §4 liệt kê `profile:mergeLayer` như ví dụ minh hoạ. Lý do: `ProfileResolver.merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()` (`ProfileResolver.ts:83-308`) là các hàm **đồng bộ, thuần in-memory, không I/O** — theo đúng nguyên tắc mục 5 CR-TRACE-000 ("KHÔNG dùng `step()` cho... biến đổi dữ liệu in-memory thuần tuý"), việc này không xứng đáng một span/tracer riêng. Thông tin merge (layer nào thắng cho field nào — `sources` object đã có sẵn trong code) được gộp vào `ok()` của `profile:resolve` thay vì tracer riêng.

## 4. Instrumentation theo từng sub-flow

### BL-PRF-01 — Tạo và Cập nhật Profile (Company/Dept/User)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `scope: 'company'\|'dept'\|'user'`, `targetId` | `src/main/profile/profile-rpc-handler.ts` (`profile.updateCompany`/`profile.updateDept`/`profile.updateUser`, dòng 142/161/110) |
| UPDATE DB | gộp vào `ok()` (single-row UPDATE, không cần `step()` riêng theo mục 5 CR-TRACE-000) | `scope` | `src/main/profile/ProfileService.ts` (`setCompanyProfile()`:52, `setDeptProfile()`:101, `setUserProfile()`:134) |
| Invalidate cache | `step('invalidateCache')` — **đáng trace riêng** vì đây là điểm dễ quên/dễ sai phạm vi (invalidate 1 user vs. invalidate toàn bộ) | `scope`, `affectedUserId?` | `src/main/profile/ProfileResolver.ts:73` (`invalidate()`) |
| Hoàn tất | `ok` | `scope`, `targetId` | — |
| Lỗi (validate Zod fail, DB error) | `fail` | `scope` | — |

```typescript
// src/main/profile/profile-rpc-handler.ts — 'profile.updateCompany' handler
defineMethod({
  name: 'profile.updateCompany',
  handler: async (params, ctx) => {
    requireAdmin(ctx)
    const span = Tracers.profileUpdateLayerFlow.start({ scope: 'company' })
    try {
      await ctx.profileService.setCompanyProfile(params.companyId, params.profile)
      span.step('invalidateCache', { scope: 'company' })
      ctx.profileResolver.invalidate() // company thay đổi → invalidate toàn bộ (mọi user)
      span.ok({ scope: 'company' })
      return { ok: true }
    } catch (err) {
      span.fail(err, { scope: 'company' })
      throw err
    }
  }
})
```

### BL-PRF-02 — Profile Inheritance Resolution (3-layer merge)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `userId` | `src/main/profile/ProfileResolver.ts:44` (`resolve()`) |
| Cache hit/miss — **điểm rẽ nhánh quan trọng** (mục 5 CR-TRACE-000 rule 3) | `step('cacheCheck')` | `cacheHit: boolean` | `ProfileResolver.ts:46-49` |
| Load 3 layer song song (chỉ khi cache miss) | gộp vào `ok()`, KHÔNG step riêng cho từng SELECT (3 SELECT đơn giản trong `Promise.all`, cùng in-process) | — | `ProfileResolver.ts:52-56` |
| Merge (in-memory) | KHÔNG có step riêng — xem lý do ở mục 3 | — | `ProfileResolver.ts:58-62` |
| Hoàn tất | `ok` | `cacheHit`, `hasSecurityLock: boolean` (từ `sources`) | — |

```typescript
// src/main/profile/ProfileResolver.ts — resolve()
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

  const [companyProfile, deptProfile, userProfile] = await Promise.all([
    this.profileService.getCompanyProfileForUser(userId),
    this.profileService.getDeptProfileForUser(userId),
    this.profileService.getUserProfile(userId),
  ])
  const resolved = this.merge(companyProfile ?? {}, deptProfile ?? {}, userProfile ?? {})
  this.cache.set(userId, { resolved, expiresAt: now + PROFILE_TTL_MS })
  span.ok({ cacheHit: false, hasSecurityLock: resolved.security !== undefined })
  return resolved
}
```

### BL-PRF-03 — Project-Dev Server Assignment

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu (create) | `start` | `devServerId`, `memberCount` | `src/main/project/ProjectService.ts:81` (`create()`) |
| Validate dev server tồn tại | `step('validateDevServer')` | `devServerId` | `ProjectService.ts:83` |
| INSERT project + members | gộp vào `ok()` (INSERT hàng loạt trong cùng transaction, không cần step riêng cho từng bảng theo mục 5) | `projectId`, `memberCount` | `ProjectService.ts:~100-120` |
| Hoàn tất | `ok` | `projectId`, `devServerId` | — |
| Lỗi (`DEV_SERVER_NOT_FOUND`) | `fail` | `devServerId` | `ProjectService.ts:85` |
| (Usage, per-request) Lấy relay cho project | `start`/`ok` — tracer riêng vì đây là hot path gọi mỗi lần cần relay, khác với create (1 lần) | `projectId` | `src/main/project/ProjectServerRouter.ts:32` (`getRelayForProject()`) |

```typescript
// src/main/project/ProjectService.ts — create()
async create(params: CreateProjectParams): Promise<OrcaProject> {
  const span = Tracers.profileProjectRouteFlow.start({ devServerId: params.devServerId, memberCount: params.members.length })
  span.step('validateDevServer', { devServerId: params.devServerId })
  const server = this.devServerManager.get(params.devServerId)
  if (!server) {
    span.fail('DEV_SERVER_NOT_FOUND', { devServerId: params.devServerId })
    throw new Error(`DEV_SERVER_NOT_FOUND: devServerId '${params.devServerId}' does not exist`)
  }
  // ...existing INSERT logic...
  span.ok({ projectId: project.id, devServerId: params.devServerId })
  return project
}
```

```typescript
// src/main/project/ProjectServerRouter.ts — getRelayForProject() (hot path, gọi mỗi request cần relay)
async getRelayForProject(projectId: string, userId: string): Promise<DevServerRelayBridge> {
  const span = Tracers.profileProjectRouteFlow.start({ op: 'getRelay', projectId })
  await this.projectService.assertAccess(projectId, userId)
  const project = await this.projectService.get(projectId)
  if (!project) { span.fail('PROJECT_NOT_FOUND', { projectId }); throw new Error('PROJECT_NOT_FOUND') }
  const server = this.devServerManager.get(project.devServerId)
  if (!server) { span.fail('DEV_SERVER_NOT_FOUND', { projectId }); throw new Error('DEV_SERVER_NOT_FOUND') }
  const relay = this.relayPool.getOrConnect(project.devServerId, server)
  span.ok({ projectId, devServerId: project.devServerId })
  return relay
}
```

> Lưu ý: dùng chung 1 tracer (`profile:projectRoute`) cho cả `create()` và `getRelayForProject()` với field `op` phân biệt — 2 hàm này cùng thuộc BL-PRF-03 trong flow doc (tạo project + routing table), theo đúng nguyên tắc "1 tracer = 1 sub-flow" thay vì "1 tracer = 1 hàm".

### BL-PRF-04 — Profile-Aware Agent Execution Routing

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `projectId`, `userId` | `src/main/project/ProfileAwareAgentSpawner.ts:67` (`spawn()`) |
| Lấy ProjectContext (access check + resolve profile — **băng qua 2 dependency**) | `step('getProjectContext')` | `projectId` | `ProfileAwareAgentSpawner.ts:71` → gọi `ProjectServerRouter.getProjectContext()` (`ProjectServerRouter.ts:49`), bên trong đó gọi `ProfileResolver.resolve()` (đã có tracer riêng `profile:resolve` — KHÔNG lồng thêm span con ở đây, chỉ đo tổng thời gian bước này) |
| Compose env (in-memory) | KHÔNG step riêng — biến đổi thuần in-memory (mục 5) | — | `ProfileAwareAgentSpawner.ts:74-92` |
| Resolve AI provider | `step('resolveProvider')` — **có khả năng fail độc lập** (mục 5 rule 2) dù implementation cụ thể chưa tồn tại | `projectId`, `preferredModel?` | `ProfileAwareAgentSpawner.ts:96` (`providerService.resolveForProject()`) — interface only, xem CR-TRACE-016 khi có implementation |
| Lấy relay cho project | gộp vào bước tiếp theo (đã có span riêng ở `profile:projectRoute` nếu cần chi tiết) | `projectId` | `ProfileAwareAgentSpawner.ts:110` (`router.getRelayForProject()`) |
| Gọi `agent.exec` qua relay (băng qua process boundary) | `step('relayExec')`, forward `traceId` | `binary` | `ProfileAwareAgentSpawner.ts:115` — `relay.call('agent.exec', {...})`; hop này tự động có `relay:agentCall` span sẵn (`DevServerRelayBridge.call()`) |
| Hoàn tất | `ok` | `sessionId`, `providerId?` | `ProfileAwareAgentSpawner.ts:125-128` |
| Lỗi (bất kỳ bước nào ném exception) | `fail` | `projectId`, `phase` | — |

```typescript
// src/main/project/ProfileAwareAgentSpawner.ts — spawn()
async spawn(options: AgentSpawnOptions): Promise<AgentSpawnResult> {
  const { projectId, userId, command, extraEnv, workdir } = options
  const span = Tracers.profileAgentSpawnFlow.start({ projectId, userId })

  try {
    span.step('getProjectContext', { projectId })
    const ctx = await this.router.getProjectContext(projectId, userId, this.profileResolver)
    const { project, resolvedProfile } = ctx

    const profileEnv: Record<string, string> = {
      ...(resolvedProfile.envVars ?? {}),
      ...(resolvedProfile.shell?.envVars ?? {}),
      ...(extraEnv ?? {}),
    }
    // ...existing PATH/ORCA_* env composition unchanged...

    span.step('resolveProvider', { projectId, preferredModel: resolvedProfile.agent?.preferredModel })
    const provider = await this.providerService.resolveForProject(projectId, resolvedProfile.agent?.preferredModel)
    if (provider) {
      profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
      profileEnv['ORCA_AI_MODEL_ID'] = provider.modelId
      profileEnv['ORCA_ACCOUNT_ID'] = provider.providerId
    }

    const relay = await this.router.getRelayForProject(projectId, userId)
    const commandParts = command.trim().split(/\s+/).filter(Boolean)
    span.step('relayExec', { binary: commandParts[0] ?? '', traceId: span.id })
    const result = await relay.call('agent.exec', {
      binary: commandParts[0] ?? '', args: commandParts.slice(1),
      cwd: workdir ?? project.repoPath, env: profileEnv, timeoutMs: 5 * 60 * 1000,
    })

    const sessionId = (result as { sessionId?: string }).sessionId ?? randomId()
    span.ok({ sessionId, providerId: provider?.providerId })
    return { sessionId, provider: provider ? { providerId: provider.providerId, modelId: provider.modelId } : undefined }
  } catch (err) {
    span.fail(err, { projectId })
    throw err
  }
}
```

## 5. Lan truyền traceId qua transport của flow này

1. **Browser → WebSocket RPC (`profile.updateCompany`/`project.create`/`project.agentSpawn`)**: theo CR-TRACE-000 §3.3 hàng "WebSocket RPC", `params.traceId` (nếu Browser có tracer riêng cho action Settings/Project UI) nên được đọc tại `defineMethod()` handler và dùng để resume `profile:updateLayer`/`profile:projectRoute`/`profile:agentSpawnRoute`. Hiện tại các Zod schema trong `profile-rpc-handler.ts`/`project-rpc-handler.ts` không có field `traceId` — cần bổ sung optional khi core API ship.
2. **`ProfileAwareAgentSpawner.spawn()` → `ProfileResolver.resolve()` (cùng process, không phải network)**: đây là lời gọi hàm nội bộ (không qua transport nào) — theo CR-TRACE-000 §3.1, KHÔNG cần resume `profile:resolve` bằng id của `profile:agentSpawnRoute` vì đây là 2 concern khác nhau (resolve có cache riêng, được gọi từ nhiều nơi khác nhau ngoài spawn — ví dụ `profile.getResolved` RPC). Việc lồng `resume` ở đây sẽ làm nhiễu: mọi lần resolve profile (kể cả khi user chỉ mở Settings) sẽ mang `id` của lần spawn agent gần nhất nếu implement sai. Giữ 2 tracer độc lập là đúng.
3. **`ProfileAwareAgentSpawner.spawn()` → `relay.call('agent.exec', ...)` (băng qua process boundary tới Dev Server)**: đây MỚI là điểm cần propagate theo hàng `relay.call()` trong §3.3 — sửa lời gọi thành `relay.call('agent.exec', { ...existingParams, traceId: span.id }, 5 * 60 * 1000)` để `relayCallTracer` (`relay:agentCall`, đã tồn tại tại `dev-server-relay-bridge.ts:607`) resume đúng `id`, nối `profile:agentSpawnRoute` (Main) với `relay:agentCall` (transport) thành 1 trace end-to-end trong TracePanel — cần core API `resume` (CR-TRACE-000 mục 3) trước khi implement được.
4. **`AIProviderResolver.resolveForProject()`**: interface chưa có implementation cụ thể — khi `AIProviderService` (CR-TRACE-016) được xây và nó tự gọi ra ngoài (ví dụ relay để lấy credential từ Dev Server), việc propagate `traceId` từ `profile:agentSpawnRoute` sang tracer của CR-TRACE-016 thuộc phạm vi CR đó, không phải CR này — chỉ ghi nhận `step('resolveProvider')` đo latency tổng của lời gọi.

## Acceptance Criteria

- [ ] `profile:resolve` phân biệt rõ `cacheHit: true/false` trong mọi `ok()` — dùng để đo tỉ lệ cache hit thực tế của TTL 60s
- [ ] `profile:updateLayer` luôn gọi `step('invalidateCache')` trước `ok()` — verify bằng cách bật `ORCA_TRACE=1`, update company profile, xác nhận `profile:resolve` lần gọi tiếp theo có `cacheHit: false`
- [ ] `profile:projectRoute` dùng chung 1 tracer cho cả `ProjectService.create()` và `ProjectServerRouter.getRelayForProject()`, phân biệt bằng field `op`
- [ ] `profile:agentSpawnRoute` có breakdown latency riêng cho `getProjectContext`/`resolveProvider`/`relayExec` — không gộp chung thành 1 khoảng thời gian duy nhất
- [ ] KHÔNG có tracer/span riêng cho hàm `merge()`/`mergeScalar()`/`mergeShell()`/`mergeMcpServers()` (thuần in-memory, vi phạm mục 5 CR-TRACE-000 nếu thêm)
- [ ] `relay.call('agent.exec', ...)` trong `ProfileAwareAgentSpawner.spawn()` gửi `traceId: span.id` trong params khi CR-TRACE-000 mục 3 (core API resume) ship
- [ ] Không tracer nào trong CR này trùng tên với `relay:agentCall`/`agent:rpc`/`devServer:manager` đã tồn tại
- [ ] Khi `AIProviderResolver` có implementation cụ thể (CR-TRACE-016), `step('resolveProvider')` trong CR này được review lại để đảm bảo không trùng lặp instrumentation với tracer mới của CR-TRACE-016
