# Frontend-Side Full-Flow Tracing — Implementation Solutions

**Ngày:** 2026-08-02
**Phạm vi:** Renderer/Web UI-side (`src/renderer/src/`, `src/platform/adapters/web/`) implementation cho 10/19 CR trong [full-flow-tracing](../../../../../../docs/crs/v2/full-flow-tracing/README.md) được đánh giá là "agent-related" (chạm vào AI Agent execution/spawn/dispatch path)
**TDD Ref:** [specs/frontend/tdd/v5/](../../../../tdd/v5/00-index.md) (TDD-FE-01 → 17)
**Trạng thái:** 🚧 Proposed — chưa triển khai, chờ review
**Companion solutions:** [Agent](../../../../../agent/crs/v2/full-flow-tracing/solutions/00-index.md) | [Backend](../../../../../backend/crs/v2/full-flow-tracing/solutions/00-index.md)
**Dependency:** F40 core tracing infra đã có sẵn — `src/shared/trace/browser.ts` (`initBrowserTrace`), TracePanel UI. Các solution dưới đây chỉ THÊM instrumentation vào call site hiện có, không xây lại hạ tầng.

---

## Solution Files

| File | CR Ref | TDD Ref | Status |
|------|--------|---------|--------|
| [SOL-FE-TRACE-001](./SOL-FE-TRACE-001-worktree-management.md) | CR-TRACE-001 | TDD-FE-03, TDD-FE-05 | Proposed |
| [SOL-FE-TRACE-002](./SOL-FE-TRACE-002-agent-orchestration.md) | CR-TRACE-002 | TDD-FE-07 (hooks-and-ipc) | Proposed — target code chưa mount |
| [SOL-FE-TRACE-003](./SOL-FE-TRACE-003-terminal-management.md) | CR-TRACE-003 | TDD-FE-04 (terminal-subsystem) | Proposed |
| [SOL-FE-TRACE-005](./SOL-FE-TRACE-005-code-review.md) | CR-TRACE-005 | TDD-FE-16 (remote-git-ui) | Proposed — 2 cây component song song |
| [SOL-FE-TRACE-013](./SOL-FE-TRACE-013-agent-ws.md) | CR-TRACE-013 | TDD-FE-10 (fleet-management) | Proposed (minimal — không có trigger từ browser) |
| [SOL-FE-TRACE-014](./SOL-FE-TRACE-014-remote-integration.md) | CR-TRACE-014 | TDD-FE-16 | Proposed |
| [SOL-FE-TRACE-015](./SOL-FE-TRACE-015-profile.md) | CR-TRACE-015 | TDD-FE-11 (profile-ui) | Proposed |
| [SOL-FE-TRACE-016](./SOL-FE-TRACE-016-ai-providers.md) | CR-TRACE-016 | TDD-FE-13 (ai-provider-ui) | Proposed |
| [SOL-FE-TRACE-017](./SOL-FE-TRACE-017-workflow-orchestration.md) | CR-TRACE-017 | TDD-FE-14 (workflow-ui) | Proposed |
| [SOL-FE-TRACE-018](./SOL-FE-TRACE-018-task-graph.md) | CR-TRACE-018 | TDD-FE-15 (task-graph-ui) | Proposed |

---

## ⚠️ Blocker chung — chưa thể implement bất kỳ solution nào cho đến khi xong

**CR-TRACE-000 §3.1** yêu cầu `Tracer.start(fields?, resume?: { id: string })` — chưa tồn tại trong `src/shared/trace/index.ts` (verify 2026-08-02). Đây là precondition chung của cả 3 domain, implement 1 lần duy nhất trước khi áp dụng 30 solution.

**Thêm 1 blocker riêng của frontend, mức độ ưu tiên CAO hơn cả API trên:**

### `initBrowserTrace()` dispatch hiện là no-op — trace event KHÔNG đến được UI

Verify tại `src/renderer/src/web/main-web-bootstrap.tsx:294-295` (số dòng chính xác đã re-verify khi viết task decomposition — bản đầu ghi nhầm 294-296): lệnh gọi `addTraceEvent()` trong dispatch callback đang **bị comment out**. Nghĩa là dù có instrument đủ 10 CR này, TracePanel vẫn sẽ không hiển thị gì cho tới khi dòng này được uncomment/fix — đây là bug có sẵn, ngoài phạm vi 10 solution (không được tự ý sửa khi implement chúng), nhưng **PHẢI fix trước khi bất kỳ solution frontend nào có thể verify bằng mắt qua TracePanel**.

---

## Phát hiện xuyên suốt (Frontend domain) — quan trọng, đọc trước khi implement

### 1. Convention mới bắt buộc: prefix `ui:` cho mọi tracer khởi tạo từ browser

`TracePanel.tsx:42` có heuristic `isBackend`: bất kỳ flow name nào chứa `:` và KHÔNG bắt đầu bằng `ui:` sẽ bị gắn nhãn "backend". Vì CR-TRACE-000 đặt tên tracer theo `domain:operation` (`profile:resolve`, `workflow:execute`...) — đúng những tên đó khi dùng ở frontend sẽ bị TracePanel hiểu nhầm là backend event. **Quyết định: mọi tracer khởi tạo ở renderer phải dùng prefix `ui:`** (vd: `ui:profile.resolve`, `ui:workflow.execute`), đăng ký trong `src/shared/trace/tracers.ts` (isomorphic, dùng chung được cả 2 phía) cùng với các tracer backend/agent.

