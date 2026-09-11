# frontend Tasks — Annotate AI Diffs (F08, v4)

**Solutions:** [../solutions/](../solutions/README.md)

## Solution → Task ID map

| Solution | Task IDs | Notes |
|---|---|---|
| [SOL-FE-ANNOTATE-001](../solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) | [TASK-FE-ANNOTATE-001](./TASK-FE-ANNOTATE-001-persist-remote-create-delete.md) ✅, [TASK-FE-ANNOTATE-002](./TASK-FE-ANNOTATE-002-hydrate-and-backfill.md) ✅, [TASK-FE-ANNOTATE-003](./TASK-FE-ANNOTATE-003-fix-or-remove-annotation-panel.md) ✅ | 003 độc lập với 001/002 (file khác, vấn đề khác — có thể làm song song) |
| [SOL-FE-ANNOTATE-002](../solutions/SOL-FE-ANNOTATE-002-call-composed-prompt-before-delivery.md) | [TASK-FE-ANNOTATE-004](./TASK-FE-ANNOTATE-004-composed-prompt-hook.md) ✅, [TASK-FE-ANNOTATE-005](./TASK-FE-ANNOTATE-005-wire-mark-sent.md) ✅ | Cả 2 phụ thuộc 001/002 xong trước (cần dữ liệu đã persist thật ở `annotation-service` để có gì compose) và backend-go's `annotation.composeReviewPrompt` (TASK-BE-ANNOTATE-002) tồn tại |

## Thứ tự thực thi

```
TASK-FE-ANNOTATE-001 (persist() remote branch: create/delete)
        │
        ▼
TASK-FE-ANNOTATE-002 (hydrate qua annotation.list + backfill JSONB cũ)
        │                                    TASK-FE-ANNOTATE-003
        │                                    (fix/xoá annotation-panel.tsx)
        │                                    — độc lập, có thể làm song song
        │                                    với 001/002
        ▼
   [dữ liệu review buffer giờ nằm thật trong annotation-service]
        │
        │  (chờ backend-go's TASK-BE-ANNOTATE-001/002 xong)
        ▼
TASK-FE-ANNOTATE-004 (useComposedAllNotesPrompt hook)
        │
        ▼
TASK-FE-ANNOTATE-005 (wire annotation.markSent vào onDelivered)
```

TASK-FE-ANNOTATE-003 không phụ thuộc kỹ thuật vào 001/002/004/005 — có thể
làm bất kỳ lúc nào, xếp song song để không kéo dài timeline tổng.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore`/`context()` trước khi sửa
  symbol** — đặc biệt TASK-FE-ANNOTATE-002 (điểm hook chính xác trong
  `sync-runtime-graph.ts` cần xác nhận bằng công cụ, solution chỉ mô tả ý
  tưởng) và TASK-FE-ANNOTATE-003 (quyết định sửa/xoá `annotation-panel.tsx`
  phụ thuộc hoàn toàn vào kết quả `context()`, không phải quyết định đã
  chốt sẵn).
- **Desktop mode (`target.kind === 'local'`) không được đổi hành vi** ở bất
  kỳ task nào trong nhóm 001/002/004/005 — không có cầu nối tới
  `annotation-service` ở đó (0 IPC handler, xác nhận trong
  SOL-FE-ANNOTATE-001 §0). Mọi thay đổi chỉ chạm nhánh `environment`
  (web/multi-user). Test case cho nhánh `local` là regression guard bắt
  buộc ở mọi task đụng tới `persist()`/`DiffNotesSendMenu.tsx`.
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê.
  Nếu phát hiện gap khác lúc làm (vd. `updateDiffComment` không có đường
  persist remote nào cả — xem TASK-FE-ANNOTATE-001 Bước 3), ghi nhận lại
  trong "Ghi chú thực thi" của task đó, không tự sửa ngoài phạm vi.
- **Test thật, không giả định pass** — chạy `vitest run`/`tsc --noEmit`
  thật, ghi kết quả cụ thể.


**Toàn bộ 5 task trong nhóm frontend này đã ✅ DONE (2026-09-11).**
