# SOL-FE-TRACE-005: Code Review — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**TDD Ref:** TDD-FE-16 (Remote Git UI — `16-remote-git-ui.md`), TDD-FE-05 (UI Components)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts`, `src/shared/trace/tracers.ts`, TracePanel. CR-TRACE-000 (naming convention, quy ước field `traceId`).

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 Phát hiện quan trọng: HAI implementation song song cho BL-CR-01→05, một cái không được mount

Trước khi map sub-flow → call site, cần ghi nhận một phát hiện doc/code-drift **sâu hơn** những gì CR-TRACE-005 (phía backend) đã tìm ra — CR đó grep chủ yếu ở `src/main/`, còn renderer thực tế có **hai cây component riêng biệt** cùng tự nhận là "code review panel":

| Cây component | Vị trí | Mount trong app? | RPC method gọi | Khớp với backend registry? |
|---|---|---|---|---|
| `GitPanel` (TDD-FE-16, tab "Changes"/"Pull Requests") | `src/renderer/src/components/workspace/git/*` | **CÓ** — mount tại `WorkspaceLayout.tsx` (xác nhận qua grep `GitPanel\b`) | `git.getDiff`, `git.getStatus`, `git.stageFile`, `git.unstageFile`, `git.stageAll`, `git.unstageAll`, `git.aiCommitMessage`, `git.pr.list`, `git.commit`, `git.push` | **KHÔNG** cho phần lớn — chỉ `git.commit`/`git.push` có method cùng tên đăng ký trong `src/main/runtime/rpc/methods/git.ts`/`git-remote.ts`. `git.getDiff`, `git.getStatus`, `git.stageFile`, `git.unstageFile`, `git.stageAll`, `git.unstageAll`, `git.aiCommitMessage`, `git.pr.list` **không tồn tại** trong backend (đã grep `name: 'git.*'` trên toàn bộ `src/main` — chỉ có `git.diff`, `git.status`, `git.stage`, `git.bulkStage`, `git.unstage`, `git.bulkUnstage`, `git.generateCommitMessage`, không phải các tên trên) |
| `CodeReviewPanel` (`code-review-panel.tsx`, comment tự ghi "BL-CR-01~05") | `src/renderer/src/components/code-review/*` | **KHÔNG** — đã grep `import.*CodeReviewPanel\|<CodeReviewPanel` trên toàn bộ `src/renderer/src`, không có kết quả nào mount component này | `git.diff`, `git.exec`, `git.commit`, `git.generateCommitMessage`, `git.pr.create`, `annotation.create`, `annotation.list` | Phần lớn **CÓ** khớp (`git.diff`, `git.commit`, `git.generateCommitMessage`, `git.pr.create` đều có `name:` tương ứng trong `git.ts`/`git-remote.ts`), TRỪ `annotation.create`/`annotation.list` — không tồn tại ở backend, khớp phát hiện gốc của CR-TRACE-005 |

Cả hai cây đều dùng chung một `DiffViewer` (`src/renderer/src/components/workspace/git/DiffViewer.tsx` — `code-review-panel.tsx:15` import trực tiếp từ đường dẫn này, comment "Lazy-import the DiffViewer that already exists in workspace/git"), nên BL-CR-01 chỉ có **một** điểm instrument dùng chung cho cả hai đường.

Hệ quả cho CR này: instrumentation phải bám theo **code thực tế đang chạy được** (`GitPanel`, theo đúng nguyên tắc CR-TRACE-000 §8), đồng thời vẫn thêm tracer vào các call site thật của `CodeReviewPanel` (annotate, feedback, PR create, AI commit) vì code đó tồn tại thật — chỉ đánh dấu rõ "hiện chưa reachable từ UI" thay vì bỏ qua, theo đúng tinh thần "không bịa component, nhưng cũng không bỏ sót component có thật".

### 1.2 BL-CR-01 — Xem Diff (`DiffViewer.tsx`, dùng chung, ĐÃ mount)

