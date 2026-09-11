# frontend Solutions — Annotate AI Diffs (F08, v4)

**CRs:** [docs/crs/v4/annotate/](../../../../../docs/crs/v4/annotate/README.md)
**Backend-go counterpart:** [specs/backend-go/crs/v4/annotate/solutions/](../../../../backend-go/crs/v4/annotate/solutions/README.md)
**Agent counterpart:** [specs/agent/crs/v4/annotate/solutions/](../../../../agent/crs/v4/annotate/solutions/README.md) — zero scope, see there
**TDD tham chiếu:** [`03-runtime-client-layer.md`](../../../tdd/v5/03-runtime-client-layer.md) §2 (`RuntimeClientTarget`), [`02-state-management.md`](../../../tdd/v5/02-state-management.md) §2 (`diff-comments` slice registry entry)

## Đánh giá trạng thái hiện tại — trước khi thiết kế bất kỳ giải pháp nào (bắt buộc theo yêu cầu)

Đọc trực tiếp `frontend/src/renderer/src/store/slices/diffComments.ts`
(không tin theo mô tả ban đầu của CR-ANNOTATE-001) phát hiện 1 sự thật quan
trọng làm hẹp phạm vi sửa: review buffer **đã persist thật** (qua
`worktree.set` → `project-service`'s `WorktreeMeta.metadata` JSONB), không
phải "hoàn toàn mất khi reload" như đánh giá đầu tiên của CR đó — CR gốc đã
được tự đính chính lại (`CR-ANNOTATE-001` §1 "Đính chính lần 2") dựa trên
phát hiện này trước khi viết 2 solution dưới đây. Gap thật hẹp hơn: dữ liệu
không nằm trong `annotation-service` (không query được, không có OPA
author-guard), và `annotation-panel.tsx` gọi API sai shape (theo đúng thiết
kế TDD gốc đã lỗi thời, xem SOL-FE-ANNOTATE-001 §3).

Phát hiện quan trọng thứ 2: `desktop/src` **không có cầu nối** tới
`annotation.create`/`list`/`delete`/`markSent` (0 IPC handler) —
`annotation-service` chỉ khả dụng khi target runtime là `{kind:
'environment'}` (web/multi-user, qua backend-go), không bao giờ khả dụng ở
`{kind: 'local'}` (desktop, qua Electron IPC). Cả 2 solution dưới đây **chỉ
chạm vào nhánh `target.kind !== 'local'`** — desktop mode giữ nguyên 100%
hành vi hiện có, không có lợi ích/rủi ro gì từ việc đổi.

## Solutions

| Solution | CR | File chính | Status |
|---|---|---|---|
| [SOL-FE-ANNOTATE-001](./SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) | CR-ANNOTATE-001 | `store/slices/diffComments.ts` (nhánh remote của `persist()`), `code-review/annotation-panel.tsx` | 🔲 Designed — chưa implement |
| [SOL-FE-ANNOTATE-002](./SOL-FE-ANNOTATE-002-call-composed-prompt-before-delivery.md) | CR-ANNOTATE-002 | `editor/DiffNotesSendMenu.tsx` (+ 1 hook mới, nhỏ) | 🔲 Designed — chưa implement; phụ thuộc SOL-FE-ANNOTATE-001 xong (dữ liệu) |

## Thứ tự implement

```
SOL-FE-ANNOTATE-001 (persist() remote branch → annotation.create/delete/
                      markSent + hydrate qua annotation.list; sửa/xoá
                      annotation-panel.tsx)
                    │ (annotation.composeReviewPrompt cần đọc annotation đã
                    │  persist thật — nếu vẫn chỉ ở JSONB cũ, không có gì
                    │  để compose)
                    ▼
SOL-FE-ANNOTATE-002 (DiffNotesSendMenu.tsx gọi annotation.composeReviewPrompt
                      trước khi truyền prompt vào sendNotesToActiveAgentSession
                      — cơ chế delivery hiện có KHÔNG đổi)
```

## Nguyên tắc chung — "ít thay đổi code nhất, tận dụng code đã có"

Cả 2 solution đều **giữ nguyên** phần khó nhất đã viết đúng của mỗi file:

- `diffComments.ts`'s `mutateComments`/`enqueuePersist`/optimistic-rollback
  queue (dòng 98-208) — không đổi, chỉ đổi nội dung `persist()` ghi đi đâu.
- `active-agent-note-send.ts`'s guarded bracketed-paste + đợi idle + Enter
  riêng — không đổi 1 dòng, chỉ đổi nguồn chuỗi `prompt` truyền vào.
- `NotesSendMenu.tsx`/`ReviewNotesSendMenuContent.tsx` (dùng chung với
  markdown-notes flow) — không đổi, chỉ `DiffNotesSendMenu.tsx` (diff-specific)
  tính `prompt` khác đi trước khi truyền xuống.
