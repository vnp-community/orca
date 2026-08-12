# TASK-BIGFILE-032 — Investigate: `TaskPage` component chính (83 state)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-027..031 đã xong · **Status:** ✅ Done (ghi chú
thiết kế — sinh 7 task Move mới TASK-BIGFILE-235..241)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2,
đã cập nhật với dữ liệu thật thay ước đoán ban đầu)

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

## Kết quả thật (sau khi thực thi, thay thế ước đoán ở trên)

**⚠️ Con số 83/58 trong đầu bài SAI — thực tế 153 `useState` / 57
`useEffect`** (đo bằng script Python bracket-depth-aware, không đếm tay/grep
nông — xem chi tiết phương pháp và toàn bộ bảng phân nhóm ở
`../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` mục "Giai đoạn 2").

Tóm tắt:

- Linear chiếm 80/153 state (52%) — 1 hook `useTaskPageLinearState()` duy
  nhất như đề xuất gốc sẽ tái tạo vấn đề bigfile ở quy mô nhỏ hơn. Tách
  thành 3 hook con: teams (3 state), draft (26 state), browse (51 state —
  lớn nhất, làm cuối).
- Jira 36/153 (24%) — tách 2 hook: draft (20 state), browse (16 state).
- GitHub 24/153 (16%) — 1 hook, giữ nguyên đề xuất gốc.
- **GitLab 9/153 (6%) — domain bị đề xuất gốc BỎ SÓT HOÀN TOÀN** (không có
  trong danh sách 4 hook ban đầu). Domain nhỏ nhất, cô lập nhất — làm task
  Move đầu tiên (235) để xác nhận pattern.
- 4 state bootstrap (`repoSelection`, `taskSource`,
  `runtimePreflightStatusByHostId`, `taskResumeApplied`) + 4 `useEffect`
  thực sự cross-domain (dòng 3726–3762, 4523–4567, 5114–5157, 5167–5177 —
  điều phối modal/connect-check xuyên 2-3 provider cùng lúc) KHÔNG tách
  được bằng Move — giữ nguyên ở `TaskPage.tsx`, cần 1 lớp điều phối riêng
  thiết kế sau, giống `RuntimeGraphStore` (TASK-BIGFILE-041) với
  `OrcaRuntimeService`.

7 task Move con đã tạo (dải số riêng cấp cho nhóm xử lý task này, KHÔNG
dùng `TASK-BIGFILE-036` như gợi ý ban đầu — dải đó đã bị nhóm khác chiếm
trước khi task này chạy):

- `TASK-BIGFILE-235-taskpage-gitlab-state-hook.md`
- `TASK-BIGFILE-236-taskpage-github-state-hook.md`
- `TASK-BIGFILE-237-taskpage-jira-draft-state-hook.md`
- `TASK-BIGFILE-238-taskpage-jira-browse-state-hook.md`
- `TASK-BIGFILE-239-taskpage-linear-draft-state-hook.md`
- `TASK-BIGFILE-240-taskpage-linear-teams-state-hook.md`
- `TASK-BIGFILE-241-taskpage-linear-browse-state-hook.md`

KHÔNG tự thực thi các task Move con này trong phiên này — để lại cho vòng
sau, đúng nguyên tắc Investigate.
