# BUG-FE-BIGFILE-007 — `PullRequestPage.tsx` (7,372 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-007](./solutions/SOLUTION-FE-BIGFILE-007-pullrequestpage.md) (trỏ tới `SOLUTION-FE-BIGFILE-005`, xử lý chung với `GitHubItemDialog.tsx`)
**Module:** `frontend/src/renderer/src/components/PullRequestPage.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

7,372 dòng. 30 `useState` + 13 `useEffect`. Comment dòng 1 xác nhận trực tiếp
đây là file **cố ý duplicate** từ `GitHubItemDialog.tsx` (BUG-FE-BIGFILE-005):

```
/* eslint-disable max-lines -- Why: duplicated from GitHubItemDialog so the
dedicated PR full-page surface can evolve its Primer-styled header without
destabilizing the issue dialog; planned to refactor shared parts out later. */
```

Cùng `type ItemDialogTab`, cùng `type ...ProjectOrigin` (tên khác:
`PullRequestPageProjectOrigin`), cùng hàm
`invalidateWorkItemDetailsCacheForKey` — trùng gần như 1:1 với
`GitHubItemDialog.tsx`.

## Hậu quả

Xem chi tiết tại `BUG-FE-BIGFILE-005` (file song sinh) — 2 file này nên được
xử lý CÙNG NHAU, không tách riêng lẻ, vì phần lớn giá trị của việc tách nằm ở
việc gỡ bỏ trùng lặp giữa 2 file trước khi tách nội bộ từng file.

## Bằng chứng

```
wc -l PullRequestPage.tsx                              → 7372
grep -n "^export type ItemDialogTab\|invalidateWorkItemDetailsCacheForKey" \
  PullRequestPage.tsx GitHubItemDialog.tsx               → trùng tên ở cả 2 file
head -1 PullRequestPage.tsx                              → xác nhận "duplicated from GitHubItemDialog"
```

## Đề xuất fix

Xem `BUG-FE-BIGFILE-005` — thực hiện bước "trích phần dùng chung" ở đó trước,
áp dụng đồng thời cho cả 2 file. Sau bước đó, phần còn lại riêng của trang PR
(Primer-styled header theo đúng comment gốc) là phần cần tách tiếp nếu vẫn
còn lớn.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File song sinh (đọc trước): `BUG-FE-BIGFILE-005` (`GitHubItemDialog.tsx`)
