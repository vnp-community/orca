# SOLUTION-FE-BIGFILE-005 — Gỡ trùng lặp & tách `GitHubItemDialog.tsx` + `PullRequestPage.tsx`

**Bug:** `../BUG-FE-BIGFILE-005-githubitemdialog.md` VÀ
`../BUG-FE-BIGFILE-007-pullrequestpage.md` (xử lý cùng nhau — xem lý do trong
`SOLUTION-FE-BIGFILE-001` mục 4)
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #6 — xem `SOLUTION-FE-BIGFILE-001` mục 3

---

## Vì sao xử lý chung

`PullRequestPage.tsx` (7,372 dòng) tự nhận trong comment dòng 1 là
"duplicated from GitHubItemDialog" — cả 2 file có `type ItemDialogTab`,
hàm `invalidateWorkItemDetailsCacheForKey` trùng tên gần như 1:1. Tách riêng
từng file trước khi gỡ trùng lặp sẽ nhân đôi công sức (phải tách logic giống
nhau ở 2 nơi).

## Giai đoạn 1 — Trích phần dùng chung (làm TRƯỚC, bắt buộc)

| File mới | Nội dung | Nguồn |
|---|---|---|
| `github-item-dialog-shared.ts` | `type ItemDialogTab`, `invalidateWorkItemDetailsCacheForKey` | `GitHubItemDialog.tsx:275,1467` (giữ nguyên, xoá bản trùng ở `PullRequestPage.tsx:250,1509`) |

**Bước thực hiện:**
1. `gitnexus impact({target: "invalidateWorkItemDetailsCacheForKey", direction: "upstream"})`
   — chạy RIÊNG cho từng file (`GitHubItemDialog.tsx` và `PullRequestPage.tsx`
   có thể trả về 2 symbol khác nhau dù trùng tên — xác nhận cả 2 caller list
   trước khi gộp).
2. So sánh nội dung 2 hàm/type trùng tên **theo đúng từng dòng** (không chỉ
   tin vào tên trùng) — nếu có sai khác dù nhỏ (edge case xử lý khác nhau
   giữa Issue/PR), ghi chú lại sai khác đó, KHÔNG tự ý gộp nếu chưa xác nhận
   sai khác là vô ý (bug) hay cố ý (khác biệt nghiệp vụ thật giữa 2 luồng).
3. Tạo `github-item-dialog-shared.ts`, copy bản của `GitHubItemDialog.tsx`
   (file "gốc", theo đúng comment của `PullRequestPage.tsx` xác nhận nó là
   bản sao).
4. `GitHubItemDialog.tsx` đổi `export type ItemDialogTab`/
   `export function invalidateWorkItemDetailsCacheForKey` thành
   `export { ... } from './github-item-dialog-shared'`.
5. `PullRequestPage.tsx` XOÁ định nghĩa trùng, import từ
   `github-item-dialog-shared.ts` thay vì tự định nghĩa lại — đổi
   `export type PullRequestPageProjectOrigin` (type riêng, không trùng) giữ
   nguyên tại chỗ.

## Giai đoạn 2 — Tách phần riêng từng file (sau khi giai đoạn 1 xanh)

**Kết quả Investigate (TASK-BIGFILE-018 + 019, 2026-08-12) — thay thế toàn
bộ giả định gốc bên dưới bằng dữ liệu thật.** Bản kế hoạch gốc (mục 1-3
dưới đây) đoán "khối JSX lớn: header, conversation tab, checks tab, files
tab" đều là JSX inline cần tách — **sai một phần quan trọng**: đọc trực
tiếp source hiện tại cho thấy conversation/checks tab **đã được tách sẵn**
thành component dùng chung (`<ConversationTab>`, `<ChecksTab>`,
`<PRFilesCombinedDiffViewer>`, import từ nơi khác) — task doc gốc dựa trên
comment dòng 1 mô tả cấu trúc ("keeps its header, conversation, files, and
checks tabs co-located") mà không grep xác nhận nội dung thật.

### Số liệu đã sửa lại (task doc gốc đoán sai — có thể do đếm nhầm phạm vi
toàn file thay vì chỉ trong component chính)

| | Task doc gốc | Thật (2026-08-12) |
|---|---|---|
| `GitHubItemDialog` — dòng bắt đầu | 6,738 | 6,741 (lệch nhẹ do TASK-017 thêm 3 dòng) |
| `GitHubItemDialog` — useState trong component chính | 32 | **7** (32 là tổng CẢ FILE, gồm nhiều sub-component khác định nghĩa phía trên) |
| `GitHubItemDialog` — useEffect trong component chính | 9 | **5** |
| `PullRequestPage` — dòng bắt đầu | 6,433 | 6,432 |
| `PullRequestPage` — useState trong component chính | 30 | **7** (46 là tổng cả file) |
| `PullRequestPage` — useEffect trong component chính | 13 | **6** (13 là tổng cả file — trùng khớp ngẫu nhiên với con số component chính của GitHubItemDialog, gây nhầm lẫn khi viết task doc gốc) |