**Entry point UI thật:** user chọn 1 file trong `ChangedFilesTree`/`StagingArea` → `GitPanel.tsx:48-51` (`handleViewDiff`) hoặc `code-review-panel.tsx:86-89` → render `<DiffViewer filePath={...} />`. `useEffect` trong `DiffViewer.tsx:43-91` chạy 1-2 RPC call song song tuỳ `staged`:
- `staged=true`: 1 call `git.getDiff` (dòng 53-57)
- `staged=false` (mặc định): 2 call song song — `git.getDiff` (dòng 70-75, lấy bản HEAD) và `fs.readFile` (dòng 78-82, lấy bản working-tree hiện tại)

**Lưu ý bắt buộc nêu trong prose:** RPC method `git.getDiff` gọi từ đây **không tồn tại trong backend** (chỉ có `git.diff`, viết hoa/thường khác — xem 1.1). Khi CR này ship, `span.fail()` sẽ tự động lộ ra lỗi "method not found" mỗi lần user click 1 file — đây chính là giá trị thực tế của việc thêm tracing ở đây: nó biến một bug âm thầm (component fail silent, hiển thị `error` state chung chung "Failed to load diff") thành một tín hiệu rõ ràng trong TracePanel. Solution này **không tự sửa** tên RPC method (ngoài phạm vi tracing CR, thuộc companion backend/UI-bugfix CR).

### 1.3 BL-CR-02 — Annotate Dòng Code (`annotation-panel.tsx`, CÓ code thật, CHƯA mount)

Trái với giả định của CR-TRACE-005 (backend) rằng "annotation.create không tìm thấy call site nào" — renderer **có** call site thật: `src/renderer/src/components/code-review/annotation-panel.tsx`:
- `useEffect` (dòng 48-62): gọi `annotation.list` khi mở panel cho 1 dòng
- `submit()` (dòng 64-83): gọi `annotation.create` khi user bấm nút "Comment"

Component này chỉ được render từ `code-review-panel.tsx:98-105` (`annotationLine !== null && selectedFile`) — mà `CodeReviewPanel` bản thân **không được mount ở đâu cả** (1.1). Đồng thời `annotation.create`/`annotation.list` cũng không tồn tại ở backend. Vậy đây là code "kép treo" (built nhưng cả hai đầu — UI mount point và RPC handler — đều chưa nối). Solution này vẫn viết instrumentation đầy đủ cho `submit()` (mục 2.3) vì code UI có thật và cụ thể, nhưng đánh dấu rõ ràng trong Acceptance Criteria rằng span này **sẽ không bao giờ emit** cho tới khi có một companion CR mount `<CodeReviewPanel>` vào `WorkspaceLayout.tsx` (hoặc tương đương) VÀ backend đăng ký `annotation.create`/`annotation.list`.

### 1.4 BL-CR-03 — Gửi Feedback về Agent (implementation thật nằm NGOÀI `components/code-review/*`)

`AnnotationPanel` (1.3) chỉ lưu comment qua `annotation.create` — **không có nút "gửi cho agent"** nào trong toàn bộ cây `components/code-review/*`. Cơ chế "gửi ghi chú review cho agent đang chạy" **có thật và đang hoạt động**, nhưng sống trong một hệ thống annotation khác, cũ hơn, không thuộc `code-review/*`:

- Model dữ liệu: `DiffComment` (`src/renderer/src/store/slices/diffComments.ts`) — persist qua RPC `worktree.set` (dòng 111-116, khi `target.kind !== 'local'`) hoặc IPC `window.api.worktrees.updateMeta` (dòng 104-109, khi local) — **cả hai method đều CÓ THẬT** trong backend (`worktree.set` xác nhận tại `src/main/runtime/rpc/methods/worktree.ts:159`)
- UI tạo comment: `DiffSectionItem.tsx` → `submitDiffSectionComment()` (`src/renderer/src/components/editor/diff-section-comment-submit.ts:18-65`) → `addDiffComment()` (store action)
- UI gửi cho agent: `DiffNotesSendMenu.tsx` (mount thật tại `SourceControl.tsx:5639`, `CombinedDiffViewer.tsx:1850,1977`, `EditorPanelHeader.tsx:174` — cả ba đều là component đã mount trong app) → `NotesSendMenu.tsx` → `ReviewNotesSendMenuContent.tsx:143` (`sendToAgentTarget`) → `sendNotesToActiveAgentSession()` (`src/renderer/src/lib/active-agent-note-send.ts:42-137`)
- RPC thật bên trong `sendNotesToActiveAgentSession()`: `terminal.wait` (dòng 95-100) rồi `terminal.send` (dòng 145-155 hoặc dòng 183-247 tuỳ agent hỗ trợ guarded-paste) — cả hai method **CÓ THẬT**, xác nhận tại `src/main/runtime/rpc/methods/terminal.ts:1085,1274`

