# TASK-BE-018.4: Xác nhận `AgentSpawnOptions.traceId` — resume `agentOrch:spawn` từ Task Graph (không patch thêm)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-018](../solutions/SOL-BE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + **TASK-BE-002.2** (sở hữu `AgentSpawnOptions.traceId` + resume logic — mới đổi, xem "✅ Đã giải quyết" bên dưới), TASK-BE-018.1
**Status:** ✅ Done (2026-08-04) — Verify-only per spec, no production edit to `ProfileAwareAgentSpawner.ts`. SIGNIFICANT DRIFT found and documented: mid-session, `codegraph_explore`/direct file checks showed `ProfileAwareAgentSpawner.ts` had temporarily lost `AgentSpawnOptions.traceId` and the `Tracers.agentOrchSpawn` resume wrapping entirely (despite `00-index.md` marking TASK-BE-002.2 "✅ Done") — this was due to unrelated concurrent multi-agent activity on the shared working tree, observed independently via both `codegraph_explore` and direct `bash`/`grep` reads of the file at the same point in time. Per the explicit "do not touch ProfileAwareAgentSpawner.ts" constraint, did not restore it myself. `TaskAgentExecutor.ts` (018.5) was written with a local, non-invasive type-widening (`AgentSpawnOptions & { traceId?: string }`) so it forwards `traceId` and typechecks regardless of whether the field exists upstream yet — forward-compatible, zero further change needed once 002.2's work lands. By the end of this session TASK-BE-002.2's `traceId` field was observed landing back into `ProfileAwareAgentSpawner.ts` (presumably a sibling agent recovering its own work), confirming the field/resume design is correct as documented once restored. No test changes made to `ProfileAwareAgentSpawner.test.ts` for this task (resume behavior verified instead from the caller side in `TaskAgentExecutor.test.ts`, see TASK-BE-018.6 for rationale). `pnpm tsc --noEmit` clean.

---

## ✅ Known Conflicts với TASK-BE-002.2 — đã giải quyết 2026-08-02 (xem `tasks/00-index.md`)

Bản gốc của task này giả định `spawn()` đang dùng `Tracers.profileAgentSpawnFlow` (do `TASK-BE-015.4` thiết lập) và tự thêm `AgentSpawnOptions.traceId` + patch `spawn()` để resume vào tracer đó — xung đột trực tiếp với `TASK-BE-002.2`, vốn bọc CÙNG hàm bằng `Tracers.agentOrchSpawn` và **đã tự thêm `AgentSpawnOptions.traceId` + resume logic ngay từ đầu**.

Quyết định resolve: `agentOrch:spawn` (`TASK-BE-002.2`) là span CANONICAL duy nhất bọc `spawn()`, và `AgentSpawnOptions.traceId` do `TASK-BE-002.2` sở hữu. Task này **không còn patch `ProfileAwareAgentSpawner.ts`** — nó chỉ xác nhận (qua test) rằng field/logic đã có sẵn từ `TASK-BE-002.2` đáp ứng đúng nhu cầu resume của Task Graph (`taskGraph:execute` → `agentOrch:spawn`, xem `TASK-BE-018.5`).

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này KHÔNG patch code sản xuất (chỉ viết test verify hành vi đã có từ `TASK-BE-002.2`) — nên KHÔNG cần `gitnexus_impact`. Trước khi viết test, khám phá lại symbol đã implement để xác nhận đúng field/logic hiện có:

```bash
codegraph explore "ProfileAwareAgentSpawner.spawn"
```

Xác nhận `AgentSpawnOptions.traceId` và cơ chế resume (`options.traceId ? { id: options.traceId } : undefined`) đã tồn tại đúng như `TASK-BE-002.2` mô tả trước khi viết test.

## Mô tả

Đây là điểm khác biệt then chốt giữa Task Graph (CR-TRACE-018) và Workflow (CR-TRACE-017): thay vì dùng `parentTraceId` (field nghiệp vụ nhóm N span độc lập — xem `TASK-BE-017.2`), Task Graph cần `taskGraph:execute` (TASK-BE-018.5) **RESUME thật sự** vào `agentOrch:spawn` — vì đây là 1 lời gọi hàm nội bộ nối tiếp thật (`TaskAgentExecutor.executeTask()` gọi trực tiếp `agentSpawner.spawn()`), không phải N span song song. `AgentSpawnOptions.traceId` + resume logic đã tồn tại (`TASK-BE-002.2`) — task này chỉ verify bằng test, không patch thêm code sản xuất.

**Khi nào dùng cơ chế nào — tham chiếu bắt buộc `TASK-BE-017.1`:** Task Graph có thể tuỳ chọn tái dùng pattern `parentTraceId` (field nghiệp vụ, persisted qua migration `0013_workflow_trace_correlation.ts`, xem `TASK-BE-017.1`) để nhóm các sub-flow độc lập trên cùng 1 task (vd. `addEdge` rồi `execute` liên tiếp) — nhưng đây là **optional** cho Task Graph, không bắt buộc như Workflow (Task Graph không có cấu trúc wave/DAG-dispatch multi-span-per-execution). Task này **không** tạo migration mới cho Task Graph — mọi tham chiếu tới thiết kế `parentTraceId`/cột persist PHẢI trỏ về `TASK-BE-017.1`, không tạo bản sao.

## File: `src/main/project/ProfileAwareAgentSpawner.ts` [KHÔNG SỬA — ngoài phạm vi task này]

`AgentSpawnOptions.traceId` và logic resume (`options.traceId ? { id: options.traceId } : undefined` bọc quanh `Tracers.agentOrchSpawn.start(...)`) đã được `TASK-BE-002.2` implement đầy đủ. Task này không patch lại file này.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Xác nhận `AgentSpawnOptions` (`ProfileAwareAgentSpawner.ts`, đã thêm bởi TASK-BE-002.2) có optional field `traceId?: string`
- [ ] Xác nhận: khi `options.traceId` có mặt → `agentOrch:spawn` **resume** đúng id đó thay vì tạo span mới
- [ ] Xác nhận: khi `options.traceId` KHÔNG có mặt → hành vi giữ nguyên (span độc lập, `id` ngẫu nhiên)
- [ ] `spawn({ traceId })` → `agentOrch:spawn` `span.id === traceId` (test resume, có thể tái dùng test đã viết ở TASK-BE-002.4)
- [ ] `spawn()` không có `traceId` → `span.id` là random mới (đảm bảo default path của TASK-BE-002.2 không bị phá vỡ)
- [ ] Task này KHÔNG tạo thêm patch nào lên `ProfileAwareAgentSpawner.ts` — chỉ verify hành vi đã có
- [ ] Known Conflict với `TASK-BE-002.2` đã resolve theo mô hình "1 span canonical + resume" — không còn 2 lần thêm field `traceId` trùng lặp vào `AgentSpawnOptions`
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