Đáng chú ý: cả 2 file có **đúng 7 useState, cùng tên, cùng thứ tự khai báo**
(`tab, localState, localLabels, linkCopyState, optimisticTick, refetchTick,
pendingViewedPaths`) — xác nhận toàn bộ khung state-management bị
copy-paste 1:1, không chỉ 2 symbol đã xử lý ở TASK-BIGFILE-017.

### Ranh giới JSX thật (thay bảng "4 khối lớn" bằng cấu trúc thật)

`GitHubItemDialog.tsx` (component chính dòng 6,741–7,855, ~1,114 dòng):
- Dòng 6,741–7,275 (~535 dòng): hooks/state/callbacks thuần (không JSX) —
  fetch/cache orchestration (`optimisticTick`/`refetchTick`/`details`
  `useMemo`), link-copy state machine, `handlePRFileViewedChange`, v.v.
- Dòng 7,276–7,692 (~416 dòng): JSX "header" — nhánh
  `isIssuePage ? (...) : (...)`. Nhánh Issue (7,278–7,690) tự chứa **1 lần
  gọi `<ConversationTab>` inline** (dòng 7,651, vì Issue không có tab
  switcher — `normalizeItemDialogTab` ép `tab` luôn `'conversation'` khi
  `item.type !== 'pr'`). Nhánh PR (else, ngắn) chứa `<Tabs>`.
- Dòng 7,694–7,845 (~151 dòng): `<Tabs>`/`<TabsList>`/`<TabsContent>` — mỗi
  `TabsContent` chỉ gọi 1 component đã tách sẵn:
  `<ConversationTab>` (7,731, ~40 dòng props), `<ChecksTab>` (7,775, ~16
  dòng props), và **files tab** (7,792–7,835, ~44 dòng) — khối JSX inline
  DUY NHẤT còn lại thật sự (loading/unavailable/empty state + fallback gọi
  `<PRFilesCombinedDiffViewer>`).

`PullRequestPage.tsx` (component chính dòng 6,432–7,371, ~940 dòng) —
**không có nhánh Issue** (PR-only, đúng comment gốc): cấu trúc tương tự
nhưng phần JSX chỉ có 1 nhánh (Primer-style header ~), rồi `<Tabs>` ở dòng
7,225 với 3 `TabsContent` gần như **giống hệt về hình dạng** với
`GitHubItemDialog.tsx` (`<ConversationTab>` dòng 7,263, `<ChecksTab>` dòng
7,305, files-tab inline dòng 7,322–7,362).

### Phát hiện chính — vì sao KHÔNG sinh task Move mới ở đợt này

1. **Conversation/Checks tab đã dùng chung rồi** (`<ConversationTab>`,
   `<ChecksTab>`, `<PRFilesCombinedDiffViewer>` — component riêng, import
   từ file khác) — không còn gì để tách thêm ở đây, mục 2-3 kế hoạch gốc
   coi như đã xong từ trước khi 2 file này bị đưa vào danh sách bigfile.
