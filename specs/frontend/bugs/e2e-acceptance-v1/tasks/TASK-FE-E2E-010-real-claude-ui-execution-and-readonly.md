# TASK-FE-E2E-010: Nghiệm thu qua UI với Claude Code THẬT: phân tích chỉ đọc và thực thi Plan

**From Solution:** SOL-FE-E2E-003 · **Resolves:** BUG-E2E-001 (đường "duyệt Plan rồi thực thi" chưa từng được chạy với agent thật)
**Priority:** P1 · **Area:** frontend (spec e2e) + tìm lỗi backend/agent
**Files:**
- tests/stages/e2e/orca-flows/ui-live/readonly-real.web.e2e.ts, execute-real.web.e2e.ts (spec mới)
- tests/stages/e2e/orca-flows/ui-live/real-flow-harness.ts, real-repo-fixture.ts, live-safety-invariants.ts, live-execution-steps.ts, live-console-audit.ts (mới)
- tests/stages/e2e/orca-flows/ui-live/fixture-repo/ (repo mẫu `template/`, hook `tool-guard.mjs`, `tool-guard.test.mjs`)
- tests/stages/e2e/orca-flows/ui-live/live-flow-harness.ts, live-project-fixture.ts, live-flow-steps.ts, live-request-ui-steps.ts (sửa nhỏ: repo riêng, kiểm console, dọn có thử lại, câu hỏi lựa chọn)
- tests/stages/e2e/orca-flows/run_ui_flow.py (cờ `--real`), README.md (mục "Chạy với Claude Code THẬT")
- frontend/src/renderer/src/components/request/CreateRequestDialog.tsx (+ `CreateRequestDialog.description.test.tsx`)
**Depends on:** TASK-FE-E2E-008, 009
**Status:** `[x]` Chạy thật, KHÔNG stub. Sau khi B1 (cổng Phase) và B2 (task có `request_id` luôn direct-agent) được sửa và redeploy (2026-10-10 chiều): `execute-real` 3/3 XANH (`change_request` có Phase, `task` có phụ thuộc, `task, single step`), `readonly-real` 2/2 XANH (question, bug). Các mục B3 (PATH agent), B4 (trust `full`) vẫn là đề xuất cho người điều phối.
---

## Context

Yêu cầu của người dùng: phải chạy e2e thật, không stub, từ UI: "Duyệt Plan để thực thi agent AI, Phân tích chỉ đọc với Claude thật", và test không được ảnh hưởng mã thật đang chạy. Người dùng đã cho phép chạy agent AI thật trên máy dev cho mục đích này.

## Thiết kế an toàn

### Đã đọc ra gì về đường thực thi (bằng chứng, không đoán)

1. `request-service` `AdvanceExecution.run` gọi `TaskClient.Execute` (task-service) cho từng task `open` trong phạm vi; `EvaluateExecution.completePhases` mở Approval `phase`; người bấm "Start phase" (`StartPhase`).
2. task-service `ExecuteTask` tạo worktree qua git-gateway (`WorktreeProvisioner.EnsureWorktree`: nhánh `task/<taskId>`, đường dẫn `<repoPath>-<tên nhánh>`, tức **thư mục anh em của repo**), rồi `SimpleExecutor.runAgentPrompt` relay `agent.execPrompt` tới agent với `worktreePath`.
3. `agent.execPrompt` (`agent/src/relay/agent-print-mode-exec.ts`): với `accessMode=write` và `trustPreset=="full"` thêm `--dangerously-skip-permissions`; **task-service luôn gửi `TrustPreset: "full"`** (`simple_executor.go`, có comment là quyết định chủ đích). Loại `worktree` không kiểm đường dẫn (`agent-workspace-validation.ts`). cwd của claude = `worktreePath`. Timeout mặc định 5 phút, tối đa 15 phút; khi quá giờ chỉ `child.kill('SIGKILL')` (không giết cháu).
4. Chế độ chỉ đọc (`accessMode=readonly`, loại `repo_root`) chạy `claude --print <prompt> --permission-mode plan --tools Read,Glob,Grep` ngay trong repo của project và bỏ cờ YOLO. Chỉ có công cụ đọc nên không ghi được.
5. PATH mà agent đưa cho claude là `~/.local/bin:~/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/snap/bin` (`agent-config.ts buildToolPath`): không có `/opt/homebrew/bin`.

