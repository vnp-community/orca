# TASK-FE-005.2: Instrument AI Commit Message (mounted) + gửi feedback cho agent (mounted)

**Phase:** 2
**SOL Ref:** [SOL-FE-TRACE-005 §1.4, §1.5, §2.4, §2.6](../solutions/SOL-FE-TRACE-005-code-review.md)
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-005.1 (tracer `codeReviewAiCommitFlow`/`codeReviewFeedbackFlow` đã khai báo)
**Status:** ✅ Done (2026-08-03) — Discovered mid-task that `pnpm tsc --noEmit` (as literally specified in every task's Verification section) is a silent no-op in this repo — the root `tsconfig.json` has `"files": []` with only project `references`, which bare `tsc --noEmit` (no `--build`) never follows, so it always reports zero errors regardless of real state. Real typecheck is `pnpm typecheck` (3 chained `tsc -p` invocations) or directly `npx tsc --noEmit -p config/tsconfig.tc.web.json` for renderer code — switched to the latter for this task and all subsequent verification. This surfaced the task's documented pre-existing `callRuntimeRpc` signature bug in `useGit.ts` as real (7 other call sites in the file besides `aiCommitMessage` still fail type-check — intentionally left alone, out of scope). Fixed `aiCommitMessage()` to call `callRuntimeRpc(target, method, params)` with the correct signature while adding `Tracers.codeReviewAiCommitFlow` (entry:'commit-form', ok({messageChars})/fail). Instrumented `ReviewNotesSendMenuContent.tsx`'s `sendToAgentTarget()` with `Tracers.codeReviewFeedbackFlow`, gated on `launchSource === 'notes_send'`, ok() only on `onSent` (status 'sent'), fail() only on genuine promise rejection (re-thrown through `runNotesSend`'s existing catch so its toast UX is unchanged). Fixed a test-mock gap (missing `getActiveRuntimeTarget` mock) and added 4 new tracing tests to `useGit.test.ts`-adjacent coverage is n/a (aiCommitMessage test already existed unchanged) plus 4 new tests to `ReviewNotesSendMenuContent.test.tsx` (ok on notes_send success, no span on other launchSource, fail on real rejection not on resolved non-sent status, no double-fire on rapid re-click). Real web typecheck shows zero new errors from my changes (only pre-existing unrelated ones, confirmed via git diff/status). 28/28 tests pass across both files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này sửa 2 symbol độc lập ở 2 file khác nhau — chạy `codegraph explore` + `gitnexus_impact` cho từng symbol trước khi sửa:

```bash
codegraph explore "useGit"
```

```
gitnexus_impact({ target: "useGit", direction: "upstream" })
```

```bash
codegraph explore "ReviewNotesSendMenuContent"
```

```
gitnexus_impact({ target: "ReviewNotesSendMenuContent", direction: "upstream" })
```

Báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) của cả hai trước khi sửa. Cả hai đều đã mount/reachable thật trong UI (khác TASK-FE-005.3) nên kỳ vọng có caller thực — nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Hai flow **đã mount và reachable** từ UI thật, khác với TASK-FE-005.3 (unmounted code):

**BL-CR-04 (AI Commit Message), nhánh đã mount:** `CommitForm.tsx:27-35` (`fillAIMessage`) → `useGit().aiCommitMessage()` → `callRuntimeRpc('git.aiCommitMessage', ...)`. **Bug tiền tồn tại:** call site này thiếu tham số `target` đầu tiên so với chữ ký thật `callRuntimeRpc<T>(target, method, params, options)`. Task này sửa theo chữ ký đúng (kèm `target`), không lặp lại lỗi.

**BL-CR-03 (Gửi Feedback về Agent):** implementation thật nằm ngoài `components/code-review/*` — `DiffNotesSendMenu.tsx` (mount thật tại `SourceControl.tsx:5639`, `CombinedDiffViewer.tsx:1850,1977`, `EditorPanelHeader.tsx:174`) → `NotesSendMenu.tsx` → `ReviewNotesSendMenuContent.tsx:143` (`sendToAgentTarget`) → `sendNotesToActiveAgentSession()`. Span chỉ tạo khi `launchSource === 'notes_send'` (component dùng chung cho markdown-notes, không thuộc CR này).

## File: `src/renderer/src/hooks/useGit.ts` [MODIFY]