2. **Files-tab JSX inline** (~40-44 dòng, ~7,792–7,835 vs 7,322–7,362) là
   khối GIỐNG NHAU RÕ RỆT nhất còn lại — nhưng so dòng-với-dòng phát hiện 2
   khác biệt thật, không phải lỗi copy-paste:
   - i18n key namespace theo tên file:
     `auto.components.GitHubItemDialog.filesUnavailable` vs
     `auto.components.PullRequestPage.filesUnavailable` (và tương tự
     `filesRetry`, key ngẫu nhiên `3cd5ae5b7b`/`6ad2c1ab9c` cho "No files
     changed.") — move nguyên văn sẽ cần quyết định namespace i18n chung
     (đổi catalog, ngoài phạm vi 1 task Move thuần code).
   - `<ConversationTab>` nhận props khác nhau: `GitHubItemDialog.tsx` truyền
     `timelineItems={timelineItems}` (không có ở `PullRequestPage.tsx`);
     `PullRequestPage.tsx` truyền `participants={details?.participants ?? []}`
     (không có ở `GitHubItemDialog.tsx`) — chưa xác định đây là khác biệt
     nghiệp vụ cố ý hay thiếu sót, cần hỏi người phụ trách `ConversationTab`
     trước khi coi 2 lời gọi là "giống hệt" đủ để rút thành 1 wrapper dùng
     chung.
   → **Không sinh task Move cho block này ở đợt này** — cần quyết định
     i18n-key trước (không phải quyết định kỹ thuật thuần), ghi nhận làm
     tiền đề cho 1 task riêng sau khi có quyết định đó.
3. **Header JSX** (~416 dòng `GitHubItemDialog.tsx` / tương tự
   `PullRequestPage.tsx`) là khối lớn nhất còn lại và tự chứa (không lồng
   trong khối khác) — về mặt cấu trúc CÓ THỂ tách thành
   `<file>-header.tsx` mỗi file riêng (KHÔNG dùng chung, đúng dự đoán gốc
   vì 2 style Primer/thường khác nhau thật). Nhưng bề mặt prop cần truyền
   qua nếu tách là **~30+ định danh** (toàn bộ state/callback khai báo ở
   535 dòng phía trên: `workItem, backLabel, onClose, projectOrigin,
   localState, localLabels, linkCopyState` và dẫn xuất
   (`resolvedLinkCopyState/linkCopied`), `issueAttachedWorkspace*`,
   `handleOpenOrUseIssueWorkspace`, `handleCopyWorkItemLink`,
   `setLinkCopyButtonRef`, `isIssuePage`, `ownerRepo`,
   `issueStateBadgeTone`, `onUse`, `onReviewRequestsChange`, và — RIÊNG cho
   `GitHubItemDialog.tsx` — toàn bộ props của `<ConversationTab>` inline
   trong nhánh Issue: `body, comments, timelineItems, files, headSha,
   baseSha, loading, detailsLoaded, checks, onMutated, onChecksUpdated,
   onBodyUpdated, onCommentAdded, onReviewersRequested`). Việc tách này khả
   thi về mặt cơ học nhưng KHÔNG còn là "Move nguyên văn ít rủi ro" — là 1
   refactor prop-drilling vừa/lớn trên 1 file **zero test coverage**
   (giống nhận định rủi ro ở `TASK-BIGFILE-054` cho `orca-runtime.ts`).
   **Không tự sinh task Move cho block này ở đợt Investigate này** — để
   lại làm ứng viên cho 1 đợt riêng có: (a) enumerate đầy đủ prop list bằng
   đọc kỹ (không chỉ liệt kê từ tên khai báo), (b) thêm test coverage tối
   thiểu cho header trước khi tách, giống khuyến nghị đã áp dụng cho
   `orca-runtime.ts` domain "lõi".

### Kết luận Giai đoạn 2

Không sinh task Move con (dải số 215-224 dành cho nhóm task này còn nguyên,
không dùng) — cả 2 ứng viên tách được đều bị chặn bởi 1 quyết định ngoài
phạm vi thuần kỹ thuật (i18n key cho files-tab) hoặc rủi ro cao do thiếu
test coverage + bề mặt prop lớn (header). Khuyến nghị: nếu muốn tiếp tục
giảm kích thước 2 file này, ưu tiên (1) quyết định chiến lược i18n-key dùng
chung trước, sau đó Move block files-tab (nhỏ, rủi ro thấp một khi đã có
quyết định); (2) thêm test coverage tối thiểu cho `GitHubItemDialog`/
`PullRequestPage` trước khi cân nhắc tách header.

---

**Kế hoạch gốc (đã lỗi thời một phần — giữ lại để đối chiếu lịch sử, xem
kết quả thật ở trên):**

1. Đọc nội bộ `export default function GitHubItemDialog({...})` (dòng
   6,738–cuối, ~1,100 dòng) và `export default function PullRequestPage({...})`
   (dòng 6,433–cuối, ~940 dòng) để xác định các khối JSX lớn (theo đúng
   comment gốc: header, conversation tab, checks tab, files tab).
2. Với MỖI file, tách theo tab: `<file>-header.tsx`, `<file>-conversation-tab.tsx`,
   `<file>-checks-tab.tsx`, `<file>-files-tab.tsx` — component cha giữ lại
   logic điều phối tab + state quản lý chung.
3. Vì 2 file có Primer-style header khác nhau (theo comment gốc của
   `PullRequestPage.tsx`), phần header KHÔNG dùng chung — chỉ 3 tab còn lại
   (conversation/checks/files) là ứng viên trích tiếp sang
   `github-item-dialog-shared.ts` nếu logic thực sự giống nhau sau khi đọc kỹ.

## Xác minh (cả 2 giai đoạn)

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có cho cả 2 file (nếu có)
- `gitnexus detect_changes({scope: "all"})` — đặc biệt chú ý risk_level, vì
  đây là sửa đồng thời 2 file cùng lúc (khác nguyên tắc chung ở
  `SOLUTION-FE-BIGFILE-001` mục 2.4 "không refactor 2 file cùng lúc" — NGOẠI
  LỆ áp dụng đúng cho cặp file trùng lặp này, vì tách riêng sẽ nhân đôi công
  sức, nhưng vẫn tách thành 2 giai đoạn con để review từng phần)
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

**Trung bình** — rủi ro chính không phải kỹ thuật (copy/paste rõ ràng) mà là
**xác nhận đúng 2 khối code trùng tên thực sự giống hệt nhau** trước khi gộp.
Nếu đã tồn tại sai khác âm thầm (bug tiềm ẩn ở 1 trong 2 nơi mà không ai phát
hiện do trùng lặp), việc gộp có thể vô tình sửa 1 bug thật hoặc tạo ra 1 bug
mới nếu chọn sai bản để giữ lại — bắt buộc so sánh dòng-với-dòng ở bước 2 của
Giai đoạn 1, không bỏ qua.
