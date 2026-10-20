# Bộ e2e nghiệm thu các luồng ORCA (`orca-flows`)

Bộ test chạy trên **stack Docker local đang chạy** (`deploy/dev`), đi qua đúng đường mà UI dùng: đăng nhập `POST /auth/local`, rồi gọi các kênh `invoke` trên WebSocket `/ws` của api-gateway. Không mock service nào. Báo cáo kết quả và các lỗi tìm được: [`../../../ORCA-E2E-ACCEPTANCE-REPORT.md`](../../../ORCA-E2E-ACCEPTANCE-REPORT.md).

Chỉ dùng thư viện chuẩn của Python 3.9+ (có client WebSocket riêng, `ws_rpc_client.py`), nên không cần `pip install`.

## Chạy

```bash
cd tests/stages/e2e/orca-flows
python3 run_acceptance.py                  # toàn bộ
python3 run_acceptance.py request matrix   # một số suite (tự thêm devserver)
python3 run_acceptance.py --list
python3 run_acceptance.py --strict         # BLOCKED cũng tính là lỗi: dùng cho phiên nghiệm thu chính thức
python3 run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02   # AI là `claude` giả (CI); chỉ MCP được phép BLOCKED
python3 report_orphan_requests.py          # liệt kê Request mở mà project đã bị xoá (chỉ đọc); --cancel để huỷ có lý do
```

Mã thoát 1 nếu có `FAIL` (hoặc `BLOCKED` khi `--strict`, trừ các id trong `--allow-blocked`), 2 nếu dùng sai hoặc không dựng được agent giả, 3 nếu chạy xong nhưng không khôi phục được agent. Kết quả ghi vào `reports/last-run.{json,md}` (thư mục này bị git bỏ qua).

## Điều kiện chạy

| Cần | Cách có |
|---|---|
| Stack `deploy/dev` đang chạy | xem `deploy/dev/README.md` |
| Tài khoản admin | đọc từ `deploy/dev/.env` (`BOOTSTRAP_ADMIN_*`), hoặc `ORCA_E2E_ADMIN_EMAIL/PASSWORD`. Mật khẩu không bao giờ được in ra |
| Dev server (agent) đã kết nối và được duyệt | tên mặc định `dev-local-mac` (`ORCA_E2E_DEV_SERVER_NAME`). Thiếu thì các test liên quan là `BLOCKED` |
| Repo git trên dev server | tự tạo repo tạm; nếu dev server là máy khác, đặt `ORCA_E2E_REPO_PATH` |
| `docker` CLI (tuỳ chọn) | để kiểm tra container và đọc kết quả từng step workflow; thiếu thì test đó là `BLOCKED` |
| AI cho agent (tuỳ chọn) | Claude Code đã đăng nhập trên dev server, hoặc `ANTHROPIC_API_KEY` trong môi trường của agent; thiếu thì các test dùng AI là `BLOCKED`. Không có tài khoản (CI): dùng `--ai-stub` |
| `ORCA_E2E_READONLY_ANALYSIS=1` (tuỳ chọn) | cho phép các test làm agent Claude **thật** chạy phân tích chỉ đọc (`claude --print`) trên repo của project: `RAN-01..03` và bước kiểm trạng thái đầu của `MTX` cho `bug`, `hotfix`, `security`, `performance`, `spike`, `question`. Trả lời Clarification chuẩn bị của các loại này tự khởi động một lượt phân tích tốn lượt AI và chạy lâu. Không đặt thì các test đó là `SKIP` có lý do (`MTX` vẫn kiểm bảng luồng). Với `--ai-stub` thì luôn chạy |
| `ORCA_E2E_EXECUTE_AGENT=1` (tuỳ chọn) | bật test chạy agent thật (`task.execute`); mặc định `SKIP` vì tốn lượt dùng AI |

## Chế độ `--ai-stub`: `claude` giả cho CI

CI không có tài khoản Claude. `--ai-stub` cho **cùng mã sản xuất** (request-service, gateway, agent) chạy với một `claude` giả xác định ở
`fake_claude_cli/claude`; mã sản xuất không biết gì về stub, runner chỉ đổi môi trường của tiến trình agent. Mọi báo cáo của chế độ này ghi
**"AI mode: STUB"** (`reports/last-run.md`, trường `ai.mode` trong `last-run.json`, và dòng cuối của lần chạy): kết quả chứng minh nối dây và hợp đồng,
**không** đánh giá chất lượng AI.

**Cách hoạt động** (`ai_stub_agent_switch.py`, chỉ macOS/Linux; agent Windows tìm `claude.cmd`):

1. Tạo một HOME tạm có `~/.local/bin/claude` trỏ tới stub và đặt thư mục đó đầu `PATH`. Cả hai cần: `ai.complete` tìm `claude` theo `PATH` của tiến trình, còn probe
   năng lực (`claude auth status`, quyết định `aiSources`) chỉ tìm trong danh sách cố định dưới `$HOME` (`~/.local/bin` đầu tiên).
2. Chạy lại `deploy/dev/scripts/start-local-agent.sh` với môi trường đó (nó `pkill` mọi `agent/out/agent.js` rồi khởi động agent mới).
3. **Pha 1**, stub đã đăng nhập: chạy các suite đã chọn (AI trả lời, `RQL-09`, `AIS-02/04`, `PLN-*`).
4. **Pha 2**, nếu có suite `ai`: khởi động lại agent với stub **chưa đăng nhập** (`FAKE_CLAUDE_LOGGED_IN=0`) rồi chạy `devserver`, `aistub`, `ai` để mở khoá `AIS-01/03`
   (đường "không có AI") và kiểm `REQUEST_AI_NOT_CONFIGURED` (`AST-04`). Kết quả hai pha được gộp theo test id: `FAIL` ở bất kỳ pha nào thắng, rồi `PASS`, rồi `BLOCKED`.
5. Khôi phục agent như trước khi chạy (cùng `AGENT_WORK_DIR`, `DEV_SERVER_ID`, file log; `HOME`/`PATH` thật, không còn stub). `--keep-stub-agent` bỏ bước này (agent dùng một lần của CI).

**Cảnh báo:** trong lúc chạy, dev server thật bị ngắt (mọi phiên đang dùng agent đó, ví dụ terminal trên UI, bị rớt) cho tới khi bước 5 khôi phục nó. `start-local-agent.sh` dừng **mọi** `agent/out/agent.js` của máy.

Hai điểm cần biết:

- Mỗi pha dùng **một dev server riêng** (`<tên>-stub-in`, `<tên>-stub-out`; runner tự duyệt bằng tài khoản admin) vì infra-fleet giữ hồ sơ năng lực của dev server tới 24 giờ và
  chỉ dò lại khi agent kết nối nếu đã quá 5 phút kể từ lần dò trước. Dùng một danh tính cho cả hai pha thì thay đổi đăng nhập của stub không được thấy. Hai bản ghi này ở lại
  trong DB (không có API xoá dev server); `ORCA_E2E_STUB_IDENTITY_SUFFIX=x` đổi tên để buộc dò lại.
- Agent gọi `claude` theo **hai hình thức khác nhau** và stub nhận cả hai: `ai.complete` (phân loại, Solution kind `solution`, Plan) chạy `claude -p --output-format json --tools ""`
  với prompt qua stdin, trong thư mục tạm rỗng, trả phong bì JSON; phân tích chỉ đọc (chẩn đoán, findings, answer; `agent.execPrompt` với `accessMode=readonly`) chạy
  `claude --print <prompt> --permission-mode plan --tools Read,Glob,Grep` ngay trong repo của project, trả thẳng tài liệu JSON ra stdout. Prompt nằm trên argv nên nó phải không chứa khoá.
- Stub trả lời theo nội dung prompt do **mã thật** sinh (`BuildClassificationPrompt`, `BuildSolutionPrompt`, `BuildPlanPrompt` trong request-service) và từ chối
  chạy nếu `ai.complete` không có `--tools ""`, hoặc phân tích chỉ đọc được cấp công cụ có thể ghi. Prompt lạ thì lỗi rõ ràng chứ không bịa câu trả lời. Mỗi lần gọi (argv, cwd, nội dung thư mục, kích thước stdin) được ghi vào
  `$HOME/.fake-claude/calls.jsonl` để `AST-01..03` kiểm.

**Kiểm chính stub, không cần stack:**

```bash
python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py
bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh
```

