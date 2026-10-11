# TASK-FE-E2E-028: e2e thật từ UI trên repo `deepseek-harness` của người dùng (Claude Code thật ghi mã thật)

**From Solution:** [SOL-FE-E2E-006](../solutions/SOL-FE-E2E-006-real-e2e-on-user-repo-replaces-mocked-request-web.md) · **Resolves:** yêu cầu 2026-10-11 "chuyển case 3 (UI với mock) sang chạy thật trên stack local, dùng repo `/Users/datbeohbbh/Desktop/stuff/agentic/deepseek-harness`, gồm cả ra lệnh cho agent chạy code thật"
**Priority:** P1 · **Area:** test (+ hai sửa sản phẩm do e2e phát hiện: request-service và frontend)
**Files (mới):** `tests/stages/e2e/orca-flows/ui-live/` `deepseek-{ui,readonly,review,hotfix,execute,task}-real.web.e2e.ts`, `deepseek-flow-harness.ts`, `deepseek-repo-fixture.ts`, `deepseek-add-project-steps.ts`, `deepseek-request-steps.ts`, `deepseek-solution-review-steps.ts`, `deepseek-plan-checks.ts`, `deepseek-task-sheet-checks.ts`, `deepseek-change-checks.ts`, `live-main-checkout-guard.ts`, `live-main-checkout-rules.ts` (+ `.test.ts`), `live-agent-session-targets.ts`, `live-agent-session-worktree-view.ts`; `tests/stages/e2e/orca-flows/agent_push_block.py` (+ `test_agent_push_block.py`)
**Files (sửa, tương thích ngược):** `run_ui_flow.py` (`--real deepseek[-ui|-readonly|-review|-execute|-task]`), `real-flow-harness.ts` (`workRoots`), `real-repo-fixture.ts`, `live-safety-invariants.ts` (`ORCA_E2E_GUARD_EXCLUDE`, `ignore` của `fingerprintRepo`), `live-execution-steps.ts` (Phase theo thứ tự phụ thuộc, "Start" bị khoá trước khi duyệt, kế hoạch cho phép đường dẫn repo và câu phủ định "do not git push"), `live-agent-session-monitor.ts` + `live-agent-session-judge.ts` (mục tiêu theo repo), `live-request-ui-steps.ts` + `live-flow-steps.ts` (`urgency`, `reason` cho hotfix), `live-console-audit.ts` (một quy tắc mới), `live-secret-leak-scan.ts`
**Sửa sản phẩm do e2e phát hiện:** `backend-go/services/request-service` (`ExecutionTree.NextReadyPhase`, `EvaluateExecution.nextReadyPhase`), `frontend/src/renderer/src/hooks/usePlanTree.ts`
**Status:** `[x]` XONG (xem "Kết quả thực thi")

---

## Việc cần làm

- [x] Repo của người dùng làm fixture: guard riêng cho checkout chính (HEAD, nhánh, từng file đổi/chưa theo dõi theo hash, config cục bộ, mọi ref, worktree của người dùng, thư mục cạnh repo), watchdog 15 giây, dọn đúng cái run thêm vào (worktree `<repo>-*` rồi nhánh), so lại về trạng thái ban đầu.
- [x] `git push` bị chặn suốt run: agent được khởi động lại với `GIT_CONFIG_*` (4 dạng: `origin`, `git@github.com:`, `ssh://git@github.com/`, `https://github.com/`), fixture từ chối chạy nếu chưa có khối này ở mọi tiến trình agent và `git push --dry-run` chưa thất bại cục bộ; sau run agent được khôi phục như cũ.
- [x] Project do UI tạo (Add Project, tên `e2e-ds-<mmddhhmmss>-deepseek-harness`), dọn đúng project đó (huỷ Request, `repo.rm`, `project.delete`; không xoá thư mục); so ảnh chụp dữ liệu người dùng trước/sau.
- [x] 6 spec thật: `deepseek-ui` (Add Project, chặn trùng, nhãn host, tạo worktree cạnh repo), `deepseek-readonly` (câu hỏi có đáp án đọc từ `package.json` thật), `deepseek-review` (tuỳ chọn, so sánh, Reject + Mod+Enter, backlog, Reopen, Regenerate, chọn và duyệt), `deepseek-hotfix`, `deepseek-execute` (change_request có Phase: agent viết `slugify()` + test, đọc lại từ đĩa, chạy `node --test`), `deepseek-task` (theo dõi agent đang chạy qua UI).
- [x] Bảng ánh xạ 74 ca mock → thật: [COVERAGE-request-web-mock-to-real.md](../COVERAGE-request-web-mock-to-real.md).
- [x] Hai lỗi sản phẩm thật do e2e lộ ra đã sửa kèm test hồi quy (xem "Lỗi tìm thấy").

## Tiêu chí hoàn thành

