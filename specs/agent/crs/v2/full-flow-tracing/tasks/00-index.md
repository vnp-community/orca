# Agent-Side Full-Flow Tracing — AI Execution Task Index

**Source dir:** `src/relay/`
**Solutions Ref:** [../solutions/](../solutions/)
**Build:** `pnpm run build:agent`
**Test:** `pnpm test -- --reporter verbose src/relay/__tests__/`
**TypeCheck:** `pnpm run typecheck:node`

---

## Bối cảnh

10 solution trong [`../solutions/`](../solutions/) (đã review, xem [`../solutions/00-index.md`](../solutions/00-index.md) để biết các phát hiện xuyên suốt — namespace `agent:xxx`, module path drift, functional gap không tự ý fix) được chia nhỏ thành **25 task nguyên tử** dưới đây (TASK-AG-018.1 đã xoá 2026-08-02 vì trùng lặp hoàn toàn với TASK-AG-005.1 sau khi reconcile field naming `agent:aiComplete`). Mỗi task sửa đúng 1 file (hoặc 1 nhóm rất nhỏ file liên quan chặt) và có Definition of Done kiểm tra được.

Tất cả task bắt đầu ở trạng thái **⬜ Pending** — chưa có gì được triển khai. Đây là tài liệu dự kiến (prospective), khác với `specs/agent/crs/v1/tdd-v5/tasks/` (retrospective, đã Done).

---

## ⚠️ Phase 0 — Blocker chung, KHÔNG tạo task này ở đây

Toàn bộ 10 solution (cả 3 domain agent/backend/frontend) phụ thuộc vào `Tracer.start(fields?, resume?: { id: string })` — API hiện **chưa tồn tại** trong `src/shared/trace/index.ts` (hiện tại `start(fields: TraceFields = {})` chỉ nhận 1 tham số). Đây là prerequisite dùng chung, implement **một lần duy nhất**, không phải việc của agent domain.

| Phase | Domain | Mô tả | Ref |
|-------|--------|-------|-----|
| Phase 0 | Shared prerequisite | `Tracer.start(fields?, resume?)` trong `src/shared/trace/index.ts` — implemented ONCE, xem `specs/backend/crs/v2/full-flow-tracing/tasks/TASK-BE-000-tracer-resume-api.md` | Blocks all Phase 1+ tasks below |

**KHÔNG** tự viết task này trong thư mục `tasks/` của agent domain — chỉ tham chiếu tới file trên như một external blocking dependency. Một số task bên dưới (013, 015, 017 base) không literally gọi `.start(fields, resume)` trong code mẫu vì solution gốc chủ động "forward-compatible" (dùng field nghiệp vụ thuần, ví dụ `parentSpanId`/`parentTraceId`) — nhưng toàn bộ 10 solution vẫn liệt kê Phase 0 là điều kiện tiên quyết chính thức (xem `../solutions/00-index.md` mục "Blocker chung"), nên mọi task ở đây đều giữ Phase 0 trong Precondition để nhất quán.

---

## Execution Phases

