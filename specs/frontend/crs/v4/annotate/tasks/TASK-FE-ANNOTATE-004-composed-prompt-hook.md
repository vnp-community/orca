# TASK-FE-ANNOTATE-004: `useComposedAllNotesPrompt` hook + wire into `DiffNotesSendMenu.tsx`

**Solution:** [SOL-FE-ANNOTATE-002](../solutions/SOL-FE-ANNOTATE-002-call-composed-prompt-before-delivery.md) §1-2 | **CR:** CR-ANNOTATE-002
**Depends on:** [TASK-FE-ANNOTATE-001](../../annotate/tasks/TASK-FE-ANNOTATE-001-persist-remote-create-delete.md), [TASK-FE-ANNOTATE-002](./TASK-FE-ANNOTATE-002-hydrate-and-backfill.md) (annotation phải persist thật trước thì mới có gì để compose), backend-go's TASK-BE-ANNOTATE-002 (channel `annotation.composeReviewPrompt` phải tồn tại)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Thêm 1 hook mới, nhỏ (`useComposedAllNotesPrompt`) trả về prompt đã được
backend-go làm giàu code-context (BR-CR-11) cho scope "all unsent notes"
trong `DiffNotesSendMenu.tsx`, với fallback về `formatDiffComments` hiện có
khi RPC lỗi/chưa sẵn sàng. **Không đổi** `sendNotesToActiveAgentSession`/
`sendPromptWithGuardedPasteAndEnter` — chỉ đổi nguồn chuỗi `prompt` truyền
vào chúng.

## Files cần sửa

1. `frontend/src/renderer/src/lib/use-composed-all-notes-prompt.ts` (MỚI)
2. `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (MODIFY — chỉ scope "all unsent notes", KHÔNG đổi scope "this file")

## Bước 1 — Viết hook

Theo đúng snippet SOL-FE-ANNOTATE-002 §2. Điểm cần xác nhận lại khi code
(không có trong snippet, cần đọc code thật trước khi implement):

- `worktreeDisplayName` — tên/nguồn prop thật tương ứng "{worktree-name}"
  trong `formatReviewPrompt`'s template (BL-CR-03 flow step 3) — tìm trong
  `SourceControl.tsx`'s chỗ gọi `<DiffNotesSendMenu worktreeId={...} .../>` xem
  đã có sẵn tên hiển thị nào truyền xuống chưa.
- Response shape thật của `annotation.composeReviewPrompt`
  (`{prompt, annotationIds}` theo thiết kế backend-go — xác nhận field name
  đúng chính tả camelCase phía JSON, không suy đoán từ Go struct tag).

```typescript
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
    setComposed(null)
    if (unsentNotes.length === 0) { return }
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    if (target.kind === 'local') { return }
    const timer = setTimeout(() => {
      void callRuntimeRpc<{ prompt: string; annotationIds: string[] }>(
        target, 'annotation.composeReviewPrompt', { worktreeId, worktreeName }, { timeoutMs: 8000 }
      )
        .then((result) => { if (!cancelled && result.prompt) { setComposed(result) } })
        .catch(() => {})
    }, 500)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [notesKey, worktreeId, worktreeName, unsentNotes.length])

  return composed ?? { prompt: fallbackPrompt, annotationIds: fallbackIds }
}
```

## Bước 2 — Wire vào `DiffNotesSendMenu.tsx`

Đổi CHỈ `allNotesScope`'s `prompt`:

```typescript
// Trước:
const unsentPrompt = useMemo(() => formatDiffComments(unsentNotes), [unsentNotes])
// Sau:
const { prompt: unsentPrompt, annotationIds: unsentAnnotationIds } =
  useComposedAllNotesPrompt(worktreeId, worktreeDisplayName, unsentNotes)