- `run_ui_flow.py --real deepseek` xanh cả 6 spec; `--real all` (các spec cũ) không hồi quy; repo `deepseek-harness`, project/Request của người dùng không đổi; 0 Request mồ côi; agent được khôi phục không còn khối push.

## Kết quả thực thi

**Lượt chính thức cuối (2026-10-11, stack local, dev server `dev-local-mac` với Claude Code thật, agent khởi động lại với `git push` bị chặn rồi khôi phục):**

| Lệnh | Kết quả | Thời lượng |
|---|---|---|
| `ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek` | **6 passed**: execute, hotfix, readonly, review, task, ui; `0 orphaned` | 23,8 phút |
| `run_ui_flow.py --real all` (7 spec cũ, sau các sửa chung; bộ lọc đã neo `/readonly-real`, `/execute-real`) | **7 passed** (add-project, monitor, 3 execute, 2 readonly), 0 vấn đề an toàn; hai spec `deepseek-*` bị lọc nhầm vào lượt đó tự từ chối chạy vì thiếu khối chặn push (hàng rào hoạt động) | 29,1 phút |
| `node --test .../live-main-checkout-rules.test.ts` | 7 pass | dưới 1 giây |
| `node --test .../live-agent-session-judge.test.ts` | 13 pass (không hồi quy sau khi tham số hoá theo repo) | dưới 1 giây |
| `python3 -m unittest tests/stages/e2e/orca-flows/test_agent_push_block.py` | 2 OK (hằng số Python và TypeScript khớp; cả 4 dạng push thất bại cục bộ) | dưới 1 giây |
| `go test ./...` trong `backend-go/services/request-service` | xanh toàn bộ | |
| vitest `usePlanTree.test.ts` | 8 pass (ca mới đỏ khi bỏ sửa) | |

Hành trình đỏ → xanh (mỗi lần là một hiểu sai về hành vi thật, không phải nới tiêu chí): sidebar gọi project theo TÊN project, ghi chú Create request nêu tên project (không phải tên thư mục), Reject Solution đưa Request về backlog (Reopen khởi động lại từ xác nhận loại, rồi "Regenerate"), hotfix cần Urgent + lý do và không hoàn thành trên stack một người dùng, phần phân biệt kế hoạch (đường dẫn repo và câu "do not git push" là hợp lệ), tên Phase là nút tiêu đề, hai lỗi sản phẩm (cổng Phase, tab Plan), hộp thoại rủi ro cao không với tới được, thời điểm đo sheet (sau hoạt ảnh trượt vào), so sánh dòng danh sách lẫn "N seconds ago".

**Dữ liệu của người dùng sau cả loạt chạy:** repo `deepseek-harness` HEAD `c291e7961a` (không đổi), 3 mục chưa theo dõi của người dùng không đổi, chỉ còn worktree `deepseek-harness-test-01` của họ, không nhánh hay worktree của run, không `refs/remotes/*` mới; không project `e2e-ds-*`; 0 Request mồ côi; agent khôi phục không còn `GIT_CONFIG_*`. Một lần ngắt giữa chừng (lệnh bị người dùng chặn) để lại 1 project và 1 Request của run; đã dọn đúng chúng bằng `report_orphan_requests.py --cancel` và `project.delete`.

**Dùng Claude thật:** khoảng 40 lượt gọi AI cho cả phiên phát triển (phân loại, Solution, Plan, thực thi, phân tích chỉ đọc), mỗi lượt đầy đủ của bộ ~12 lượt.

## Lỗi tìm thấy

