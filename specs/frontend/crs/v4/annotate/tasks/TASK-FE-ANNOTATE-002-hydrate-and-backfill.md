# TASK-FE-ANNOTATE-002: Hydrate `diffComments` from `annotation.list` on worktree load; backfill from `WorktreeMeta.metadata` JSONB

**Solution:** [SOL-FE-ANNOTATE-001](../solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) §2 (hydrate), §4 (backfill) | **CR:** CR-ANNOTATE-001
**Depends on:** [TASK-FE-ANNOTATE-001](./TASK-FE-ANNOTATE-001-persist-remote-create-delete.md) (cần `lastPersistedByWorktree` tồn tại để seed đúng sau hydrate)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Khi 1 worktree được mở lần đầu trong session (target `environment`), load
lại review buffer từ `annotation.list` thay vì chỉ tin dữ liệu đã có sẵn
trong `RuntimeSyncWindowGraph`'s worktree snapshot — đồng thời backfill 1
lần các comment cũ còn nằm trong `WorktreeMeta.metadata`'s `diffComments`
field (đường lưu cũ, JSONB) sang `annotation-service` nếu chưa có bản ghi
tương ứng.

## Files cần sửa

1. Điểm hook chính xác — **xác nhận bằng GitNexus trước khi sửa** (xem
   "gitnexus" bên dưới), dự kiến nơi `sync-runtime-graph.ts`'s
   `performRuntimeGraphSync`/`applyRuntimeGraph` apply worktree data lần
   đầu cho 1 worktree mới xuất hiện trong graph — **không phải file cố
   định, cần tìm bằng công cụ, không đoán tên hàm từ TDD**
2. `frontend/src/renderer/src/store/slices/diffComments.ts` (MODIFY — thêm
   action `hydrateDiffCommentsFromServer(worktreeId)` được gọi từ điểm hook
   ở trên)

## Bước 1 — Xác định chính xác điểm "worktree mở lần đầu trong session"

Đọc `specs/frontend/tdd/v5/03-runtime-client-layer.md` §3 (`sync-runtime-graph.ts`)
làm điểm khởi đầu, nhưng **xác nhận lại bằng `context()`/đọc code thật** —
TDD mô tả tổng quan, không đủ chi tiết để biết chính xác dòng nào "lần đầu
worktree này xuất hiện" (khác với "worktree đã có, chỉ update field khác").
Có thể cần thêm 1 `Set<string>` theo dõi worktreeId đã hydrate trong session
hiện tại, tránh gọi `annotation.list` lặp lại mỗi lần graph sync (mặc định
sync có thể chạy thường xuyên — debounce 16ms theo TDD).

## Bước 2 — `hydrateDiffCommentsFromServer`

```typescript
async function hydrateDiffCommentsFromServer(
  target: RuntimeClientTarget,
  worktreeId: string
): Promise<DiffComment[]> {
  const resp = await callRuntimeRpc<{ annotations: AnnotationWire[] }>(
    target, 'annotation.list',
    { worktreeId, sentToAgent: false },
    { timeoutMs: 15_000 }
  )
  return resp.annotations.map(annotationWireToDiffComment)
}

function annotationWireToDiffComment(a: AnnotationWire): DiffComment {
  // Map ngược lại field đã gửi ở TASK-FE-ANNOTATE-001's diffCommentToCreateArgs
  // — xác nhận đúng shape response thật của annotation.list trước khi viết
  // hàm này, không suy đoán từ shape request.
}
```

Sau khi hydrate, seed cả `worktreesByRepo[repoId][i].diffComments` (để UI
hiển thị) VÀ `lastPersistedByWorktree` (module-level map từ
TASK-FE-ANNOTATE-001) với cùng 1 mảng — nếu không seed
`lastPersistedByWorktree`, lần `persist()` tiếp theo sẽ coi TOÀN BỘ comment
vừa hydrate là "mới" và gọi `annotation.create` trùng lặp.

## Bước 3 — Backfill 1 lần

Sau khi hydrate xong, so `worktree.diffComments` (nếu còn tồn tại từ JSONB
cũ — đọc qua field đã có sẵn trong `RuntimeSyncWindowGraph`'s worktree data,
không phải 1 RPC riêng) với kết quả `annotation.list` vừa nhận, theo
`filePath`+`lineNumber`+`body` (không theo `id` — id cũ là client-UUID,
không map trực tiếp sang `annotation_id` mới). Với mỗi phần tử trong JSONB
cũ KHÔNG có bản ghi tương ứng trong kết quả `annotation.list`, gọi
`annotation.create` 1 lần cho nó. **Không xoá field `diffComments` khỏi
JSONB cũ** trong task này (an toàn nếu backfill lỗi giữa chừng — dọn dẹp là
task riêng, ngoài phạm vi).

## Test plan

- Mount worktree lần đầu (target `environment`) → `annotation.list` được
  gọi đúng 1 lần với `worktreeId` đúng.
- Mount lại lần 2 cùng session (worktree không đổi) → KHÔNG gọi lại
  `annotation.list` (đã hydrate rồi).