**Kết luận:** thực thi ghi là `claude --dangerously-skip-permissions` không sandbox, có thể ghi/đọc bất cứ đâu người dùng có quyền. Không thể giới hạn bằng cấu hình sản phẩm. Vì vậy dựng lưới an toàn của spec (dưới) và nói thẳng giới hạn còn lại.

### Lưới an toàn (mọi lớp đều có trong mã, `real-flow-harness.ts`)

| # | Lớp | Cách làm | Bằng chứng nó hoạt động |
|---|---|---|---|
| 1 | Repo tạm | project gắn vào `$TMPDIR/e2e-ui-<mmddhhmmss>-real-XXXX/repo` (Python nhỏ: `src/calc.py`, `tests/test_calc.py`, một lỗi cố ý ở `clamp`), 1 commit. Không dùng repo `orca` hay repo nào của người dùng | thư mục chứa cả repo lẫn worktree `repo-task-<id>`; sau lượt chạy `ls $TMPDIR/e2e-ui-*` rỗng |
| 2 | Vùng làm việc | worktree do git-gateway đặt cạnh repo nên nằm trong cùng thư mục gốc; spec xoá cả thư mục gốc cuối lượt (kể cả khi đỏ: `finally`) | lượt chạy: `worktrees: .../e2e-ui-...-real-XXXX/repo-task-<uuid>`; sau đó thư mục không còn |
| 3 | Yêu cầu và Plan nhỏ | nội dung xác định ("thêm hàm `negate` và một test"); trước khi bấm duyệt, spec đọc Plan: tối đa 8 task, không khớp `rm -rf`, `git push`, `sudo`, `curl`, `npm i`, `docker`, `deploy`, `/Users/`, `~/`, `$HOME`... | bước "review the Plan before approving it" |
| 4 | Hook chặn (`tool-guard.mjs`, `PreToolUse` trong `.claude/settings.json` của repo tạm, commit trong repo nên có cả ở worktree) | từ chối `Write/Edit/Read/Glob/Grep` ngoài thư mục gốc, Bash có `..` thoát ra, `~`, `$HOME`, đường dẫn tuyệt đối ngoài thư mục gốc, `curl/wget/ssh/sudo/docker...`, `git push/remote/config/clone/fetch/pull`, `WebFetch/WebSearch`. Ghi nhật ký mọi lệnh gọi (`tool-guard-calls.log`) và mọi từ chối (`tool-guard-denials.log`); có từ chối thì spec đỏ ngay | `tool-guard.test.mjs` 3/3 (cho phép việc trong repo, không nhầm mã thành đường dẫn: heredoc, `/** */`, regex, `../src` hợp lệ; từ chối 14 kiểu thoát). Hook chạy thật: mỗi lượt log "tool calls seen by the guard: 2..4" |
| 5 | Tự kiểm hook với PATH của agent | `selfTestGuard()` chạy lệnh hook đúng như claude sẽ chạy, với PATH của agent, mong đợi (đọc trong: 0, ghi ngoài: 2, `..` thoát: 2); lệch thì đỏ trước khi gọi AI | Phát hiện lỗi thật: lần đầu hook dùng `node` trần, PATH của agent không có `node`, hook lỗi thì claude **bỏ qua nó (fail-open)**, log guard 0 lệnh; sửa thành đường dẫn tuyệt đối `process.execPath` |
| 6 | Dấu vân tay trước/sau | repo `orca` + `ORCA_E2E_GUARD_PATHS` + `AGENT_WORK_DIR`: HEAD, refs, `worktree list`, `.git/config`, mọi file đổi/untracked kèm băm nội dung; tên mục trong `$HOME`, `~/Desktop`, thư mục cha của repo; tiến trình `claude --print`; thư mục tạm còn sót. So chính xác, không dung sai | Bắt được thật: lỗi spec đỏ vì người khác sửa file trong cây lúc chạy (xem "Hạn chế") |
| 7 | Watchdog + hạn cứng | mỗi 15 giây so dấu vân tay nhẹ + đọc nhật ký hook; vi phạm thì giết cây tiến trình `claude --print` có cwd trong thư mục tạm (theo pid và hậu duệ, không giết nhóm tiến trình vì sẽ giết agent), đóng trang, đỏ. Hạn cứng 25 phút (thực thi) / 15 phút (chỉ đọc), `ORCA_E2E_REAL_TIMEOUT_MIN` | Đã chạy thật: watchdog dừng lượt chạy khi cây thay đổi, và khi hook từ chối (kể cả lần hook nhầm mã Python/JS thành đường dẫn, đã sửa) |
| 8 | Cổng | chỉ chạy khi `ORCA_E2E_LIVE=1` và `ORCA_E2E_EXECUTE_AGENT=1` (thực thi) / `ORCA_E2E_READONLY_ANALYSIS=1` (chỉ đọc) và không phải `--ai-stub`; `--real` và `--ai-stub` loại trừ nhau | spec `skipped` khi thiếu |
| 9 | Cây yên tĩnh | trước khi bắt đầu, đợi `ORCA_E2E_QUIET_SECONDS` (90) giây không ai sửa guarded repo | log "guarded checkout quiet for 90s after waiting 94s" |
| 10 | Không lộ bí mật | hook chặn đọc `.env`; spec không in token/mật khẩu; ảnh/trace ở `test-results/` (đã xoá), trace chỉ bật sau khi đăng nhập | |

