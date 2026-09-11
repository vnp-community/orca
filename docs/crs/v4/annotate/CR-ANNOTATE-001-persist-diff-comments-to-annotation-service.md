# CR-ANNOTATE-001 — Persist review-buffer diff comments to `annotation-service`; fix/retire the broken `annotation-panel.tsx`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-ANNOTATE-001 |
| **Tên** | Nối review-buffer (`useDiffCommentDecorator`/`DiffCommentCard`, hoàn toàn client-local hôm nay) tới `annotation-service` thật; sửa hoặc xoá `annotation-panel.tsx` (gọi API sai shape, không hoạt động) |
| **Loại** | Architecture correction + Bug fix |
| **Priority** | 🟡 P2 — *(hạ từ 🔴 P1 ban đầu, xem §1 "Đính chính lần 2" — không phải bug mất dữ liệu như đánh giá đầu tiên)* |
| **Effort** | Large (~1–2 tuần: store wiring 2 chiều, optimistic UI, backfill từ `Worktree.metadata` JSONB sang `annotation-service`, xử lý `annotation-panel.tsx`) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-11 |
| **Trạng thái** | ✅ DONE (2026-09-11) — xem tasks/ + solutions/ ở cả frontend và backend-go |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "tạo CR để thực thi F08 Annotate AI Diffs" |
| **Tác động Features** | F08 (Annotate AI Diffs) |
| **Phụ thuộc** | Không — `annotation-service`'s CRUD + `side`/`end_line`/`original_code`/`sent_to_agent`/`worktree_id` đã có sẵn thật (CR-02 series, `specs/backend-go/bugs/logic-v1/`, 12/12 task DONE) |

---

## Bối cảnh & Vấn đề

### 0. Đính chính lại khung audit trước (`docs/roadmap/feature-completion-matrix.md`)

Dòng F08 của ma trận viết: *"AG chỉ có shared format type, chưa có logic thực
thi phía relay"* — ngụ ý gap nằm ở `agent/` (Dev Server Agent). Khảo sát trực
tiếp code xác nhận **điều này sai**:

- `frontend/src/shared/diff-comments-format.ts`'s `formatDiffComment`/
  `formatDiffComments` **đang được gọi thật** từ
  `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx:4,35,41`
  — không phải "type chết".
- Việc "thực thi phía relay" — tiêm text vào PTY của agent — **đã có sẵn,
  đang chạy thật**: `frontend/src/renderer/src/lib/active-agent-note-send.ts`'s
  `sendNotesToActiveAgentSession` gọi `terminal.send` (dòng 137-232) với cơ
  chế guarded bracketed-paste + đợi agent idle (`terminal.wait`) + Enter —
  đây CHÍNH LÀ luồng PTY injection mà `agent/`'s `pty-handler.ts` (F02
  Terminal Splits) đã phục vụ từ trước, dùng lại nguyên vẹn, không thiếu gì.
  Không có gap nào ở `agent/` cho F08.

→ **Gap thật nằm ở một tầng khác hẳn**: 2 hệ thống lưu trữ comment độc lập,
không đồng bộ với nhau, đang tồn tại song song trong `frontend/`.

### 1. Hệ thống đang dùng thật — CÓ persist, nhưng không qua `annotation-service`

`SourceControl.tsx`'s panel "Notes" (dòng ~5000-5090), `useDiffCommentDecorator.tsx`
(733 dòng, dùng chung bởi `DiffViewer.tsx`, `MonacoEditor.tsx`, `DiffSectionItem.tsx`,
`GitHubItemDialog.tsx`, `PullRequestPage.tsx`), và `DiffCommentCard.tsx` tạo
thành review-buffer THẬT mà người dùng tương tác hôm nay — UX hoàn chỉnh:
click dòng diff → thêm comment, badge đếm số lượng, dropdown "Send to Agent"
chọn 1 trong các agent session đang chạy trong worktree
(`ReviewNotesSendMenuContent.tsx`), hoặc launch agent mới với prompt đã
điền sẵn. State thật nằm trong `frontend/src/renderer/src/store/slices/diffComments.ts`
(1 mảng `DiffComment[]` gắn vào từng `Worktree` trong `worktreesByRepo`).