- JSONB cũ có 2 comment, `annotation.list` trả về 1 (khớp) → chỉ 1 comment
  còn lại được backfill qua `annotation.create`.
- JSONB cũ rỗng, `annotation.list` trả về N → không gọi `annotation.create`
  nào (không có gì backfill).

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/store/slices/diffComments.test.ts
npx tsc --noEmit
```

## gitnexus

**Bắt buộc** — `query({search_query: "worktree opened first time in session sync graph"})`
hoặc `context({name: "performRuntimeGraphSync"})` để xác định chính xác
điểm hook trước khi viết code Bước 1 — solution chỉ mô tả ý tưởng, không
xác nhận tên hàm/vị trí chính xác. Đây là task **duy nhất** trong nhóm
frontend cần khảo sát thêm trước khi biết chính xác file nào cần sửa ngoài
`diffComments.ts`.


---

## ✅ Kết quả thực tế (2026-09-11)

**Lệch đáng kể so với kế hoạch ban đầu — quyết định kiến trúc, không phải
chi tiết implementation**: Bước 1 của task này yêu cầu tìm chính xác điểm
"worktree mở lần đầu trong session" bên trong `sync-runtime-graph.ts`
(~56K, không có ranh giới rõ ràng cho khái niệm này — đã tự xác nhận qua
`query()` GitNexus, không tìm được 1 hàm/điểm hook đơn lẻ khớp đúng ý
nghĩa "lần đầu", chỉ có `performRuntimeGraphSync`/`applyRuntimeGraph` chạy
định kỳ mỗi lần đồng bộ, không phân biệt "lần đầu" vs "lần thứ N").

**Quyết định thay thế**: thay vì móc vào tầng sync-graph (rủi ro cao, khó
xác minh đúng, hàm phụ trợ để lộ), triển khai hydration như 1 action mới
của chính slice này (`ensureDiffCommentsHydrated`), gọi từ 1 `useEffect`
trong `DiffViewer.tsx` (component tiêu thụ `diffComments` thật, đã xác nhận
đang hoạt động) — idempotent qua `hydratedWorktreeIds: Set<string>` +
khử trùng lặp lời gọi đồng thời qua `hydrationInFlight: Map`. Đây là thiết
kế **tự chứa, ít rủi ro hơn** bản sketch gốc — không đụng vào file đồng bộ
56K dòng, không cần hiểu hết cơ chế `RuntimeSyncWindowGraph` để làm đúng.
Đã ghi rõ lý do trong code comment tại chỗ.

**Backfill dùng matching theo nội dung, không theo id** (đúng như solution
đã lường trước — `filePath`+`lineNumber`+`body`) — đã kiểm chứng bằng 2 test
riêng (khớp → không backfill; không khớp → backfill qua `annotation.create`).

**Phát hiện phụ dẫn tới TASK-BE-ANNOTATE-003 (backend-go, ✅ DONE)**: khi
viết `annotationWireToDiffComment`, tra proto `Annotation`/`Anchor`'s Go
struct cho thấy `annotation.list`/`create`/`update`/`markSent` trả proto
thô, dính lỗi "JSON tag snake_case, envelope serialize bằng encoding/json
thường" — cùng lớp bug đã được nhiều channel khác trong repo tự tìm và sửa
trước đây nhưng bỏ sót nhóm `annotation.*`. Đã tách thành task backend-go
riêng (TASK-BE-ANNOTATE-003) và sửa trước khi tiếp tục task này — nếu không
sửa, mọi field nhiều từ (`worktreeId`, `endLine`, `createdAtUnixMs`...) sẽ
về `undefined` phía frontend.

**Verify thật đã chạy**: `vitest run src/renderer/src/store/slices/diffComments.test.ts` —
24/24 PASS (19 cũ không đổi + 5 test mới cho `ensureDiffCommentsHydrated`,
tất cả pass ngay lần chạy đầu — dấu hiệu mock/thiết kế khớp đúng contract
thật đã xác nhận với backend-go). `tsc --noEmit`: không có lỗi mới liên
quan `diffComments.ts`/`DiffViewer.tsx` (lỗi `diffComments.test.ts(151,...)`
"AppState 1005 more props" đã xác nhận pre-existing, không đổi so với
TASK-FE-ANNOTATE-001's lần kiểm tra trước).

**Files đã sửa:**
- `frontend/src/renderer/src/store/slices/diffComments.ts` (MODIFY — thêm `ensureDiffCommentsHydrated`, `hydrateFromAnnotationService`, `annotationWireToDiffComment`, mở rộng `clearDiffCommentsPersistCacheForTests`)
- `frontend/src/renderer/src/components/editor/DiffViewer.tsx` (MODIFY — 1 `useEffect` mới)
- `frontend/src/renderer/src/store/slices/diffComments.test.ts` (MODIFY — thêm `describe('ensureDiffCommentsHydrated', ...)`, 5 test)
- `backend-go/...` — xem TASK-BE-ANNOTATE-003 (phát sinh từ task này, không phải file của task này)
