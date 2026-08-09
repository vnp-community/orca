# BUG-FE-HLD-007 — `ConflictPanel` (F39 Remote Git UI) được tài liệu hoá nhưng chưa implement

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `frontend/src/renderer/src/components/workspace/git/`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/03-hld-doc-drift.md` §4)

---

## Mô tả

`docs/hld/web-server-architecture.md` §10.7 ("Remote Git UI (F39) — GitPanel Chi tiết") liệt kê `ConflictPanel` ("Conflict files + AI resolve") như một sub-component của `GitPanel`.

Grep toàn repo cho `ConflictPanel` — **0 kết quả**. Các sub-component khác doc nhắc đều tồn tại (`DiffViewer.tsx`, `CommitForm.tsx`, `BranchManager.tsx`, `PullRequestForm.tsx`, và `GitLog` — tồn tại dưới tên `GitHistory.tsx`), chỉ riêng `ConflictPanel` là hoàn toàn vắng mặt, không phải bị đổi tên.

## Hậu quả

Khác với các bug khác trong series này (nơi doc sai/lệch so với code), đây là trường hợp **code chưa bắt kịp doc** — F39 Remote Git UI được liệt kê trạng thái 🚧 (Phát triển) trong `docs/features/README.md`, nên việc thiếu 1 sub-component không hẳn là "bug" theo nghĩa cổ điển, nhưng đáng ghi nhận vì:
- Người dùng cố merge/pull qua GitPanel gặp conflict sẽ không có UI xử lý trong app — phải thoát ra dùng git CLI/terminal, phá vỡ trải nghiệm "unified workspace" mà F38/F39 hướng tới.
- Đây là gap chức năng thật (không chỉ doc lỗi thời) — cần xác nhận có còn trong scope hay đã chủ động bỏ.

## Bằng chứng

```
docs/hld/web-server-architecture.md:854+ (§10.7) → liệt kê ConflictPanel trong bảng sub-component
grep -r "ConflictPanel" toàn repo               → 0 kết quả
components/workspace/git/                        → có StagingArea.tsx, PullRequestList.tsx (không có trong doc — bổ sung ngược)
```

## Đề xuất fix

1. Xác nhận với product owner: `ConflictPanel` còn trong roadmap F39 hay đã bị loại — nếu bị loại, xoá khỏi doc §10.7; nếu còn, tạo task implement.
2. Nếu implement: tối thiểu cần hiển thị danh sách file conflict (`git status` parse), cho phép mở từng file trong editor với marker `<<<<<<<`/`=======`/`>>>>>>>`, và (theo mô tả "AI resolve") 1 action gọi agent hỗ trợ resolve — kiến trúc tương tự `DiffViewer.tsx` đã có sẵn để tham khảo.

## Tham khảo

- Audit: `audit/frontend/03-hld-doc-drift.md` §4
- Doc gốc: `docs/hld/web-server-architecture.md` §10.7
- Feature liên quan: `docs/features/F39-remote-git-ui.md`
