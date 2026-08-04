# TASK-BE-002.1: Đăng ký tracer `agentOrch:*`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-002](../solutions/SOL-BE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-002)
**Status:** ✅ Done (2026-08-04) — All 5 entries (`agentOrchSpawn`/`Stop`/`Resume`/`Switch`/`StatusPoll`, flow names `agentOrch:spawn|stop|resume|switch|statusPoll`) already existed in `tracers.ts`, added earlier by the concurrent sibling agent-domain effort (`src/relay/agent-spawner.ts`'s CR-TRACE-002 work) with names/flows matching this task's spec exactly — no name collision with `agent:rpc`. No production change needed; verified via Read of `tracers.ts` and `pnpm tsc --noEmit`.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` là object đã tồn tại (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

Task này chỉ thêm 5 entry mới (`agentOrchSpawn`/`Stop`/`Resume`/`Switch`/`StatusPoll`), không đổi entry cũ. Fan-in lớn của `Tracers` là bình thường với registry object — chỉ dừng lại nếu risk HIGH/CRITICAL đến từ nguyên nhân khác (vd. trùng tên với `agent:rpc` đã tồn tại), xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 5 tracer `agentOrch:spawn|stop|resume|switch|statusPoll` trong `tracers.ts`. Chỉ `agentOrchSpawn` có call site thật (`ProfileAwareAgentSpawner.spawn()`, xem TASK-BE-002.2); 4 tracer còn lại chỉ đăng ký tên vì chưa có RPC method/call site thật ở Orca Server tương ứng (`agent.sendInput`/`agent.kill`/switch-account/polling loop).

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các entry từ SOL-BE-TRACE-001: `worktreeCreate`, `worktreeFanOut`, `worktreeDelete`, `worktreeCompare`, `worktreeMerge`, cùng các entry gốc `browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow`):

```typescript
export const Tracers = {
  // ...existing entries (worktree:* từ SOL-BE-TRACE-001, devServer:*, agentWs:lifecycle, ...) unchanged...

  // ─── CR-TRACE-002: Agent Orchestration (BL-AG-01→05) ───────────────────────
  /** ProfileAwareAgentSpawner.spawn() — BL-AG-01 */
  agentOrchSpawn:      createTracer('agentOrch:spawn'),
  /** BL-AG-02 — reserved, chưa có call site thật ở Orca Server */
  agentOrchStop:       createTracer('agentOrch:stop'),
  /** BL-AG-03 — reserved */
  agentOrchResume:     createTracer('agentOrch:resume'),
  /** BL-AG-04 — reserved */
  agentOrchSwitch:     createTracer('agentOrch:switch'),
  /** BL-AG-05 — reserved, KHÔNG dùng cho stream agent.output per-frame */
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'),
} as const
```

**Lưu ý đặt tên:** dùng prefix `agentOrch:*` (không phải `agent:*`) để tránh va chạm với `agent:rpc` (`agent-rpc-dispatch.ts:21`) — tracer hạ tầng wrap generic mọi JSON-RPC method trên Dev Server (CR-TRACE-000 §4). Không đổi tên tracer đã tồn tại nào.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` tồn tại trong `tracers.ts` với đúng flow name `agentOrch:spawn|stop|resume|switch|statusPoll`
- [ ] Tên tracer không trùng với `agent:rpc` (tracer hạ tầng đã tồn tại ở `agent-rpc-dispatch.ts`)
- [ ] Không có call site giả định nào được viết cho `agentOrch:stop/resume/switch/statusPoll` (chưa có RPC method/call site thật tương ứng ở Orca Server)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
