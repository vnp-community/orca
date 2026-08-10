# TASK-BIGFILE-018 — Investigate: `GitHubItemDialog.tsx` component chính

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-017 đã xong · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`
(Giai đoạn 2)

## Input

- File nguồn: `frontend/src/renderer/src/components/GitHubItemDialog.tsx`
  (sau TASK-BIGFILE-017, kích thước giảm nhẹ)
- Đọc **dòng 6,738–cuối file** (`export default function GitHubItemDialog({...})`,
  ~1,100 dòng, 32 `useState` + 9 `useEffect`)

## Nhiệm vụ

1. Xác định các khối JSX lớn theo đúng comment gốc dòng 1: header,
   conversation tab, checks tab, files tab.
2. Với mỗi khối, xác định state (trong 32 `useState`) thuộc riêng khối đó vs
   state dùng chung giữa nhiều khối (vd active tab index).
3. So sánh với `PullRequestPage.tsx` (xem TASK-BIGFILE-019, làm song song
   hoặc sau) — nếu phần "conversation/checks/files tab" thực sự giống hệt
   giữa 2 file (ngoài phần header Primer-style khác nhau theo comment gốc
   của `PullRequestPage.tsx`), đây là ứng viên trích tiếp vào
   `github-item-dialog-shared.ts` (từ TASK-BIGFILE-017) — nhưng CHỈ ghi nhận
   phát hiện này, không tự ý gộp trong task Investigate.

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`
  mục "Giai đoạn 2" với: danh sách khối JSX + dòng bắt đầu/kết thúc, danh
  sách state theo từng khối, kết luận có nên tách thêm vào
  `github-item-dialog-shared.ts` hay không.
- Nếu tách được: tạo task Move mới (`TASK-BIGFILE-036`, `037`, ... tiếp theo
  dãy số hiện có trong `TASKS-INDEX.md`) theo đúng format task Move đã có.

## Không làm trong task này

Không sửa code.
