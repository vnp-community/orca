# TASK-FE-E2E-013: Sheet task của Request: "Open in Git", bố cục chật, phím Escape, gợi ý tab Result

**From Solution:** SOL-FE-E2E-003 · **Resolves:** TASK-FE-E2E-011 G3 (a, b, c) và G4 (phía frontend)
**Priority:** P1 · **Area:** frontend + spec mock
**Files:**
- frontend/src/renderer/src/components/task/open-task-worktree-in-git.ts (mới) + `.test.ts` (9 test), TaskDetail.tsx (`handleOpenInGit`, bố cục, nối gợi ý)
- frontend/src/renderer/src/components/request/RequestPage.tsx (+ test), request/execution/execution-saved-output.ts (mới) + `.test.ts`, ExecutionResultPanel.tsx (chỉ khối `<pre>` của "Agent output")
- frontend/src/renderer/src/components/task/agent-activity/TaskAgentActivityPanel.tsx (+ test), task/__tests__/TaskDetail.test.tsx, TaskDetail-request-artifacts.test.tsx
- frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json (`auto.components.task.TaskDetail.openInGit.*`, `auto.components.task.agentActivity.{savedElsewhere,openResult}`), i18n/task-open-in-git-locale-coverage.test.ts
- tests/stages/e2e/legacy/request-web/request-task-sheet.web.e2e.ts (mới, 10 ca)
**Depends on:** TASK-FE-E2E-011 (khảo sát, G3/G4), TASK-FE-E2E-012 (panel Agent activity)
**Status:** `[x]` XONG (2026-10-10): code, vitest, Playwright mock và kiểm trên stack thật (`--real monitor`: Open in Git, sheet không tràn, Escape).

---

## Gốc rễ (đã tái hiện bằng Playwright mock TRƯỚC khi sửa: 5/10 ca đỏ trên mã cũ)

**G3a. "Open in Git" im lặng.** `handleOpenInGit` thoát sớm không báo gì khi `useWorkspace().project` rỗng; `project` của `WorkspaceContext` chỉ được gán khi người dùng chọn project trong Project Workspace (`ProjectSwitcher.switchProject`), không bao giờ ở trang Requests. Kể cả khi có `project`, nút chỉ đổi `currentWorktree` của context và `activeWorkspaceTab` của màn Workspace phía sau trong khi `activeView` vẫn là `requests`, nên màn hình không đổi; và `WorktreeList` đồng bộ lại `currentWorktree` theo `activeWorktreeId` của store nên giá trị đó còn có thể bị ghi đè. Sửa:
- `projectId` lấy từ chính task (`task.projectId`, dự phòng `project?.id`); không có thì toast rõ ràng. Cùng sửa `projectId: project!.id` của "Execute with Agent" (sẽ ném TypeError khi context rỗng).
- `openTaskWorktreeInGit(projectId, worktreeId)`: worktree đã có trong store thì dùng luôn; chưa có thì `worktree.list {projectId}` để tìm hàng, nạp repo nếu store chưa có (`fetchRepos`), rồi `fetchWorktrees(repoId)` (hoặc `fetchAllWorktrees` nếu hàng không có `repoId`). Sau đó `activateAndRevealWorktree` (chuyển sang view `terminal` của worktree, chọn dòng trong sidebar) và mở Source Control ở thanh phải (`setRightSidebarTab('source-control')` + `setRightSidebarOpen(true)`), cùng đích với "View" sau khi xoá worktree thất bại. Đang ở Project Workspace (`activeView === 'workspace'`) thì ở lại đó và chọn tab Git. Không xây UI diff mới.
- Mọi kết quả không điều hướng đều có thông báo: `unknown` ("This task's worktree no longer exists": backend không có hàng), `not-on-host` ("...no longer on this host. It may have been cleaned up.": hàng còn nhưng host không liệt kê checkout, ví dụ đã bị dọn), lỗi RPC ("Failed to open worktree: ..."), không có project. Nút mở lại được sau mỗi lần.

**G3b. Sheet tràn ngang.** Đo trước khi sửa: `scrollWidth - clientWidth` của sheet là 97 px ở 1440 và 700 px, 227 px ở 390 px. Nguyên nhân: hàng nút `flex` không `wrap` (badge + "Attach Workflow Template" + "Execute with Agent" + "Open in Git"), `TabsList` rộng `w-fit` với 5 đến 6 tab, và gốc `TaskDetail` không `min-w-0`. Sửa: hàng nút `flex-wrap`; `TabsList` `max-w-full overflow-x-auto scrollbar-sleek`; gốc và `Tabs` `min-w-0`; ô tiêu đề `truncate` kèm `title` (tooltip đủ chữ). Không dùng màu/cỡ chữ mới.