Lệnh thứ hai chạy `go test -overlay` (không thêm file vào `backend-go`): dựng prompt bằng hàm thật, cho stub trả lời đúng như agent gọi, rồi đưa vào
`ParseClassificationProposal`, `SolutionOptions.Validate` + kiểm bao phủ AC, `ParsePlanOutput`, `ValidatePlanProposal` và chính sách theo loại, cho cả 11 loại và mọi size; và `ValidateByKind` cho chẩn đoán, findings, answer (loại nào của registry có phân tích chỉ đọc). Prompt hoặc
validator đổi mà stub không theo kịp thì lệnh này đỏ trước khi e2e đỏ.

## Chạy qua giao diện thật (Playwright, `ui-live/`)

Các suite Python ở trên đi qua WebSocket. Spec `ui-live/request-live-flow.web.e2e.ts` đi **qua trình duyệt**, trên frontend thật của stack local, không mock. Ba test:

1. `change_request`: đăng nhập bằng form, tạo Request trong hộp thoại, đợi phân loại, chọn loại và "Confirm type", trả lời Clarification, Solution **tự sinh** (sau 20 giây chưa có run thì ghi gap `SOLUTION-NOT-AUTO-STARTED` và bấm "Generate solution"), chọn option và duyệt, "Generate plan", thấy Approval `Plan` trong hộp thư, từ chối nó (về backlog, **không bao giờ duyệt** vì sẽ chạy agent AI trên máy dev) rồi huỷ Request bằng UI. Nếu Solution có câu hỏi chặn (hoặc bị từ chối duyệt vì câu hỏi chặn) thì làm như người dùng: "Ask back" (hộp thoại, lý do), trả lời Clarification, đợi Solution mới (cũ thành `superseded`), chọn lại và duyệt; tối đa 2 vòng. Số vòng ghi ở log (`ask-back rounds: N`). Với Claude thật nhánh này chạy khi AI tình cờ hỏi chặn.
2. `ask-back` (chỉ chế độ stub): tiêu đề chứa mã `[stub:blocking-question]` khiến `claude` giả trả Solution đầu tiên có một câu hỏi chặn và Solution sinh lại (prompt có khối `<clarifications>` sau `</request>`) thì không còn; test xác nhận đúng 1 vòng Hỏi lại, Solution cũ `superseded`, rồi huỷ Request.
3. `question`: loại có phân tích "answer" (agent chỉ đọc repo), kết thúc ở `completed`.

Gateway chỉ dùng để đối chiếu trạng thái thật (và dọn dẹp); **không có đường lùi qua gateway**: bước nào UI không làm được thì test đỏ. Gap (UI chậm hơn service quá 45 giây, màn hình còn Solution đã bị thay, gateway restart/rớt kết nối, Solution không tự bắt đầu) làm test đỏ qua soft assertion; `ORCA_E2E_LIVE_GAPS=annotate` chỉ ghi chú. Kênh `decision.*` chưa có ở gateway (allowlist của CONTRACT) là nhiễu đã biết: chỉ đếm, không làm đỏ.

```bash
# AI giả, xác định (đổi agent sang claude giả rồi KHÔI PHỤC agent chạy Claude thật); chạy được cả 3 test:
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub
# Claude thật của dev server (chậm hơn, tốn lượt dùng; test ask-back tự bỏ qua):
python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- --grep change_request
# Spec CR-REQ-039/041 (chỉ stub; cần build lại container frontend sau khi đổi mã giao diện):
#   run_ui_flow.py --ai-stub -- request-base-branch      # nhánh bắt đầu của Request: chọn, sửa, khóa, lỗi, tính năng tắt
#   run_ui_flow.py --ai-stub -- analysis-revision       # chỉnh phân tích bằng prompt: phiên bản, Approval, bản cũ chỉ đọc
# Spec CR-REQ-043 (cần build lại container frontend; backend CR-043 đã triển khai):
#   run_ui_flow.py --ai-stub -- approve-run-all          # nút phụ "Approve and run all steps", hộp xác nhận, nhãn tab Execution, cờ tính năng tắt, Project Settings
#   run_ui_flow.py --real continuous                     # loại B: Claude thật chạy hết một Plan sau MỘT lần duyệt (repo tạm; chi phí nhỏ; chỉ khi được yêu cầu)
# Spec CR-REQ-044 (Plan tự sinh sau khi duyệt Analysis; claude giả; cần build lại container frontend; dev server phải ở cùng máy vì lỗi được gây bằng tệp `fail_plan_next` của claude giả):
#   run_ui_flow.py --ai-stub -- auto-plan-generation     # Plan tự ra không cần bấm; thẻ lỗi bền qua tải lại và Retry; lỗi tạm thời tự thử lại; công tắc dự án tắt / gateway cũ giữ nút Generate
# Spec CR-REQ-046 (Vietnamese output; claude giả):
#   run_ui_flow.py --ai-stub -- analysis-output-language # chữ Việt có dấu hiển thị nguyên vẹn ở tab Analysis, nhãn giao diện vẫn tiếng Anh
#   run_ui_flow.py --real language                       # loại B: một phân tích `question` chỉ đọc bằng Claude thật, trả lời tiếng Việt, JSON hợp lệ (opt-in; đo token)
# Spec CR-REQ-045 (Plan; claude giả; cần build lại container frontend và triển khai backend CR-045 trước):
#   run_ui_flow.py --ai-stub -- plan-documents          # xem Plan: "Proposal" (native), "Documents" (OpenSpec, dữ liệu gieo bằng plan_documents_seed.py), chỉ tải tài liệu khi mở tab, che bí mật
#   run_ui_flow.py --ai-stub -- plan-revision           # chỉnh Plan bằng prompt: 3 phiên bản, bản cũ chỉ đọc, một Approval mỗi lúc, dòng History, quá 2000 ký tự, màn hình cũ, lượt thất bại + "Try again"
#   run_ui_flow.py --real plan-revision                 # loại B: một vòng chỉnh Plan native bằng Claude thật trên repo tạm (chi phí nhỏ; chỉ chạy khi được yêu cầu, không sửa checkout trong lúc chạy)
#   ORCA_E2E_OPENSPEC=1 run_ui_flow.py -- plan-documents  # loại C: Plan OpenSpec thật của kịch bản 3 trên release/1.x, 7 đến 13 USD mỗi lượt sinh Plan (cộng một lượt chỉnh); cần đồng ý rõ ràng, cần openspec và tdd-agent trên máy; KHÔNG tự chạy
# Chỉ Playwright, khi agent đã chạy sẵn (tên dev server qua ORCA_E2E_DEV_SERVER_NAME):
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live
```

Không có `ORCA_E2E_LIVE=1` spec tự bỏ qua. Cần: stack `deploy/dev` chạy, frontend ở `FRONTEND_HTTP_PORT` (đọc từ `deploy/dev/.env`, hoặc `ORCA_E2E_FRONTEND_URL`), admin từ `.env`/`ORCA_E2E_ADMIN_*`, `git` trên máy chạy test. Nếu có `docker` (đặt `DOCKER_HOST` cho Colima), spec so `RestartCount` của container `orca-go-api-gateway` (`ORCA_E2E_GATEWAY_CONTAINER`) trước và sau, và ghi gap nếu gateway restart. Dữ liệu tạo ra mang tiền tố `e2e-ui-<mmddhhmmss>` (project, repo tạm, Request) và bị dọn ở cuối (Request mở bị huỷ trước khi xoá project).

Ảnh từng bước, ảnh lúc lỗi (`99-at-failure.png`) và `trace.zip` (chỉ khi lỗi hoặc có gap) nằm ở `test-results/` (git bỏ qua); cuối mỗi test in tóm tắt gồm các RPC mà UI bị từ chối. Trace chỉ bật **sau** khi đăng nhập vì trace ghi lại giá trị đã gõ; mật khẩu không bao giờ vào artifact. Test `change_request` ở trên không bao giờ duyệt Plan (chỉ ghi chú nếu `ORCA_E2E_EXECUTE_AGENT=1`) vì duyệt sẽ chạy agent AI trên máy dev; việc duyệt Plan để agent thật thực thi nằm ở nhóm `--real execute` ngay dưới, có lưới an toàn riêng.

## Chạy với Claude Code THẬT, từ UI (`run_ui_flow.py --real`)

