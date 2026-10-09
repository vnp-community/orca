# TASK-E2E-035: Stub `claude` cho CI và kiểm thử

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Service/Area:** tests, ci
**Files:**
- tests/stages/e2e/orca-flows/fake_claude_cli/claude (mới, script Python, chỉ macOS/Linux)
- tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py (mới, tự kiểm stub)
- tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh + request_service_contract.overlay.go.txt (mới, kiểm stub với bộ parse/validate thật của request-service)
- tests/stages/e2e/orca-flows/ai_stub_agent_switch.py (mới, đổi agent sang stub rồi khôi phục)
- tests/stages/e2e/orca-flows/suite_ai_stub_login.py (mới, suite `aistub`), acceptance_phase_merge.py (mới)
- tests/stages/e2e/orca-flows/run_acceptance.py (`--ai-stub`, `--keep-stub-agent`, `--allow-blocked`)
- ai_stub_server.py (HTTP, tuỳ chọn): **không làm**
**Depends on:** TASK-E2E-033
**Status:** `[x]` XONG, trừ stub HTTP tuỳ chọn (cập nhật 2026-10-09, chưa commit)

---

## Context

CI không thể đăng nhập tài khoản thật; cần bằng chứng nguồn CLI chạy đúng mà vẫn xác định.

## Việc cần làm

- [x] Script `claude` giả: nhận `-p`, `--output-format json`, `--tools`, `--system-prompt`; đọc stdin; in JSON xác định cho phân loại, Solution, Plan (nhận diện theo nội dung prompt); `auth status --json` trả `loggedIn` theo biến `FAKE_CLAUDE_LOGGED_IN`. — _đã làm; thêm `--version`, `--help` (agent dò cờ `--tools`), file trạng thái `logged_in` vì agent lọc biến `FAKE_*` khỏi môi trường của `claude -p`; chưa đăng nhập thì thoát mã 1 như CLI thật; không có `--tools ""` thì từ chối; prompt lạ thì lỗi rõ ràng_
- [x] Ghi lại đối số và `cwd` nhận được vào tệp để test kiểm tra `--tools ""` và cwd rỗng. — _`$HOME/.fake-claude/calls.jsonl` (argv, cwd, nội dung thư mục, kích thước stdin, loại prompt); kiểm bởi `AST-01..03`_
- [x] Runner `--ai-stub`: đặt stub vào `PATH` của agent, khởi động lại agent, chạy test, rồi trả `PATH`. — _đặt cả HOME tạm có `~/.local/bin/claude` vì probe năng lực của agent không đọc `PATH` mà đọc danh sách cố định dưới `$HOME`; khởi động lại bằng `start-local-agent.sh`; khôi phục đúng `AGENT_WORK_DIR`, `DEV_SERVER_ID`, file log của agent trước đó_
- [ ] Bổ sung stub HTTP cho nguồn 1/2 (tuỳ chọn). — _chưa làm: nguồn khoá API được phủ ở test đơn vị agent_

## Tiêu chí hoàn thành

- [x] `python3 run_acceptance.py --ai-stub` chạy `RQL-09`, `AIS-02`, `AIS-04` thành PASS. — _PASS trong lần chạy `--strict --ai-stub` đầy đủ_
- [x] Có test âm: `FAKE_CLAUDE_LOGGED_IN=0` cho `REQUEST_AI_NOT_CONFIGURED`. — _`AST-04` (phân loại kết thúc ở `awaiting_type_confirmation`, `classificationReason=REQUEST_AI_NOT_CONFIGURED`, không spawn `claude`), `AIS-01` (run `failed` mã `REQUEST_AI_NOT_CONFIGURED`), `AIS-03` (run Plan `failed`, cùng mã); mở khoá `AIS-01/03`_

## Kiểm thử

Chạy thật; báo cáo ghi rõ là stub.

## Kết quả thực thi

**Làm xong** (2026-10-09, chưa commit). Báo cáo của chế độ này ghi "AI mode: STUB" ở `last-run.md`, `ai.mode` trong `last-run.json` và dòng cuối của lần chạy.

Quyết định thiết kế (ghi vì không hiển nhiên):