### Giới hạn còn lại (nói thẳng, không có bằng chứng ngược lại)

- Hook chặn Bash theo heuristic (chuỗi lệnh), không phải sandbox: một lệnh được viết cố tình vòng được. Mô hình hợp tác không làm vậy; dấu vân tay trước/sau là lớp phát hiện thứ hai, nhưng chỉ thấy file được git theo dõi/untracked (không thấy file bị gitignore) và các thư mục liệt kê.
- Việc thực thi vẫn là `--dangerously-skip-permissions` trên máy dev thật. Biện pháp mạnh hơn cần thay đổi sản phẩm hoặc môi trường (xem "Đề xuất" cuối file).
- Dấu vân tay chính xác không dung được người khác sửa cây đang chạy: spec đỏ (an toàn) và phải chạy lại trong lúc yên. `ORCA_E2E_GUARD_IGNORE=specs/:docs/` chỉ nên bỏ qua thư mục tài liệu; đã **không** thêm chế độ "audit" nới lỏng vì nó làm yếu điều kiện an toàn.
- Số lượt AI: không có số liệu từ Claude. Mỗi lượt `question`/`bug` chỉ đọc dùng 2 đến 4 lệnh gọi công cụ (hook đếm) cộng phân loại, Solution/analysis; lượt thực thi 3 lệnh gọi công cụ cộng phân loại, Clarification, Solution, Plan.

## Việc cần làm

