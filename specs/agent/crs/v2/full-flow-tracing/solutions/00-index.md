# Agent-Side Full-Flow Tracing — Implementation Solutions

**Ngày:** 2026-08-02
**Phạm vi:** Agent-side (`src/relay/agent-*.ts`, `src/shared/agent-wire-protocol.ts`) implementation cho 10/19 CR trong [full-flow-tracing](../../../../../../docs/crs/v2/full-flow-tracing/README.md) được đánh giá là "agent-related" (chạm vào AI Agent execution/spawn/dispatch path)
**TDD Ref:** [specs/agent/tdd/v5/](../../../../tdd/v5/00-index.md) (TDD-AG-01 → 13)
**Trạng thái:** 🚧 Proposed — chưa triển khai, chờ review
**Companion solutions:** [Backend](../../../../../backend/crs/v2/full-flow-tracing/solutions/00-index.md) | [Frontend](../../../../../frontend/crs/v2/full-flow-tracing/solutions/00-index.md)

---

## Solution Files

| File | CR Ref | TDD Ref | Status |
|------|--------|---------|--------|
| [SOL-AG-TRACE-001](./SOL-AG-TRACE-001-worktree-management.md) | CR-TRACE-001 | TDD-AG-10 (git-handler-extension) | Proposed |
| [SOL-AG-TRACE-002](./SOL-AG-TRACE-002-agent-orchestration.md) | CR-TRACE-002 | TDD-AG-12 (agent-spawner), TDD-AG-06 | Proposed |
| [SOL-AG-TRACE-003](./SOL-AG-TRACE-003-terminal-management.md) | CR-TRACE-003 | TDD-AG-01, TDD-AG-12 | Proposed |
| [SOL-AG-TRACE-005](./SOL-AG-TRACE-005-code-review.md) | CR-TRACE-005 | TDD-AG-06, TDD-AG-09 | Proposed |
| [SOL-AG-TRACE-013](./SOL-AG-TRACE-013-agent-ws.md) | CR-TRACE-013 | TDD-AG-02/03/04 (wire/connection/handshake) | Proposed |
| [SOL-AG-TRACE-014](./SOL-AG-TRACE-014-remote-integration.md) | CR-TRACE-014 | TDD-AG-13 (external-api-connectors) | Proposed |
| [SOL-AG-TRACE-015](./SOL-AG-TRACE-015-profile.md) | CR-TRACE-015 | TDD-AG-12 | Proposed (minimal — xem ghi chú) |
| [SOL-AG-TRACE-016](./SOL-AG-TRACE-016-ai-providers.md) | CR-TRACE-016 | TDD-AG-09 (ai-credential-relay) | Proposed |
| [SOL-AG-TRACE-017](./SOL-AG-TRACE-017-workflow-orchestration.md) | CR-TRACE-017 | TDD-AG-12, TDD-AG-06 | Proposed |
| [SOL-AG-TRACE-018](./SOL-AG-TRACE-018-task-graph.md) | CR-TRACE-018 | TDD-AG-12 | Proposed |

---

## ⚠️ Blocker chung — chưa thể implement bất kỳ solution nào cho đến khi xong

**CR-TRACE-000 §3.1** yêu cầu `Tracer.start(fields?, resume?: { id: string })` — API này **chưa tồn tại** trong `src/shared/trace/index.ts` hiện tại (verify lại 2026-08-02: `start()` vẫn chỉ nhận `fields`). Đây là precondition của MỌI solution trong bộ này (cả 3 domain) vì không có `resume`, span id không thể nối tiếp qua RPC boundary — mỗi layer vẫn tạo id ngẫu nhiên độc lập như GAP-1 đã mô tả.

**Khuyến nghị thứ tự triển khai:** implement CR-TRACE-000 §3.1 trước tiên (core API, backward-compatible, không breaking; task cụ thể: [TASK-BE-000](../../../../../backend/crs/v2/full-flow-tracing/tasks/TASK-BE-000-tracer-resume-api.md)), rồi mới áp dụng 30 solution trong 3 thư mục `solutions/`.

> ✅ **Resolved 2026-08-02:** SOL-AG-TRACE-005 và SOL-AG-TRACE-018 từng độc lập đề xuất cùng một tracer trên `ai-complete-handler.ts` (`agent:aiComplete`) với field name khác nhau. Đã sửa cả 2 solution doc để dùng chung 1 field shape (`promptLength`/`contentLength`/`'provider-call'`/`providerNameFromModel`) — không còn drift. `TASK-AG-005.1` (Phase 2) là điểm tạo tracer duy nhất; task reconcile riêng (`TASK-AG-018.1`) đã bị xoá vì trùng lặp hoàn toàn, xem `../tasks/00-index.md` mục "Ghi chú điều phối".

---

## Phát hiện xuyên suốt (Agent domain) — quan trọng, đọc trước khi implement

### 1. Agent-side đã có 9 tracer riêng, dùng convention khác CR-TRACE-000 đề xuất

CR-TRACE-000's GAP-3 liệt kê 11 tracer tồn tại trước rollout — nhưng đó là góc nhìn từ **backend/Main process**. Khi các agent grep trực tiếp `src/relay/`, phát hiện agent side ĐÃ CÓ instrumentation riêng, không nằm trong danh sách đó:

| Tracer | File |
|--------|------|
| `agent:rpc` | `agent-rpc-dispatch.ts:21` |
| `agent:git` | `agent-git-handler.ts` |
| `agent:credential` | `agent-credential-store.ts:20` |
| `agent:fs` | `fs-agent-extensions.ts` |
| `agent:spawn` | `agent-spawner.ts:29` |
| `agent:ext-api` | `external-api-connector.ts` |
| `agent:tokenManager` | `agent-token-manager.ts` |
| `agent:connection` | `agent-connection-direct.ts` |
| `agent:session` | `agent-session.ts` |

Agent side dùng convention `agent:xxx` (namespace theo **file**, không theo **BL-flow domain** như `worktree:*`/`codeReview:*`). Cả 10 solution trong thư mục này **cố ý theo convention `agent:xxx` sẵn có** thay vì áp literal tên `domain:operation` mà CR-TRACE đề xuất — vì agent.js là process build riêng (esbuild bundle, deploy độc lập lên Dev Server), 2 namespace không đụng nhau và việc trộn 2 convention trong cùng 1 file sẽ gây nhầm lẫn hơn là thống nhất cưỡng ép. **CR-TRACE-000 nên được cập nhật để ghi nhận namespace `agent:xxx` này** như một ngoại lệ hợp lệ, không phải một gap.

### 2. Module path drift đã tìm thấy và sửa trong solution (không sửa CR gốc)

- **CR-TRACE-001** trỏ tới `src/relay/git-handler.ts` (theo TDD-AG-10) — thực tế `agent-rpc-dispatch.ts` import git/worktree handlers từ `./agent-git-handler`, một file khác. `git-handler.ts` (1498 dòng) là module của relay daemon (không phải agent), chỉ tái dùng 2 helper thuần. → SOL-AG-TRACE-001 nhắm đúng `agent-git-handler.ts`.
- **CR-TRACE-002** chỉ mô tả 1 code path (`agent.exec`, `child_process.spawn`, không PTY — cái `ProfileAwareAgentSpawner.spawn()` gọi). Thực tế có **2 path song song**: path còn lại là `agent.spawn` → `agent-spawner.ts::handleAgentSpawn` (node-pty, đã có field `resumeId` sẵn cho BL-AG-03). SOL-AG-TRACE-002 instrument cả hai.
- **CR-TRACE-005** giả định `relay.call('shell.exec', {cmd:'gh',...})` cho PR creation — dispatcher thực tế KHÔNG có method `shell.exec`. PR creation thật sự có 2 handler song song: `git.pr.create` (agent-git-handler.ts, chưa trace) và `github.pr.create` (external-api-connector.ts, đã trace).
- **CR-TRACE-014** bỏ sót một implementation preflight thứ 3: `handlePreflightCheck` trong `fs-agent-extensions.ts` (Agent-WS-JSON-RPC path), khác với `PreflightHandler.checkFullPreflight()` (SSH-relay-mode) mà CR đã loại trừ rõ ràng.

### 3. Functional gap phát hiện được (không phải tracing gap) — flag riêng, không tự ý fix

- `type='shell'` và `type='notification'` trong Workflow steps **không có agent-side RPC handler nào cả** — sẽ hit `MethodNotFound` nếu gọi thật. SOL-AG-TRACE-017 chỉ trace `type='agent'` (qua `agent.exec`), và thêm regression test khẳng định rõ trạng thái thiếu handler này thay vì giả vờ nó tồn tại.
- `ai-complete-handler.ts` (dùng bởi cả CR-TRACE-005 code-review và CR-TRACE-018 task-graph AI planning) **chưa có tracer nào** trước solution này — SOL-AG-TRACE-018 thêm `agent:aiComplete` mới.
- `TaskDAGValidator`/`TaskGrantService` (BL-TG-01/03) xác nhận **không có relay call nào** — không có counterpart phía agent, SOL-AG-TRACE-018 bỏ qua đúng theo phạm vi.

### 4. Chưa có cross-process traceId propagation nào tồn tại

Verify bằng grep: `params._trace` không xuất hiện ở đâu trong `src/relay/` — khớp với CR-TRACE-000 GAP-2 (chưa có wire-envelope convention nào được implement).

---

## Nguyên tắc chung khi review 10 solution này

1. **Additive-only** — chỉ thêm tracer calls, không đổi business logic.
2. **Không log secret** — API key, token, credential đã decrypt KHÔNG BAO GIỜ vào `TraceFields` (áp dụng nghiêm ở SOL-005, 014, 016).
3. **Không trace mọi keystroke/data frame** — theo CR-TRACE-000 §5, ví dụ `pty.write`/`pty.onData` không trace per-frame, chỉ trace ở boundary/lifecycle event.
4. **Tôn trọng convention `agent:xxx` sẵn có** — không đổi tên tracer cũ, chỉ thêm tracer mới theo cùng pattern.
5. Mọi file:line trong 10 solution đã được verify qua grep/Read trực tiếp vào code hiện tại (2026-08-02), không copy mù từ CR-TRACE gốc — nơi nào lệch, solution đã ghi rõ và sửa.