- **Không sửa agent hay mã sản xuất.** Stub chỉ là một file dưới `tests/`; runner chỉ đổi môi trường của tiến trình agent. Mã agent không nhận diện stub.
- **Hai đường tìm `claude` của agent khác nhau** (đọc `ai-complete-cli-session.ts`, `agent-capability-report.ts`, `agent-config.ts`): `ai.complete` tìm theo `PATH` của tiến trình, còn probe năng lực (quyết định `aiSources`) dùng `PATH` cố định gồm `~/.local/bin`, `~/bin`, `/usr/local/bin`... Đặt stub chỉ vào `PATH` thì readiness vẫn nhìn thấy `claude` thật. Hence HOME tạm.
- **Mỗi pha một dev server riêng** (`dev-local-mac-stub-in`, `-stub-out`, runner tự duyệt): infra-fleet lưu hồ sơ năng lực 24 giờ và chỉ dò lại khi kết nối nếu đã quá 5 phút; khi đổi đăng nhập của stub trên cùng một danh tính thì request-service vẫn thấy hồ sơ cũ (thấy "sẵn sàng", `claude` lại trả "Not logged in"). Hai bản ghi dev server ở lại DB sau mỗi lần chạy (không có API xoá).
- **Hai pha**: pha 1 stub đã đăng nhập, pha 2 chưa đăng nhập (chỉ khi có suite `ai`); gộp kết quả theo test id (FAIL thắng, rồi PASS, rồi BLOCKED).

Lệnh đã chạy và kết quả thật:

| Lệnh | Kết quả |
|---|---|
| `python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py` | 8 test OK |
| `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh` | PASS: phân loại, Solution (change_request, refactor; 0/1/3 AC; mọi option đều bao phủ AC; không rủi ro cao), Plan (cả 11 loại x S/M/L qua `ParsePlanOutput`, `ValidatePlanProposal`, chính sách theo loại), feedback tới Plan. Đã thử phá stub (hai option cùng `recommended`) thì đỏ đúng chỗ |
| esbuild bundle một lần mã agent thật (`buildCapabilityReport`, `completeWithCliSession`) với HOME/PATH của stub | agent thấy `claude` phiên bản `0.0.0-fake-claude`, `auth=logged_in`, `aiSources=["cli_session"]`; khi `FAKE_CLAUDE_LOGGED_IN=0` thì `auth=unknown`, `aiSources=[]` (CLI thật cũng thoát mã 1 nên agent ghi `unknown`); `ai.complete` chạy `-p --output-format json --tools "" --no-session-persistence --system-prompt ...` trong thư mục `orca-ai-*` rỗng |
| `run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02` (stack local, 88 giây) | **109 PASS, 1 FAIL, 2 BLOCKED (chỉ MCP-01/02), 1 SKIP (TSK-08 tuỳ chọn)**; ổn định qua hai lần chạy cuối |

Điều chưa làm / chưa kiểm:

- **1 FAIL: `AST-05` là lỗi sản phẩm thật, không phải lỗi stub.** Khi AI đề xuất `hotfix` (hoặc `security`) mà tenant chỉ có một user, mở Approval `request_type` trả `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` (`usecase/open_approval.go:140`), giao dịch phân loại bị huỷ, run `failed` và Request kẹt ở `classifying` mãi, vi phạm bất biến `RQL-08`. Chưa sửa (ngoài phạm vi: Go). Với Claude thật lỗi này chỉ lộ khi AI đề xuất hai loại đó. Stub cố ý không đề xuất `hotfix`/`security` theo tiêu đề `matrix hotfix` của bộ test (từ khoá cụ thể hơn) để `MTX-03/08` không dính lỗi này; `AST-05` là test riêng cho nó.
- Chưa chạy trên runner CI thật (xem TASK-029). Chưa kiểm trên Linux.
- Windows không áp dụng (stub là script, agent Windows tìm `claude.cmd`); runner từ chối trên Windows.
- Stub chỉ nhận ba loại prompt (phân loại, Solution kind `solution`, Plan). Chẩn đoán/Findings/Answer chạy qua `agent.execPrompt` với công cụ chỉ-đọc, chưa có stub và chưa có e2e.
- Khi chạy `--ai-stub` trên máy dev, dev server thật bị ngắt tới khi khôi phục (đã kiểm khôi phục: cùng `AGENT_WORK_DIR`, `DEV_SERVER_ID`, file log, HOME/PATH thật).

### Cập nhật sau khi sửa AST-05

`AST-05` đã xanh sau khi sửa `ProposeRequestClassification` (xem TASK-E2E-026). Chạy lại `--strict --ai-stub --allow-blocked MCP-01,MCP-02`: 110 PASS, 0 FAIL. Agent dev server đã khôi phục chạy `claude` thật (không có `fake_claude_cli` trong môi trường tiến trình).
