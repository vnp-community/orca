# TASK-FE-001: Đăng ký toàn bộ tracer `ui:*` dùng chung cho 10 CR frontend

**Phase:** 0
**SOL Ref:** [solutions/00-index.md §"Phát hiện xuyên suốt #1"](../solutions/00-index.md), [SOL-FE-TRACE-015 §1](../solutions/SOL-FE-TRACE-015-profile.md), [SOL-FE-TRACE-016 §2.2](../solutions/SOL-FE-TRACE-016-ai-providers.md), [SOL-FE-TRACE-017 §2.2](../solutions/SOL-FE-TRACE-017-workflow-orchestration.md), [SOL-FE-TRACE-018 §2.1](../solutions/SOL-FE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-000](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md)
**Prerequisite:** TASK-FE-000 (có thể làm song song — không phụ thuộc code, chỉ là 2 precondition độc lập của Phase 0)
**Status:** ✅ Done (2026-08-03) — Added all 9 `ui:*` entries (profile/aiProvider/workflow/taskGraph) to `src/shared/trace/tracers.ts`, additive only; noted existing non-prefixed `worktree:*`/`agentOrch:*`/`terminal:*`/`ui:codeReview.*` entries already present will be touched by their own CR task (001.1/002.1/003.1/005.1/014.1), not this task; `pnpm tsc --noEmit` clean; no `tracers.test.ts` file exists in repo yet.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

**Lưu ý:** `tracers.ts` là registry isomorphic dùng chung giữa domain frontend/agent/backend (xem `src/shared/trace/tracers.ts`) — `gitnexus_impact` ở đây đặc biệt hữu ích để phát hiện xung đột tên biến/flow-string với các task domain khác (agent, backend) đang thêm entry song song vào cùng file này.

## Mô tả

`TracePanel.tsx:42` có heuristic `isBackend`: bất kỳ `flow` chứa `:` mà KHÔNG bắt đầu bằng `ui:` sẽ bị gắn nhãn "backend" sai. CR-TRACE-000 §4 đặt tên tracer theo `domain:operation` — khi dùng đúng tên đó ở renderer (`profile:resolve`, `workflow:execute`, ...) sẽ bị TracePanel hiểu nhầm. Toàn bộ 10/10 solution frontend đã áp dụng nhất quán convention prefix `ui:` cho tracer khởi tạo từ browser (đồng bộ lại 2026-08-02, xem `solutions/00-index.md` mục "Phát hiện xuyên suốt #1"). Task này đăng ký MỘT LẦN 9 tracer `ui:*` do 4 solution 015/016/017/018 liệt kê, vào `src/shared/trace/tracers.ts` — để mỗi task con của CR-015/016/017/018 (Phase 3) không phải sửa trùng lặp file này.

**Về 6 CR còn lại (001, 002, 003, 005, 013, 014):** đã đồng bộ lại 2026-08-02 — 5/6 solution có tracer (001, 002, 003, 005, 014) hiện dùng đúng prefix `ui:` (`ui:worktree.create|fanOut|delete|compare|merge`, `ui:agentOrch.spawn|stop|resume|switch|statusPoll`, `ui:terminal.create|resize|destroy|reconnect`, `ui:codeReview.diff|annotate|sendFeedback|aiCommitMessage|createPr`, `ui:remoteIntegration.preflight|credentialStore`), nhất quán với 015/016/017/018. Các tracer này KHÔNG đăng ký trong task này — mỗi CR tự đăng ký tracer của mình trong task con riêng (xem `TASK-FE-001.1`, `TASK-FE-002.1`, `TASK-FE-003.1`, `TASK-FE-005.1`, `TASK-FE-014.1`), vì đây vẫn là additive edit an toàn vào cùng file `tracers.ts` miễn không trùng tên biến/flow-string (đã xác nhận không trùng). CR-013 không thêm tracer nào (xem SOL-FE-TRACE-013 §2.1 — không có hành động browser-initiated nào để trace). Acceptance Criteria của task này chỉ bao phủ 9 tracer `ui:*` liệt kê dưới đây; 21 tracer còn lại được xác nhận trong Acceptance Criteria của các task con tương ứng.

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