**Quyết định kiến trúc:** `Tracers.codeReviewFeedbackFlow` (BL-CR-03) được gắn vào `sendToAgentTarget()` trong `ReviewNotesSendMenuContent.tsx` — đây là component **dùng chung** cho cả diff-notes lẫn markdown-notes (điều hướng qua prop `source`), nhưng khi gọi từ `DiffNotesSendMenu` (`source` mặc định `'diff-notes'`), đây chính là hành động nghiệp vụ BL-CR-03. Không sửa sâu vào `sendNotesToActiveAgentSession()` (file dùng chung nhiều luồng khác ngoài code review) — span bọc quanh lời gọi ở outer layer, không `step()` riêng cho `terminal.wait`/`terminal.send` bên trong (tránh phải đổi signature của một hàm lib dùng chung nhiều nơi ngoài phạm vi CR này).

### 1.5 BL-CR-04 — AI Commit Message (2 call site thật, method name khác nhau)

- **Call site ĐÃ mount, method KHÔNG khớp backend:** `CommitForm.tsx:27-35` (`fillAIMessage`) → `useGit().aiCommitMessage()` (`src/renderer/src/hooks/useGit.ts:100-106`) → `callRuntimeRpc('git.aiCommitMessage', ...)`. **Lưu ý phụ:** call site này trong `useGit.ts` gọi `callRuntimeRpc(method, params)` — thiếu tham số `target` đầu tiên so với chữ ký thật `callRuntimeRpc<T>(target, method, params, options)` xác nhận tại `src/renderer/src/runtime/runtime-rpc-client.ts:68`. Đây là một type-mismatch có sẵn trong code (ngoài phạm vi CR này) — code mẫu ở mục 2.4 sửa theo chữ ký đúng (kèm `target`), không lặp lại lỗi.
- **Call site CHƯA mount, method khớp backend:** `commit-message-generator.tsx:31-39` (`generateMessage`) → `callRuntimeRpc(target, 'git.generateCommitMessage', ...)` — đúng tên method thật (`git.ts:263`, `git-remote.ts:353`), nhưng chỉ render bên trong `CodeReviewPanel` (không mount).
- **Call site phụ, cùng method đúng, CHƯA mount:** `PullRequestForm.tsx:123-158` (`handleGenerateDescription`) dùng `git.generateCommitMessage` làm "proxy" để soạn PR description (comment dòng 133 tự ghi rõ điều này).

### 1.6 BL-CR-05 — Tạo Pull Request (`PullRequestForm.tsx`, CHƯA mount)

Method `git.pr.create` (`PullRequestForm.tsx:169`) khớp đúng backend (`git-remote.ts:394`). Nhưng **không có đường UI reachable nào** dẫn tới `PullRequestForm`:
- `GitPanel.tsx` tab "Pull Requests" render `PullRequestList.tsx` — component này **chỉ list** (`git.pr.list`, method cũng không tồn tại backend), không có nút "New PR" nào gọi `PullRequestForm`
- `PullRequestForm` chỉ được render từ `PrCreateDialog` (`src/renderer/src/components/code-review/pr-create-dialog.tsx:45-50`), mà `PrCreateDialog` chỉ được render từ `CodeReviewPanel:109-115` — cả hai đều thuộc cây không-mount (1.1)

