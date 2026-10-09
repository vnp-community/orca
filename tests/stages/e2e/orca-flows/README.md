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
- Stub trả lời theo nội dung prompt do **mã thật** sinh (`BuildClassificationPrompt`, `BuildSolutionPrompt`, `BuildPlanPrompt` trong request-service) và từ chối
  chạy nếu không có `--tools ""`. Prompt lạ thì lỗi rõ ràng chứ không bịa câu trả lời. Mỗi lần gọi (argv, cwd, nội dung thư mục, kích thước stdin) được ghi vào
  `$HOME/.fake-claude/calls.jsonl` để `AST-01..03` kiểm.

**Kiểm chính stub, không cần stack:**

```bash
python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py
bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh
```

Lệnh thứ hai chạy `go test -overlay` (không thêm file vào `backend-go`): dựng prompt bằng hàm thật, cho stub trả lời đúng như agent gọi, rồi đưa vào
`ParseClassificationProposal`, `SolutionOptions.Validate` + kiểm bao phủ AC, `ParsePlanOutput`, `ValidatePlanProposal` và chính sách theo loại, cho cả 11 loại và mọi size. Prompt hoặc
validator đổi mà stub không theo kịp thì lệnh này đỏ trước khi e2e đỏ.

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
| `matrix` | MTX-01..13 | mỗi trong 11 loại: trạng thái đầu sau khi xác nhận và đường đi/cổng/Phase đúng bảng; đúng 11 loại; loại câu hỏi khớp điều kiện hợp lệ |
| `exits` | EXT-01..11 | trả backlog (cần lý do), mở lại, huỷ (cần lý do), trạng thái cuối bất biến, đổi loại (chỉ được mở rộng), Request con, id lạ |
| `ai` | AIS-01..06 | khi không có AI, sinh Solution/Plan thất bại an toàn (run `failed` có mã lỗi, Request đứng yên; với `--ai-stub` mã phải là `REQUEST_AI_NOT_CONFIGURED`); khi có AI, sinh được Solution (kind `solution`, 2..4 option, một option khuyến nghị) và Plan (đúng container theo shape); hộp thư duyệt và backlog trả lời. `AIS-01/03` chỉ chạy khi không có AI nên chỉ hết `BLOCKED` ở pha 2 của `--ai-stub` |
| `plan` | PLN-01..08 | giai đoạn Plan với AI (thật hoặc `claude` giả): đề xuất chạy nền và không ghi gì; đề xuất hợp lệ; vòng phụ thuộc bị từ chối; commit ghi Plan, chuyển `awaiting_plan_approval` và mở Approval `task_list`; commit lặp lại không nhân đôi; từ chối Approval về backlog; sai trạng thái; `change_request` đi theo Phase từ Solution đã duyệt. Không bao giờ duyệt Plan (sẽ chạy agent AI trên máy dev) |
| `aistub` | AST-01..05 | chỉ với `--ai-stub`: agent tìm thấy stub qua `PATH`; mọi lần gọi là `claude -p --output-format json --tools ""` trong thư mục tạm rỗng, prompt qua stdin; stub nhận ra mọi prompt; `claude` chưa đăng nhập là `REQUEST_AI_NOT_CONFIGURED` và không bị spawn; AI đề xuất loại cần người duyệt (`hotfix`) vẫn kết thúc ở `awaiting_type_confirmation` (**`AST-05` đang FAIL**: lỗi sản phẩm đã biết, Request kẹt `classifying` khi tenant không có người duyệt hợp lệ khác người báo) |
| `notfound` | NFC-01..08 | id lạ là NotFound ở task, project (kể cả id sai định dạng), workflow, request, clarification, approval |
| `task` | TSK-01..10, MCP-01..02 | CRUD task, task con và cây con, không đặt được `in_progress` bằng update, xoá task lạ là `NOT_FOUND`, `task.execute` (tuỳ chọn); MCP công bố metadata và từ chối ẩn danh |

## Thêm test mới

Một test là hàm không tham số; ném `AssertionError` khi sai, `Blocked("lý do")` khi thiếu môi trường, `Skipped("lý do")` khi chưa bật. Đăng ký bằng `rec.run("ID", "Tiêu đề", fn)` trong file suite. Dùng `ctx.on_cleanup(...)` cho mọi dữ liệu tạo ra. Đặt tên file theo nghiệp vụ, không dùng tên chung chung.

## CI

`.github/workflows/orca-flows-e2e.yml` chạy bộ này bằng `--ai-stub` qua `ci/run-orca-flows-ci.sh`: dựng stack bằng `docker-compose.yml` + `docker-compose.local.yml` (Vault dev-mode trong job),
chạy Dev Server Agent ngay trên runner, hằng đêm `platform workflow request matrix exits ai plan`, mỗi Chủ nhật `--strict` với `MCP-01/02` được phép BLOCKED, và có `workflow_dispatch`.
Script từ chối chạy ngoài CI (nó kết thúc bằng `docker compose down -v`). Chưa chạy trên runner thật; chưa là cổng chặn PR (điều kiện: 5 đêm xanh liên tiếp).
`.github/workflows/backend-go-request-contracts.yml` chạy trên PR: method backend gọi agent, dial gRPC qua `grpcmw`, test parity RPC-kênh và tệp vàng danh mục kênh của api-gateway, và kiểm chính `claude` giả.
