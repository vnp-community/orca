# TASK-BE-015.4: Instrument profile-driven agent-spawn routing (BL-PRF-04) — `profile:agentSpawnRoute` bọc prep tại `project-rpc-handler.ts`

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-015](../solutions/SOL-BE-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-015.1, TASK-BE-015.3, **TASK-BE-002.2, TASK-BE-002.3** (mới thêm — xem "✅ Đã giải quyết" bên dưới; task này resume vào `agentOrch:spawn` và sửa tiếp file đã bị TASK-BE-002.3 patch)
**Status:** ✅ Done (2026-08-04) — Implemented exactly per spec: `profile:agentSpawnRoute` wraps only `assertAccess` in the `project.agentSpawn` handler and forwards `routeSpan.id` as `traceId` into `agentSpawner.spawn()`; `ProfileAwareAgentSpawner.ts` untouched as required. Verified TASK-BE-002.2/002.3 were already done before starting. One notable ripple: this changes `project.agentSpawn`'s downstream `traceId` semantics — it is now ALWAYS a truthy string (routeSpan's fresh-or-resumed id), never `undefined`, even when the client omits it. Updated a TASK-BE-002.4 test in `project-rpc.test.ts` that had asserted `traceId` stays `undefined` when absent from params — that assumption predated this task and is now incorrect by design (routeSpan always forwards its own id so `agentOrch:spawn` resumes into the routing span). typecheck clean; 10/10 tests pass in `project-rpc.test.ts`; detect_changes (staged) confirms LOW risk, only expected symbols touched.

---

## ✅ Known Conflicts với TASK-BE-002.2 — đã giải quyết 2026-08-02 (xem `tasks/00-index.md`)

Bản gốc của task này bọc `ProfileAwareAgentSpawner.spawn()` bằng `Tracers.profileAgentSpawnFlow` (flow `profile:agentSpawnRoute`) — xung đột trực tiếp với `TASK-BE-002.2`, vốn bọc CÙNG hàm bằng `Tracers.agentOrchSpawn` (flow `agentOrch:spawn`). Quyết định resolve: `agentOrch:spawn` là span CANONICAL duy nhất cho `spawn()` — điểm hội tụ thật sự "spawn 1 AI agent" theo CR-TRACE-000 §4. `profile:agentSpawnRoute` không còn bọc `spawn()` nữa; nó chuyển xuống bọc phần chuẩn bị/routing theo profile domain (`assertAccess`) xảy ra **trước khi** gọi `spawn()`, tại `project-rpc-handler.ts`'s `project.agentSpawn` handler — rồi forward span id của chính nó vào `agentSpawner.spawn({ ..., traceId: routeSpan.id })` để `agentOrch:spawn` **resume** đúng id đó (CR-TRACE-000 §3.1) thay vì 2 span độc lập chạy song song cho cùng 1 lần spawn.

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "project.agentSpawn"
```

Symbol đã tồn tại (MODIFY case) — handler này đã bị `TASK-BE-002.3` patch trước đó (schema/forward `traceId`). Chạy:

```
gitnexus_impact({ target: "project.agentSpawn", direction: "upstream" })
```

**Lưu ý phối hợp:** task này PHẢI chạy sau `TASK-BE-002.2`/`TASK-BE-002.3` (xem Known Conflicts ở trên) — trước khi sửa, xác nhận 2 task đó đã DONE và `agentOrch:spawn` đã tồn tại trong `ProfileAwareAgentSpawner.spawn()`. KHÔNG bọc lại `ProfileAwareAgentSpawner.spawn()` — chỉ bọc phần `assertAccess` TRƯỚC khi gọi `spawn()`. Báo cáo blast radius trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc phần `assertAccess(...)` trong `project.agentSpawn` handler (`project-rpc-handler.ts` — đã tồn tại sẵn, do `TASK-BE-002.3` patch schema/forward trước đó) bằng span `profile:agentSpawnRoute`, rồi forward `routeSpan.id` làm `traceId` khi gọi `agentSpawner.spawn()`. Task này **không** đụng tới `ProfileAwareAgentSpawner.ts` — thân `spawn()` (3 bước `resolve-context`/`resolve-provider`/`relay-agent-exec`, span `agentOrch:spawn`) đã được `TASK-BE-002.2` sở hữu trọn vẹn.

## File: `src/main/project/ProfileAwareAgentSpawner.ts` [KHÔNG SỬA — ngoài phạm vi task này]

`spawn()` và `AgentSpawnOptions.traceId` đã được `TASK-BE-002.2` implement đầy đủ, bao gồm cả cơ chế resume (`options.traceId ? { id: options.traceId } : undefined`). Task này không patch lại file này dưới bất kỳ hình thức nào.

## File: `src/main/project/project-rpc-handler.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

// project-rpc-handler.ts — 'project.agentSpawn' handler (schema/traceId field đã có từ TASK-BE-002.3)
// profile:agentSpawnRoute bọc riêng phần access-check theo profile/project domain (assertAccess)
// xảy ra TRƯỚC khi delegate vào spawn() — KHÔNG bọc lại spawn() (đã thuộc agentOrch:spawn,
// TASK-BE-002.2). Forward routeSpan.id làm traceId để agentOrch:spawn RESUME đúng id đó.
defineMethod({
  name: 'project.agentSpawn',
  params: AgentSpawnParam,
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

    // agentOrch:spawn (TASK-BE-002.2) resume đúng id của routeSpan — 1 chuỗi liên tục
    // profile:agentSpawnRoute → agentOrch:spawn → relay:agentCall trên TracePanel.
    return agentSpawner.spawn({ ...params, userId, traceId: routeSpan.id })
  }
}),
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/project/__tests__/project-rpc-handler.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.profileAgentSpawnFlow` (`profile:agentSpawnRoute`) bọc riêng phần `assertAccess` trong `project.agentSpawn` handler — **KHÔNG bọc `ProfileAwareAgentSpawner.spawn()`** (đó là `agentOrch:spawn`, TASK-BE-002.2)
- [ ] `project.agentSpawn` forward `routeSpan.id` làm `traceId` vào `agentSpawner.spawn()` để `agentOrch:spawn` resume đúng id đó
- [ ] `assertAccess` reject → `routeSpan.fail()`, KHÔNG gọi `agentSpawner.spawn()`
- [ ] `ProfileAwareAgentSpawner.ts`/`AgentSpawnOptions` KHÔNG bị sửa trong task này (thân `spawn()` và field `traceId?: string` đều thuộc TASK-BE-002.2)
- [ ] Known Conflict với `TASK-BE-002.2` đã resolve theo mô hình "1 span canonical + resume" — không còn 2 span gốc khác tracer bao trọn cùng 1 thân hàm `spawn()`
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