**Đính chính lần 2 (quan trọng)**: đánh giá ban đầu của CR này cho rằng
review buffer "hoàn toàn client-local, mất khi reload" — **sai, đã tự sửa
lại sau khi đọc tiếp `diffComments.ts`'s `persist()` (dòng 98-117) và
backend-go's `channels_worktree.go`**. Comment **CÓ được persist thật**:

- Desktop: `window.api.worktrees.updateMeta(...)` — ghi xuống `orca-data.json`
  cục bộ.
- Web/multi-user: `callRuntimeRpc(target, 'worktree.set', {worktree, diffComments}, ...)`
  — và `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:405-494`
  xác nhận `diffComments` (không khớp `worktree`/`active`/`parentWorktree`/
  `noParent`) được coi là 1 field của `WorktreeMeta` patch, gọi
  `projectClient.UpdateWorktreeMeta` → merge vào cột **`metadata` JSONB thật**
  của bảng worktree (`project-service/migrations/0028_worktree_metadata.up.sql`)
  — **durable trong Postgres**, không mất khi reload.

**Vậy gap thật là gì?** Không phải mất dữ liệu — mà là **sai tầng lưu trữ**:

- Comment sống trong 1 blob JSONB tuỳ ý (`WorktreeMeta.metadata`), không phải
  hàng có cấu trúc trong `annotations` table — nên **không query được** theo
  `file_path`/`line`/`sent_to_agent` độc lập với việc load nguyên object
  worktree, và **`annotation.list`/`annotation.sendToAgent` (CR-ANNOTATE-002)
  không đọc được gì từ đây** — 2 hệ thống hoàn toàn tách biệt dữ liệu, dù cả
  2 đều "hoạt động" riêng lẻ.
- **Không có OPA author-only enforcement** khi sửa/xoá comment qua đường này
  — `worktree.set`/`UpdateWorktreeMeta` là 1 patch chung cho toàn bộ
  metadata, không có khái niệm "chỉ tác giả comment mới được sửa/xoá comment
  đó" mà `annotation-service`'s `UpdateAnnotation`/`DeleteAnnotation` đã có
  sẵn (`specs/backend-go/tdd/services/annotation-service.md` §9). Trong chế
  độ multi-user, bất kỳ ai có quyền ghi metadata worktree đó đều sửa/xoá được
  comment của người khác, không có guard nào.
- `annotation-service`'s toàn bộ schema `side`/`end_line`/`original_code`/
  `worktree_id`/`sent_to_agent`/`sent_at` (SOL-CR-02, đã ship thật, đã
  verify build/test sạch) **vẫn không có ý nghĩa gì trong luồng người dùng
  thật hôm nay** — không phải vì dữ liệu bị mất, mà vì nó là 1 con đường
  song song, chưa từng được nối vào.

### 2. `annotation-panel.tsx` — component riêng, gọi API thật nhưng sai shape, không hoạt động

`frontend/src/renderer/src/components/code-review/annotation-panel.tsx`
**cũng được render** trong cùng cây component (`DiffViewer.tsx`,
`DiffSectionBody.tsx`, `MonacoEditor.tsx`, `RichMarkdownAnnotationOverlay.tsx`,
`code-review-panel.tsx` đều import `AnnotationPanel`) — không phải dead code
theo nghĩa "0 caller". Nó gọi:

```ts
callRuntimeRpc<Annotation[]>(target, 'annotation.list', {
  projectId: project.id, reviewId, filePath, lineNumber
})
callRuntimeRpc<Annotation>(target, 'annotation.create', {
  projectId: project.id, reviewId, filePath, lineNumber, content: newComment.trim(), traceId: span.id
})
```