Nhóm này (`ui-live/readonly-real.web.e2e.ts`, `ui-live/execute-real.web.e2e.ts`, TASK-FE-E2E-010) chạy agent Claude Code **thật** của dev server, không stub. Báo cáo ghi "AI mode: REAL Claude Code". Thiết kế an toàn và kết quả: `specs/frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-010-real-claude-ui-execution-and-readonly.md`.

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real readonly   # question + bug: phân tích chỉ đọc (claude --print --permission-mode plan --tools Read,Glob,Grep)
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute    # duyệt Plan trên UI, agent thật THỰC THI (claude --dangerously-skip-permissions) trong repo tạm
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real monitor    # theo dõi phiên agent trên web (mục dưới)
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real addproject # Add Project tạo project riêng, chặn trùng, Request chọn project, Analysis hiện Answer (mục "Add Project tạo project riêng")
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real all -- --grep "single step"   # lọc test
```

Vì task-service luôn gửi `trustPreset: "full"` cho `task.execute`, agent chạy `claude --dangerously-skip-permissions` với cwd là worktree: **không có sandbox nào của sản phẩm**. Nên spec tự dựng lưới an toàn (`real-flow-harness.ts`), và đây là điều kiện để được chạy:

| Lớp | Việc làm |
|---|---|
| Repo tạm | project gắn vào `os.tmpdir()/e2e-ui-<mmddhhmmss>-real-XXXX/repo` (git init, 1 commit; mẫu ở `ui-live/fixture-repo/template`); worktree của task do git-gateway đặt cạnh nó (`repo-task-<id>`), nên mọi thứ nằm trong một thư mục gốc duy nhất và bị xoá sau. Không bao giờ dùng repo `orca` hay repo của người dùng |
| Hook chặn | `.claude/settings.json` của repo tạm có hook `PreToolUse` (`tool-guard.mjs`) từ chối đọc/ghi ngoài thư mục gốc, `..`/`~`/`$HOME` thoát ra, mạng, `git push/remote/config`, `sudo`, `curl`... Nó ghi nhật ký mọi lệnh gọi và mọi từ chối; có từ chối thì spec đỏ ngay. Chạy hook bằng đường dẫn tuyệt đối tới `node` (PATH của agent không có `/opt/homebrew/bin`, hook lỗi thì claude BỎ QUA nó, tức fail-open) và có tự kiểm tra trước khi chạy (`selfTestGuard`) với đúng PATH của agent. Giới hạn thật: heuristic cho Bash, không phải sandbox |
| Dấu vân tay trước/sau | repo `orca` (và `ORCA_E2E_GUARD_PATHS`, mặc định thêm `AGENT_WORK_DIR`): HEAD, refs, worktree list, `.git/config`, mọi file đổi/untracked kèm băm nội dung; danh sách tên trong `$HOME`, `~/Desktop`, thư mục cha của repo; tiến trình `claude --print` còn sót; thư mục tạm còn sót. Khác một byte là đỏ. Cần cây yên tĩnh: spec đợi `ORCA_E2E_QUIET_SECONDS` (mặc định 90 giây) không ai sửa rồi mới bắt đầu. `ORCA_E2E_GUARD_IGNORE=specs/:docs/` bỏ qua tiền tố mà con người sửa trong lúc chạy (chỉ nên đặt cho thư mục tài liệu) |
| Watchdog | mỗi 15 giây so dấu vân tay nhẹ; vi phạm hoặc hook từ chối thì giết cây tiến trình `claude --print` có cwd trong thư mục tạm (không phải cả nhóm tiến trình, vì sẽ giết cả agent), đóng trang, đỏ |
| Giới hạn thời gian | hạn cứng `ORCA_E2E_REAL_TIMEOUT_MIN` (mặc định 25 phút thực thi, 15 phút chỉ đọc); task `in_progress` 90 giây mà không có tiến trình claude thì đỏ `TASK-STUCK-IN-PROGRESS`; cổng Phase không mở sau 90 giây thì đỏ `PHASE-GATE-NEVER-OPENED` |
| Duyệt Plan | trước khi bấm duyệt, spec đọc Plan (tối đa 8 task, không có đường dẫn ngoài repo/cài đặt/mạng/deploy/push) |
| Dọn | kể cả khi đỏ: giết claude trước, huỷ Request (thử lại tới 90 giây; task kẹt `in_progress` không có tiến trình thì bị huỷ), xoá project, xoá thư mục tạm; sau lượt chạy `report_orphan_requests.py` phải sạch (`run_ui_flow.py` kiểm và trả mã 3 nếu không) |

Điều kiện chạy: `ORCA_E2E_LIVE=1`, `ORCA_E2E_READONLY_ANALYSIS=1` (chỉ đọc) và/hoặc `ORCA_E2E_EXECUTE_AGENT=1` (thực thi), không có `ORCA_E2E_AI_MODE=stub`. `run_ui_flow.py --real` đặt đủ. Thiếu thì spec `skipped`.

Mỗi lượt in số liệu: số lệnh gọi công cụ mà hook thấy, số lần hook từ chối, số vấn đề an toàn, thời lượng. Console của trình duyệt: mọi `error`/`warning` được ghi kèm bước đang chạy vào `console-report.json` và log (`[console:<slug>]`), phân loại theo `ui-live/live-console-audit.ts`; mục ngoài allowlist làm đỏ spec (áp dụng cho cả 3 test cũ).

## Theo dõi phiên agent trên web (`run_ui_flow.py --real monitor`)

Spec `ui-live/agent-session-monitoring-real.web.e2e.ts` (TASK-FE-E2E-011) chạy một Plan một task với Claude Code **thật**, cùng lưới an toàn như nhóm `--real` ở trên, nhưng kiểm điều mà các test trên chưa kiểm: người dùng có theo dõi được **agent** (không chỉ trạng thái task) từ giao diện web hay không.

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real monitor            # chỉ test này
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real all                # readonly + execute + monitor
node --test tests/stages/e2e/orca-flows/ui-live/live-agent-session-judge.test.ts   # luật phán quyết, không cần stack hay AI
```

Trong lúc agent chạy, spec mở sheet task từ tab Plan (trang chính), mở Cài đặt → Dev Servers ở tab thứ hai, và nghe kênh `task.activity.subscribe` của gateway bằng kết nối riêng (mở trước khi duyệt Plan). Sau khi xong nó tải lại trang, mở lại task, đọc các tab của Request, bấm "Open in Git". Mỗi điều kiện không đạt là một **gap** có mã và chủ sở hữu (`[frontend]`/`[backend]`/`[agent]`), không bao giờ được nới để xanh:

| Mã | Ý nghĩa |
|---|---|
| `DEVSERVER-OFFLINE-IN-UI`, `DEVSERVER-NO-BUSY-STATE` | dev server phải online, và trạng thái phải khác nhau giữa "agent đang chạy" và "rảnh" |
| `UI-NO-PATH-TO-TASK`, `UI-NO-RUNNING-INDICATOR`, `UI-TASK-STATUS-STALE` | đi từ Request tới task đang chạy, thấy "Agent running", trạng thái cập nhật không cần F5 |
| `AGENT-OUTPUT-NOT-BROADCAST`, `AGENT-OUTPUT-NOT-STREAMED` | backend/agent phải phát output của agent khi nó đang chạy (không chỉ ở cuối) |
| `UI-AGENT-OUTPUT-NOT-SHOWN`, `UI-AGENT-OUTPUT-NOT-LIVE` | điều backend phát phải hiện trên màn hình task, trong lúc chạy |
| `UI-NO-SESSION-HISTORY`, `UI-NO-RUN-TRACE`, `UI-NO-WORKTREE-VIEW` | sau khi xong và tải lại, phải xem lại được agent đã làm gì (sửa `src/calc.py`, chạy test) |

Kiểm cứng: không có bí mật (biến có tên PASSWORD/TOKEN/SECRET/KEY của `deploy/dev/.env` và môi trường, dạng `sk-ant-`, khối khoá riêng, `Bearer eyJ`) trong bất kỳ màn hình nào đã đọc hay frame hoạt động (`SECRET-LEAK`, chỉ in tên biến); lỗi console của cả hai trang qua allowlist. Kết quả khảo sát (cái gì có/không có trên web) và danh sách lỗi: `specs/frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-011-agent-session-monitoring-in-ui.md`. Kết quả thật (2026-10-10, sau redeploy lần 4): `--real monitor` XANH và `--real all` 6/6 XANH (21.4 phút). Từ UI: panel Agent activity hiện dòng `▸ Read/Edit/Bash` lúc chạy rồi `Finished`, dev server Busy/Idle/healthy/platform, nút "Agent running", tab Result xem lại được, "Open in Git" mở terminal + Source Control liệt kê `src/calc.py` và `tests/test_calc.py`, không lộ bí mật. Còn mở nhưng không làm đỏ: G5d/G5e (bấm vào file đã đổi thì panel diff vỡ; spec ghi annotation `known-gap`, console nằm trong allowlist `diff-view-crash-g5d`), G5b (`scm-empty-branch-g5b`). `ORCA_E2E_LIVE_GAPS=annotate` chỉ ghi lại thay vì làm đỏ. Hai tab của cùng một trình duyệt dùng chung "view gần nhất" của SPA nên spec rời Settings ("Back to app") trước khi tải lại.

