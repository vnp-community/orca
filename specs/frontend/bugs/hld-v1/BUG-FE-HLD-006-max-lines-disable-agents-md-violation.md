# BUG-FE-HLD-006 — 240 file disable rule `max-lines`, vi phạm trực tiếp `AGENTS.md`

**Mức độ:** 🔴 Critical (policy)
**Status:** 🔴 Open
**Module:** `frontend/src/**` (240 file, xem danh sách trong audit)
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/04-code-health-and-standards.md` §1)

---

## Mô tả

`AGENTS.md:15` quy định nguyên văn, không có ngoại lệ:

> "Never add a `max-lines` disable (`eslint-disable max-lines`, `oxlint-disable max-lines`, or line-specific variants), and never add a per-file `max-lines` bump in `mobile/.oxlintrc.json`."

Grep toàn `frontend/src`:

| Loại disable | Số lượng |
|---|---|
| Tổng số disable comment (`eslint-disable*` + `oxlint-disable*`) | 438 |
| Trong đó `max-lines` | **240** (217 eslint + 22 oxlint + 1 kết hợp) |

240/3,865 file (~6.2%) chứa đúng loại disable bị cấm tuyệt đối. Ví dụ: `App.tsx:1`, `Terminal.tsx:1` (không có comment giải thích); `LinearIssueWorkspace.tsx`, `GitHubItemDialog.tsx`, `GitLabItemDialog.tsx`, `JiraIssueWorkspace.tsx`, `LinearItemDrawer.tsx`, `PullRequestPage.tsx`, `UpdateCard.tsx`, `NewWorkspaceComposerCard.tsx`, `web-runtime-client.ts`, `web-preload-api.ts` (có comment `-- Why:` giải thích nhưng rule không có ngoại lệ "có lý do thì được").

## Hậu quả

Đây là vi phạm chính sách rõ ràng nhất, dễ trích dẫn nhất trong toàn bộ audit — chỉ cần đối chiếu 1 câu quy định với kết quả grep. Nếu không xử lý, quy định trong `AGENTS.md` mất hiệu lực thực tế (240 chỗ đã "lách luật" thành công).

**Cần xác nhận trước khi coi đây là 240 việc phải sửa ngay:** rất có thể quy định này được thêm **sau khi** phần lớn 240 file đã tồn tại — kiểm tra `config/max-lines-baseline.txt`/`config/scripts/check-max-lines-ratchet.mjs` (nếu còn tồn tại trong repo) xem có cơ chế ratchet "đóng băng, không cho tăng thêm" hay không. Nếu có, đây là nợ kỹ thuật lịch sử đã biết và đang được kiểm soát; nếu không có cơ chế nào đang chạy, số lượng vi phạm có nguy cơ tăng dần không kiểm soát.

## Bằng chứng

```
AGENTS.md:15                     → quy định "never" nguyên văn
grep eslint-disable/oxlint-disable max-lines frontend/src → 240 hit
App.tsx:1, Terminal.tsx:1        → disable không kèm giải thích
```

## Đề xuất fix

1. Xác nhận cơ chế ratchet hiện tại (nếu có) có đang chặn tăng thêm số file vi phạm.
2. Nếu không có, bổ sung ngay 1 CI check chặn PR mới thêm `max-lines` disable (rẻ, ngăn nợ tăng thêm trước khi tính đến việc dọn 240 file cũ).
3. Dọn dần 240 file cũ theo domain, ưu tiên file không có comment giải thích trước (khả năng cao là chưa từng được cân nhắc kỹ, chỉ tắt rule cho nhanh).

## Tham khảo

- Audit: `audit/frontend/04-code-health-and-standards.md` §1
- Quy định: `AGENTS.md:15`