Nhưng backend-go's proto thật (`backend-go/proto/orca/annotation/v1/annotation.proto`)
không có field nào tên `projectId`/`reviewId` — `Anchor` dùng `repo_id`,
`file_path`, `line`, `ref`, `side`, `end_line`; `Annotation` không có
`lineNumber` top-level (nó nằm trong `Anchor`). Grep xác nhận thêm:
`desktop/src` **không có bất kỳ IPC handler nào** cho method name
`'annotation.create'`/`'annotation.list'` (0 kết quả ngoài chính file này +
test của nó) — nghĩa là:

- **Desktop mode**: `callRuntimeRpc` gọi 1 method không tồn tại → reject/lỗi
  ngay lập tức. Panel này chưa từng hoạt động ở desktop.
- **Web mode** (kết nối backend-go thật): request tới đúng method
  `annotation.create`/`annotation.list` (backend-go CÓ implement 2 method
  này) nhưng với field sai tên — `repo_id`/`file_path` phía Go sẽ nhận giá
  trị rỗng, và `domain.NewAnchor`/`NewAnnotation`'s validation (đã xác nhận
  thật trong BUG-CR-02) sẽ reject request rỗng đó. Panel này cũng không
  hoạt động ở web mode.

**Kết luận: `annotation-panel.tsx` là 1 component hỏng, không hoạt động ở
bất kỳ mode nào hôm nay** — không phải do backend-go thiếu gì (backend-go
đúng), mà do frontend gọi sai hợp đồng. Đây là bug độc lập, không phải do
CR này gây ra, nhưng nằm đúng trong phạm vi cần giải quyết vì nó là component
DUY NHẤT hiện tại gọi tới `annotation-service` thật cho luồng diff-review.

## Giải pháp đề xuất

### A. Coi `annotation-service` là nguồn sự thật; review buffer Zustand trở thành cache đồng bộ 2 chiều

Không thiết kế lại UX (đã tốt) — chỉ thêm tầng persistence phía dưới:

```
Click dòng diff → thêm comment (UX không đổi)
        │
        ▼
Zustand action mới: annotation.create qua wscompat thật
  { repo_id, worktree_id, file_path, line, end_line?, side: SIDE_NEW,
    original_code, ref, content }
        │ optimistic: thêm vào state Zustand ngay, rollback nếu request fail
        ▼
Response { id, ... } → cập nhật DiffComment.id = id thật (thay id tạm)
```

- **Load buffer khi mở worktree**: gọi `annotation.list` với
  `worktree_id`/`sent_to_agent=false` (đã có field lọc này, SOL-CR-02) thay
  vì chỉ đọc từ local storage/session — Zustand state hydrate từ server khi
  mở lại app.
- **`side`**: giữ nguyên `'modified'`/`SIDE_NEW` cho v1 — đúng như comment có
  sẵn ở `frontend/src/shared/types.ts:778-779` ("Reserved for future
  'comments on the original side' — always 'modified' in v1"). CR này
  KHÔNG mở rộng sang comment ở old-side, chỉ nối dây cho case đã có.
- **`end_line`**: map trực tiếp từ `DiffComment.startLine` khi khác
  `lineNumber` (range đã có sẵn client-side, backend-go's `Anchor.end_line`
  đã có sẵn từ SOL-CR-02 — chỉ cần truyền đúng field, không cần thêm logic
  mới ở 1 trong 2 phía).
- **`original_code`**: lấy dòng code hiện tại tại thời điểm comment (đã có
  sẵn trong tầm với — `DiffViewer` render diff nên có content dòng đó ngay
  tại chỗ tạo comment), gửi kèm khi `annotation.create`.
- **Xoá comment**: nối `annotation.delete` — BR-CR-08's "confirm trước khi
  xoá comment đã gửi" đã có sẵn ở backend (`DeleteAnnotationInput.Confirmed`,
  TASK-CR-02-05 DONE) — chỉ cần frontend truyền `confirmed: true` sau khi
  người dùng xác nhận dialog, đúng contract đã thiết kế sẵn.