Đọc file hiện tại trước khi sửa để xác nhận cấu trúc `export const Tracers = { ... } as const` và `createTracer` import từ `./index`. Thêm 9 entry mới (gộp từ 4 solution 015/016/017/018), KHÔNG xoá/sửa entry hiện có:

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (bao gồm mọi tracer backend/agent đã đăng ký)...

  // --- ui:* — tracer khởi tạo từ browser/renderer (CR-TRACE-015/016/017/018) ---

  /** Browser-initiated: mount ProfileEditor → fetch resolved + user profile (SOL-FE-TRACE-015 BL-PRF-02) */
  uiProfileResolveFlow: createTracer('ui:profile.resolve'),
  /** Browser-initiated: click "Save Changes" trong ProfileEditor (SOL-FE-TRACE-015 BL-PRF-01) */
  uiProfileUpdateFlow: createTracer('ui:profile.update'),

  /** Browser-initiated: click "Save" trong ProviderForm khi có credential mới (SOL-FE-TRACE-016 BL-AIP-01) */
  uiAiProviderWriteCredFlow: createTracer('ui:aiProvider.writeCredential'),
  /** Browser-initiated: click "Test" trên 1 provider account (SOL-FE-TRACE-016 BL-AIP-03) */
  uiAiProviderTestConnFlow: createTracer('ui:aiProvider.testConnection'),

  /** Browser-initiated: click "Save" trong WorkflowBuilder (SOL-FE-TRACE-017 BL-WF-01) */
  uiWorkflowTemplateSaveFlow: createTracer('ui:workflow.templateSave'),
  /** Browser-initiated: click "Run" — root span của execution nhìn từ browser (SOL-FE-TRACE-017 BL-WF-02) */
  uiWorkflowExecuteFlow: createTracer('ui:workflow.execute'),
  /** Browser-initiated: click "Cancel" trên execution đang chạy (SOL-FE-TRACE-017) */
  uiWorkflowCancelFlow: createTracer('ui:workflow.cancel'),

  /** Browser-initiated: click "Decompose with AI" trong TaskAIDecompose (SOL-FE-TRACE-018 BL-TG-02) */
  uiTaskGraphAiPlanFlow: createTracer('ui:taskGraph.aiPlan'),
  /** Browser-initiated: click "Execute/Run with Agent" — dùng chung bởi TaskDetail + TaskPromptEditor (SOL-FE-TRACE-018 BL-TG-04) */
  uiTaskGraphExecuteFlow: createTracer('ui:taskGraph.execute'),
} as const
```

> File `tracers.ts` là registry isomorphic (dùng chung Node + browser, `src/shared/trace/`). Nếu companion backend solution (CR-TRACE-015/016/017/018 backend) đã thêm entry tên khác (không prefix `ui:`, ví dụ `profileResolveFlow: createTracer('profile:resolve')`), KHÔNG trùng tên biến/flow với 9 entry ở trên — chúng độc lập, chỉ liên kết qua cơ chế `resume`/`traceId` field, không qua tên tracer.

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

- [ ] `src/shared/trace/tracers.ts` có đủ 9 entry: `uiProfileResolveFlow`, `uiProfileUpdateFlow`, `uiAiProviderWriteCredFlow`, `uiAiProviderTestConnFlow`, `uiWorkflowTemplateSaveFlow`, `uiWorkflowExecuteFlow`, `uiWorkflowCancelFlow`, `uiTaskGraphAiPlanFlow`, `uiTaskGraphExecuteFlow`
- [ ] Mọi `flow` name tương ứng đúng bắt đầu bằng `ui:` (`ui:profile.resolve`, `ui:profile.update`, `ui:aiProvider.writeCredential`, `ui:aiProvider.testConnection`, `ui:workflow.templateSave`, `ui:workflow.execute`, `ui:workflow.cancel`, `ui:taskGraph.aiPlan`, `ui:taskGraph.execute`)
- [ ] Không entry nào trong 9 entry mới trùng tên biến hoặc trùng flow-string với entry hiện có trong `tracers.ts`
- [ ] Không xoá hoặc đổi tên bất kỳ entry hiện có nào (additive-only)
- [ ] `pnpm tsc --noEmit` pass, `Tracers` type vẫn suy luận đúng dạng `as const` (mỗi field có type `Tracer`, không bị widen thành `string`)
- [ ] Test `tracers.test.ts` xác nhận cả 9 flow-name đúng chuỗi ký tự (không có typo `ui.profile.resolve` thay vì `ui:profile.resolve`)
- [ ] Ghi rõ trong PR/commit: 6 CR còn lại (001/002/003/005/013/014) KHÔNG có tracer nào được thêm trong task này — tracer của 001/002/003/005/014 đã đăng ký riêng trong task con tương ứng, đều dùng đúng prefix `ui:`; CR-013 không có tracer nào (xem mục Mô tả)