```typescript
import { Tracers } from '../../../shared/trace/tracers'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

const aiCommitMessage = useCallback(async () => {
  if (!project) return ''
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const span = Tracers.codeReviewAiCommitFlow.start({ projectId: project.id, entry: 'commit-form' })
  try {
    // Why: chữ ký thật của callRuntimeRpc yêu cầu `target` làm tham số đầu — call
    // site gốc thiếu tham số này (bug có sẵn, không thuộc phạm vi CR tracing); mẫu
    // dưới đây sửa theo chữ ký đúng thay vì lặp lại lỗi.
    const result = await callRuntimeRpc<{ message: string }>(target, 'git.aiCommitMessage', { projectId: project.id, traceId: span.id })
    span.ok({ messageChars: result.message.length })
    return result.message
  } catch (err) {
    span.fail(err, { projectId: project.id })
    throw err
  }
}, [project])
```

## File: `src/renderer/src/components/editor/ReviewNotesSendMenuContent.tsx` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

const sendToAgentTarget = useCallback(
  (target: NotesSendAgentTarget) => {
    if (!hasPrompt || target.status !== 'eligible') {
      return
    }
    const currentEligibility = resolveCurrentSendTargetEligibility(target, worktreeId)
    if (currentEligibility.status !== 'eligible') {
      toast.message(currentEligibility.disabledReason)
      return
    }

    // Why: span chỉ tạo cho nguồn 'diff-notes' (BL-CR-03) — component này dùng
    // chung cho markdown-notes qua cùng đường code, không thuộc CR này.
    const span = launchSource === 'notes_send'
      ? Tracers.codeReviewFeedbackFlow.start({ worktreeId, paneKey: target.paneKey })
      : null

    runNotesSend(
      () => sendNotesToActiveAgentSession({ worktreeId, prompt, noteTarget: { tabId: target.tabId, leafId: target.leafId } }),
      () => {
        onPromptDelivered?.()
        span?.ok({ paneKey: target.paneKey })
        track('agent_prompt_sent', { agent_kind: agentKindForAgentType(target.agentType), launch_source: launchSource, request_kind: 'followup' })
      },
      { explicitTarget: true }
    )
    // Why: runNotesSend() nuốt lỗi bằng toast thay vì throw — span.fail() chỉ có ý
    // nghĩa khi promise reject thật (network/RPC); các status "disabled"/"not-writable"
    // không tính là fail nghiệp vụ.
  },
  [hasPrompt, runNotesSend, worktreeId, prompt, onPromptDelivered, launchSource]
)
```

> `runNotesSend()` nuốt lỗi thành `toast.error`/`toast.message` thay vì propagate exception — cần bọc thêm `.catch()` riêng để gọi `span?.fail()` khi promise reject thật; chi tiết implementation đầy đủ để lại cho code review khi implement (không sửa sâu vào hàm dùng chung nhiều nơi ngoài phạm vi CR).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useGit.test.ts
pnpm test --run src/renderer/src/components/editor/__tests__/ReviewNotesSendMenuContent.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useGit.ts.aiCommitMessage()` sửa để gọi `callRuntimeRpc` với đủ 4 tham số đúng chữ ký (bao gồm `target`) — không lặp lại type-mismatch có sẵn khi thêm tracing
- [ ] `aiCommitMessage()` mở span `ui:codeReview.aiCommitMessage` field `entry: 'commit-form'`, `traceId` trong params, `ok({ messageChars })`/`fail(err, { projectId })`
- [ ] `Tracers.codeReviewFeedbackFlow` chỉ tạo span khi `launchSource === 'notes_send'` trong `ReviewNotesSendMenuContent.tsx` — không gắn nhầm vào luồng markdown-notes dùng chung code
- [ ] `result.status === 'sent'` → `span.ok()` được gọi
- [ ] Test suite xác nhận `Tracers.codeReviewFeedbackFlow` không double-fire khi `sendToAgentTarget()` được gọi lại trước khi promise trước hoàn tất
- [ ] Test suite đạt ≥ 4 test case mới: `aiCommitMessage()` gọi đúng chữ ký + `traceId`; `notes_send` → tạo span trước `sendNotesToActiveAgentSession()`; nguồn khác `notes_send` → KHÔNG tạo span; `sent` → `ok()`