| # | Nơi | Lỗi | Trạng thái |
|---|---|---|---|
| 1 | request-service | **Cổng Phase mở cho Phase phụ thuộc trước**: `NextPhase` lấy Phase đầu tiên theo thứ tự task-service liệt kê (không phải thứ tự phụ thuộc); khi Phase phụ thuộc đứng trước, Approval của nó mở, `request.startPhase` bị `REQUEST_PHASE_PREDECESSOR_NOT_DONE`, còn Phase nó chờ không bao giờ có cổng: Request kẹt ở `executing` vô hạn. Gặp ở hai lượt có hai Phase (một lượt kẹt tới giới hạn 25 phút, một lượt `request.startPhase` bị từ chối). | **Đã sửa**: `NextReadyPhase` (bỏ qua Phase đang chờ Phase chưa xong), `nextReadyPhase` đọc cạnh `depends_on` (một `GetSubtree`; lỗi đọc thì trả lỗi để reconcile thử lại, không đoán). Test: `TestNextReadyPhase` (6 ca) và `TestStatusConsumer_FirstPhaseApprovalSkipsAPhaseThatWaitsForAnother` (đỏ khi bỏ sửa, xanh khi có); toàn bộ `go test ./...` của service xanh; đã redeploy |
| 2 | frontend | **Tab Plan giữ phần trăm cũ sau khi Request hoàn thành** (`UI-STALE-PLAN-TAB`, "40%/83% (estimated)"): `usePlanTree` chỉ làm mới theo sự kiện và theo vòng thăm dò **khi đang `executing`**; sự kiện `request.status_changed` tới trước khi task-service có trạng thái `done` dẫn xuất, rồi không còn gì làm mới. Làm đỏ `--real monitor` ở lượt 2026-10-11. | **Đã sửa**: thăm dò tiếp khi Request `completed` mà cây còn việc chưa xong, rồi dừng. Test hook mới (đỏ khi bỏ sửa); đã redeploy frontend |
| 3 | request-service / UI | Hotfix trên stack một người dùng không hoàn thành: chẩn đoán lỗi `REQUEST_HOTFIX_APPROVAL_FAILED` (`REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER`: người duyệt hậu kiểm không được là người báo cáo), Request ở `analyzing` | **Mở**: giới hạn của stack/chính sách; spec ghi `stack-limit` và vẫn kiểm "không có thanh quyết định". Cần quyết định: tenant cần người duyệt thứ hai, hoặc hotfix hậu kiểm cho phép tự duyệt |
| 4 | frontend | Request ở backlog sau khi Reject Solution vẫn hiện nút "Regenerate" ("This was already decided"); bấm thì `solution.generate -> REQUEST_SOLUTION_WRONG_STATE: request in status request_backlog` | **Mở** (thấp): đưa ra hành động không thể thực hiện; đường đúng là Reopen (khởi động lại từ bước xác nhận loại) rồi Regenerate |
| 5 | frontend | Chip trạng thái cạnh tiêu đề Plan hiện "Unknown" khi Plan đang chờ duyệt (ảnh chụp tab Plan) | **Mở** (cần xác minh nguồn: trạng thái approval nào không được ánh xạ) |
| 6 | frontend | Bảng markdown trong ô Answer hiện dấu `\|` thô (không `remark-gfm`) | **Mở** (đã biết) |
| 7 | frontend/backend | Cảnh báo console `[workItems] ... no GitHub remote configured ... WORKTREE_REPO_NOT_FOUND` ngay sau khi tạo worktree trên repo có remote GitHub | **Mở** (thấp): đã vào allowlist có lý do |
| 8 | Add Project | Đổi tên project trong hộp thoại cũng đổi nhãn repo (ghi chú Create request nêu `repo <tên project>`) | Ghi nhận hành vi, chưa coi là lỗi |
| 9 | môi trường | Hai project `codex` và `deepseek-harness` của người dùng hiện **không có repo nào** trong DB project (và bảng worktree trống); không phải do e2e (đã có từ trước lượt này) | Không đụng; báo người dùng |
| 10 | frontend | Worktree của task bị gỡ trên host trong khi trang đang mở: "Open in Git" vẫn mở Source Control rỗng cho worktree cũ (sidebar giữ danh sách cũ), terminal ở `~`, console lặp `fetchUpstreamStatus failed GITGATEWAY_UPSTREAM_STATUS_FAILED`; không có lời giải thích. Sau khi tải lại trang thì thông báo "This task's worktree is no longer on this host" hiện đúng | **Mở** (thấp): sidebar nên làm mới danh sách worktree (hoặc nút phải kiểm host trước khi điều hướng) |
| 11 | frontend | Nhãn trạng thái ở thẻ Plan "Unknown" (xem #5) và chấm đỏ đếm chờ duyệt xuất hiện chậm (vài giây tới hơn 5 giây sau khi Request tới `awaiting_analysis_approval`) | Ghi nhận; spec chờ tới 45 giây |
| 12 | api-gateway / frontend | **Phương án rủi ro cao không duyệt được từ web**: chọn một phương án không khuyến nghị mà dịch vụ chấm rủi ro cao thì `approval.approve` bị `REQUEST_DECISION_NOT_EFFECTIVE: the chosen option is high risk: confirm it by typing its title first`; hộp thoại xác nhận (`HighRiskDecisionConfirmDialog`) không bao giờ mở vì `solution.choose` không trả `decision` dùng được và gateway không có kênh `decision.*` (đang nằm trong allowlist hợp đồng "chưa phục vụ"); màn hình chỉ hiện "The option was chosen but not yet approved" và "This was already decided." (sai nghĩa). Người dùng chỉ thoát được bằng cách chọn phương án khác | **Mở** (P1, chức năng): cần kênh `decision.confirm` (+ `decision.list`) ở gateway, và `solution.choose` trả `decision`; spec ghi `known-gap` rồi chọn phương án khuyến nghị để chạy tiếp. Chỉ xảy ra khi AI chấm phương án đó là rủi ro cao (không tất định); gặp ở 2 trong 3 lượt `deepseek-review` cuối |
