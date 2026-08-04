# Backend-Side Full-Flow Tracing — Implementation Solutions

**Ngày:** 2026-08-02
**Phạm vi:** Backend/Gateway-side (`src/main/`, `src/server/`, `src/shared/`) implementation cho 10/19 CR trong [full-flow-tracing](../../../../../../docs/crs/v2/full-flow-tracing/README.md) được đánh giá là "agent-related" (chạm vào AI Agent execution/spawn/dispatch path)
**TDD Ref:** [specs/backend/tdd/v5/](../../../../tdd/v5/00-index.md) (TDD-01 → 20)
**Trạng thái:** 🚧 Proposed — chưa triển khai, chờ review
**Companion solutions:** [Agent](../../../../../agent/crs/v2/full-flow-tracing/solutions/00-index.md) | [Frontend](../../../../../frontend/crs/v2/full-flow-tracing/solutions/00-index.md)

---

## Solution Files

| File | CR Ref | TDD Ref | Status |
|------|--------|---------|--------|
| [SOL-BE-TRACE-001](./SOL-BE-TRACE-001-worktree-management.md) | CR-TRACE-001 | TDD-07 (runtime-service), TDD-05 (ssh-relay) | Proposed |
| [SOL-BE-TRACE-002](./SOL-BE-TRACE-002-agent-orchestration.md) | CR-TRACE-002 | TDD-08 (agent-orchestration) | Proposed |
| [SOL-BE-TRACE-003](./SOL-BE-TRACE-003-terminal-management.md) | CR-TRACE-003 | TDD-03 (daemon-layer), TDD-08 | Proposed |
| [SOL-BE-TRACE-005](./SOL-BE-TRACE-005-code-review.md) | CR-TRACE-005 | TDD-08, TDD-20 (remote-git-ui) | Proposed |
| [SOL-BE-TRACE-013](./SOL-BE-TRACE-013-agent-ws.md) | CR-TRACE-013 | TDD-04 (rpc-server, AgentWebSocketServer) | Proposed |
| [SOL-BE-TRACE-014](./SOL-BE-TRACE-014-remote-integration.md) | CR-TRACE-014 | TDD-20, TDD-05 | Proposed |
| [SOL-BE-TRACE-015](./SOL-BE-TRACE-015-profile.md) | CR-TRACE-015 | TDD-14 (profile-hierarchy) | Proposed |
| [SOL-BE-TRACE-016](./SOL-BE-TRACE-016-ai-providers.md) | CR-TRACE-016 | TDD-16 (ai-provider-management) | Proposed |
| [SOL-BE-TRACE-017](./SOL-BE-TRACE-017-workflow-orchestration.md) | CR-TRACE-017 | TDD-17 (workflow-orchestration) | Proposed |
| [SOL-BE-TRACE-018](./SOL-BE-TRACE-018-task-graph.md) | CR-TRACE-018 | TDD-18 (task-graph) | Proposed |

---

## ⚠️ Blocker chung — chưa thể implement bất kỳ solution nào cho đến khi xong

**CR-TRACE-000 §3.1** yêu cầu `Tracer.start(fields?, resume?: { id: string })` — API này **chưa tồn tại** trong `src/shared/trace/index.ts` (verify 2026-08-02). Tất cả 30 solution (3 domain) coi đây là precondition đã biết, không tự implement lại 30 lần — implement 1 lần duy nhất ở `src/shared/trace/index.ts` trước (task cụ thể: [TASK-BE-000](../tasks/TASK-BE-000-tracer-resume-api.md)).

> ✅ **Cập nhật 2026-08-02 — Known Conflict đã resolve (xem [`../tasks/00-index.md`](../tasks/00-index.md) mục "Known Conflicts — Resolved"):** SOL-BE-TRACE-002 và SOL-BE-TRACE-015/018 ban đầu instrument cùng 1 call site (`ProfileAwareAgentSpawner.spawn()`) theo 2 cách khác nhau — SOL-002 gắn `Tracers.agentOrchSpawn`, SOL-015/018 gắn `Tracers.profileAgentSpawnFlow` — và bất đồng về việc `TaskAgentExecutor.executeTask()` có nên sở hữu span riêng hay không. Quyết định resolve (theo CR-TRACE-000 §3.1, cơ chế `resume`): `Tracers.agentOrchSpawn` (`agentOrch:spawn`) là span **canonical duy nhất** bọc `spawn()` — khớp đúng bảng quy ước CR-TRACE-000 §4. `profile:agentSpawnRoute` (SOL-015) chuyển sang bọc riêng phần prep (`assertAccess`) tại `project-rpc-handler.ts` trước khi gọi `spawn()`, forward span id để `agentOrch:spawn` resume. `TaskAgentExecutor.executeTask()` (SOL-018) ĐƯỢC sở hữu span riêng `taskGraph:execute` bao trọn grant-check + AI-planning + lời gọi `spawn()`, cũng forward span id để `agentOrch:spawn` resume — không qua `profile:agentSpawnRoute`. Cả 3 solution doc (`SOL-BE-TRACE-002`, `SOL-BE-TRACE-015`, `SOL-BE-TRACE-018`) và 5 task bị ảnh hưởng (`TASK-BE-002.2`, `TASK-BE-002.3`, `TASK-BE-015.4`, `TASK-BE-018.4`, `TASK-BE-018.5`) đã được cập nhật theo quyết định này.

---