- [x] A. Thiết kế an toàn viết ra và cài trong mã (mục trên); đọc `agent/src/relay/*`, task-service, request-service để xác định cờ quyền, cwd và vị trí worktree.
- [x] B1. `readonly-real` / `question`: Request qua UI, xác nhận loại, trả lời Clarification, agent chỉ đọc chạy với Claude thật, tab Analysis hiện câu trả lời nhắc `max_of` và `test_calc`, duyệt, Request `completed`; backend: run `answer` `succeeded`, cây làm việc sạch, HEAD không đổi, không có nhánh/worktree mới, hook chỉ thấy `Read/Glob/Grep`.
- [x] B1'. `readonly-real` / `bug` (loại chỉ đọc thứ hai): Clarification có câu hỏi lựa chọn (đã dạy helper chọn `ToggleGroup`), chẩn đoán chỉ ra `clamp`, duyệt, `planning`, huỷ qua UI; cùng các kiểm tra repo.
- [x] B2. `execute-real`: ba test, cả ba xanh.
  - [x] `task, single step` (Plan một task): xanh. UI duyệt Plan (`plan-approve`), Request `executing`, agent thật chạy 1 task ~30 giây, màn hình hiện task đang chạy, `Completed`, tab Plan 100%, worktree `repo-task-<uuid>` có `negate()` và test, `python3 -m unittest` OK, chỉ sửa `src/` và `tests/`, repo chính không đổi.
  - [x] `task` (Plan có task phụ thuộc nhau, tự nhiên): XANH sau khi sửa B2 (subtract + test, 2 task, 3.2 phút).
  - [x] `change_request` (có Phase, duyệt cổng Phase qua UI, "Start phase"): XANH sau khi sửa B1 (5 task phụ thuộc trong 1 Phase, duyệt và Start Phase qua UI `phase-<id>-approve`/`phase-<id>-start`, Request `completed`, 6.1 phút).
- [x] B3. Dọn khi đỏ: giết claude trước, huỷ Request (thử lại tới 90 giây), task kẹt `in_progress` không có tiến trình thì bị huỷ, xoá project, xoá thư mục tạm; sau mọi lượt `report_orphan_requests.py`: "0 orphaned", 0 thư mục `e2e-ui-*` trong `$TMPDIR`, 0 tiến trình `claude --print` còn sót.
- [x] B4. `run_ui_flow.py --real readonly|execute|all`, in "AI mode: REAL Claude Code", kiểm orphan sau lượt chạy (mã thoát 3 nếu còn).
- [x] D. Console: mọi `error`/`warning` kèm bước, ghi `console-report.json` + log `[console:<slug>]`, phân loại bằng allowlist có lý do; mục ngoài allowlist làm đỏ (áp dụng cả 3 test cũ). Kết quả ở dưới.
- [x] Sửa frontend nhỏ do console tìm ra: `CreateRequestDialog` thiếu mô tả (Radix cảnh báo mỗi lần mở); thêm `aria-describedby={undefined}`, vitest `CreateRequestDialog.description.test.tsx` (đỏ khi bỏ sửa, xanh khi có).
- [ ] Chạy trên CI, vai không phải admin, Electron: chưa.

## Lỗi sản phẩm tìm thấy (thật, có bằng chứng)

**B1. [backend request-service] Cổng Phase không bao giờ mở: mọi `change_request` đứng ở `executing` mãi.**
- Triệu chứng: sau khi duyệt Plan, Request `executing`, các Phase `open`, không có Approval `phase` nào; UI không có gì để bấm. Spec `execute-real` / `change_request` đỏ `PHASE-GATE-NEVER-OPENED` sau 90 giây (2 lượt: 04:21 và 05:17 UTC).
- Log: `docker logs orca-go-request`: `reconcile of a request failed ... REQUEST_PHASE_NOT_IN_PLAN: phase de840f06-... does not belong to this request's plan` (04:29:48Z).
- Gốc rễ (đọc mã, tin cậy cao): `usecase/evaluate_execution.go` `completePhases` gọi `e.Open.Execute(ctx, ...)` với `ctx` chỉ có tenant (consumer trạng thái và reconcile chỉ gắn tenant). `PhaseArtifacts.Describe` gọi `Tasks.GetSubtree`; task-service `GetSubtree` lọc theo quyền (`get_subtree.go`: owner hoặc grant) nên với ngữ cảnh không có user, nút gốc Phase bị loại, `findTask` không thấy, trả `REQUEST_PHASE_NOT_IN_PLAN`. `AdvanceExecution.run` thì đã bọc `tenant.WithUserID(ctx, actor)` nên việc giao task chạy được.
- Đề xuất: trong `completePhases`, mở Approval với `tenant.WithUserID(ctx, actor)` (actor đã tính ngay trước đó); tương tự `AdvanceExecution.openGate` (đang dùng `ctx`, không phải `execCtx`) cho cổng `pre_deploy` khi `Describe` đọc task. Thêm test: ngữ cảnh chỉ có tenant mở được cổng Phase.
- Hệ quả đang giấu: StartPhase, giao task trong Phase, `ReportTaskOutcome` cho đường có Phase CHƯA kiểm được.