```

`unsentFilePrompt` (scope "this file") **giữ nguyên** `formatDiffComments`
— không gọi hook mới, theo đúng quyết định phạm vi ở SOL-FE-ANNOTATE-002 §2
("chỉ scope all unsent notes").

`unsentAnnotationIds` được dùng ở TASK-FE-ANNOTATE-005 (không dùng trong
task này) — vẫn cần lưu lại biến này ở đây để task sau dùng, không tính là
"code chết" tạm thời trong lúc 2 task này chưa cả 2 xong; nếu implement 2
task này trong 1 PR duy nhất thì không có giai đoạn trung gian này.

## Test plan

- RPC thành công → `unsentPrompt` là bản compose (chứa "Context:"), không
  phải `formatDiffComments`'s output.
- RPC lỗi/timeout → `unsentPrompt` là `formatDiffComments`'s output (không
  throw, không hiện lỗi UI).
- `unsentNotes` rỗng → không gọi RPC.
- `target.kind === 'local'` → không gọi RPC (spy xác nhận 0 lần gọi).
- File-scope (`unsentFilePrompt`) không đổi hành vi — vẫn `formatDiffComments`
  thuần, không có test mới nào cho nó trong task này (regression guard: xác
  nhận vẫn dùng `useMemo` cũ, không vô tình đổi sang hook mới).

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/lib/use-composed-all-notes-prompt.test.ts src/renderer/src/components/editor/DiffNotesSendMenu.test.tsx
npx tsc --noEmit
```

(Tên file test cụ thể xác nhận lại theo convention hiện có của thư mục —
`DiffNotesSendMenu.tsx` có thể chưa có file test riêng, kiểm tra trước khi
tạo mới hay thêm vào file có sẵn.)

## gitnexus

`context({name: "DiffNotesSendMenu"})` trước khi sửa — xác nhận props thật
(có `worktreeId` nhưng cần thêm tên hiển thị worktree ở đâu) và không có
caller nào khác ngoài `SourceControl.tsx` phụ thuộc vào `unsentPrompt`'s
hành vi đồng bộ hiện có theo cách sẽ bị phá bởi việc chuyển sang giá trị có
thể đổi bất đồng bộ sau mount.


---

## ✅ Kết quả thực tế (2026-09-11)

Đúng theo thiết kế solution, với 1 chi tiết xác nhận thêm khi implement:
`worktreeName` lấy qua `findWorktreeById(s.worktreesByRepo, worktreeId)?.displayName`
(field `Worktree.displayName`, xác nhận trực tiếp trong `shared/types.ts`),
fallback về `worktreeId` nếu không tìm thấy worktree trong store.

**Phụ thuộc backend-go thật đã hoạt động đúng**: nhờ TASK-BE-ANNOTATE-003
(sửa bug snake_case JSON) chạy trước, response `annotation.composeReviewPrompt`
đã đúng camelCase `{prompt, annotationIds}` — không cần patch gì thêm ở
tầng frontend cho vấn đề này.

**Test viết mới hoàn toàn** (task gốc dự kiến tên file test khác, thực tế
chưa có file test nào cho `DiffNotesSendMenu.tsx` từ trước — viết test cho
hook `useComposedAllNotesPrompt` theo pattern `renderHook`-thủ-công đã có
sẵn trong codebase, `use-tab-agent.test.ts`, vì hook này dùng
`useAppStore` singleton thật, không phải store cô lập per-test như
`diffComments.test.ts`): 5 test — trả về fallback ngay lập tức, swap sang
composed prompt sau khi RPC resolve, giữ fallback khi RPC lỗi, không gọi
RPC ở target local, không gọi RPC khi rỗng. Cả 5 PASS ngay lần chạy đầu.

**Verify thật đã chạy**: `vitest run src/renderer/src/lib/use-composed-all-notes-prompt.test.ts` —
5/5 PASS. `tsc --noEmit`: sạch cho mọi file bị sửa (không có lỗi mới ngoài
lỗi pre-existing đã xác nhận từ TASK-FE-ANNOTATE-001/002).

**Files đã sửa:**
- `frontend/src/renderer/src/lib/use-composed-all-notes-prompt.ts` (MỚI)
- `frontend/src/renderer/src/lib/use-composed-all-notes-prompt.test.ts` (MỚI)
- `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (MODIFY — wire hook vào scope "all unsent notes", giữ nguyên scope "this file")
