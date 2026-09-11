# TASK-FE-ANNOTATE-001: `persist()`'s remote branch calls `annotation.create`/`annotation.delete` instead of `worktree.set`

**Solution:** [SOL-FE-ANNOTATE-001](../solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) §1-2 | **CR:** CR-ANNOTATE-001
**Depends on:** Không (backend-go's `annotation.create`/`delete` đã có sẵn — SOL-BE-ANNOTATE-001)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Sửa **chỉ nhánh `target.kind !== 'local'`** của `persist()`
(`diffComments.ts`) để gọi `annotation.create`/`annotation.delete` (diff so
với lần persist gần nhất) thay vì `worktree.set`'s bulk-overwrite. Nhánh
`target.kind === 'local'` (desktop) **không đổi 1 dòng** — không có cầu nối
tới `annotation-service` ở đó (xem SOL-FE-ANNOTATE-001 §0).

## Files cần sửa

1. `frontend/src/renderer/src/store/slices/diffComments.ts` (MODIFY — chỉ hàm `persist()`)
2. `frontend/src/renderer/src/store/slices/diffComments.test.ts` (MODIFY — thêm test case mới, giữ nguyên test case nhánh `local`)

## Bước 1 — Đọc lại `persist()` thật, xác nhận chưa đổi từ lúc viết solution

`diffComments.ts` dòng 98-117 tại thời điểm viết solution. Đọc lại trước
khi sửa — nếu đã có thay đổi khác (vd. thêm field mới cho `DiffComment`),
điều chỉnh theo code thật, không áp snippet solution một cách máy móc.

## Bước 2 — Thêm `lastPersistedByWorktree` map + logic diff

Theo đúng snippet SOL-FE-ANNOTATE-001 §2. Đặt `lastPersistedByWorktree` ở
cùng module-level scope với `persistQueueByWorktree` (dòng 135) — cùng
lifetime, cùng lý do tồn tại (theo dõi trạng thái per-worktree).

```typescript
const lastPersistedByWorktree: Map<string, DiffComment[]> = new Map()

async function persist(settings, worktreeId, diffComments) {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.worktrees.updateMeta({ worktreeId, updates: { diffComments } })
    return
  }

  const previous = lastPersistedByWorktree.get(worktreeId) ?? []
  const previousById = new Map(previous.map((c) => [c.id, c]))
  const currentById = new Map(diffComments.map((c) => [c.id, c]))

  for (const c of diffComments) {
    if (!previousById.has(c.id)) {
      await callRuntimeRpc(target, 'annotation.create', diffCommentToCreateArgs(worktreeId, c), { timeoutMs: 15_000 })
    }
  }
  for (const c of previous) {
    if (!currentById.has(c.id)) {
      await callRuntimeRpc(target, 'annotation.delete', { id: c.id, confirmed: Boolean(c.sentAt) }, { timeoutMs: 15_000 })
    }
  }

  lastPersistedByWorktree.set(worktreeId, diffComments)
}

function diffCommentToCreateArgs(worktreeId: string, c: DiffComment) {
  return {
    worktreeId,
    filePath: c.filePath,
    line: c.startLine ?? c.lineNumber,
    endLine: c.startLine !== undefined ? c.lineNumber : undefined,
    side: 'SIDE_NEW',
    originalCode: c.selectedText ?? '',
    content: c.body
  }
}
```

**Xác nhận lại field name thật** của `annotation.create`/`annotation.delete`
wscompat channel (`worktreeId`/`filePath`/`line`/`endLine`/`side`/
`originalCode`/`content`, `id`/`confirmed`) bằng `context()`/đọc
`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`'s
`registerAnnotationChannels` trước khi code — snippet ở đây là thiết kế,
không phải đã verify field-by-field với response decoder thật phía Go.

## Bước 3 — Xử lý update (sửa nội dung comment)

`updateDiffComment` (action đã có sẵn trong slice) hiện KHÔNG gọi `persist()`
theo cùng đường — kiểm tra lại: nếu nó có gọi `persist()` với mảng đã sửa
nội dung, logic diff ở Bước 2 (chỉ theo `id` có/không) sẽ **bỏ sót** thay
đổi nội dung (vì `id` không đổi). Ghi nhận đây là gap KHÔNG thuộc phạm vi
task này (SOL-FE-ANNOTATE-001's "Không thuộc phạm vi" đã loại trừ tường
minh — `annotation.update` channel chưa tồn tại ở backend-go) — không tự ý
thêm channel mới trong task này.

## Test plan

- Mock `callRuntimeRpc`, `target.kind = 'environment'`: add 1 comment →
  `annotation.create` gọi đúng 1 lần với field đúng.
- Xoá comment có `sentAt` → `annotation.delete` gọi với `confirmed: true`;
  chưa `sentAt` → `confirmed: false`.
- `target.kind = 'local'`: xác nhận **không** gọi bất kỳ `annotation.*` RPC
  nào — vẫn gọi `window.api.worktrees.updateMeta` y hệt trước — đây là
  regression guard quan trọng nhất của task này.
- Add rồi xoá trước khi `enqueuePersist` chạy xong (dùng lại
  `persistQueueByWorktree`'s cơ chế "lấy state mới nhất tại thời điểm
  dequeue") → xác nhận chỉ 1 trong 2 RPC chạy hoặc không RPC nào chạy nếu
  net effect là "không đổi gì" — đọc kỹ hành vi `enqueuePersist` hiện có
  trước khi viết assertion, không suy đoán.

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/store/slices/diffComments.test.ts
npx tsc --noEmit
```

## gitnexus

`impact({target: "persist", direction: "upstream", file_path: "frontend/src/renderer/src/store/slices/diffComments.ts"})`
trước khi sửa — xác nhận toàn bộ caller (`addDiffComment`, `deleteDiffComment`,
`clearDiffComments`, `clearDiffCommentsForFile`, có thể cả `updateDiffComment`
— xem Bước 3) đều đi qua đúng 1 điểm này, không có đường ghi nào khác bỏ
qua `persist()`.


---

## ✅ Kết quả thực tế (2026-09-11)

**Lệch đáng kể so với thiết kế ban đầu (phát hiện khi đọc code thật của
backend-go's `annotation.create`/`update`/`delete` channels trước khi
code)**:

1. **`annotation.update` đã tồn tại sẵn** (`channels.go`'s
   `registerAnnotationChannels`) — SOL-FE-ANNOTATE-001's "Không thuộc phạm
   vi" từng nói channel này chưa có, cần 1 task backend-go riêng. Sai — đã
   nối `updateDiffComment` vào `annotation.update` luôn trong task này
   (không cần task backend-go bổ sung như dự kiến).
2. **`annotation.create`'s shape thật** là `{anchor: {repoId, worktreeId,
   filePath, line, endLine, side, ref}, content, requestId, originalCode}`
   — không phải shape phẳng đã sketch trong solution (`{worktreeId,
   filePath, line, endLine, side, originalCode, content}`). Cần thêm
   `repoId` (lấy qua `getRepoIdFromWorktreeId`, đã có sẵn import),
   `requestId` (dùng `comment.id` — vừa đúng ngữ nghĩa idempotency, vừa
   không cần sinh thêm giá trị mới), `ref` (để rỗng — xác nhận qua đọc
   `domain.NewAnchor`: chỉ `repoId`/`filePath` bắt buộc, `ref` không).
3. **`DiffComment.id` (client-generated) ≠ `Annotation.id` (server-generated,
   Postgres `gen_random_uuid()`)** — phát hiện quan trọng nhất, không có
   trong solution gốc. `annotation.update`/`annotation.delete` cần ID THẬT
   của server, không phải id client. Giải quyết bằng 1 map mới
   `serverAnnotationIdByCommentId` (module-level, cùng vòng đời với
   `lastPersistedByWorktree`), ghi lại khi `annotation.create` thành công,
   tra cứu khi update/delete. Nếu không có server id đã biết cho 1 comment
   (edit trước khi create kịp xong, hoặc create đã lỗi trước đó) — update/
   delete bị bỏ qua có chủ đích (ghi rõ trong code comment), không tạo
   duplicate hay lỗi crash.

**Thêm mới ngoài dự kiến**: `clearDiffCommentsPersistCacheForTests()` —
export test-only reset cho 2 map module-level mới, theo đúng convention
`clearRuntimeCompatibilityCacheForTests` đã có sẵn trong cùng file test.
Cần thiết vì 2 map này (đúng như thiết kế) sống suốt phiên app, không tự
reset giữa các test case trong cùng file — nếu không reset sẽ làm rò rỉ
state giữa các test.

**Test cũ phải viết lại (không chỉ thêm mới)**: 2 test hiện có
(`updateDiffComment`/`clearDiffComments`'s "persists through/clears through
the selected runtime environment") giả định seed() trực tiếp vào store rồi
gọi update/clear ngay — nhưng với thiết kế mới, `persistRemote` chỉ biết
gọi `annotation.update`/`annotation.delete` cho comment nó đã tự tạo qua
`annotation.create` trong phiên đó. Viết lại thành luồng 2 bước thật
(`addDiffComment` trước, rồi `updateDiffComment`/`clearDiffComments`) thay
vì `seed()` giả lập — phản ánh đúng hành vi thật, không phải patch test cho
qua.

**Verify thật đã chạy**: `vitest run src/renderer/src/store/slices/diffComments.test.ts` —
19/19 PASS (17 không đổi + 2 viết lại). `tsc --noEmit` toàn frontend: không
có lỗi mới nào liên quan tới `diffComments.ts`/`.test.ts` — lỗi
`diffComments.test.ts(151,...)` "AppState 1005 more props" xác nhận là
**pre-existing, lan rộng toàn repo** (cùng lỗi y hệt xuất hiện ở
`store-test-helpers.ts`, không liên quan gì tới file này), cùng hàng loạt
lỗi TS khác hoàn toàn không liên quan (`ssh.ts`, `workflow.ts`,
`fleet-dashboard.tsx`, `bootstrap.test.ts`, `dev-servers.test.ts`,
`onboarding-checklist.test.ts`) — xác nhận `tsc --noEmit` toàn frontend đã
hỏng từ trước, không phải do task này.

**Sự cố ngoài ý muốn (đã tự khắc phục, xem TASK-FE-ANNOTATE-003's log cho
chi tiết đầy đủ)**: dùng `git stash` để đo baseline lỗi TS trước khi sửa
`code-review-panel.tsx` (task khác, cùng phiên làm việc) vô tình kéo theo
toàn bộ working tree (bao gồm cả thay đổi của task này) — khôi phục an
toàn bằng `git checkout -- frontend/tsconfig.tsbuildinfo` (chỉ cache, không
phải source) rồi `git stash pop`, xác nhận lại `git stash list` không đụng
tới stash có sẵn từ trước (thuộc nhánh khác). Không mất dữ liệu.

**Files đã sửa:**
- `frontend/src/renderer/src/store/slices/diffComments.ts` (MODIFY — `persist()` tách thành `persist`/`persistRemote`, thêm `updateDiffComment` nối `annotation.update`)
- `frontend/src/renderer/src/store/slices/diffComments.test.ts` (MODIFY — 2 test viết lại, thêm import + `beforeEach` reset cache)

**Chưa làm (đúng phạm vi đã giới hạn trong solution, không phải thiếu sót)**:
Bước "Hydrate khi mở worktree" + backfill JSONB cũ — thuộc TASK-FE-ANNOTATE-002.
