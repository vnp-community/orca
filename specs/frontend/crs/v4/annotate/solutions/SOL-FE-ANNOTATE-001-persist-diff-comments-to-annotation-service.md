# SOL-FE-ANNOTATE-001: Route `diffComments` slice's remote persistence through `annotation-service`; fix `annotation-panel.tsx`

**Resolves:** [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
**Service/layer:** `frontend/` renderer — `store/slices/diffComments.ts` (existing slice, not a new one) + `components/code-review/annotation-panel.tsx`
**Depends on:** [SOL-BE-ANNOTATE-001](../../../../backend-go/crs/v4/annotate/solutions/SOL-BE-ANNOTATE-001-existing-crud-suffices.md) (confirms backend-go needs zero changes — this solution only calls RPCs that already exist)
**Affected files:**
- `frontend/src/renderer/src/store/slices/diffComments.ts` (MODIFY — `persist()`'s non-local branch only, see §2)
- `frontend/src/renderer/src/components/code-review/annotation-panel.tsx` (MODIFY or DELETE, see §3)
- `frontend/src/renderer/src/components/code-review/annotation-panel.test.tsx` (MODIFY or DELETE, matching above)
**Status:** 📋 Proposed — not yet implemented

---

## 0. Đánh giá trạng thái hiện tại — bắt buộc trước khi thiết kế

Đọc trực tiếp `diffComments.ts` (không chỉ theo mô tả sẵn có trong CR) phát
hiện 1 sự thật quan trọng làm hẹp phạm vi sửa lại **rất nhiều** so với ước
tính ban đầu của CR-ANNOTATE-001: `persist()` (dòng 98-117) **đã** ghi dữ
liệu xuống 2 nơi tuỳ target —

```typescript
async function persist(settings, worktreeId, diffComments) {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.worktrees.updateMeta({ worktreeId, updates: { diffComments } })
    return
  }
  await callRuntimeRpc(target, 'worktree.set', { worktree: ..., diffComments }, { timeoutMs: 15_000 })
}
```

`specs/frontend/tdd/v5/03-runtime-client-layer.md` §2's `RuntimeClientTarget`
union (`{kind: 'local'}` = Electron IPC | `{kind: 'environment'}` = WebSocket
tới backend-go) xác nhận đúng 2 nhánh này ánh xạ tới 2 kiến trúc hoàn toàn
khác nhau — **và chỉ nhánh `environment` (web/multi-user) mới có đường tới
`annotation-service`.** Grep xác nhận `desktop/src` **không có bất kỳ IPC
handler nào** cho method `annotation.create`/`annotation.list` — nghĩa là
dù có sửa gì đi nữa, **desktop/local mode không bao giờ gọi được
`annotation-service`** (không có cầu nối, không phải thiếu code 1 dòng).
`annotation-service` vốn dĩ là 1 service backend-go — chỉ tồn tại/áp dụng
được cho kiến trúc web/multi-user, đúng như toàn bộ backend-go stack.

**Hệ quả cho thiết kế**: sửa chỉ cần chạm vào nhánh `target.kind !==
'local'` của `persist()`. Nhánh `local` giữ nguyên 100% — không có lợi ích
nào từ việc nối `annotation-service` cho desktop single-user (không có vấn
đề multi-session/OPA-authorization để giải quyết ở đó), và cũng không có
hạ tầng để làm việc đó. Đây chính là "ít thay đổi code nhất, tận dụng code
đã có" — chỉ 1 nhánh của 1 hàm cần đổi behavior, toàn bộ optimistic-update/
rollback/queue logic (dòng 119-208, `enqueuePersist`/`mutateComments`) giữ
nguyên vẹn.

## 1. Không đổi: reducer, optimistic update, persist queue

`mutateComments`/`enqueuePersist`/`persistQueueByWorktree` (dòng 98-208) xử
lý đúng vấn đề khó (concurrent writes, per-worktree ordering, latest-wins) —
không có lý do viết lại. Solution này chỉ thay **nội dung của việc "ghi" ở
cuối chuỗi** cho nhánh remote, không đổi *khi nào*/*theo thứ tự nào* việc ghi
đó chạy.

## 2. Thiết kế — nhánh `target.kind !== 'local'` của `persist()`

Vấn đề: `persist()` hiện nhận **toàn bộ mảng `diffComments`** và ghi đè
1 lần (`worktree.set`) — không biết đâu là comment mới/sửa/xoá so với lần
trước. `annotation-service`'s API là CRUD theo từng bản ghi
(`create`/`update`/`delete`), không có "set toàn bộ mảng" tương đương.
Cách tối thiểu để không phải viết lại toàn bộ interface của slice: giữ
nguyên `persist(settings, worktreeId, diffComments)`'s signature, nhưng bên
trong nhánh remote, diff so với **lần persist gần nhất đã biết** (lưu trong
1 `Map<worktreeId, DiffComment[]>` module-level, cập nhật sau mỗi lần
persist thành công — không cần đọc lại từ server) để suy ra
created/updated/deleted, rồi gọi đúng RPC cho từng thay đổi:

```typescript
// Thêm cạnh persistQueueByWorktree — cùng lifetime, cùng lý do tồn tại
// (theo dõi trạng thái đã biết per-worktree để tính diff, không phải state
// nguồn — state nguồn vẫn là store).
const lastPersistedByWorktree: Map<string, DiffComment[]> = new Map()

async function persist(
  settings: AppState['settings'],
  worktreeId: string,
  diffComments: DiffComment[]
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.worktrees.updateMeta({ worktreeId, updates: { diffComments } })
    return
  }

  const previous = lastPersistedByWorktree.get(worktreeId) ?? []
  const previousById = new Map(previous.map((c) => [c.id, c]))
  const currentById = new Map(diffComments.map((c) => [c.id, c]))

  // Created: id chưa từng thấy — gọi annotation.create, KHÔNG gửi id (server
  // sinh id thật); comment.id tạm thời (client UUID) chỉ dùng làm optimistic
  // key nội bộ, không gửi lên server.
  for (const c of diffComments) {
    if (!previousById.has(c.id)) {
      await callRuntimeRpc(target, 'annotation.create', diffCommentToCreateArgs(worktreeId, c), { timeoutMs: 15_000 })
    }
  }
  // Deleted: có ở previous, không còn ở current.
  for (const c of previous) {
    if (!currentById.has(c.id)) {
      await callRuntimeRpc(
        target, 'annotation.delete',
        { id: c.id, confirmed: Boolean(c.sentAt) }, // BR-CR-08: xác nhận nếu đã sentAt
        { timeoutMs: 15_000 }
      )
    }
  }
  // Updated: body đổi (sửa comment) — annotation-service có UpdateAnnotation
  // thật (author-only OPA, §9 TDD) nhưng CHƯA có wscompat channel nào lộ ra
  // (chỉ create/list/delete/markSent, xem SOL-BE-ANNOTATE-001 §3's bảng) —
  // ghi chú lại đây làm gap cần 1 task backend-go nhỏ nếu update-in-place
  // là yêu cầu thật (updateDiffComment đã tồn tại như 1 action riêng của
  // slice này — xem "Không thuộc phạm vi" bên dưới).

  lastPersistedByWorktree.set(worktreeId, diffComments)
}

function diffCommentToCreateArgs(worktreeId: string, c: DiffComment) {
  return {
    worktreeId,
    filePath: c.filePath,
    line: c.startLine ?? c.lineNumber,
    endLine: c.startLine !== undefined ? c.lineNumber : undefined,
    side: 'SIDE_NEW', // types.ts:778-779's v1 limitation — không đổi trong CR này
    originalCode: c.selectedText ?? '',
    content: c.body
  }
}
```

**Hydrate khi mở worktree** (đọc lại buffer khi vào lại app/session khác):
thêm 1 lần gọi `annotation.list({worktreeId, sentToAgent: false})` khi
worktree được chọn lần đầu trong session (nơi gọi cụ thể: nơi
`RuntimeSyncWindowGraph`'s worktree data được apply lần đầu cho target
`environment`, theo TDD-FE-03 §3 — xác nhận lại điểm hook chính xác bằng
`context()` trước khi code, không suy đoán tên hàm), map response về shape
`DiffComment[]`, seed vào `worktreesByRepo[repoId][i].diffComments` VÀ
`lastPersistedByWorktree` cùng lúc để lần persist tiếp theo diff đúng.

## 3. `annotation-panel.tsx` — nguồn gốc lệch shape, theo đúng TDD

`specs/backend-go/tdd/services/annotation-service.md` §3/§4 (thiết kế GỐC,
trước khi triển khai thật) mô tả `ProjectID`/`ReviewID` — khớp gần như
chính xác với field mà `annotation-panel.tsx` đang gửi
(`projectId`/`reviewId`). Đây gần như chắc chắn là bằng chứng
`annotation-panel.tsx` được viết theo đúng bản thiết kế TDD ban đầu, trước
khi implementation thật (SOL-CR-02) chủ động rẽ hướng sang
`RepoID`+`WorktreeID` (không có `ReviewID`) vì lý do đã ghi trong chính
SOL-CR-02. `annotation-panel.tsx` chưa từng được cập nhật theo hướng rẽ đó.

**Quyết định (theo CR-ANNOTATE-001 §B, nghiêng phương án xoá)**: `context()`
xác nhận 5 caller (`DiffViewer.tsx`, `DiffSectionBody.tsx`, `MonacoEditor.tsx`,
`RichMarkdownAnnotationOverlay.tsx`, `code-review-panel.tsx`) — nếu
`AnnotationPanelProps.reviewId` (optional) xác nhận không có nơi nào gán
giá trị thật (kiểm tra lại tại 5 caller này trước khi xoá, không suy đoán
từ 1 lần grep), xoá file này và 2 nơi gọi nó, thay bằng
`DiffCommentCard`/`useDiffCommentDecorator` (đã hoạt động đúng, đã nối
`annotation-service` sau §2). Nếu có 1 caller thật sự cần `reviewId` (1 khái
niệm review-session tách biệt khỏi worktree), sửa shape đúng thay vì xoá —
xem CR gốc §B phương án 2.

## 4. Backfill — comment cũ trong `WorktreeMeta.metadata` JSONB

Theo CR-ANNOTATE-001 §A.1: khi `RuntimeSyncWindowGraph` load 1 worktree lần
đầu ở target `environment` sau khi solution này deploy, nếu
`worktree.diffComments` (từ JSONB cũ) có phần tử mà `annotation.list` không
trả về tương ứng (so theo `filePath`+`lineNumber`+`body`, vì id cũ là
client-UUID không map trực tiếp sang `annotation_id` mới), gọi
`annotation.create` cho từng phần tử đó 1 lần, rồi để `lastPersistedByWorktree`
coi chúng như "đã persist" — không xoá field `diffComments` khỏi JSONB cũ
trong lần này (an toàn nếu backfill lỗi giữa chừng, dọn dẹp là việc sau).

## Test plan

- `diffComments.test.ts` (file test đã có, mở rộng): mock `callRuntimeRpc`
  cho `annotation.create`/`delete`/`list`; xác nhận nhánh `target.kind ===
  'local'` KHÔNG đổi hành vi (regression guard — đây là phần rủi ro nhất
  của 1 thay đổi tưởng nhỏ: vô tình phá desktop mode trong lúc sửa remote
  mode).
- Case: add 1 comment (remote target) → `annotation.create` gọi đúng 1 lần,
  đúng field.
- Case: add rồi xoá trước khi persist kịp chạy (đã có logic "queue lấy state
  mới nhất tại thời điểm dequeue", dòng 145-153) → chỉ 1 trong 2 RPC chạy
  (không tạo rồi xoá ngay, tránh round-trip thừa) — cần xác nhận hành vi
  hiện có của `enqueuePersist` xử lý đúng case này trước khi kết luận không
  cần sửa gì thêm.
- Case: xoá comment có `sentAt` → `annotation.delete` gọi với
  `confirmed: true`; xoá comment chưa `sentAt` → `confirmed: false`.
- `annotation-panel.test.tsx`: cập nhật theo quyết định §3 (xoá test nếu xoá
  file, hoặc sửa mock RPC theo shape đúng nếu giữ).

## Không thuộc phạm vi CR/solution này

- `updateDiffComment` (sửa nội dung comment đã tạo) route qua
  `annotation.update` — backend-go's `UpdateAnnotation` usecase tồn tại
  (theo TDD §9, author-only OPA) nhưng **chưa có wscompat channel** lộ ra
  (chỉ `create`/`list`/`delete`/`markSent`, xác nhận ở SOL-BE-ANNOTATE-001
  §3). Cần 1 task backend-go nhỏ riêng (thêm channel `annotation.update`,
  cùng pattern các channel khác) nếu tính năng sửa-tại-chỗ là yêu cầu thật —
  hiện tại `updateDiffComment` chỉ đổi Zustand state, không persist remote
  qua đường nào cả (kể cả JSONB cũ — kiểm tra lại `persist()`'s call site
  của `updateDiffComment` để xác nhận có bị bug tương tự hay không, ngoài
  phạm vi khảo sát của solution này).
- `clearDiffComments`/`clearDiffCommentsForFile` (xoá hàng loạt) — map sang
  N lệnh `annotation.delete` hay cần 1 RPC bulk-delete mới là quyết định
  triển khai, không quyết trong solution này.

## References

- [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
- [SOL-BE-ANNOTATE-001](../../../../backend-go/crs/v4/annotate/solutions/SOL-BE-ANNOTATE-001-existing-crud-suffices.md)
- `frontend/src/renderer/src/store/slices/diffComments.ts` (đọc toàn bộ cho solution này)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:405-494` (`worktree.set`'s metadata-patch behavior, xác nhận đường lưu cũ)
- `backend-go/services/project-service/migrations/0028_worktree_metadata.up.sql`
- `specs/frontend/tdd/v5/03-runtime-client-layer.md` §2 (`RuntimeClientTarget`)
- `specs/frontend/tdd/v5/02-state-management.md` §2 (`diff-comments` slice registry entry)
- `specs/backend-go/tdd/services/annotation-service.md` §3/§4/§9 (thiết kế gốc — nguồn gốc shape mismatch của `annotation-panel.tsx`; §9 cho OPA author-only rule)