## Phát hiện xuyên suốt (Backend domain) — quan trọng, đọc trước khi implement

### 1. Xung đột convention `traceId` phẳng vs `_trace.id` lồng nhau — đã giải quyết thực dụng

CR-TRACE-001 (relay.call — dùng field phẳng `traceId`) và CR-TRACE-002/013 (Agent WS JSON-RPC — dùng `params._trace.id` lồng nhau per CR-TRACE-000 §3.3) tạo ra căng thẳng khi `DevServerRelayBridge` là điểm dùng chung cho cả hai. **SOL-BE-TRACE-002 giải quyết bằng cách gửi cả hai field** (`traceId` phẳng cho relay.call thường, `_trace.id` khi target là Agent WS) thay vì ép một convention duy nhất — ghi rõ lý do trong solution đó.

### 2. Lỗi trong bản vẽ code của CR gốc đã được sửa khi grounding vào source thật

- **CR-TRACE-014** giả định `ctx.credentialStore` tồn tại trên `RpcContext` — thực tế code dùng singleton `getWebCredentialStore()`. SOL-BE-TRACE-014 sửa lại đúng.
- **CR-TRACE-018** giả định `TaskDAGValidator.wouldCreateCycle()` dùng BFS — code thật là DFS (stack-based). SOL-BE-TRACE-018 sửa lại.
- **CR-TRACE-016**: `ProviderHealthChecker.runCheck()` thật khác đáng kể so với code minh hoạ trong CR (3-way status classification + `onStatusChanged` callback, không có tham số `relayPool`) — SOL-BE-TRACE-016 trace đúng method thật.
- **CR-TRACE-013**: phát hiện + sửa 1 bug thật trong lúc thiết kế tracing — handshake fail hiện tại tạo span với id ngẫu nhiên bị "mồ côi" (orphaned) vì span chỉ mở SAU KHI handshake resolve; SOL-BE-TRACE-013 chuyển điểm mở span lên lúc socket-upgrade.

### 3. Phát hiện thêm ngoài phạm vi CR gốc (bổ sung, không thay thế)

- **CR-TRACE-002**: tìm thêm 2 caller thật của `ProfileAwareAgentSpawner.spawn()` mà CR không đề cập — `project.agentSpawn` RPC (`project-rpc-handler.ts`) và `TaskAgentExecutor.executeTask()` (`task-rpc-handler.ts`) — xác nhận `spawn()` đúng là điểm instrument duy nhất cần thiết.
- **CR-TRACE-003**: xác nhận `LocalPtyProvider.spawn()` và `SshPtyProvider.spawn()` cùng tên method (CR đánh dấu "chưa xác định" — nay đã xác nhận). Scrollback save chạy qua hàm batch `migrateWorkspaceSessionTerminalScrollbackSnapshots()` (Electron IPC `src/main/ipc/session.ts`), KHÔNG phải hook per-terminal-destroy như CR giả định — Web Server mode equivalent chưa xác nhận được, ghi rõ trong solution.
- **CR-TRACE-005**: xác nhận `annotation.create`/`review.sendFeedback` (BL-CR-02/03) thật sự không tồn tại trong code — tracer đã khai báo trong `tracers.ts` nhưng KHÔNG wire vào đâu cả, chờ tính năng gốc được implement trước.

### 4. Thiết kế mới: `parentTraceId` cho DAG/multi-step correlation — quan trọng nhất trong bộ backend

CR-TRACE-017 (Workflow) và CR-TRACE-018 (Task Graph) đề xuất field `parentTraceId` (khác với cơ chế `resume`/`traceId` nối tiếp span của CR-TRACE-000 §3.1) để nhóm nhiều step/wave/span con dưới một "execution run" cha trên TracePanel. SOL-BE-TRACE-017 thiết kế cụ thể:
- `parentTraceId` là **business field**, lưu persistent (migration mới `0013_workflow_trace_correlation.ts`, cột `root_trace_id`) để correlation sống sót qua restart server (`resumeRunningExecutions()`).
- SOL-BE-TRACE-018 tái dùng đúng pattern này cho Task Graph, đồng thời dùng `AgentSpawnOptions.traceId` (✅ resolved 2026-08-02 — field này do SOL-BE-TRACE-002 sở hữu, không phải SOL-BE-TRACE-015) để `taskGraph:execute` có thể **resume** (không chỉ correlate) thẳng vào `agentOrch:spawn` (span canonical duy nhất bọc `spawn()`, KHÔNG qua `profile:agentSpawnRoute`) — khác mô hình sibling-span của workflow, có giải thích rõ khi nào dùng cái nào.
- **Số migration tiếp theo còn trống: `0013`** (0011/0012 đã dùng cho terminal-sessions/port-forwards-push).

---

## Nguyên tắc chung khi review 10 solution này

1. **Additive-only** — chỉ thêm tracer calls + 1 migration mới (`0013`), không đổi business logic.
2. **Không log secret** — token/credential/API key đã decrypt KHÔNG BAO GIỜ vào `TraceFields` (SOL-005, 014, 016).
3. **Phạm vi rõ ràng** — mỗi solution nêu rõ ranh giới "dừng lại ở `relay.call()`", phần sau đó thuộc solution phía agent (companion).
4. Mọi file:line đã verify qua grep/Read trực tiếp vào code hiện tại (2026-08-02) — nơi CR gốc sai, solution đã sửa và ghi chú rõ.