Đồng thời `pr-create-dialog.tsx:45-50` truyền các prop `sourceBranch`/`onSuccess`/`onCancel` cho `PullRequestForm`, nhưng chữ ký thật của `PullRequestForm` (`PullRequestForm.tsx:97-105`) chỉ nhận `projectId`/`worktreePath`/`currentBranch` — một mismatch khác, ngoài phạm vi CR này, không sửa ở đây.

---

## 2. Full Implementation

### 2.1 Thêm tracer mới vào `tracers.ts`

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged...
  codeReviewDiffFlow:      createTracer('ui:codeReview.diff'),           // BL-CR-01
  codeReviewAnnotateFlow:  createTracer('ui:codeReview.annotate'),       // BL-CR-02
  codeReviewFeedbackFlow:  createTracer('ui:codeReview.sendFeedback'),   // BL-CR-03
  codeReviewAiCommitFlow:  createTracer('ui:codeReview.aiCommitMessage'),// BL-CR-04
  codeReviewCreatePrFlow:  createTracer('ui:codeReview.createPr'),       // BL-CR-05
} as const
```

> File này dùng chung isomorphic (`src/shared/trace/`) giữa renderer/main/relay. Prefix `ui:` bắt buộc theo quyết định chung (`00-index.md` mục 1) — khác `codeReview:diff|annotate|sendFeedback|aiCommitMessage|createPr` mà companion backend solution (SOL-BE-TRACE-005) dùng, để tránh `isBackend` heuristic của `TracePanel.tsx:42` gắn nhầm badge cho event do browser tự phát.

### 2.2 `DiffViewer.tsx` — BL-CR-01

```typescript
// src/renderer/src/components/workspace/git/DiffViewer.tsx
import { Tracers } from '../../../../../shared/trace/tracers'

// ...existing imports/component signature unchanged...

useEffect(() => {
  if (!project || !filePath) return
  setIsLoading(true)
  setError(null)

  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const span = Tracers.codeReviewDiffFlow.start({ filePath, staged, mode: target.kind })

  if (staged) {
    callRuntimeRpc<string>(target, 'git.getDiff', {
      projectId: project.id,
      path: filePath,
      staged: true,
      traceId: span.id,
    })
      .then(diff => {
        const idx = diff.indexOf('@@')
        setOriginal(idx >= 0 ? diff.slice(0, idx) : '')
        setModified(diff)
        span.ok({ staged: true })
      })
      .catch(err => {
        setError(err?.message ?? 'Failed to load diff')
        span.fail(err, { staged: true })
      })
      .finally(() => setIsLoading(false))
  } else {
    span.step('parallelFetch', { staged: false })
    Promise.all([
      callRuntimeRpc<string>(target, 'git.getDiff', {
        projectId: project.id, path: filePath, staged: false, side: 'original', traceId: span.id,
      }).catch(() => ''),
      callRuntimeRpc<{ content: string }>(target, 'fs.readFile', {
        projectId: project.id, path: filePath, encoding: 'utf-8', traceId: span.id,
      }).then(r => r.content).catch(() => ''),
    ])
      .then(([original, modified]) => {
        setOriginal(original)
        setModified(modified)
        span.ok({ staged: false })
      })
      .catch(err => {
        setError(err?.message ?? 'Failed to load diff')
        span.fail(err, { staged: false })
      })
      .finally(() => setIsLoading(false))
  }
}, [filePath, worktreePath, project, staged])
```

> `span.step('parallelFetch', ...)` đánh dấu điểm rẽ nhánh quan trọng (staged vs unstaged dùng chiến lược fetch khác nhau hẳn — 1 call vs 2 call song song), theo đúng CR-TRACE-000 §5 quy tắc 3.

### 2.3 `annotation-panel.tsx` — BL-CR-02 (code thật, chưa reachable — xem 1.3)

```typescript
// src/renderer/src/components/code-review/annotation-panel.tsx
import { Tracers } from '../../../../shared/trace/tracers'

// ...existing imports/state unchanged...