> ✅ **Resolved 2026-08-02:** đã đồng bộ lại cả 10/10 solution để dùng prefix `ui:` nhất quán (trước đó chỉ 015/016/017/018 có, còn 001/002/003/005/014 dùng tên chưa-prefix). Các tracer tên "trần" còn xuất hiện trong 002/005/014 (`agentOrch:spawn`, `codeReview:diff|annotate|sendFeedback|aiCommitMessage|createPr`, `remoteIntegration:ghExec/credentialDecrypt`) là **cố ý** — đó là tên tracer của companion backend/agent solution, được renderer forward `traceId` tới chứ không tự tạo, đã ghi chú rõ tại từng vị trí. `TASK-FE-001-register-ui-prefix-tracers.md` đã cập nhật để đăng ký đủ tracer `ui:*` từ cả 10 solution.

### 2. Method-name drift giữa frontend và backend — cần đối chiếu lại 2 bộ CR

- **CR-TRACE-003 (Terminal)**: backend CR đặt tên `terminal.resizeForClient`; RPC method thật mà frontend gọi là `terminal.updateViewport`. Solution frontend trace đúng theo tên thật, đã flag lệch tên để đối chiếu lại với SOL-BE-TRACE-003.
- **CR-TRACE-018 (Task Graph)**: `TaskDetail.tsx` gọi `tasks.runAgent` (số nhiều) trong khi `TaskPromptEditor.tsx` gọi `task.runAgent` (số ít) — cả 2 đã được instrument, lệch tên được flag như một vấn đề dọn dẹp riêng (không phải lỗi của tracing).

### 3. Code path song song / orphan code phát hiện được (ngoài phạm vi tracing, cần đội frontend biết)

- **CR-TRACE-002 (Agent Orchestration)**: `AgentPanel.tsx` + `use-agent-orchestration-events.ts` + `src/main/ipc/agent-orchestration.ts` (đã gắn tag `BUG-FE-ORCH-001`) là một implementation đầy đủ chức năng qua Electron IPC (`window.api.agentOrchestration.*`) nhưng **không được import/mount ở đâu trong app** — orphan code. Solution vẫn instrument nó (đúng yêu cầu CR) nhưng flag rõ tình trạng "chưa mount" trong acceptance criteria.
- **CR-TRACE-005 (Code Review)**: có 2 cây component song song — `components/workspace/git/*` (`GitPanel`, ĐÃ mount trong `WorkspaceLayout.tsx`, nhưng gọi một số RPC method **không tồn tại phía backend** như `git.getDiff`/`git.getStatus`/`git.aiCommitMessage`/`git.pr.list`/`git.stageFile`) và `components/code-review/*` (`CodeReviewPanel`, chưa mount, nhưng gọi đúng tên method thật ngoại trừ `annotation.create/.list`). Feature "gửi review notes cho agent" thực sự chạy được lại nằm ở một chain thứ 3 (`DiffNotesSendMenu` → `sendNotesToActiveAgentSession()`, dùng `terminal.wait`/`terminal.send`). Solution instrument dựa trên thực tế đã verify, không theo giả định của CR gốc.
- **CR-TRACE-014 (Remote Integration)**: `CredentialInputForm.tsx` (BL-INT-02, gọi đúng `credentials.set/revoke`) được build nhưng cũng chưa mount ở đâu — flag tương tự.
- **CR-TRACE-013 (Agent WS)**: xác nhận **không có bất kỳ trigger nào từ browser** cho luồng này (không có call site nào tới `/api/agent-token`; `AgentTokenPanel.tsx` build sẵn nhưng chưa nối vào `AddDevServerDialog.tsx`). Solution KHÔNG thêm tracer/span mới — chỉ đề xuất một badge đọc trace event có sẵn (qua SSE) hiển thị trên `DevServerCard.tsx`.

### 4. Transport chưa có trong CR-TRACE-000 §3.3 — cần bổ sung

Một phần luồng (đặc biệt Agent Orchestration qua `window.api.agentOrchestration.*`) đi qua **Electron contextBridge IPC**, không phải WebSocket RPC — CR-TRACE-000 §3.3 hiện chỉ liệt kê 6 transport, không có dòng nào cho Electron IPC thuần (khác với `ipc:devServerProxy` vốn là IPC proxy sang main process, không phải renderer↔main trực tiếp). Cả 3 solution liên quan (001, 002, 003) đều dùng cùng một cách xử lý tạm thời (gắn `traceId` vào IPC payload y hệt quy ước WS RPC) và đề xuất bổ sung dòng thứ 7 vào bảng CR-TRACE-000 §3.3 thay vì tự sáng tạo 3 cách khác nhau.

### 5. Ràng buộc bảo mật — SubtleCrypto credential flow (CR-TRACE-016)

API key được `SubtleCrypto.encrypt()` ngay tại client TRƯỚC khi rời browser — instrumentation phải bọc đúng quanh bước này mà không log plaintext key, thậm chí không log blob đã encrypt (chỉ `accountId`/`provider`/thời gian).

---

## Nguyên tắc chung khi review 10 solution này

1. **Additive-only** — chỉ thêm `Tracers.xxx.start()`/`.ok()`/`.fail()` quanh RPC call site có sẵn, không đổi UI/business logic.
2. **Prefix `ui:` bắt buộc** cho mọi tracer mới trong bộ này (mục 1 ở trên).
3. **Không log secret** — đặc biệt SOL-016 (AI Provider credential).
4. **Không tự sửa orphan-mount hoặc method-name drift đã phát hiện** — chỉ flag rõ trong solution, đó là quyết định của đội frontend, không phải phạm vi CR tracing.
5. Mọi file:line đã verify qua grep/Read trực tiếp vào code hiện tại (2026-08-02).