**B2. [backend task-service] Task của Plan có phụ thuộc bị giao cho engine orchestration và kẹt `in_progress` vĩnh viễn.**
- Triệu chứng: Plan có task B phụ thuộc task A (hầu hết Plan AI sinh ra). A chạy xong qua agent; B chuyển `blocked` -> `open` -> `in_progress`, nhưng không có tiến trình `claude` nào chạy cho B, mãi mãi. Request không cancel được (`REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`) và project không xoá được (`PROJECT_HAS_ACTIVE_WORKFLOWS`): Request kẹt vĩnh viễn, phải huỷ task bằng tay (`task.update status=cancelled`). Spec `execute-real` / `task` đỏ `TASK-STUCK-IN-PROGRESS` (3 lượt).
- Bằng chứng: `orca-go-orchestration` log `StartCoordinatorRun ... rpc ok` đúng lúc B được giao; log agent chỉ có một `agent.execPrompt` (của A); `Execute` của B trả sau 151 ms (task đầu mất ~30 giây vì chờ claude).
- Gốc rễ: `task-service/internal/usecase/execute_task.go` `selectEngine`: có cạnh `depends_on` thì chọn `EngineOrchestration`, trừ khi task "có spec" (`HasSpec`). Task tạo bởi `CommitPlan`/`CreatePlanTree` không có spec nên rơi vào engine 2; coordinator không chạy agent và không báo kết quả. Comment ngay trong `selectEngine` nói thứ tự giữa task của Plan là việc của `AdvanceExecution`, nên đây là chỗ lệch giữa ý định và điều kiện.
- Đề xuất: task có `RequestID` (task của Plan) luôn đi `EngineDirectAgent` bất kể `depends_on`, hoặc `CommitPlan` ghi spec cho mọi task. Thêm test `selectEngine` cho task Plan có phụ thuộc. Cân nhắc đặt thời hạn cho engine 2 để task không kẹt vô hạn.

**B3. [agent] PATH của claude do agent đặt thiếu `/opt/homebrew/bin` (macOS Homebrew).** `agent-config.ts buildToolPath` chỉ có `~/.local/bin`, `~/bin`, `/usr/local/bin`, `/usr/bin`, `/bin`, `/usr/sbin`, `/snap/bin`. Hệ quả thấy được: claude không tìm thấy `node` (`node --test` thất bại, nó thử `export PATH=...$HOME/.nvm...`), và hook `node ...` trong `.claude/settings.json` lỗi thì claude bỏ qua (fail-open). Đề xuất: thêm `/opt/homebrew/bin`, `/opt/homebrew/sbin` khi `process.platform === 'darwin'` (hoặc kế thừa PATH của tiến trình agent). Spec tránh phụ thuộc vào đó: repo mẫu dùng Python 3 (`/usr/bin/python3`) và hook gọi `node` bằng đường dẫn tuyệt đối.

**B4. [thiết kế, cần quyết định] `trustPreset: "full"` luôn bật** cho mọi `task.execute`: mọi task chạy `--dangerously-skip-permissions` không sandbox trên máy dev. Đề xuất: chính sách theo project (mặc định không `full`, hoặc chạy trong container/tài khoản sandbox), và chỉ cho `full` khi thư mục làm việc nằm trong vùng worktree đã đăng ký.

