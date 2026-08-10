# TASK-BIGFILE-032 — Investigate: `TaskPage` component chính (83 state)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-027..031 đã xong · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx` (sau TASK
  027–031, còn lại chủ yếu là `export default function TaskPage()`, dòng
  3,056 gốc → đọc lại đúng vùng này, dòng có thể lệch nhẹ)
- Component: `TaskPage` — 83 `useState`, 58 `useEffect`

## Nhiệm vụ

1. Liệt kê **toàn bộ 83 `useState`**, phân nhóm theo domain: Linear-specific
   / GitHub-specific / Jira-specific / filter-chung / pagination-chung /
   UI-chung (modal mở/đóng, loading, ...).
2. Với mỗi `useEffect` (58), xác định nó thao tác trên state nhóm nào — đặc
   biệt chú ý effect có thể đọc/ghi state CHÉO giữa nhiều nhóm (dấu hiệu
   coupling cần giữ lại cùng nhau, không tách).
3. Đề xuất 4 custom hook theo domain (gợi ý ban đầu, điều chỉnh theo thực tế
   phát hiện ở bước 1–2):
   - `useTaskPageLinearState()`
   - `useTaskPageGitHubState()`
   - `useTaskPageJiraState()`
   - `useTaskPageFilters()`

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` mục "Giai đoạn
  2" với bảng phân nhóm 83 state + 58 effect cụ thể (thay thế mô tả chung
  chung hiện tại).
- Task Move mới (`TASK-BIGFILE-036`, ... tiếp theo dãy số hiện có) cho từng
  custom hook đã xác nhận, theo đúng format task Move.

## Không làm trong task này

Không sửa code — 83 state là mật độ rất cao, tách sai có rủi ro vỡ luồng dữ
liệu giữa các tab Linear/GitHub/Jira.