| Phase | CRs | Tasks | Prerequisite |
|-------|-----|-------|---------------|
| Phase 0 | (shared, ngoài thư mục này) | — | — |
| Phase 1 — core daily flows | CR-TRACE-001 (worktree), CR-TRACE-002 (agent-orchestration), CR-TRACE-003 (terminal) | TASK-AG-001.1 → 001.2, TASK-AG-002.1 → 002.4, TASK-AG-003.1 → 003.2 | Phase 0 |
| Phase 2 — integration-heavy | CR-TRACE-005 (code-review), CR-TRACE-013 (agent-ws), CR-TRACE-014 (remote-integration) | TASK-AG-005.1 → 005.3, TASK-AG-013.1 → 013.2, TASK-AG-014.1 → 014.3 | Phase 0, Phase 1 (CR-002 cho spawnerTracer reuse trong CR-005) |
| Phase 3 — platform/DAG flows | CR-TRACE-015 (profile), CR-TRACE-016 (ai-providers), CR-TRACE-017 (workflow-orchestration), CR-TRACE-018 (task-graph) | TASK-AG-015.1 → 015.2, TASK-AG-016.1 → 016.3, TASK-AG-017.1 → 017.2, TASK-AG-018.2 → 018.3 | Phase 0, Phase 1 (agent-spawn instrumentation từ CR-002), Phase 2 (CR-005's `agent:aiComplete`, tạo tại TASK-AG-005.1 — dùng chung, không tạo lại) |

---

## Task List

| Task | Phase | SOL Ref | File(s) | Status |
|------|-------|---------|---------|--------|
| [TASK-AG-001.1](./TASK-AG-001.1-agent-git-handler-worktree-tracing.md) | 1 | SOL-AG-TRACE-001 | `src/shared/trace/tracers.ts` MODIFY, `src/relay/agent-git-handler.ts` MODIFY | ⬜ Pending |
| [TASK-AG-001.2](./TASK-AG-001.2-worktree-tracer-tests.md) | 1 | SOL-AG-TRACE-001 | `src/relay/__tests__/agent-git-handler.test.ts` MODIFY | ⬜ Pending |
| [TASK-AG-002.1](./TASK-AG-002.1-agentorch-tracers-registration.md) | 1 | SOL-AG-TRACE-002 | `src/shared/trace/tracers.ts` MODIFY | ⬜ Pending |
| [TASK-AG-002.2](./TASK-AG-002.2-agent-rpc-dispatch-resume-and-exec-span.md) | 1 | SOL-AG-TRACE-002 | `src/relay/agent-rpc-dispatch.ts` MODIFY | ⬜ Pending |
| [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md) | 1 | SOL-AG-TRACE-002 | `src/relay/agent-spawner.ts` MODIFY | ⬜ Pending |
| [TASK-AG-002.4](./TASK-AG-002.4-agent-orchestration-tests.md) | 1 | SOL-AG-TRACE-002 | `src/relay/__tests__/agent-rpc-dispatch.test.ts`, `agent-spawner.test.ts` MODIFY | ⬜ Pending |
| [TASK-AG-003.1](./TASK-AG-003.1-pty-agent-bridge-terminal-tracing.md) | 1 | SOL-AG-TRACE-003 | `src/shared/trace/tracers.ts` MODIFY (idempotent), `src/relay/pty-agent-bridge.ts` MODIFY | ⬜ Pending |
| [TASK-AG-003.2](./TASK-AG-003.2-pty-agent-bridge-tests.md) | 1 | SOL-AG-TRACE-003 | `src/relay/__tests__/pty-agent-bridge.test.ts` NEW | ⬜ Pending |
| [TASK-AG-005.1](./TASK-AG-005.1-ai-complete-handler-tracer.md) | 2 | SOL-AG-TRACE-005 | `src/relay/ai-complete-handler.ts` MODIFY | ⬜ Pending |
| [TASK-AG-005.2](./TASK-AG-005.2-git-pr-create-and-send-input-spans.md) | 2 | SOL-AG-TRACE-005 | `src/relay/agent-git-handler.ts` MODIFY, `src/relay/agent-spawner.ts` MODIFY | ⬜ Pending |
| [TASK-AG-005.3](./TASK-AG-005.3-code-review-tracing-tests.md) | 2 | SOL-AG-TRACE-005 | `src/relay/__tests__/ai-complete-handler.test.ts` NEW, `agent-git-handler.test.ts`, `agent-spawner.test.ts` MODIFY | ⬜ Pending |
| [TASK-AG-013.1](./TASK-AG-013.1-agent-connection-relay-tracer.md) | 2 | SOL-AG-TRACE-013 | `src/relay/agent-connection-relay.ts` MODIFY | ✅ Done |
| [TASK-AG-013.2](./TASK-AG-013.2-agent-connection-relay-tests.md) | 2 | SOL-AG-TRACE-013 | `src/relay/__tests__/agent-connection-relay.test.ts` MODIFY | ✅ Done |
| [TASK-AG-014.1](./TASK-AG-014.1-external-api-connector-auth-status-spans.md) | 2 | SOL-AG-TRACE-014 | `src/relay/external-api-connector.ts` MODIFY | ✅ Done |
| [TASK-AG-014.2](./TASK-AG-014.2-fs-agent-extensions-preflight-tracer.md) | 2 | SOL-AG-TRACE-014 | `src/relay/fs-agent-extensions.ts` MODIFY | ✅ Done |
| [TASK-AG-014.3](./TASK-AG-014.3-remote-integration-tracing-tests.md) | 2 | SOL-AG-TRACE-014 | `src/relay/__tests__/external-api-connector.test.ts` MODIFY, `fs-agent-extensions.test.ts` NEW/MODIFY | ✅ Done |
| [TASK-AG-015.1](./TASK-AG-015.1-agent-exec-trace-field-bucket.md) | 3 | SOL-AG-TRACE-015 | `src/relay/agent-rpc-dispatch.ts` MODIFY | ✅ Done |
| [TASK-AG-015.2](./TASK-AG-015.2-profile-agent-exec-tracing-tests.md) | 3 | SOL-AG-TRACE-015 | `src/relay/__tests__/agent-rpc-dispatch.test.ts` MODIFY | ✅ Done |
| [TASK-AG-016.1](./TASK-AG-016.1-agent-credential-store-bloblength-and-correlation.md) | 3 | SOL-AG-TRACE-016 | `src/relay/agent-credential-store.ts` MODIFY | ✅ Done |
| [TASK-AG-016.2](./TASK-AG-016.2-agent-spawner-parent-span-threading.md) | 3 | SOL-AG-TRACE-016 | `src/relay/agent-spawner.ts` MODIFY | ✅ Done |
| [TASK-AG-016.3](./TASK-AG-016.3-ai-providers-tracing-tests.md) | 3 | SOL-AG-TRACE-016 | `src/relay/__tests__/agent-credential-store.test.ts`, `agent-spawner.test.ts` MODIFY | ✅ Done |
| [TASK-AG-017.1](./TASK-AG-017.1-agent-exec-stepid-parenttraceid-bucket.md) | 3 | SOL-AG-TRACE-017 | `src/relay/agent-rpc-dispatch.ts` MODIFY | ✅ Done |
| [TASK-AG-017.2](./TASK-AG-017.2-workflow-orchestration-tracing-tests.md) | 3 | SOL-AG-TRACE-017 | `src/relay/__tests__/agent-rpc-dispatch.test.ts` MODIFY | ✅ Done |
| [TASK-AG-018.2](./TASK-AG-018.2-agent-exec-taskid-and-ai-complete-bucket.md) | 3 | SOL-AG-TRACE-018 | `src/relay/agent-rpc-dispatch.ts` MODIFY (`agent:aiComplete` chính nó đã tạo ở TASK-AG-005.1, dùng chung) | ✅ Done |
| [TASK-AG-018.3](./TASK-AG-018.3-task-graph-tracing-tests.md) | 3 | SOL-AG-TRACE-018 | `src/relay/__tests__/agent-rpc-dispatch.test.ts` MODIFY (phạm vi thu hẹp 2026-08-02 — không còn động vào `ai-complete-handler.test.ts`, đã hoàn chỉnh từ TASK-AG-005.3) | ✅ Done |

**Không có task nào cho:** `src/relay/agent-connection-direct.ts`, `src/relay/agent-session.ts` (SOL-AG-TRACE-013 §3.2 xác nhận coverage đã đủ, DOCUMENT ONLY, không sửa code) — và không có agent-side task cho BL-WT-02/04/05 (CR-001), BL-AG-04 (CR-002), `shell.exec`/`notification.send` (CR-017), BL-TG-01/03 (CR-018) vì các sub-flow đó không có call site thật phía agent (xem gap analysis trong từng solution).

---

## Ghi chú điều phối liên-task quan trọng

1. **`tracers.ts` idempotent giữa các CR:** TASK-AG-001.1 và TASK-AG-003.1 đều sửa `src/shared/trace/tracers.ts` nhưng thêm entry khác tên (`worktreeCreate/Delete` vs `terminalCreate/Resize/Destroy`) — không xung đột, chỉ cần merge cộng dồn vào cùng object `Tracers`. Thực thi theo thứ tự Task List ở trên là an toàn.
2. **`agent:aiComplete` — ✅ đã resolve (2026-08-02):** SOL-AG-TRACE-005 và SOL-AG-TRACE-018 từng độc lập đề xuất field shape khác nhau cho tracer `agent:aiComplete` trên cùng `ai-complete-handler.ts`. Đã đồng bộ lại 2 solution doc dùng chung 1 field shape (`promptLength`/`contentLength`/`'provider-call'`/`providerNameFromModel`). TASK-AG-005.1 (Phase 2) là điểm TẠO DUY NHẤT tracer này — task reconcile riêng (`TASK-AG-018.1`) đã bị xoá vì hoàn toàn trùng lặp; TASK-AG-018.2 (Phase 3) chỉ bổ sung bucket cho `agent:rpc` ở file khác (`agent-rpc-dispatch.ts`), phụ thuộc trực tiếp TASK-AG-005.1.
3. **`agent-spawner.ts::handleAgentSpawn()` bị 3 CR cùng sửa** (002, 005 — không, 005 chỉ sửa `handleAgentSendInput`; 016 sửa `buildAgentEnv()` + call site trong `handleAgentSpawn()`). Thứ tự: TASK-AG-002.3 trước (thêm `orchSpan`), rồi TASK-AG-016.2 (thêm `span.id` vào `buildAgentEnv(...)` call) — áp dụng trên code đã có `orchSpan` từ 002.3, không xung đột vì khác biến (`span` là `spawnerTracer`, `orchSpan` là `agentOrch*`).
4. **`extractTraceFields()` trong `agent-rpc-dispatch.ts` bị 3 CR cùng mở rộng bucket `method === 'agent.exec'`:** TASK-AG-015.1 tạo bucket, TASK-AG-017.1 thêm `stepId`/`parentTraceId` vào CÙNG bucket đó, TASK-AG-018.2 thêm `taskId` vào CÙNG bucket đó. Thực thi tuần tự 015.1 → 017.1 → 018.2, mỗi task chỉ thêm field mới vào object literal đã có, không tạo `if` block riêng (xem cảnh báo trong từng solution gốc).

---

## Rules cho AI thực thi

1. **Thực thi theo Phase — không bỏ qua Phase 0.** Phase 0 (`Tracer.start(fields?, resume?)`) phải merge xong trước khi bắt đầu bất kỳ TASK-AG-0XX nào — nếu chưa, mọi lệnh gọi `.start(fields, resume)` trong code mẫu sẽ lỗi kiểu TypeScript (tham số thứ 2 không tồn tại).
2. **Verify sau mỗi TASK** — chạy `pnpm run typecheck:node` trước khi sang task tiếp theo.
3. **Không sửa file ngoài `src/relay/` và `src/shared/trace/tracers.ts`** (và `src/shared/trace/index.ts` CHỈ trong Phase 0, không thuộc bộ task này).
4. **Không log giá trị secret** (API key, token, credential đã decrypt, encryptedBlob/iv, nội dung prompt/response, nội dung PTY input) vào `TraceFields` — chỉ metadata (độ dài, tên provider, accountId, boolean has-override...). Xem ràng buộc bảo mật cụ thể trong từng task (005, 013, 014, 016, 018).
5. **Giữ nguyên convention `agent:xxx` đã có** — 9 tracer ad-hoc pre-existing (`agent:rpc`, `agent:git`, `agent:credential`, `agent:fs`, `agent:spawn`, `agent:ext-api`, `agent:tokenManager`, `agent:connection`, `agent:session`) KHÔNG đổi tên. Tracer mới theo CR-TRACE-000 §4 namespace domain (`worktree:*`, `terminal:*`, `agentOrch:*`) chỉ dùng khi solution gốc chỉ định rõ — các trường hợp còn lại theo local convention `agent:xxx`.
6. **Không tự ý implement RPC handler chưa tồn tại** (`shell.exec`, `notification.send`, `annotation.create`, `review.sendFeedback`) — các gap này là gap chức năng có trước, ngoài phạm vi tracing, chỉ ghi nhận bằng regression test khẳng định `MethodNotFound` (xem TASK-AG-017.2).
7. **Tôn trọng ranh giới LOCAL vs REMOTE / Main vs Agent process** — không port bất kỳ logic nào từ `src/main/**` vào `src/relay/**` chỉ vì tên tracer nghe giống nhau (`profile:*`, `remoteIntegration:*`, `agentWs:*`, `codeReview:*` đều là tracer phía Main process/backend, KHÔNG tạo trùng tên ở agent side).
8. **Field `resume`/correlation optional, backward-compatible** — mọi tham số mới thêm vào signature hàm hiện có (`span?`, `parentSpanId?`, `_trace?`) phải là optional cuối cùng, không phá vỡ call site cũ.
9. **Bắt buộc `codegraph explore` trước khi đọc code bằng Read/grep** — khi thực thi bất kỳ TASK-AG-0XX nào, KHÔNG dùng `cat`/`Read`/`grep` thuần để tìm hiểu implementation của một symbol (function/class/const) đã tồn tại trong `src/relay/**` hoặc `src/shared/trace/**`. Luôn chạy `codegraph explore "<ExactSymbolName>"` trước — trả về source verbatim kèm call path trong 1 lượt, thay vì vòng lặp grep + Read. Mỗi task trong thư mục này đã có sẵn mục "Trước khi sửa (bắt buộc theo CLAUDE.md)" liệt kê đúng symbol cần explore — thực thi đúng lệnh đó trước khi mở file.
10. **Bắt buộc `gitnexus_impact` trước khi sửa symbol đã tồn tại + `gitnexus_detect_changes()` trước khi đánh dấu task DONE** — với mọi symbol MODIFY (function/const đã có, không phải symbol mới tạo), chạy `gitnexus_impact({ target: "<symbolName>", direction: "upstream" })` và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục. Sau khi sửa xong một task, chạy `gitnexus_detect_changes()` để xác nhận chỉ đúng symbol/flow dự kiến của task đó bị ảnh hưởng trước khi coi task là ⬜ Pending → ✅ Done. Xem đầy đủ workflow tại `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md`.