**Nhỏ, đã ghi:** (a) danh sách Request có thể chậm hơn tiêu đề chi tiết vài giây (ảnh chụp 1 giây sau khi duyệt Plan: dòng danh sách "Awaiting Plan Approval", tiêu đề "Executing"); chưa kiểm là có tự hết. (b) Khi trả lời xong Clarification của loại `task` (không có phân tích) Request đi thẳng `planning`; helper phải chấp nhận (đã tham số hoá). (c) Plan của loại `task` có nút gốc `plan` cũng báo `in_progress` song song với task lá (cascade tiến độ), không phải lỗi.

## Console errors của trình duyệt (từng dòng, đã phân loại)

Mỗi lượt có 17 đến 22 mục, tất cả được phân loại (0 chưa phân loại); tất cả xuất hiện ở bước "sign in through the login form" hoặc "create the Request in the dialog", không có ở các bước của luồng Request/thực thi, và 0 `pageerror`, 0 lệnh gọi lỗi sau đăng nhập.

| Số lượng/lượt | Loại | Nội dung | Phân loại |
|---|---|---|---|
| 1 | `error` `/auth/me` | `Failed to load resource: the server responded with a status of 401 (Unauthorized)` | nhiễu đã biết: `/auth/me` trả 401 trước khi đăng nhập, theo thiết kế (allowlist `auth-me-before-sign-in`) |
| 15 đến 20 | `error` | `Failed to fetch worktrees for repo <uuid>: RuntimeRpcCallError: rpc error: code = Internal desc = WORKTREE_DETECT_FAILED: git worktree list failed` | nhiễu có nguyên nhân: sidebar lấy worktree của mọi project; 4 project `probe-proj` cũ trên stack (còn từ phiên trước) trỏ repo tạm đã bị xoá nên git-gateway `chdir ...: no such file or directory`. Dữ liệu cũ, không phải mã Request; nhưng UI thử lại dồn dập (15 đến 20 lần trong vài giây): nên có backoff/ngừng thử với repo lỗi (chưa sửa). Allowlist `sidebar-worktrees-of-stale-projects` |
| 1 | `warning` | `Missing Description or aria-describedby={undefined} for {DialogContent}` (hộp thoại Create request) | lỗi thật của mã Request (a11y): ĐÃ SỬA trong nguồn (`CreateRequestDialog`), chờ build lại frontend; rule `create-request-dialog-description` trong allowlist phải bị xoá sau khi frontend được dựng lại |

`decision.*` (kênh chưa có ở gateway) không xuất hiện trong console ở các lượt này; rule vẫn nằm trong allowlist vì nó từng là nhiễu đã biết (FE-E2E-001).

## Kết quả thực thi

Ngày 2026-10-10, stack local đầy đủ, agent dev-server là Claude Code thật (`claude` 2.1.266, không `--ai-stub`, không fake claude), UI qua Chromium (Playwright).