**G3c. Escape trong sheet làm rời trang Requests.** Tái hiện: Escape trong sheet đóng sheet VÀ `RequestPage` (handler `keydown` ở `window`) chuyển `activeView` sang `terminal`. Radix gọi `preventDefault()` khi đóng lớp trên cùng, nên `RequestPage` nay bỏ qua sự kiện đã `defaultPrevented`. Lưu ý trung thực: tôi tái hiện việc "rời trang"; `requestPage.requestId` (Request đang chọn) không bị xoá bởi Escape trong sheet, nên "bỏ chọn Request/header trống" của agent N là hệ quả của việc trang bị đóng và mở lại, không phải một handler bỏ chọn riêng. Escape khi không có gì phủ lên vẫn đóng trang (giữ hành vi cũ).

**G4. Panel `idle` sau khi tải lại.** Khi không chạy, không có dòng nào, và tab Result có dữ liệu thật (`execution.get` trả `ready` với bản ghi, hoặc phần cuối output không rỗng; `hasSavedExecutionOutput`), panel thay câu "Nothing to show yet..." bằng "Live activity is not kept after you reload the page. See the saved output in the Result tab." kèm nút `Open the Result tab` chuyển thẳng sang tab. Tab Result rỗng/ẩn (`legacy`, `unsupported`, output rỗng) thì không có gợi ý. Giữ nguyên huy hiệu `Idle`/`Finished` và `data-state`.

## Khảo sát: `ExecutionResultPanel` với "Agent output"

Đã dùng sẵn: `<pre>` monospace `text-xs`, `max-h-64 overflow-auto`, `whitespace-pre-wrap`, rút gọn 200 dòng x 400 ký tự, che bí mật, ghi chú "Only the last part is shown". Chỉnh nhỏ: thêm `break-words` (đường dẫn dài không có khoảng trắng gập dòng thay vì cuộn ngang; Playwright kiểm không cuộn ngang với dòng 480 ký tự) và `tabIndex=0` + `aria-label` để cuộn bằng bàn phím. Lưu ý: nội dung tab Result là câu trả lời CUỐI của agent (`result` của stream-json, 8 KiB cuối), KHÔNG phải các dòng `▸ Read/Edit/Bash`; các dòng công cụ chỉ có ở panel Agent activity lúc chạy và không được lưu (cần backend lưu lịch sử mới xem lại được).

## Việc cần làm

- [x] G3a nút "Open in Git" điều hướng thật (Source Control của worktree), tải worktree nếu store chưa có, thông báo rõ khi không mở được.
- [x] G3b bố cục sheet cho 1440 / 700 / 390 px.
- [x] G3c Escape trong sheet chỉ đóng sheet (đã tái hiện trước khi sửa).
- [x] G4 gợi ý sang tab Result, chỉ khi có dữ liệu; i18n 5 ngôn ngữ.
- [x] Khảo sát + chỉnh nhỏ `ExecutionResultPanel`.
- [ ] Kiểm với stack thật sau khi dựng lại frontend (chưa thấy hàng `source-control-entry` của file `src/calc.py` trong mock vì `git.status` mock rỗng).

## Kiểm

| Lệnh | Kết quả |
|---|---|
| `vitest run --config config/vitest.config.ts src/renderer/src/components/task/ src/renderer/src/components/request/RequestPage.test.tsx src/renderer/src/components/request/execution src/renderer/src/i18n/task-agent-activity-locale-coverage.test.ts src/renderer/src/i18n/task-open-in-git-locale-coverage.test.ts` | 34 file, 258 test xanh |
| `playwright test -c tests/configs/playwright.web.config.ts --project=request-web` | 58 xanh, 1 skipped có sẵn (10 ca mới của `request-task-sheet`) |
| `oxlint -c .oxlintrc.json` các file đụng | sạch (không tắt `max-lines`) |
| `tsc --noEmit -p frontend/tsconfig.json --composite false` | không lỗi mới ở file đụng; còn lỗi CÓ SẴN (`web-preload-api.ts`, `RequestBacklogRow.tsx`...) |

## Selector / hành vi cho spec live (agent N)

| Điều cần kiểm | Cách kiểm |
|---|---|
| Nút | `[data-testid=open-in-git-btn]` (không đổi); sau khi bấm sheet và trang Requests biến mất (tab "Requests" không còn), sidebar chọn worktree của task |
| Trạng thái store (dev: `window.__store.getState()`) | `activeView === 'terminal'`, `activeWorktreeId === task.worktreeId`, `rightSidebarTab === 'source-control'`, `rightSidebarOpen === true` |
| File đã đổi | dòng `[data-testid=source-control-entry]` trong thanh phải (chưa kiểm hiện thật trên stack) |
| Không mở được | toast "This task's worktree no longer exists" / "...no longer on this host..." / "Failed to open worktree: ..."; sheet vẫn mở |
| Gợi ý tab Result | `[data-testid=task-agent-activity-empty]` chứa "Result tab" và `[data-testid=task-agent-activity-open-result]` (nút) chỉ khi tab Result có dữ liệu |
| Sheet | `getByRole('dialog')`: không cuộn ngang; Escape chỉ đóng sheet (`activeView` vẫn `requests`) |
| Tab Result | `[data-testid=execution-agent-output-text]` |