- **`sentAt`**: sau khi gửi thành công qua `sendNotesToActiveAgentSession`
  (giữ nguyên cơ chế delivery hiện tại — xem CR-ANNOTATE-002 cho việc có
  route qua `annotation.sendToAgent` hay không), gọi thêm
  `annotation.markSent` (đã có sẵn, TASK-CR-02-06/07 DONE) để đồng bộ
  `sent_to_agent`/`sent_at` phía server, không chỉ set local `sentAt`.

### A.1. Backfill comment đã tồn tại trong `WorktreeMeta.metadata` JSONB

Vì đường cũ (`worktree.set` → JSONB) đã chạy thật và đang chứa dữ liệu người
dùng thật, chuyển sang `annotation-service` cần 1 bước backfill, không chỉ
đổi code cho request mới: đọc `diffComments` hiện có trong
`WorktreeMeta.metadata` (qua `project-service`'s `GetWorktree`) 1 lần khi
mở worktree lần đầu sau khi CR này deploy, gọi `annotation.create` cho từng
comment chưa có `annotation_id` tương ứng, rồi ngừng đọc/ghi field
`diffComments` trong JSONB đó (giữ lại field cũ, không xoá, để tránh mất dữ
liệu nếu backfill lỗi giữa chừng — dọn dẹp field cũ là việc riêng, ngoài
phạm vi CR này).

### B. `annotation-panel.tsx` — sửa lại shape, hoặc xoá nếu review-buffer đã đủ

Component này và review-buffer chính (`DiffCommentCard`) có UX **trùng lặp
đáng kể** (cả hai đều là "click dòng → hộp comment"). 2 hướng, cần chốt khi
implement (không tự quyết trong CR này vì ảnh hưởng UX quyết định sản phẩm):

1. **Xoá `annotation-panel.tsx`**, giữ `DiffCommentCard`/`useDiffCommentDecorator`
   làm UI duy nhất — đơn giản hơn, tránh 2 UI cho cùng 1 việc. Cần xác nhận
   qua GitNexus `context()` rằng không có luồng nào khác phụ thuộc riêng vào
   `AnnotationPanelProps`'s `reviewId` (một khái niệm review-session mà
   `DiffComment` không có) trước khi xoá.
2. **Sửa shape đúng** (`projectId`→bỏ, `reviewId`→`worktree_id` hoặc tương
   đương, `lineNumber`→`line` trong `Anchor`) nếu `reviewId`/luồng review
   riêng (khác worktree-scoped) là 1 khái niệm sản phẩm thật sự cần giữ
   riêng biệt với review-buffer chính.

Nghiêng về hướng 1 (xoá) vì `reviewId` là optional (`AnnotationPanelProps.reviewId?:
string`) và không thấy nơi nào gán giá trị thật cho nó trong lần khảo sát
này — nhưng **cần xác nhận lại bằng GitNexus trước khi xoá**, không suy
đoán.

## Changes Required

| File | Thay đổi |
|------|---------|
| `frontend/src/renderer/src/store` (slice quản lý `diffComments`, tên file cụ thể xác nhận lại lúc implement) | Thêm action `createDiffCommentRemote`/`loadDiffCommentsFromServer`/`deleteDiffCommentRemote` gọi `annotation.create`/`list`/`delete`/`markSent` qua `callRuntimeRpc`; giữ optimistic update, rollback khi lỗi |
| `frontend/src/renderer/src/components/diff-comments/DiffCommentCard.tsx` | `handleSubmit`/xoá comment gọi action mới thay vì chỉ set Zustand state trực tiếp |
| `frontend/src/renderer/src/components/right-sidebar/use-source-control-diff-comments-panel.ts` | Hydrate buffer từ `annotation.list` khi mount/đổi worktree, không chỉ đọc state cục bộ |
| `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` | Sau khi gửi thành công (`onDelivered`), gọi thêm `annotation.markSent` song song với `clearDeliveredDiffComments` hiện có |
| `frontend/src/renderer/src/components/code-review/annotation-panel.tsx` | Sửa shape param cho khớp proto thật, HOẶC xoá file — quyết định theo mục B, xác nhận bằng GitNexus trước |
| `frontend/src/renderer/src/components/code-review/annotation-panel.test.tsx` | Cập nhật/xoá theo quyết định trên |