const submit = async () => {
  if (!newComment.trim() || !project || lineNumber === null) return
  setIsSaving(true)
  const span = Tracers.codeReviewAnnotateFlow.start({
    filePath, lineNumber, reviewId: reviewId ?? '',
  })
  try {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const created = await callRuntimeRpc<Annotation>(target, 'annotation.create', {
      projectId: project.id,
      reviewId,
      filePath,
      lineNumber,
      content: newComment.trim(),
      traceId: span.id,
    })
    setAnnotations(prev => [...prev, created])
    setNewComment('')
    span.ok({ annotationId: created.id })
  } catch (err) {
    toast.error('Failed to save comment')
    span.fail(err, { filePath, lineNumber })
  } finally {
    setIsSaving(false)
  }
}
```

> Không instrument `useEffect` load `annotation.list` (dòng 48-62) — đây là một GET đơn giản, thất bại đã được nuốt lặng lẽ (`catch(() => {})`) theo thiết kế UX hiện tại (không có annotation nào thì không hiển thị lỗi); theo CR-TRACE-000 §5, một GET không phân nhánh quan trọng và không phải nguồn debug chính không bắt buộc `step()`/span riêng.

### 2.4 `useGit.ts` — BL-CR-04, nhánh ĐÃ mount (sửa kèm chữ ký `callRuntimeRpc` đúng)

```typescript
// src/renderer/src/hooks/useGit.ts
import { Tracers } from '../../../shared/trace/tracers'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

// ...existing hook body unchanged, chỉ sửa aiCommitMessage()...

const aiCommitMessage = useCallback(async () => {
  if (!project) return ''
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const span = Tracers.codeReviewAiCommitFlow.start({ projectId: project.id, entry: 'commit-form' })
  try {
    // Why: chữ ký thật của callRuntimeRpc yêu cầu `target` làm tham số đầu —
    // call site gốc thiếu tham số này (bug có sẵn, không thuộc phạm vi CR
    // tracing); mẫu dưới đây sửa theo chữ ký đúng thay vì lặp lại lỗi.
    const result = await callRuntimeRpc<{ message: string }>(target, 'git.aiCommitMessage', {
      projectId: project.id,
      traceId: span.id,
    })
    span.ok({ messageChars: result.message.length })
    return result.message
  } catch (err) {
    span.fail(err, { projectId: project.id })
    throw err
  }
}, [project])
```

### 2.5 `commit-message-generator.tsx` — BL-CR-04, nhánh CHƯA mount, method khớp backend

```typescript
// src/renderer/src/components/code-review/commit-message-generator.tsx
import { Tracers } from '../../../../shared/trace/tracers'

