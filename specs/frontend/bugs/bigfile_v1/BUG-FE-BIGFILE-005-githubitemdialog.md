# BUG-FE-BIGFILE-005 — `GitHubItemDialog.tsx` (7,852 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-005](./solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md)
**Module:** `frontend/src/renderer/src/components/GitHubItemDialog.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

7,852 dòng. 32 `useState` + 9 `useEffect` — component chính quản lý rất nhiều
state cục bộ cho 1 dialog. Chỉ có 3 export top-level tách biệt được (`type
ItemDialogTab`, `type GitHubItemDialogProjectOrigin`,
`invalidateWorkItemDetailsCacheForKey`) — nghĩa là phần lớn logic (conversation
tab, checks tab, files tab — theo đúng comment giải thích ở dòng 1) nằm gói
gọn trong component chính, chưa được tách thành sub-component riêng như
`SourceControl.tsx` (BUG-FE-BIGFILE-004) đã làm một phần.

Comment dòng 1: "the GH item dialog keeps its header, conversation, files, and
checks tabs co-located so the read-only PR/Issue surface stays in one place
while this view evolves."

**Lưu ý quan trọng**: `PullRequestPage.tsx` (BUG-FE-BIGFILE-007, 7,372 dòng)
có `type ItemDialogTab` và `invalidateWorkItemDetailsCacheForKey` **trùng tên
hệt** với file này — commit message của `PullRequestPage.tsx` (xem bug đó)
xác nhận: "duplicated from GitHubItemDialog ... planned to refactor shared
parts out later." Đây là 2 file gần như song sinh, tổng cộng **15,224 dòng**
phần lớn trùng lặp logic.

## Hậu quả

- 2 file (`GitHubItemDialog.tsx` + `PullRequestPage.tsx`) cùng lúc tồn tại với
  logic trùng lặp cố ý (theo chính comment của tác giả) — bất kỳ bugfix nào
  cho 1 trong 2 (vd: cách hiển thị check status, assignee selector) có nguy cơ
  cần sửa **2 lần** ở 2 nơi khác nhau nếu không đồng bộ chủ động.
- File lớn + nhiều state (32) khiến việc thêm 1 tab mới hoặc 1 field mới rủi
  ro cao vì phải hiểu toàn bộ state machine của dialog trước khi sửa.

## Bằng chứng

```
wc -l GitHubItemDialog.tsx                             → 7852
wc -l PullRequestPage.tsx                               → 7372
grep -c "useState(" GitHubItemDialog.tsx                → 32
grep -n "type ItemDialogTab\|invalidateWorkItemDetailsCacheForKey" \
  GitHubItemDialog.tsx PullRequestPage.tsx               → trùng tên ở cả 2 file
```

## Đề xuất fix

1. **Ưu tiên cao hơn tách file thuần túy**: thực hiện đúng kế hoạch đã ghi
   trong comment của `PullRequestPage.tsx` — trích phần dùng chung (types,
   `invalidateWorkItemDetailsCacheForKey`, logic tab conversation/checks/files
   không phụ thuộc Primer-style riêng của PR page) ra 1 module chung, ví dụ
   `github-item-dialog-shared.ts(x)`. Việc này giải quyết CẢ 2 vấn đề file lớn
   lẫn trùng lặp logic cùng lúc — nên làm trước khi tách riêng từng file.
2. Sau khi tách phần dùng chung, mỗi file còn lại (phần UI đặc thù riêng của
   dialog và của trang PR) nhỏ hơn đáng kể, dễ đánh giá lại có cần tách tiếp
   theo tab (conversation/checks/files) hay không.
3. Theo comment gốc, 3 tab (conversation/checks/files) là ranh giới tách rõ
   ràng nhất nếu vẫn còn lớn sau bước 1 — tương tự cách `ChecksPanel.tsx`
   (#17, 3,919 dòng) và `checks-panel-content.tsx` (#29, 2,709 dòng) đã tách
   riêng phần "checks" ra khỏi các trang khác.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File song sinh: `BUG-FE-BIGFILE-007` (`PullRequestPage.tsx`)
- File liên quan cùng domain: `ChecksPanel.tsx`, `checks-panel-content.tsx`
