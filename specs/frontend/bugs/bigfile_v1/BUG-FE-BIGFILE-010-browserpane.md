# BUG-FE-BIGFILE-010 — `BrowserPane.tsx` (5,841 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-010](./solutions/SOLUTION-FE-BIGFILE-010-browserpane.md)
**Module:** `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

5,841 dòng. `/* eslint-disable max-lines */` không kèm giải thích. 12
`useState` + 64 `useEffect` — **mật độ `useEffect` cao nhất trong toàn bộ
nhóm Critical** (hơn 5 lần số `useState`), gợi ý phần lớn logic là đồng bộ
hoá với remote browser session (CDP), không phải local UI state.

File có **3 component top-level tách biệt rõ ràng**, không chỉ 1:

```
377   function PendingBrowserAnnotationCard({...})
783   export default function BrowserPane({...})       ← component chính (~118 dòng tới component kế)
901   function RemoteBrowserPagePane({...})              ← ~1,774 dòng
2675  function BrowserPagePane({...})                    ← ~3,166 dòng, tới cuối file
```

`BrowserPagePane` (dòng 2675 → 5841) chiếm hơn **một nửa file** (~54%) trong
1 component duy nhất.

Nhiều type định nghĩa riêng cho remote browser streaming ở đầu file (dòng
185–307): `BrowserTabPageState`, `BrowserDownloadState`, `GrabIntent`,
`BrowserOverlayAnchor`, `BrowserOverlayViewport`, `RemoteBrowserStreamToken`,
`RemoteBrowserStreamSubscription`, `RemoteBrowserOperationToken`,
`RemoteBrowserContextMenu`, `RemoteBrowserViewportSize`,
`RemoteBrowserImagePoint`, `PendingRemoteBrowserWheel` — 11 type, tất cả liên
quan `Remote*`, gợi ý nên tách cùng với `RemoteBrowserPagePane`.

## Hậu quả

- `BrowserPane` (component export mặc định, ~118 dòng) chỉ là **wrapper mỏng**
  quyết định render `RemoteBrowserPagePane` hay `BrowserPagePane` — nhưng cả
  2 component con lại nằm CHUNG file, khiến file "wrapper" trông như 1 file
  đơn giản nhưng thực chất mang theo ~5,700 dòng logic của 2 implementation
  hoàn toàn khác nhau (remote CDP-streamed browser vs local browser pane).
- 64 `useEffect` tổng cộng trên 3 component — muốn hiểu lifecycle của riêng
  `BrowserPagePane` phải tự lọc ra effect nào thuộc component nào trong 1
  file dài.

## Bằng chứng

```
wc -l BrowserPane.tsx                                  → 5841
grep -n "^function \|^export default function" ...     → 3 component (dòng 377, 783, 901, 2675)
grep -c "useEffect(" BrowserPane.tsx                    → 64
head -1 BrowserPane.tsx                                 → "/* eslint-disable max-lines */" (không giải thích)
```

## Đề xuất fix

1. **Tách theo đúng ranh giới component đã có sẵn** (rủi ro thấp nhất, không
   cần thiết kế lại):
   - `browser-pane-remote.tsx` — `RemoteBrowserPagePane` + 11 type
     `Remote*`/`Pending*` liên quan (dòng 185–307, 901–2674)
   - `browser-pane-local.tsx` — `BrowserPagePane` (dòng 2675–5841)
   - `browser-pane-annotation-card.tsx` — `PendingBrowserAnnotationCard`
     (dòng 377–782)
   - `BrowserPane.tsx` giữ lại component wrapper export mặc định (dòng
     783–900), import 3 file trên.
2. Sau bước 1, `BrowserPane.tsx` giảm từ 5,841 dòng xuống dưới 150 dòng —
   không cần thêm bước tách nào nữa cho file gốc.
3. Bổ sung `-- Why:` cho disable comment ở MỖI file mới nếu vẫn còn vượt
   ngưỡng sau khi tách (`RemoteBrowserPagePane`/`BrowserPagePane` ước tính vẫn
   >1,700–3,000 dòng, cần đánh giá lại có cần tách sâu hơn theo domain
   `useEffect` hay không).

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- Liên quan: `BUG-FE-HLD-006` (disable không giải thích)