const generateMessage = async () => {
  if (!project) return
  setIsGenerating(true)
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const span = Tracers.codeReviewAiCommitFlow.start({ projectId: project.id, entry: 'code-review-panel' })
  try {
    const message = await callRuntimeRpc<string>(target, 'git.generateCommitMessage', {
      projectId: project.id,
      worktreePath: worktreePath ?? project.rootPath,
      traceId: span.id,
    })
    onChange(message)
    span.ok({ messageChars: message.length })
  } catch (err: any) {
    if (err?.code === 'GIT_NO_STAGED_CHANGES' || err?.message?.includes('no staged')) {
      toast.error('Stage some files first before generating a commit message')
    } else {
      toast.error('Failed to generate commit message')
    }
    span.fail(err, { projectId: project.id })
  } finally {
    setIsGenerating(false)
  }
}
```

### 2.6 `ReviewNotesSendMenuContent.tsx` — BL-CR-03 (đường thật, đã mount)

```typescript
// src/renderer/src/components/editor/ReviewNotesSendMenuContent.tsx
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

    // Why: span chỉ tạo cho nguồn 'diff-notes' (BL-CR-03) — component này
    // dùng chung cho markdown-notes qua cùng đường code, không thuộc CR này.
    const span = launchSource === 'notes_send'
      ? Tracers.codeReviewFeedbackFlow.start({ worktreeId, paneKey: target.paneKey })
      : null

    runNotesSend(
      () => sendNotesToActiveAgentSession({
        worktreeId,
        prompt,
        noteTarget: { tabId: target.tabId, leafId: target.leafId },
      }),
      () => {
        onPromptDelivered?.()
        span?.ok({ paneKey: target.paneKey })
        track('agent_prompt_sent', {
          agent_kind: agentKindForAgentType(target.agentType),
          launch_source: launchSource,
          request_kind: 'followup',
        })
      },
      { explicitTarget: true }
    )
    // Why: runNotesSend() nuốt lỗi bằng toast (xem NotesSendMenuContent.tsx:127-138)
    // thay vì throw — span.fail() chỉ có ý nghĩa khi promise reject thật (network/RPC),
    // các status "disabled"/"not-writable" không phải fail nghiệp vụ, không gọi fail().
  },
  [hasPrompt, runNotesSend, worktreeId, prompt, onPromptDelivered, launchSource]
)
```

> Vì `runNotesSend()` nuốt lỗi thành `toast.error`/`toast.message` thay vì propagate exception (xem `ReviewNotesSendMenuContent.tsx:108-138`), `span.fail()` chỉ bắt được lỗi network/RPC thật sự (promise reject từ `sendNotesToActiveAgentSession()`); các trạng thái nghiệp vụ như `not-ready`/`permission` không tính là fail theo CR-TRACE-000 §5 (không phải process/host boundary crash). Cần bọc thêm `.catch()` riêng để gọi `span?.fail()` khi promise reject — chi tiết implementation đầy đủ (bọc `runNotesSend` để expose reject) để lại cho code review khi implement thật, không mở rộng thêm ở đây để tránh sửa quá sâu vào một hàm dùng chung nhiều nơi.

### 2.7 `PullRequestForm.tsx` — BL-CR-05 (chưa reachable — xem 1.6)

```typescript
// src/renderer/src/components/workspace/git/PullRequestForm.tsx
import { Tracers } from '../../../../../shared/trace/tracers'

const handleSubmit = useCallback(async () => {
  if (!title.trim()) {
    setError('PR title is required')
    return
  }
  setIsSubmitting(true)
  setError(null)
  const span = Tracers.codeReviewCreatePrFlow.start({ projectId, base, draft })
  try {
    const result = await callRuntimeRpc<PrCreateResult>(rpcTarget(), 'git.pr.create', {
      projectId,
      worktreePath,
      title: title.trim(),
      body: body.trim(),
      base,
      draft,
      head: currentBranch,
      traceId: span.id,
    })
    setPrUrl(result.url)
    emit('git.pr.created', { projectId, url: result.url, title })
    span.ok({ prUrl: result.url, exitCode: result.exitCode })
  } catch (err) {
    setError((err as Error).message)
    span.fail(err, { projectId, base })
  } finally {
    setIsSubmitting(false)
  }
}, [title, body, base, draft, currentBranch, projectId, worktreePath, rpcTarget, emit])
```

---

## 3. Test Plan (Vitest)

```
src/renderer/src/components/workspace/git/__tests__/DiffViewer.test.tsx   (file đã tồn tại — thêm test case)
├── staged=true → Tracers.codeReviewDiffFlow.start({ staged: true, ... }) rồi ok() khi git.getDiff resolve
├── staged=false → span.step('parallelFetch') được gọi trước Promise.all
├── traceId: span.id có trong params của cả 2 lời gọi callRuntimeRpc khi staged=false
└── git.getDiff reject → span.fail() với field staged đúng nhánh

src/renderer/src/components/code-review/__tests__/annotation-panel.test.tsx   (mới)
├── submit() gọi Tracers.codeReviewAnnotateFlow.start({ filePath, lineNumber }) trước callRuntimeRpc
├── annotation.create thành công → span.ok({ annotationId })
└── annotation.create reject → span.fail(), toast.error được gọi

src/renderer/src/hooks/__tests__/useGit.test.ts   (file đã tồn tại — thêm test case)
└── aiCommitMessage() gọi callRuntimeRpc với target làm tham số đầu + traceId: span.id trong params