## Hồi quy "Add Project rồi tạo worktree" (`run_ui_flow.py --repro add-project-worktree`)

Spec `ui-live/add-project-worktree-repro.web.e2e.ts` (TASK-FE-E2E-015) làm đúng các bước người dùng đã làm, hoàn toàn qua giao diện: đăng nhập, Add Project (nút `+` ở thanh Projects), host `dev-local-mac`, Browse folder, chọn repo git, Select folder, Add Git Project, `+` trên project vừa thêm, worktree `test-02`, Create worktree. Không AI, không Request. Repo là repo git **tạm** (`e2e-ui-repro-XXXXXX`, một commit) do spec tự tạo; không bao giờ đụng project hay repo của người dùng.

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live add-project-worktree-repro
```

Spec in `BUG REPRODUCED` kèm thông báo lỗi nguyên văn trên UI khi khớp `WORKTREE_CREATE_FAILED` + `chdir` + `no such file or directory`, rồi bằng chứng nguyên nhân chỉ đọc: `devServerId` của repo (`repo.list`), project chứa nó, dòng log `orca-go-git-gateway` (nếu có docker), container có thấy thư mục không. Các assert cuối mô tả hành vi ĐÚNG (worktree được tạo, thư mục `<repo>-test-02` có trên đĩa, nằm trong danh sách worktree, repo gắn với `dev-local-mac`) nên spec XANH khi gắn host (SOL-FE-E2E-004 frontend + backend) đã triển khai và ĐỎ nếu repo/project mất dev server trở lại; spec cũng kiểm project chứa repo mang `devServerId` của host đã chọn. Dọn trong `finally`: `repo.rm` đúng repo của lượt chạy và `project.delete` đúng project Add Project đã tạo cho nó (tên = tên thư mục; từ SOL-FE-E2E-005 Add Project tạo project riêng), xoá thư mục tạm (cả `<repo>-test-02`), rồi kiểm không còn repo/project `e2e-ui-repro-*`; spec cũng khẳng định project mang tên thư mục. Ảnh từng bước ở `test-results/` (ngoài git).

Repo tạm nằm ở thư mục có mọi thư mục cha nhỏ (mặc định `/private/tmp`, hoặc `ORCA_E2E_REPRO_BASE`), KHÔNG phải `$TMPDIR` trên macOS: hộp thoại Browse folder liệt kê mọi thư mục cha của đường dẫn gõ vào, và thư mục có ~1000 mục làm đáp ứng vượt 32 KiB nên agent của dev server bị ngắt kết nối (xem TASK-FE-E2E-015, phát hiện phụ).

## Add Project tạo project riêng, chặn trùng, Request chọn project, ô Answer (`run_ui_flow.py --real addproject`)

Spec `ui-live/add-project-own-project-request-real.web.e2e.ts` (TASK-FE-E2E-027, nghiệm thu SOL-FE-E2E-005) tái hiện đợt test "thêm repo bằng Add Project, tạo Request chọn project, Claude thật phân tích chỉ đọc, đọc ở tab Analysis", hoàn toàn qua giao diện, với Claude Code **thật** (cùng lưới an toàn `--real`: `real-flow-harness.ts`). Hai repo git tạm `/private/tmp/e2e-ui-addproj-<mmddhhmmss>-alpha` (mồi nhử) và `-bravo` (repo được hỏi), mỗi repo có README nhận biết, hàm `alpha_mul/alpha_div` hoặc `bravo_add/bravo_sub`, hook chặn đọc/ghi ngoài thư mục (log của hook trong repo chứng minh claude chạy Ở ĐÂU). Không bao giờ dùng repo/project của người dùng.

```bash
ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real addproject
ORCA_E2E_ADDPROJECT_STOP=projects python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real addproject   # chỉ nửa không AI (Add Project + chặn trùng), ~25 giây
node --test tests/stages/e2e/orca-flows/ui-live/live-answer-snippet.test.ts                              # luật chọn đoạn câu trả lời để so với màn hình, không cần stack
```

| Bước | Khẳng định (chứng cứ) |
|---|---|
| Add Project alpha, rồi bravo (host `dev-local-mac`, Browse folder, Select folder, Add Git Project) | MỖI lần đúng 1 project MỚI, tên = tên thư mục, `devServerId` = dev-local-mac, chứa đúng 1 repo (cũng gắn host); không project cũ nào có thêm repo (`project.list`/`repo.list` trước/sau); UI gửi đúng 1 `project.create {name, devServerId, repoPath}` và không `repo.add`; sidebar có hai project riêng |
| Add Project alpha lần hai | khối `add-project-duplicate` nêu tên project đang chứa, nút `add-project-open-existing`; không có `project.create`/`repo.*` ghi nào được gửi; số project/repo không đổi; "Open that project" đóng hộp thoại và worktree đang chọn thuộc repo của alpha; đường dẫn gõ có `/` cuối vẫn bị chặn |
| Gateway `project.create {devServerId, repoPath=alpha}` (kênh tắt, chỉ để kiểm hàng rào; dữ liệu tạm của lượt chạy) | lỗi `PROJECT_REPO_ALREADY_REGISTERED ... already registered in project "<tên>" (<id>)`; không tạo project nào |
| Requests → Create request | chọn alpha rồi bravo: ghi chú `create-request-repo-note` theo project đang chọn (đúng repo, đường dẫn, host); tạo Request tiêu đề `e2e-ui-<mmddhhmmss> addproj question`; Request thuộc project bravo; Claude thật phân loại, xác nhận `Question` size `S`, trả lời Clarification trên UI |
| Tab Analysis (không bấm Approve) | `answer-view` hiện nội dung `answer_markdown` (KHÔNG "No data"), một đoạn của câu trả lời trong `solution.list` có trên màn hình, `answer-confidence` dạng `NN%`, nhắc tới hàm/module của bravo, `answer-citations` có tham chiếu file của bravo, không có gì của alpha |
| Chạy trên repo đúng | log hook của bravo > 0 lệnh gọi chỉ `Read/Glob/Grep`; log hook của alpha rỗng; cả hai repo: cây sạch, HEAD/nhánh/worktree không đổi |
| Dọn dẹp (cả khi đỏ) | huỷ Request qua UI, rồi `request.cancel` các Request mở của project của lượt chạy, `repo.rm` + `project.delete` CHỈ project/repo mang tên/đường dẫn của lượt chạy (project của người dùng nếu lỡ chứa repo của lượt chạy chỉ bị gỡ repo, không bị xoá), xoá thư mục; kiểm 0 project/repo còn lại, `report_orphan_requests.py` 0 orphaned |
| Dữ liệu thật của người dùng | chụp trước/sau: mọi project + repo (kể cả 'Demo (this Mac)', 'codex'), mọi Request đã kết thúc (kể cả #1136), và dấu vân tay git (HEAD, refs, worktree, config, file đổi) của MỌI repo đã đăng ký còn trên đĩa (kể cả repo codex) qua `ORCA_E2E_GUARD_PATHS`; khác một byte là đỏ |

Console của trình duyệt qua `live-console-audit.ts` (mục ngoài allowlist làm đỏ). Ba mục mới có lý do trong allowlist, đều có từ trước và không thuộc ba lỗi đã sửa: cảnh báo Radix thiếu `Description` của hộp thoại Add Project (`radix-dialog-description-add-project`), `fs.scanNestedRepos` agent chưa có (`scan-nested-agent-method-missing`), kênh `orcaProfiles.findProjectProfiles` chưa có ở gateway (`profiles-channel-not-in-gateway`).

## Agent ghi mã THẬT trên repo của người dùng, từ UI (`run_ui_flow.py --real deepseek`)

Thay cho bộ Playwright mock `request-web` (đã gỡ, TASK-FE-E2E-029): sáu spec `ui-live/deepseek-*-real.web.e2e.ts` đi qua UI thật, gateway thật, backend thật và Claude Code thật của dev server, trên repo `/Users/datbeohbbh/Desktop/stuff/agentic/deepseek-harness` (đổi bằng `ORCA_E2E_DEEPSEEK_REPO`). Thiết kế an toàn và lý do: `specs/frontend/bugs/e2e-acceptance-v1/solutions/SOL-FE-E2E-006-*.md`; ánh xạ từng ca mock sang spec thật: `specs/frontend/bugs/e2e-acceptance-v1/COVERAGE-request-web-mock-to-real.md`.

```bash
ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek            # cả bộ, ~25 phút
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-ui|deepseek-readonly|deepseek-review|deepseek-execute|deepseek-task              # một phần
node --test tests/stages/e2e/orca-flows/ui-live/live-main-checkout-rules.test.ts                                                                      # luật bảo vệ repo, không cần stack
python3 -m unittest tests/stages/e2e/orca-flows/test_agent_push_block.py                                                                              # khối chặn push, không cần stack
```

| Spec | AI | Khẳng định chính |
|---|---|---|
| `deepseek-ui` | không | Add Project trên repo thật (tên mặc định = tên thư mục, sửa được, project riêng gắn host), chặn trùng có/không `/` cuối, "Mở project đó", nhãn host của repo, worktree tạo từ "+" nằm cạnh repo `<repo>-<tên>` và bị gỡ cùng nhánh khi xong |
| `deepseek-readonly` | thật, chỉ đọc | câu hỏi về `package.json` gốc: câu trả lời chứa tên gói, `engines.node`, số workspaces đọc từ repo lúc chạy; Create request (ghi chú repo/host, không `source`), danh sách, j/k, số chờ duyệt, tab, Escape |
| `deepseek-review` + `deepseek-hotfix` | thật | tuỳ chọn/so sánh, Reject lý do ngắn bị khoá, Mod+Enter, Request về backlog, Reopen (khởi động lại từ xác nhận loại), Regenerate, chọn phương án (có xử lý khi dịch vụ từ chối vì phương án không trả lời một AC), hotfix không có thanh quyết định |
| `deepseek-execute` | thật, GHI mã | change_request có Phase: Plan tab khớp `task.list`, Reject ngắn bị khoá, Tree/Graph, "View graph", cổng Phase (Start bị khoá tới khi duyệt, Phase theo thứ tự phụ thuộc), agent viết `scripts/e2e-sandbox/slug.mjs` + test trong worktree task, đọc lại từ đĩa và chạy `node --test`, chỉ thư mục đó thay đổi, task sheet sau run (Result, 1440/700/390 px, Escape) |
| `deepseek-task` | thật, GHI mã | Plan một task: agent đang chạy hiện trên task sheet, dev server bận rồi rảnh, dòng hoạt động trực tiếp, phát lại sau tải lại, Open in Git thấy file agent đổi, rồi gỡ worktree và Open in Git giải thích đúng |

An toàn (mọi spec `deepseek-*`): checkout chính, mọi ref, worktree của người dùng và thư mục cạnh repo được lấy dấu vân tay, so mỗi 15 giây và sau dọn dẹp; `git push` bị chặn bằng `GIT_CONFIG_*` trong môi trường agent (agent khởi động lại cho run rồi khôi phục); fixture từ chối chạy nếu khối chặn chưa có. Không cài được hook `tool-guard` trong repo này (`.claude/settings.json` bị `.gitignore`), nên agent vẫn chạy `--dangerously-skip-permissions` không sandbox (B4): các biện pháp trên PHÁT HIỆN và DỪNG, không NGĂN mọi ghi ngoài worktree.

## Change request THẬT từ backlog của người dùng, chi phí loại C (`run_ui_flow.py --real ci-cd-kit`)

**CẢNH BÁO CHI PHÍ: loại C, tốn lượt AI thật.** Một Plan OpenSpec thật tốn vài đến hơn chục USD (đo trước đây: 1 đến 13 USD), cộng Analysis do agent viết và các task chạy thật bằng `claude --dangerously-skip-permissions`, tối đa 110 phút. Chỉ chạy khi chủ repo đồng ý rõ ràng cho từng lượt; KHÔNG nằm trong `--real all`.

Spec `ui-live/ci-cd-kit-real.web.e2e.ts` lấy mục backlog `docs/backlog/BL-E07_ci-cd-kit.md` của repo `vnp-qa-platform` (đổi bằng `ORCA_E2E_CICDKIT_REPO`) làm Request mới trên project **có sẵn** `vnp-qa-platform` (không Add Project, không repo tạm, không xoá gì của người dùng), qua UI như người dùng:

1. Create request: tiêu đề `[TMS] CI-CD-KIT (BL-E07)`, nội dung = mục tiêu, hiện trạng, phạm vi, tiêu chí nghiệm thu đọc từ BL-E07 lúc chạy (ghi rõ nguồn) cộng ba dòng giới hạn thực thi (không docker/deploy/push), "Start from" chọn nhánh `claude/tms-bug-triage-report-09f310` trong danh sách (không còn nhánh đó thì dừng trước khi tốn gì).
2. AI phân loại, xác nhận `change_request` cỡ M; Clarification được đọc rồi trả lời từng câu bằng câu trong BL-E07 (`ci-cd-kit-backlog.ts`), chỉ dùng câu "See BL-E07 acceptance criteria" khi không câu nào khớp.
3. Analysis (CR-REQ-046: tiếng Việt, đo tỉ lệ chữ có dấu) rồi duyệt; Plan OpenSpec **tự hiện, không bấm Generate** (CR-REQ-044); tab Documents (CR-REQ-045) được mở từng tab, chụp ảnh, kiểm không có bí mật; Plan được đọc (số task, lệnh không an toàn: `ci-cd-kit-plan-safety.ts`).
4. "Approve and run all steps" (CR-REQ-043), agent thật chạy từng task trong worktree `<repo>-task-<id>` (CR-REQ-039: kiểm bằng `git merge-base --is-ancestor` rằng mọi worktree, cả `request/<n>-proposal`, là hậu duệ của tip nhánh đã chọn). Thời gian từng bước, file agent viết (diff stat, kiểm cú pháp, không chạy) và số liệu chi phí của sản phẩm vào báo cáo tiếng Việt `ci-cd-kit-report.vi.md` cạnh ảnh chụp trong `test-results/`.

```bash
ORCA_E2E_CICDKIT_STOP=dialog python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit   # diễn tập: điền hộp thoại, chọn nhánh, KHÔNG tạo Request, không tốn AI
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit                               # lượt thật (loại C)
node --test tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-backlog.test.ts tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-plan-safety.test.ts   # không cần stack
```

Kết quả thật (2026-10-20, Request #2297, 13 phút, ~3,5 USD cho Analysis + Plan): xanh; Analysis tiếng Việt, Plan tự hiện sau 3 phút, tab Documents 6 tài liệu, run-all chạy 4 Phase / 9 task trong ~6 phút, mọi nhánh là hậu duệ của tip. Điều cần biết khi đọc kết quả: (1) cả 9 task chạy chung MỘT worktree `<repo>-task-<id của task đầu>` và để file chưa commit; worktree `request/<n>-proposal` do Orca tự gỡ sau khi archive (nhánh còn); (2) agent thật có thể cài gói vào user-site của Python (`pip install --user pytest` đã xảy ra), ngoài worktree: hook tool-guard không có ở repo này; (3) Analysis có thể hỏi chặn (ask-back): câu trả lời lấy từ BL-E07 bằng `answerAskBackFromBacklog`; (4) kiểm tĩnh chạy Python với `-E` trong thư mục rỗng (không `-I`, vì PyYAML nằm ở user-site).

An toàn: như `deepseek` (dấu vân tay checkout, mọi ref, worktree của người dùng; `git push` bị chặn cho cả lượt; watchdog 15 giây), thêm: agent được khởi động với `AGENT_WORK_DIR` là thư mục cha của hai repo trong lượt này (relay chỉ ghi trong thư mục đó, mà worktree nằm cạnh repo) rồi khôi phục như cũ. Worktree và nhánh mới **được giữ lại** để xem (thư mục cha chỉ được thêm mục `vnp-qa-platform-*`, `ORCA_E2E_GUARD_KEEP_NEW_PREFIXES`); mọi thay đổi khác là đỏ. Dừng lỗ: task thất bại hai lần, lượt Plan lỗi không tạm thời, Request rời `executing`, hay 20 phút không tiến triển thì kill `claude` của lượt, huỷ Request qua UI và đỏ.

## Bảo vệ repo của người dùng khi chạy thật (pilot, `pilot_guard.py`)

Dùng cho mọi repo của người dùng mà agent thật làm việc trên đó (pilot vnp-qa-platform), không phụ thuộc deepseek-harness. Quy tắc lấy từ `ui-live/live-main-checkout-rules.ts`: Orca chỉ được thêm nhánh `refs/heads/*` và worktree `<repo>-*` cạnh repo; đổi HEAD, tệp, cấu hình git cục bộ, ref đã có, tag, stash, worktree của người dùng đều là vi phạm (ref `refs/remotes/*` được phép dịch chuyển vì tạo worktree có fetch).

```bash
python3 agent_push_block.py start            # khởi động lại agent với khối chặn push; in "push block ON" cho từng tiến trình agent
python3 agent_push_block.py probe --repo <repo>   # mọi dạng git push --dry-run phải thất bại cục bộ
python3 pilot_guard.py snapshot --repo <repo> --out before.json
python3 pilot_guard.py orca-snapshot --out orca-before.json     # project và Request của người dùng trong Orca
python3 pilot_guard.py watch --repo <repo> --before before.json --interval 15   # thoát 1 khi repo của người dùng đổi
# ... chạy ...
python3 pilot_guard.py compare --repo <repo> --before before.json              # trong lúc chạy: chỉ cho phép phần Orca được thêm
python3 pilot_guard.py compare --repo <repo> --before before.json --strict     # sau khi dọn: phải như trước
python3 pilot_guard.py orca-compare --before orca-before.json --allow-project <id của project pilot> --strict
python3 report_orphan_requests.py; python3 report_orphan_worktrees.py --repo <repo>
python3 agent_push_block.py restore          # trả agent về như cũ
```

Biện pháp này phát hiện và dừng, không ngăn mọi thao tác ghi ngoài worktree (rủi ro B4). Test: `python3 -m unittest tests/stages/e2e/orca-flows/test_pilot_guard.py`.

## Luồng tri thức từ dòng lệnh, Claude thật (`run_cli_flow.py --real knowledge`)

Chạy cả luồng Request `change_request` cỡ M bằng CLI `orca` trên một repo Go tạm (`knowledge_fixture/`: ba tài liệu ngắn và một dịch vụ nhỏ), đồ thị tri thức dựng bằng `tdd-agent init --no-prose` (không gọi mô hình), project tạm trên `dev-local-mac` với engine `openspec` và Solution bằng agent. Kiểm tự động từng bước: Solution trích đồ thị (`design_references`, có snapshot, có phần tử đã đọc, run báo số lần gọi tool tri thức), gói OpenSpec đạt kiểm cấu trúc (`STRUCTURE_OK`), mọi task đủ `Serves/Code/Verify/Done when`, mọi tiêu chí chấp nhận có scenario, mọi task chạy kiểm tra của nó và thoát 0, Request `completed`, gói được archive trên nhánh proposal, không có nhánh remote-tracking (không push), bàn giao được liệt kê. Cỡ M được chọn có chủ ý: chính sách duyệt mặc định cho người báo tự duyệt, nên stack một người dùng chạy được mà không đổi chính sách.

```bash
python3 tests/stages/e2e/orca-flows/run_cli_flow.py --real knowledge          # dọn sạch khi xong
python3 tests/stages/e2e/orca-flows/run_cli_flow.py --real knowledge --keep   # giữ repo tạm, project và Request để xem
```

Điều kiện: stack local, dev server đã duyệt và đang nối, `claude` đã đăng nhập, `tdd-agent`, `openspec`, `go` trong `PATH`, bốn skill tri thức trong `~/.claude/skills/`. Tốn lượt AI thật (khoảng 15 đến 25 phút) nên không có job CI. Mã thoát: 0 đạt, 1 có kiểm tra hỏng, 2 thiếu điều kiện.

## Bốn trạng thái kết quả

| Trạng thái | Nghĩa |
|---|---|
| `PASS` | hành vi đúng tiêu chí nghiệm thu |
| `FAIL` | sản phẩm sai (hoặc một điều kiện mà chính sản phẩm phải đảm bảo bị hỏng) |
| `BLOCKED` | môi trường thiếu thứ test cần (API key, agent, docker, MCP bật). **Không** nói gì về chất lượng sản phẩm |
| `SKIP` | test tuỳ chọn chưa được bật |

Tách `BLOCKED` khỏi `FAIL` để báo cáo không lẫn lỗi sản phẩm với việc thiếu môi trường, và để phiên `--strict` buộc phải dựng đủ môi trường trước khi ký nghiệm thu.

## Nguyên tắc thiết kế

- **Oracle độc lập với mã.** `expected_request_flows.py` ghi lại bảng 11 loại Request từ README v6 mục 3.4 và CR-REQ-003; test so sánh với đáp ứng của `request.flow`, không sinh kỳ vọng từ chính registry Go.
- **Đi qua cổng công khai.** Mọi bước trả lời Clarification, xác nhận loại, huỷ… đều qua kênh WS. Nhờ vậy test bắt được cả lỗi "RPC có nhưng gateway không mở" (xem lỗi D1 trong báo cáo).
- **Tự dọn.** Dữ liệu tạo ra mang tiền tố `e2e-<mmddhhmmss>`; Request được huỷ, task và project được xoá ở cuối. Template workflow không có API xoá nên vẫn còn (tên có tiền tố để dễ nhận).
- **Giãn nhịp ghi.** `request-service` giới hạn ghi 5 lần/giây (burst 10). Client tự thử lại khi gặp `REQUEST_RATE_LIMITED`; riêng `RQL-20` tắt thử lại để chứng minh bộ giới hạn hoạt động. Số lần bị giới hạn được in cuối mỗi lượt.

## Danh mục test

| Suite | ID | Kiểm tra |
|---|---|---|
| `platform` | PLT-01..11 | UI phục vụ được, service worker, bundle JS; đăng nhập đúng/sai; `/auth/me`; WS ẩn danh bị chặn; container chạy, không crash-loop; gateway tới được `request-service` và `workflow-service` |
| `devserver` | DEV-01..07 | dev server đăng ký, được duyệt, đang kết nối; tạo project gắn dev server (agent kiểm tra đường dẫn repo); từ chối đường dẫn không tồn tại; từ chối dev server lạ |
| `workflow` | WFL-01..13 | DAG tuyến tính chạy hết; thứ tự wave; fan-out/fan-in; điều kiện sai; biểu thức hỏng; chu trình, phụ thuộc thiếu, id trùng không bao giờ `completed`; trạng thái cuối bất biến; bước `shell` chạy trên dev server (đích `server:` và `project:`) |
| `request` | RQL-01..20 (không có 18) | cờ luồng bật/tắt và lối thoát an toàn khi tắt; tạo Request, idempotency, phân loại luôn kết thúc ở `awaiting_type_confirmation`; AI đề xuất loại; xác nhận loại, version cũ, đồng thời; Clarification (liệt kê, nháp, hoàn tất, không ghi đè); lọc danh sách; giới hạn tốc độ |
| `matrix` | MTX-01..13 | mỗi trong 11 loại: trạng thái đầu sau khi xác nhận và đường đi/cổng/Phase đúng bảng; với 6 loại có phân tích chỉ đọc, bước kiểm trạng thái đầu cần trả lời Clarification (khởi động agent thật) nên là `SKIP` nếu thiếu `ORCA_E2E_READONLY_ANALYSIS=1` hoặc `--ai-stub` (bảng luồng vẫn được kiểm); đúng 11 loại; loại câu hỏi khớp điều kiện hợp lệ |
| `exits` | EXT-01..11 | trả backlog (cần lý do), mở lại, huỷ (cần lý do), trạng thái cuối bất biến, đổi loại (chỉ được mở rộng), Request con, id lạ |
| `ai` | AIS-01..06 | khi không có AI, sinh Solution/Plan thất bại an toàn (run `failed` có mã lỗi, Request đứng yên; với `--ai-stub` mã phải là `REQUEST_AI_NOT_CONFIGURED`); khi có AI, sinh được Solution (kind `solution`, 2..4 option, một option khuyến nghị) và Plan (đúng container theo shape); hộp thư duyệt và backlog trả lời. `AIS-01/03` chỉ chạy khi không có AI nên chỉ hết `BLOCKED` ở pha 2 của `--ai-stub` |
| `analysis` | RAN-01..03 | phân tích chỉ đọc qua agent: Request `bug`, `spike`, `question` tự chạy chẩn đoán / findings / answer sau khi trả lời Clarification, run `succeeded`, tài liệu đúng `kind`, Request dừng ở `awaiting_analysis_approval` và có Approval `solution` / `findings` / `answer` đang chờ. Chỉ chạy với `--ai-stub` hoặc `ORCA_E2E_READONLY_ANALYSIS=1` (xem trên) |
| `plan` | PLN-01..08 | giai đoạn Plan với AI (thật hoặc `claude` giả): đề xuất chạy nền và không ghi gì; đề xuất hợp lệ; vòng phụ thuộc bị từ chối; commit ghi Plan, chuyển `awaiting_plan_approval` và mở Approval `task_list`; commit lặp lại không nhân đôi; từ chối Approval về backlog; sai trạng thái; `change_request` đi theo Phase từ Solution đã duyệt. Không bao giờ duyệt Plan (sẽ chạy agent AI trên máy dev) |
| `aistub` | AST-01..06 | chỉ với `--ai-stub`: agent tìm thấy stub qua `PATH`; mọi lần gọi `claude` đều bị khoá chặt: `ai.complete` là `-p --output-format json --tools ""` trong thư mục tạm rỗng với prompt qua stdin, phân tích chỉ đọc là `--print` với `--permission-mode plan`, chỉ công cụ `Read,Glob,Grep`, cwd đúng là repo của project, không cờ bỏ qua quyền, không khoá trong prompt (`AST-02`); stub nhận ra mọi prompt (`AST-03`); các phân tích chỉ đọc thật sự do stub trả lời (`AST-06`); `claude` chưa đăng nhập là `REQUEST_AI_NOT_CONFIGURED` và không bị spawn; AI đề xuất loại cần người duyệt (`hotfix`) vẫn kết thúc ở `awaiting_type_confirmation` (`AST-05`; từng FAIL vì lỗi sản phẩm Request kẹt `classifying` khi tenant không có người duyệt hợp lệ khác người báo, nay xanh) |
| `notfound` | NFC-01..08 | id lạ là NotFound ở task, project (kể cả id sai định dạng), workflow, request, clarification, approval |
| `task` | TSK-01..10, MCP-01..02 | CRUD task, task con và cây con, không đặt được `in_progress` bằng update, xoá task lạ là `NOT_FOUND`, `task.execute` (tuỳ chọn); MCP công bố metadata và từ chối ẩn danh |
| `request_base_ref` | BRF-01..10 | nhánh gốc của worktree theo Request (CR-REQ-039), trên repo fixture LedgerLite với git thật: tạo không `baseRef` (rỗng, chưa khoá, `request.flowStatus` có `request.baseRef`); tạo với `release/1.x`; `-x`/`a..b` là `REQUEST_BASE_REF_INVALID`; ref lạ là `REQUEST_BASE_REF_NOT_FOUND` kèm gợi ý; `request.setBaseRef` tăng version, version cũ là `REQUEST_VERSION_CONFLICT`. `BRF-06/07` là bằng chứng: Request `task` (native) đi tới duyệt Plan, task-service tạo worktree của task, và `git rev-parse HEAD` trong worktree bằng đỉnh `release/1.x` (khi đã chọn) hoặc đỉnh `main` (khi không chọn), dù checkout chính của repo đang ở nhánh thứ ba. `BRF-08`: có worktree rồi thì `setBaseRef` là `REQUEST_BASE_REF_LOCKED` (lý do `task` hoặc `execution`). `BRF-10`: `repo.listBranches` (hợp đồng 2.12) trả `main`, `feature/other`, `release/1.x` là nhánh local, không có `HEAD`, không lặp. `BRF-09` (worktree đề xuất OpenSpec) cần Claude thật và tốn tiền nên `SKIP` trừ khi đặt `ORCA_E2E_OPENSPEC=1`. Duyệt Plan khởi động agent: với `--ai-stub` agent là `claude` giả (từ chối ghi nên task hỏng ngay sau khi worktree đã có, đúng thứ cần kiểm); với Claude thật cần `ORCA_E2E_EXECUTE_AGENT=1`, nếu không `BRF-06..08` là `SKIP`. Chạy: `python3 run_acceptance.py --ai-stub devserver request_base_ref`. Suite xếp sau `aistub` để các lời gọi `claude` của bước thực thi không làm `AST-02/03` đỏ |
| `analysis_revision` | ARV-01..13 | chỉnh phân tích bằng prompt (CR-REQ-041): change request chỉnh hai lần (3 phiên bản, mỗi vòng một Approval mới), bug/spike/question chỉnh một lần, dòng Activity `analysis.revision_requested` không lộ nội dung, approver không phải reporter chỉnh được (approver là thành viên dự án; chính sách ghi qua `docker exec orca-go-postgres`) còn người lạ bị từ chối, bấm đúp một idempotencyKey, lượt chỉnh thứ hai khi một lượt đang chạy, lượt chỉnh lỗi vẫn thao tác được; cần `--ai-stub` (mã `[stub:fail-once]`, `[stub:slow]`). ARV-13 (hotfix) BLOCKED trên stack một người dùng |
| `approve_run_all` | ARA-01..07 | "Approve and run all steps" (CR-REQ-043), chỉ với `--ai-stub`, dự án `native` trên repo LedgerLite, `change_request` có Phase: `ARA-01` duyệt thường ở dự án `step_by_step` (`executionMode` step_by_step, Approval `phase` chờ, không Phase nào tự chạy, duyệt Phase rồi `request.startPhase` vẫn chạy); `ARA-02` duyệt kèm `executionMode:"continuous"` (executing, continuous, không Approval `phase`, Phase đầu tự bắt đầu); `ARA-03` chạy hết mọi Phase không bấm thêm (số `phase.started` = số Phase); `ARA-04` `step_by_step`/`bogus`/kèm `approval.reject` là `REQUEST_EXECUTION_MODE_INVALID`, trên Approval `solution` là `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`, Approval vẫn `pending`; `ARA-05` nhấp đúp trả cùng kết quả; `ARA-06` Request đã ghim `step_by_step` (chạy, về backlog, mở lại) bị `REQUEST_EXECUTION_MODE_ALREADY_PINNED`; `ARA-07` người duyệt thiếu quyền execute bị `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`: tài khoản `member` tạo bằng `admin.createUser`, chính sách `plan`/`task_list` ghi qua `docker exec orca-go-postgres`; nhưng cây Plan do admin sở hữu nên `ExecutionAccess` cấp quyền execute cho member khi họ duyệt: lời gọi thành công và ca là `BLOCKED` có lý do (lỗi chỉ có thể xảy ra với người duyệt là chủ cây mất quyền; đã có unit và contract test). Lần cấp quyền này sinh sự kiện `task.grant_received`, nơi từng lộ lỗi outbox của task-service (xem bản ghi `TASK-REQ-043-06`). `claude` giả trả lời cả prompt thực thi task (một khối kết quả `done`, không đổi tệp nào) nên mọi Phase chạy miễn phí. Chạy: `python3 run_acceptance.py --ai-stub devserver approve_run_all`. Xếp sau `aistub` vì prompt thực thi làm `AST-02` đỏ nếu chạy trước |
| `output_language` | OLG-01..06 | ngôn ngữ đầu ra của AI (CR-REQ-046), chỉ với `--ai-stub`: `OLG-01` mọi prompt Solution, Plan, phân tích chỉ đọc có chỉ thị `Output language: Vietnamese.` (`claude` giả ghi `outputLanguage` vào `calls.jsonl`); `OLG-02` Solution của `change_request` tiếng Anh ra tiếng Việt, id/enum/cỡ nguyên; `OLG-03` Plan native của bug tiếng Việt, giữ nhãn `test:regression`, qua validator, mở Approval; `OLG-04` Diagnosis và Answer tiếng Việt đúng `kind` và schema; `OLG-05` Request thân tiếng Việt và thân tiếng Anh nhận cùng chỉ thị; `OLG-06` chỉ chạy khi `ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en` (stack đã đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en`; khi đó các ca khác kỳ vọng tiếng Anh) nếu không là `SKIP`. Chạy: `python3 run_acceptance.py --ai-stub devserver output_language`. Mã mặc định compose là `vi` |
| `request_auto_plan` | APG-01..11 | Plan tự sinh sau khi duyệt Analysis (CR-REQ-044), chỉ với `--ai-stub`, dự án `native` trên repo LedgerLite với `planAutoGenerate` bật: `APG-01` duyệt Analysis của `bug` -> trong 90 giây Request ở `awaiting_plan_approval` có `planTaskId` và đúng một Approval Plan, từ một run khoá `plan-auto-...` (client không gọi `generatePlan`), `autoPlan.state=done`; `APG-02` `fail_plan_next` `2:invalid` -> job `gave_up` `REQUEST_PLAN_AI_INVALID_JSON` sau đúng một run, Request ở `planning`, rồi `request.retryPlan` ra Plan; `APG-03` `1:error` -> run 1 `failed`, run 2 (khoá `-2`) thành công; `APG-04` dự án không dev server -> job `skipped`, không run; `APG-05` `spike`/`question` hoàn thành sau Analysis và `APG-05b` hotfix đi từ chẩn đoán tới Approval của task sửa (người duyệt `pre_deploy` thứ hai, chính sách ghi qua `docker exec orca-go-postgres`): không job, không run; `APG-06` công tắc dự án tắt -> `planning`, không run, `autoPlan.enabled=false`, bấm tay vẫn chạy; `APG-07` bấm Generate ngay sau khi duyệt, lúc run của người đang chạy (`fail_plan_next` `1:slow`) -> job nhập vào run đó (một run, một Plan); `APG-07b` nếu run của người xong trước khi job bắt đầu thì có thể hai run (stub nhanh) nhưng vẫn một Plan, một Approval, không lỗi; `APG-08` hỏi sửa Plan (`feedback`) không tạo job `event`/`retry` (chỉ job `revision`), từ chối Plan không tạo gì; `APG-09` `task` đã có tiêu chí chấp nhận khi xác nhận loại (hàng được ghi tiêu chí vì không kênh gateway nào đặt tiêu chí lúc tạo) -> Plan tự ra (khoá `plan-auto-ty-...`); `APG-09b` `task` phải trả lời Clarification chuẩn bị -> ở `planning`, không job (trigger `information_provided` không bao giờ tự sinh Plan, D3); `APG-10` OpenSpec thật loại C (`ORCA_E2E_OPENSPEC=1`, 7 đến 13 USD, không tự chạy; ca OpenSpec thật chạy qua `ui-live/plan-documents.web.e2e.ts`); `APG-11` Request cũ chờ ở `planning` từ trước tính năng không có job, và sự kiện `status_changed` giả dated 3 giờ trước (ghi vào `request.outbox_events`) bị bỏ qua với dòng log "older than the start window". Công tắc lỗi là tệp `fail_plan_next` (`<số lần>:<error|invalid|slow>`) trong thư mục trạng thái của `claude` giả; số run và job đếm bằng `docker exec orca-go-postgres` (thiếu docker -> `BLOCKED`). Xếp sau `aistub`. Chạy: `python3 run_acceptance.py --ai-stub devserver request_auto_plan` |
| `plan_revision` | PRV-01..15 | chỉnh Plan bằng prompt và đọc tài liệu Plan (CR-REQ-045), chỉ với `--ai-stub`, Plan native của `bug`: `PRV-01` `request.planRevisions` một bản `current`, `planProposal {revision:1}`; `PRV-02` chỉnh hai lần (`request.generatePlan` có `feedback` và `idempotencyKey`, **không** gọi commit: job `revision` của server commit) -> 3 bản, mỗi vòng Approval cũ `cancelled` (`revision_requested`) và đúng một Approval mới, `revisionFeedback` đúng chữ, ghi chú Plan có dấu `[revised, round N, asked: ...]`; `PRV-03` đúng hai dòng Activity `plan.revision_requested`, không chữ phản hồi; `PRV-04` 2000 ký tự được, 2001 là `REQUEST_PLAN_FEEDBACK_TOO_LONG`; `PRV-05a/b` approver là thành viên dự án chỉnh được (ghi là tác nhân), thành viên không là approver bị `REQUEST_PLAN_FORBIDDEN`, người lạ bị chặn (tài khoản tạo bằng `admin.createUser`, chính sách `plan`/`task_list` ghi qua `docker exec`); `PRV-06` bấm đúp một `idempotencyKey` là một run, ask thứ hai khi đang chạy (`[stub:slow]`) là `REQUEST_PLAN_REVISION_IN_PROGRESS`; `PRV-07` lượt chỉnh lỗi (`[stub:fail-once]`) để Request ở `planning` không còn Approval, job `revision` `gave_up`, Generate không chữ chạy lại với cùng ask; `PRV-08` hotfix bị `REQUEST_PLAN_REVISION_NOT_APPLICABLE` (từng FAIL: hotfix không ghi `planTaskId` nên chặn bị bỏ qua, ask ở cổng huỷ Approval của task sửa; đã sửa ở `checkPlanRevisable`); `PRV-08b` không client nào gọi commit mà các bản đều đã commit; `PRV-09` dev server offline (SKIP: suite không có công cụ ngắt dev server); `PRV-10` 11 vòng liên tiếp (chỉ khi `ORCA_E2E_PLAN_REVISION_LONG=1`); `PRV-11..13` Plan OpenSpec bằng dữ liệu GIẢ gieo bởi `plan_documents_seed.py` (xem dưới): danh sách file với kích thước thật của file bị lược, đọc file che bí mật và báo `omitted`, bản cũ còn chữ hoặc bị lược chữ (`REQUEST_PLAN_REVISION_TEXT_PRUNED`), so sánh file giữa các bản; `PRV-14/15` claude giả tác giả OpenSpec: không làm (xem ghi chú TASK-REQ-045-09). Xếp sau `aistub`. Chạy: `python3 run_acceptance.py --ai-stub devserver plan_revision` |

### Dữ liệu GIẢ cho Plan OpenSpec (`plan_documents_seed.py`)

`claude` giả không tác giả được một thay đổi OpenSpec (cần shim `openspec`, đồ thị tri thức, worktree đề xuất và khối kết quả; đánh giá khó trong nửa ngày), nên phần xem tài liệu Plan OpenSpec chạy trên **dữ liệu gieo vào cơ sở dữ liệu `request` của stack local** (`docker exec orca-go-postgres`): không bao giờ dùng trên stack dùng chung.

```bash
python3 tests/stages/e2e/orca-flows/plan_documents_seed.py seed  <request-id>   # dòng cuối là JSON: secret, omittedPath, omittedSize, changeId, versions, prunedVersion
python3 tests/stages/e2e/orca-flows/plan_documents_seed.py clean <request-id>   # xoá hàng đã gieo, trả chế độ run và engine như cũ
python3 -m unittest tests/stages/e2e/orca-flows/test_plan_documents_seed.py      # kiểm các tệp gieo, không cần stack
```

Request phải ở `awaiting_plan_approval` và có ít nhất một run Plan thành công. Seed đặt các run đó thành `agent_proposal`, `solution_engine='openspec'`, chèn dòng `openspec_changes` (sẵn sàng, bốn nhóm tài liệu, một spec chỉ còn dấu `<omitted size=300000 ...>`, một khóa giả dạng AWS và một dòng `password = ...` còn NGUYÊN trong `design.md` để kiểm che khi đọc) và một dòng `openspec_change_revisions` mỗi run (từ ba run trở lên thì bản 1 không còn chữ). Dùng bởi `PRV-11..13` và `ui-live/plan-documents.web.e2e.ts` ca (b).

## Thêm test mới

Một test là hàm không tham số; ném `AssertionError` khi sai, `Blocked("lý do")` khi thiếu môi trường, `Skipped("lý do")` khi chưa bật. Đăng ký bằng `rec.run("ID", "Tiêu đề", fn)` trong file suite. Dùng `ctx.on_cleanup(...)` cho mọi dữ liệu tạo ra. Đặt tên file theo nghiệp vụ, không dùng tên chung chung.

## CI

`.github/workflows/orca-flows-e2e.yml` chạy bộ này bằng `--ai-stub` qua `ci/run-orca-flows-ci.sh`: dựng stack bằng `docker-compose.yml` + `docker-compose.local.yml` (Vault dev-mode trong job),
chạy Dev Server Agent ngay trên runner, hằng đêm `platform workflow request matrix exits ai plan`, mỗi Chủ nhật `--strict` với `MCP-01/02` được phép BLOCKED, và có `workflow_dispatch`.
Script từ chối chạy ngoài CI (nó kết thúc bằng `docker compose down -v`). Chưa chạy trên runner thật; chưa là cổng chặn PR (điều kiện: 5 đêm xanh liên tiếp).
`.github/workflows/backend-go-request-contracts.yml` chạy trên PR: method backend gọi agent, dial gRPC qua `grpcmw`, test parity RPC-kênh và tệp vàng danh mục kênh của api-gateway, và kiểm chính `claude` giả.