## Không thuộc phạm vi CR này

- Mở rộng `side` sang comment ở dòng old (removed) — giữ nguyên giới hạn v1
  đã ghi rõ trong code (`types.ts:778`).
- Route delivery qua `annotation.sendToAgent` để có code-context tự động —
  xem CR-ANNOTATE-002 (phụ thuộc CR này xong trước).
- Sửa `desktop/`'s legacy annotation code (nếu có, ngoài phạm vi khảo sát
  frontend/backend-go của bộ CR này).

## Tiêu chí chấp nhận

- [ ] Thêm comment trên diff → gọi `annotation.create` thật, thấy row mới
      trong Postgres của `annotation-service` (không chỉ Zustand state)
- [ ] Reload app (hoặc mở project ở 1 client thứ 2) → review buffer của
      worktree đó load lại đúng từ `annotation.list`, không rỗng
- [ ] Xoá comment đã gửi (`sent_to_agent=true`) → yêu cầu xác nhận
      (BR-CR-08), gọi `annotation.delete` với `confirmed: true`
- [ ] Sau khi "Send to Agent" thành công → `annotation.markSent` được gọi,
      `sent_to_agent=true` phản ánh đúng ở lần `annotation.list` tiếp theo
- [ ] `annotation-panel.tsx` hoặc hoạt động đúng (shape khớp proto) hoặc đã
      bị xoá khỏi cây component — không còn trạng thái "gọi API luôn lỗi"
      như hiện tại
- [ ] `tsc --noEmit`/`vitest run` sạch cho toàn bộ file bị sửa

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Ghi chú |
|---|---|---|---|
| `DiffComment` (`frontend/src/shared/types.ts:759`) | upstream | Cần chạy `impact()` thật trước khi sửa — dùng ở `useDiffCommentDecorator`, `DiffCommentCard`, `DiffNotesSendMenu`, `SourceControl.tsx`, mobile diff review (`MobileDiffReviewFileState` liên quan) | Thêm field optional (server id, sync status) không phá interface hiện có nếu làm đúng additive |
| `annotation-panel.tsx` / `AnnotationPanel` | upstream | Xác nhận danh sách caller thật (`DiffViewer.tsx`, `DiffSectionBody.tsx`, `MonacoEditor.tsx`, `RichMarkdownAnnotationOverlay.tsx`, `code-review-panel.tsx`) trước khi quyết định sửa hay xoá | Không tin riêng kết quả grep — chạy `context()` để chắc chắn không sót caller nào |
| `registerAnnotationChannels` (backend-go, `api-gateway/internal/adapter/wscompat/channels.go`) | downstream (tham chiếu, không sửa) | Xác nhận signature `annotation.create`/`list`/`delete`/`markSent` thật trước khi viết code frontend gọi tới, không suy đoán từ proto source một mình | Không sửa backend-go trong CR này — chỉ đọc để khớp đúng contract |

`detect_changes({scope: "compare", base_ref: "main"})` bắt buộc trước khi
commit, đặc biệt vì CR này có thể xoá file (`annotation-panel.tsx`).

## Liên quan

- [F08-annotate-ai-diffs.md](../../features/F08-annotate-ai-diffs.md)
- [docs/crs/v4/annotate/README.md](./README.md)
- `specs/backend-go/bugs/logic-v1/BUG-CR-02-annotate-diff-partial.md`, `solutions/SOL-CR-02-annotation-side-range-sent-state.md` (schema `annotation-service` mà CR này nối tới, đã DONE)
- `frontend/src/shared/types.ts:759-780` (`DiffComment`)
- `frontend/src/shared/diff-comments-format.ts` (format thật đang dùng)
- `frontend/src/renderer/src/lib/active-agent-note-send.ts` (delivery thật đang dùng)
- [CR-ANNOTATE-002](./CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md) (phụ thuộc CR này)