src/renderer/src/components/code-review/__tests__/commit-message-generator.test.tsx   (mới)
├── generateMessage() → Tracers.codeReviewAiCommitFlow.start({ entry: 'code-review-panel' })
└── GIT_NO_STAGED_CHANGES error → span.fail() vẫn được gọi dù toast khác message

src/renderer/src/components/editor/__tests__/ReviewNotesSendMenuContent.test.tsx   (file đã tồn tại — thêm test case)
├── launchSource='notes_send' → tạo span codeReviewFeedbackFlow trước sendNotesToActiveAgentSession()
├── launchSource khác 'notes_send' (markdown notes) → KHÔNG tạo span codeReviewFeedbackFlow
└── result.status === 'sent' → span.ok() được gọi

src/renderer/src/components/workspace/git/__tests__/PullRequestForm.test.tsx   (mới)
├── handleSubmit() thành công → Tracers.codeReviewCreatePrFlow start → ok({ prUrl })
└── git.pr.create reject → span.fail(err, { projectId, base })

src/shared/trace/__tests__/tracers.test.ts   (file đã tồn tại — thêm assertion)
└── Tracers.codeReviewDiffFlow/AnnotateFlow/FeedbackFlow/AiCommitFlow/CreatePrFlow tồn tại đúng flow name 'ui:codeReview.diff|annotate|sendFeedback|aiCommitMessage|createPr'
```

**Mock pattern:** dùng `vi.mock('../../../runtime/runtime-rpc-client')` để spy `params` truyền vào `callRuntimeRpc`, assert `params.traceId === span.id` bằng cách `vi.spyOn(Tracers.codeReviewXxxFlow, 'start')` để capture span mock trước.

**Target:** ≥ 20 test case mới.

---

## 4. Acceptance Criteria

- [ ] `Tracers.codeReviewDiffFlow/AnnotateFlow/FeedbackFlow/AiCommitFlow/CreatePrFlow` được thêm vào `src/shared/trace/tracers.ts` đúng tên `ui:codeReview.diff|annotate|sendFeedback|aiCommitMessage|createPr`
- [ ] `DiffViewer.tsx` (dùng chung cho cả `GitPanel` đã mount và `CodeReviewPanel` chưa mount) phát `span.step('parallelFetch')` khi `staged=false`, phân biệt `staged` trong `ok()`/`fail()`
- [ ] `useGit.ts.aiCommitMessage()` được sửa để gọi `callRuntimeRpc` với đủ 4 tham số đúng chữ ký (bao gồm `target`) — không lặp lại type-mismatch có sẵn khi thêm tracing
- [ ] `Tracers.codeReviewAnnotateFlow`/`Tracers.codeReviewCreatePrFlow` được cắm vào code thật (`annotation-panel.tsx`, `PullRequestForm.tsx`) dù cả hai hiện **không reachable** từ UI đã mount — Acceptance Criteria không yêu cầu các span này thực sự emit cho tới khi có companion CR mount `CodeReviewPanel`
- [ ] `Tracers.codeReviewFeedbackFlow` chỉ tạo span khi `launchSource === 'notes_send'` trong `ReviewNotesSendMenuContent.tsx` — không gắn nhầm vào luồng markdown-notes dùng chung code
- [ ] Không method RPC nào bị đổi tên trong quá trình thêm tracing — các mismatch `git.getDiff`/`git.aiCommitMessage`/`git.pr.list` vs backend registry được ghi nhận trong mục 1 nhưng KHÔNG sửa (ngoài phạm vi CR tracing)
- [ ] `traceId: span.id` xuất hiện trong `params` của mọi lời gọi `callRuntimeRpc` được instrument ở CR này (không thêm vào nhánh `window.api.*` Electron IPC nếu có, theo đúng phạm vi WebSocket RPC của CR-TRACE-000 §3.3)
- [ ] Test suite xác nhận `Tracers.codeReviewFeedbackFlow` không double-fire khi `sendToAgentTarget()` được gọi lại trước khi promise trước hoàn tất