| Lượt | Test | Kết quả | Ghi chú |
|---|---|---|---|
| ro1 | readonly question | ĐỎ | hook guard 0 lệnh gọi: hook dùng `node` trần, PATH agent không có `node` (fail-open) → sửa (B3, lớp 5) |
| ro2 | readonly question (JS) | XANH 67 giây | 3 lệnh đọc |
| ro3 | readonly bug | ĐỎ | Clarification có câu hỏi lựa chọn (ToggleGroup) mà helper chưa điền → sửa helper |
| ro4 | readonly bug | ĐỎ (an toàn) | dấu vân tay thấy file lạ do agent khác sửa trong cây lúc chạy → thêm so file chi tiết và đợi cây yên |
| ro5 | readonly question + bug (JS) | XANH 2/2 | 77 và 63 giây |
| ro6 | readonly question + bug (Python) | XANH 2/2 | 154 và 147 giây (gồm 94 giây đợi cây yên); 2 lệnh đọc mỗi test |
| ex1 | execute change_request | ĐỎ | B1 |
| ex3 | execute task | ĐỎ (an toàn) | hook từ chối `/**` trong Bash (nhầm mã thành đường dẫn) → sửa hook + test |
| ex4 | execute task | ĐỎ | B2 + cây bị người khác sửa |
| ex6 | execute single | ĐỎ (an toàn) | hook từ chối `'../src/calc.mjs'` trong heredoc → sửa |
| ex7 | execute single | ĐỎ (an toàn) | claude đi tìm `node` bằng `$HOME/.nvm` (B3), hook từ chối đúng → đổi fixture sang Python |
| ex8 | execute single | ĐỎ | luồng chạy đến `completed`; kiểm repo sai vì `unittest` in ra stderr → sửa kiểm tra |
| ex9 | execute single | **XANH** 167 giây (thực thi ~30 giây) | task `in_progress` → `review` → `done`; Request `completed`; 3 lệnh công cụ |
| ex10 | execute task | ĐỎ | B2 (`TASK-STUCK-IN-PROGRESS`), dọn sạch |
| ex11 | execute change_request | ĐỎ | B1 (`PHASE-GATE-NEVER-OPENED`), dọn sạch |
| old1 | 3 test cũ (`request-live-flow`, Claude thật): change_request 4.2 phút, question 1.5 phút (ask-back bỏ qua vì chỉ stub) | XANH 2/2 | 0 mục console chưa phân loại |

