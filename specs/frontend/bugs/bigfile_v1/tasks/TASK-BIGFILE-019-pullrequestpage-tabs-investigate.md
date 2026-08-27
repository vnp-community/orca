# TASK-BIGFILE-019 — Investigate: `PullRequestPage.tsx` component chính

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-017 đã xong (nên làm cùng lúc/ngay sau
TASK-BIGFILE-018 để so sánh chéo 2 file)
**Status:** ✅ Done (ghi chú thiết kế — không sinh task Move mới, phân tích
chung với TASK-BIGFILE-018 trong
`../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`
mục "Giai đoạn 2")
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`
(Giai đoạn 2)

## Input

- File nguồn: `frontend/src/renderer/src/components/PullRequestPage.tsx`
  (sau TASK-BIGFILE-017, kích thước giảm nhẹ)
- Đọc **dòng 6,433–cuối file** (`export default function PullRequestPage({...})`,
  ~940 dòng, 30 `useState` + 13 `useEffect`)

## Nhiệm vụ

Giống hệt TASK-BIGFILE-018 nhưng áp dụng cho file này — xác định khối JSX
lớn (header Primer-style riêng theo comment gốc + conversation/checks/files
tab), state theo từng khối, so sánh chéo với kết quả của TASK-BIGFILE-018 để
xác nhận phần nào thực sự trùng logic (ứng viên gộp vào
`github-item-dialog-shared.ts`) vs phần nào khác biệt thật (giữ riêng).

## Output

- Cập nhật CÙNG mục "Giai đoạn 2" trong
  `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`
  mà TASK-BIGFILE-018 đã bắt đầu — 2 task này nên ghi vào CÙNG 1 solution
  doc để dễ so sánh chéo, không tách rời 2 phần phân tích.
- Task Move mới (nếu có) tiếp tục dãy số sau TASK-BIGFILE-018 đã tạo (nếu
  có).

## Không làm trong task này

Không sửa code.
