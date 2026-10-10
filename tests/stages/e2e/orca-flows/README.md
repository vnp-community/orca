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
# Chỉ Playwright, khi agent đã chạy sẵn (tên dev server qua ORCA_E2E_DEV_SERVER_NAME):
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live
```

Không có `ORCA_E2E_LIVE=1` spec tự bỏ qua. Cần: stack `deploy/dev` chạy, frontend ở `FRONTEND_HTTP_PORT` (đọc từ `deploy/dev/.env`, hoặc `ORCA_E2E_FRONTEND_URL`), admin từ `.env`/`ORCA_E2E_ADMIN_*`, `git` trên máy chạy test. Nếu có `docker` (đặt `DOCKER_HOST` cho Colima), spec so `RestartCount` của container `orca-go-api-gateway` (`ORCA_E2E_GATEWAY_CONTAINER`) trước và sau, và ghi gap nếu gateway restart. Dữ liệu tạo ra mang tiền tố `e2e-ui-<mmddhhmmss>` (project, repo tạm, Request) và bị dọn ở cuối (Request mở bị huỷ trước khi xoá project).

Ảnh từng bước, ảnh lúc lỗi (`99-at-failure.png`) và `trace.zip` (chỉ khi lỗi hoặc có gap) nằm ở `test-results/` (git bỏ qua); cuối mỗi test in tóm tắt gồm các RPC mà UI bị từ chối. Trace chỉ bật **sau** khi đăng nhập vì trace ghi lại giá trị đã gõ; mật khẩu không bao giờ vào artifact. `ORCA_E2E_EXECUTE_AGENT=1` chưa tự động hoá việc duyệt Plan (spec chỉ ghi chú) vì duyệt sẽ chạy agent AI trên máy dev.

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

## Thêm test mới

Một test là hàm không tham số; ném `AssertionError` khi sai, `Blocked("lý do")` khi thiếu môi trường, `Skipped("lý do")` khi chưa bật. Đăng ký bằng `rec.run("ID", "Tiêu đề", fn)` trong file suite. Dùng `ctx.on_cleanup(...)` cho mọi dữ liệu tạo ra. Đặt tên file theo nghiệp vụ, không dùng tên chung chung.

## CI

`.github/workflows/orca-flows-e2e.yml` chạy bộ này bằng `--ai-stub` qua `ci/run-orca-flows-ci.sh`: dựng stack bằng `docker-compose.yml` + `docker-compose.local.yml` (Vault dev-mode trong job),
chạy Dev Server Agent ngay trên runner, hằng đêm `platform workflow request matrix exits ai plan`, mỗi Chủ nhật `--strict` với `MCP-01/02` được phép BLOCKED, và có `workflow_dispatch`.
Script từ chối chạy ngoài CI (nó kết thúc bằng `docker compose down -v`). Chưa chạy trên runner thật; chưa là cổng chặn PR (điều kiện: 5 đêm xanh liên tiếp).
`.github/workflows/backend-go-request-contracts.yml` chạy trên PR: method backend gọi agent, dial gRPC qua `grpcmw`, test parity RPC-kênh và tệp vàng danh mục kênh của api-gateway, và kiểm chính `claude` giả.
