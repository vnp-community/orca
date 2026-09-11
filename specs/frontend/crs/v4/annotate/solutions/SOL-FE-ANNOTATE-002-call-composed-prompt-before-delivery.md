# SOL-FE-ANNOTATE-002: Enrich the "all unsent notes" send-to-agent prompt via `annotation.composeReviewPrompt`, keep existing guarded delivery unchanged

**Resolves:** [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md) — phương án 1 (tách compose khỏi delivery)
**Service/layer:** `frontend/` renderer — `components/editor/DiffNotesSendMenu.tsx` (existing, small change) + `lib/active-agent-note-send.ts` (unchanged — reused as-is)
**Depends on:** [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) / [SOL-FE-ANNOTATE-001](./SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) — `annotation.composeReviewPrompt` reads persisted annotations; if comments never reach `annotation-service`, this has nothing to enrich with and the fallback path (see §2) is always taken
**Also depends on:** [SOL-BE-ANNOTATE-002](../../../../backend-go/crs/v4/annotate/solutions/SOL-BE-ANNOTATE-002-extract-compose-only-channel.md) — the `annotation.composeReviewPrompt` channel this calls
**Affected files:**
- `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (MODIFY)
- `frontend/src/renderer/src/components/editor/NotesSendMenu.tsx` (NO CHANGE — generic, shared with markdown-notes flow, stays untouched)
- `frontend/src/renderer/src/lib/active-agent-note-send.ts` (NO CHANGE — the guarded-paste delivery mechanism this solution deliberately keeps)
**Status:** 📋 Proposed — not yet implemented

---

## 0. Ràng buộc quan trọng nhất — không đổi cơ chế delivery

CR-ANNOTATE-002's toàn bộ lý do tồn tại là: lấy code-context (BR-CR-11) từ
backend-go's composition **mà không đánh đổi lùi** độ tin cậy delivery hiện
có. `active-agent-note-send.ts`'s `sendPromptWithGuardedPasteAndEnter` (đợi
`terminal.wait('tui-idle')`, gói bracketed-paste, Enter riêng, fallback cho
SSH runtime cũ — đọc toàn bộ file cho CR-ANNOTATE-002 đã xác nhận đây là
implementation thật, không phải mô tả) **không đổi 1 dòng nào** trong
solution này. Solution này chỉ đổi **nguồn của chuỗi `prompt`** truyền vào
`sendNotesToActiveAgentSession({worktreeId, prompt, noteTarget})` — hàm đó
nhận `prompt: string` bất kỳ, không quan tâm nó tới từ đâu.

## 1. Điểm chèn chính xác — `DiffNotesSendMenu.tsx`

Code thật hiện tại (đọc lại cho solution này):

```typescript
const unsentNotes = useMemo(() => comments.filter((c) => !c.sentAt), [comments])
const unsentPrompt = useMemo(() => formatDiffComments(unsentNotes), [unsentNotes])
// ...
const allNotesScope = { id: 'all', label: '...', notes: unsentNotes, prompt: unsentPrompt }
```

`scope.prompt` là 1 `string` đồng bộ, được `NotesSendMenu`'s `openTargetMode`
đọc ngay khi user click mở dropdown (`handleOpenChange` → `openTargetMode(defaultScope)`,
đồng bộ, không `await`). `annotation.composeReviewPrompt` là RPC bất đồng bộ
— không thể chèn trực tiếp vào `useMemo`. Thiết kế: **prefetch khi buffer
thay đổi, không phải khi user click** — khi user click, prompt đã sẵn sàng
(hoặc đã fallback) từ trước đó.

## 2. Thiết kế — hook `useComposedAllNotesPrompt`

```typescript
// frontend/src/renderer/src/lib/use-composed-all-notes-prompt.ts (mới, nhỏ)
//
// Trả về ngay formatDiffComments(unsentNotes) (đồng bộ, không đổi hành vi
// hiện có), đồng thời kích hoạt 1 lần gọi annotation.composeReviewPrompt
// mỗi khi unsentNotes đổi (debounce nhẹ — người dùng thường thêm nhiều
// comment liên tiếp trước khi gửi, không cần compose lại sau MỖI keystroke
// của MỖI comment). Khi RPC trả về thành công, thay prompt hiện có bằng bản
// đã compose + set annotationIds tương ứng. Lỗi RPC (annotation-service
// down, network) → giữ nguyên bản client-side, annotationIds suy ra từ
// unsentNotes.map(c => c.id) như hành vi cũ (khớp CR-ANNOTATE-002's tiêu
// chí chấp nhận "không chặn gửi").
export function useComposedAllNotesPrompt(
  worktreeId: string,
  worktreeName: string,
  unsentNotes: readonly DiffComment[]
): { prompt: string; annotationIds: string[] } {
  const fallbackPrompt = useMemo(() => formatDiffComments(unsentNotes), [unsentNotes])
  const fallbackIds = useMemo(() => unsentNotes.map((c) => c.id), [unsentNotes])
  const [composed, setComposed] = useState<{ prompt: string; annotationIds: string[] } | null>(null)
  const notesKey = useMemo(() => unsentNotes.map((c) => c.id).join(','), [unsentNotes])

  useEffect(() => {
    setComposed(null) // buffer đổi → bản compose cũ không còn khớp, quay về fallback ngay
    if (unsentNotes.length === 0) {
      return
    }
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Why: chỉ web/multi-user target mới có annotation-service — target
    // 'local' (desktop) không có cầu nối (xem SOL-FE-ANNOTATE-001 §0), gọi
    // RPC ở đó chỉ tốn 1 round-trip lỗi vô ích mỗi lần.
    if (target.kind === 'local') {
      return
    }
    const timer = setTimeout(() => {
      void callRuntimeRpc<{ prompt: string; annotationIds: string[] }>(
        target,
        'annotation.composeReviewPrompt',
        { worktreeId, worktreeName },
        { timeoutMs: 8000 }
      )
        .then((result) => {
          if (!cancelled && result.prompt) {
            setComposed(result)
          }
        })
        .catch(() => {
          // Why: silent — fallbackPrompt already covers the UX; a toast here
          // would fire on every keystroke-triggered debounce during
          // annotation-service downtime, which is noise, not signal.
        })
    }, 500) // debounce — xem comment ở trên
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [notesKey, worktreeId, worktreeName, unsentNotes.length])

  return composed ?? { prompt: fallbackPrompt, annotationIds: fallbackIds }
}
```

`DiffNotesSendMenu.tsx` đổi 2 dòng:

```typescript
// Trước:
const unsentPrompt = useMemo(() => formatDiffComments(unsentNotes), [unsentNotes])
// Sau:
const { prompt: unsentPrompt, annotationIds: unsentAnnotationIds } =
  useComposedAllNotesPrompt(worktreeId, worktreeDisplayName, unsentNotes)
```

(`worktreeDisplayName` — cần xác nhận lại tên/nguồn prop thật khi implement,
tương ứng "{worktree-name}" trong template `formatReviewPrompt`, BL-CR-03
flow step 3.)

**Chỉ scope "all unsent notes"**, không áp dụng cho scope "this file"
(`unsentFilePrompt`) — `annotation.composeReviewPrompt`
(SOL-BE-ANNOTATE-002) chỉ nhận `worktreeId`, không có filter theo file.
Thêm filter đó vào backend là mở rộng phạm vi CR-ANNOTATE-002 đã chốt (chỉ
tách compose/delivery, không đổi tham số) — nếu file-scope cũng cần
code-context, đó là 1 CR/task riêng mở rộng `annotation.composeReviewPrompt`
nhận thêm `filePath` optional, không làm trong solution này.

## 3. `onDelivered` → `annotation.markSent` với `annotationIds` đúng

`DiffNotesSendMenu`'s `onDelivered={(notes) => void clearDeliveredDiffComments(worktreeId, notes)}`
giữ nguyên (Zustand-side clear vẫn cần, không đổi). Thêm:

```typescript
onDelivered: (notes) => {
  void clearDeliveredDiffComments(worktreeId, notes)
  // annotationIds ở đây là ids THẬT từ annotation-service (nếu compose
  // thành công) hoặc client-local ids (nếu fallback) — annotation.markSent
  // với 1 id không tồn tại ở server chỉ no-op (theo SubscriptionRepository-
  // style "not found = 0 rows affected, not an error" convention của
  // backend-go, xác nhận lại đúng cho AnnotationRepository trước khi dựa
  // vào giả định này) — an toàn để gọi vô điều kiện, không cần biết trước
  // compose có thành công hay không.
  void callRuntimeRpc(getActiveRuntimeTarget(useAppStore.getState().settings), 'annotation.markSent', { ids: unsentAnnotationIds }, { timeoutMs: 8000 })
    .catch(() => {}) // Why: best-effort bookkeeping — a failure here must not surface as a "send failed" error; the prompt was already delivered.
}
```

## Test plan

- `useComposedAllNotesPrompt`: RPC thành công → trả `composed.prompt`, không
  phải fallback. RPC lỗi/timeout → trả fallback, không throw. `unsentNotes`
  rỗng → không gọi RPC (không có gì để compose). `target.kind === 'local'`
  → không gọi RPC (xác nhận bằng spy, không chỉ đọc code).
- `DiffNotesSendMenu`: file-scope KHÔNG gọi `annotation.composeReviewPrompt`
  (regression guard cho quyết định §2's phạm vi hẹp).
- Integration-ish test (mock `callRuntimeRpc`): gửi thành công → cả
  `clearDeliveredDiffComments` VÀ `annotation.markSent` được gọi, đúng thứ
  tự không quan trọng (cả 2 độc lập, best-effort).

## Không thuộc phạm vi solution này

- Mở rộng `annotation.composeReviewPrompt` nhận filter theo file — xem §2.
- Đổi `sendPromptWithGuardedPasteAndEnter`/`sendNotesToActiveAgentSession` —
  không chạm, theo đúng ràng buộc §0.
- `ReviewNotesSendMenuContent.tsx`/`NotesSendMenu.tsx` — generic, dùng chung
  markdown-notes flow, không sửa gì (chỉ nhận `prompt`/`onDelivered` đã được
  tính đúng từ `DiffNotesSendMenu`, đúng data flow hiện có).

## References

- [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md)
- [SOL-FE-ANNOTATE-001](./SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) (phụ thuộc dữ liệu)
- [SOL-BE-ANNOTATE-002](../../../../backend-go/crs/v4/annotate/solutions/SOL-BE-ANNOTATE-002-extract-compose-only-channel.md) (channel này gọi tới)
- `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx`, `NotesSendMenu.tsx`, `ReviewNotesSendMenuContent.tsx`
- `frontend/src/renderer/src/lib/active-agent-note-send.ts` (delivery giữ nguyên, đọc toàn bộ để xác nhận không cần đổi)
- `specs/frontend/tdd/v5/03-runtime-client-layer.md` §2 (`RuntimeClientTarget` — cơ sở cho guard "chỉ gọi RPC khi target không phải `local`")