Lệnh:

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real readonly
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute -- --grep "single step"
node --test tests/stages/e2e/orca-flows/ui-live/fixture-repo/tool-guard.test.mjs
cd frontend && NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/components/request/CreateRequestDialog.description.test.tsx
```

### Bất biến an toàn (đo được)

Trước/sau mỗi lượt chạy xanh: repo `orca` và repo của agent (`AGENT_WORK_DIR`): HEAD, refs, worktree list, config, mọi file thay đổi/untracked bằng băm nội dung: **không đổi** (spec đỏ nếu khác); 0 từ chối hook ở các lượt xanh; 0 tiến trình `claude --print` còn sót; 0 thư mục `e2e-ui-*` còn trong `$TMPDIR`; `report_orphan_requests.py`: "0 orphaned"; agent dev-server vẫn là Claude thật, không bị đổi.

### Kết quả cuối

Lượt cuối, sau khi tách file cho đúng giới hạn dòng (`live-analysis-approval-steps.ts`, `live-fixture-change-checks.ts`), chạy tuần tự (`--workers=1`):

| Chạy | Kết quả |
|---|---|
| `run_ui_flow.py request-live-flow` (2 test cũ với Claude thật; ask-back bỏ qua vì chỉ stub) | **2 xanh**, 1 bỏ qua (5.9 phút), 0 mục console chưa phân loại |
| `run_ui_flow.py --real all` (5 test thật, 18.2 phút) | **3 xanh**: `question` 152 giây, `bug` 152 giây, `task, single step` 162 giây (thực thi ~30 giây). **2 đỏ đúng như dự kiến**: `change_request` `PHASE-GATE-NEVER-OPENED` (B1), `task` `TASK-STUCK-IN-PROGRESS` (B2) |
| Bất biến an toàn ở cả 5 test | "safety problems: 0", "guard denials: 0"; hook thấy 2 (chỉ đọc) đến 4 (thực thi) lệnh gọi công cụ mỗi lượt |
| Sau lượt | `report_orphan_requests.py`: 0 orphaned; 0 dự án/Request `e2e-ui-*`; 0 thư mục `e2e-ui-*` trong `$TMPDIR`; 0 tiến trình `claude --print`; repo `orca` và repo `demo-repo` của agent: dấu vân tay không đổi (so chính xác); `test-results/` đã xoá |
| `node --test .../fixture-repo/tool-guard.test.mjs` | 3/3 |
| vitest `CreateRequestDialog.description` + `RequestPage` + `CreateRequestIssueButton` | 8/8 xanh |
| oxlint trên `ui-live/*.ts` và các file frontend đã đụng | sạch (đã tách file cho khỏi vượt 300 dòng) |

Hai test đỏ là lỗi sản phẩm thật, không phải lỗi spec: chúng sẽ xanh khi B1 và B2 được sửa và redeploy (cần thêm: đường Phase, StartPhase và giao task trong Phase chưa từng chạy, nên có thể còn lỗi phía sau B1).

## Kết quả sau redeploy B1/B2 (2026-10-10)

Cùng lưới an toàn, agent dev-server `dev-local-mac` là Claude Code thật. Lệnh: `ORCA_E2E_GUARD_IGNORE=specs/:docs/ python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute` rồi `--real readonly`.

| Test | Kết quả | Thời lượng | Ghi chú |
|---|---|---|---|
| execute `change_request` | XANH | 6.1 phút | Plan có Phase, task `inspect` → `append add()` → `append test` → `pytest` phụ thuộc nhau; cổng Phase mở, duyệt và Start trên UI; hook thấy 15 lệnh gọi công cụ |
| execute `task` (phụ thuộc) | XANH | 3.2 phút | subtract + test; task B chạy thật sau A (trước đây kẹt `in_progress`) |
| execute `task, single step` | XANH | 2.6 phút | 3 lệnh công cụ |
| readonly `question` | XANH | 2.5 phút | 2 lệnh đọc |
| readonly `bug` | XANH | 3.0 phút | 2 lệnh đọc |

Mọi lượt: "safety problems: 0", "guard denials: 0", 0 mục console chưa phân loại, sau lượt `report_orphan_requests.py`: "0 orphaned"; 0 thư mục `e2e-ui-*` trong `$TMPDIR`; 0 tiến trình `claude --print`; repo `orca` và `demo-repo`: dấu vân tay không đổi. Phần "agent đang làm gì trên web" nằm ở TASK-FE-E2E-011.

Lượt chạy lại sau redeploy lần 2 (`--real all`, 6 test, 21.6 phút): `change_request` (6.1 phút), `task, single step` (2.8), `question` (2.6), `bug` (2.6) XANH; `task` có phụ thuộc đỏ một lần vì hook từ chối đường dẫn scratchpad riêng của phiên Claude (an toàn, bất định, xem G7 của TASK-FE-E2E-011), chạy lại riêng XANH (4.0 phút); `request-live-flow` (Claude thật) 2 xanh, 1 bỏ qua. `report_orphan_requests.py`: 0 orphaned.

Lượt `--real all` cuối (sau redeploy lần 3, 22.6 phút): `change_request` 6.1 phút, `task` phụ thuộc 4.3, `task, single step` 2.6, `question` 2.6, `bug` 2.8 đều XANH (G7 không tái diễn); `monitor` đỏ một gap backend (G5c, xem TASK-FE-E2E-011).

Lượt `--real all` sau redeploy lần 4 (G5c sửa, 21.4 phút): 6/6 XANH (`monitor` 3.9, `change_request` 6.2, `task` phụ thuộc 3.5, `task, single step` 2.8, `question` 2.6, `bug` 2.5 phút); 0 orphaned.

## Đề xuất cho người điều phối

1. Sửa B1 (`completePhases`/`openGate` truyền `tenant.WithUserID(ctx, actor)`) và B2 (`selectEngine` cho task Plan), build lại `request-service` và `task-service`, rồi chạy lại: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute` (cần cây yên ~2 phút, không ai sửa repo trong lúc chạy).
2. Sửa B3 (PATH của agent), build lại agent, chạy lại `start-local-agent.sh`.
3. Build lại frontend (sửa `CreateRequestDialog`), rồi xoá rule `create-request-dialog-description` khỏi `live-console-audit.ts`.
4. Quyết định B4: nếu không muốn `--dangerously-skip-permissions` không sandbox trên máy dev, cần chạy agent trong tài khoản/container sandbox hoặc chính sách theo project.
5. Dọn dữ liệu cũ: 4 project `probe-proj` trỏ repo đã xoá gây 15 đến 20 lỗi console mỗi lần tải.
