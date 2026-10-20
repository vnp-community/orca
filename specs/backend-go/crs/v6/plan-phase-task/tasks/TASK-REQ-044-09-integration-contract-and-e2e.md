# TASK-REQ-044-09: Tích hợp, hợp đồng và e2e (`APG-xx`) cho tự động sinh Plan

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `tests/e2e, request-service`
**File:** `tests/stages/e2e/orca-flows/suite_request_auto_plan.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/orca-flows/fake_claude_cli/claude` (sửa: lỗi plan theo trạng thái), `tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py` (sửa), `backend-go/services/request-service/internal/adapter/contracttest/plan_auto_flow_contract.go` (mới), `backend-go/services/request-service/internal/adapter/postgres/plan_auto_integration_test.go`, `internal/adapter/mysql/plan_auto_integration_test.go` (mới, tag `integration`), `tests/stages/e2e/orca-flows/README.md` (liệt kê suite mới)
**Depends on:** TASK-REQ-044-01 đến 044-08, FE-REQ-TASK-044-03 cho ca UI (task này chỉ làm phía backend/acceptance)
**Status:** [ ] TODO

---

## Context

- Bộ acceptance chạy qua WebSocket gateway trên stack local; dev server là agent thật. `--ai-stub` đặt `claude` giả (`fake_claude_cli/claude`) vào chỗ `claude` thật: trả lời theo prompt Plan (`PLAN_MARK`, `plan_answer`), lỗi chỉ gây được bằng khối `<feedback>` chứa `[stub:fail-once]` hoặc `[stub:slow]`. **Run tự động không có `<feedback>`**, nên stub cần cách mới để gây lỗi cho lần gọi Plan kế tiếp.
- Mẫu suite: `suite_request_plan_stage.py` (`PLN-xx`, `propose_and_wait`), `suite_analysis_revision.py` (`ARV-xx`), `suite_request_base_ref.py` (`BRF-xx`). Hai trình điều khiển chung: `request_flow_driver.py`, `acceptance_context.py`.
- Phân loại chi phí (kế hoạch `specs/IMPLEMENTATION-PLAN-CR-REQ-039-042.md` mục 4.5): A (stub, mỗi PR), B (Claude thật native), C (OpenSpec thật, 7 đến 13 USD, cần người dùng đồng ý).
- Ca "không dev server" (`APG-04`) cần một dự án không có kết nối dev server; **chưa kiểm** cách harness dựng được dự án như vậy (xem DEV-04 trong suite hiện có); nếu không dựng được thì ca này chỉ phủ ở tầng tích hợp.

## Việc cần làm

1. Stub: thêm tệp trạng thái `$FAKE_CLAUDE_STATE_DIR/fail_plan_next` (nội dung `<số lần>:<kiểu>`, kiểu `error` = thoát mã khác 0, `invalid` = trả văn bản không phải JSON); mỗi lần nhận prompt Plan, nếu tệp còn số lần > 0 thì giảm và thực hiện lỗi. Cập nhật docstring đầu file và `test_fake_claude_cli.py`. Lưu ý lần sinh Plan native có một lượt sửa lại (`RecordAttempt`), nên kiểu `invalid` cần số lần ≥ 2 để run thật sự `failed` với `REQUEST_PLAN_AI_INVALID_JSON`.
2. Suite `suite_request_auto_plan.py` (`--ai-stub`, loại A): `APG-01` duyệt Analysis của `bug` → trong 90 giây Request ở `awaiting_plan_approval` có `planTaskId` và Approval `plan` mở, **không có lời gọi `request.generatePlan` nào từ client**, `request.planProposal.autoPlan.state=done`; `APG-02` stub `invalid` ×2 → `gave_up` với `REQUEST_PLAN_AI_INVALID_JSON`, đúng một run, Request `planning`; rồi `request.retryPlan` (stub hết lỗi) → Plan thành công; `APG-03` stub `error` ×1 → run 1 `failed`, run 2 (khoá `-2`) thành công; tổng hai run; `APG-04` không dev server → `skipped` (nếu dựng được; xem Context); `APG-05` `spike` và `question` (hoàn thành sau Analysis) và `hotfix` (thẳng tới `awaiting_plan_approval`) không có job/run; `APG-06` công tắc dự án tắt qua `request.engineSettingsSet` (admin) → Request ở `planning`, không run, `autoPlan.enabled=false`, nút thủ công (`request.generatePlan`) vẫn chạy; (công tắc env cần khởi động lại stack: ghi rõ chỉ kiểm ở tầng tích hợp); `APG-07` bấm tay `request.generatePlan` ngay sau khi duyệt → đúng một run Plan và một Plan, không lỗi; `APG-08` replan có `feedback` từ `awaiting_plan_approval`, và từ chối duyệt Plan kèm sinh lại, không tạo job; `APG-09` `task` xác nhận loại → Plan tự xuất hiện.
3. `APG-10` (loại C, opt-in `ORCA_E2E_OPENSPEC=1`, không `--ai-stub`, fixture LedgerLite kịch bản 3 `change_request`, engine `openspec`): duyệt Analysis → run `agent_proposal` tự chạy → commit phía server; kiểm `request.planProposal.engine=openspec`, `bundle` có file, các task của Plan có dòng tiêu chí chấp nhận từ `ac=` (`satisfies`). **Tốn 7 đến 13 USD**; mặc định `SKIP` kèm thông báo, chỉ chạy khi người dùng đồng ý rõ ràng.
4. Hợp đồng/tích hợp hai dialect (`plan_auto_flow_contract.go` + hai file `*_integration_test.go`): chuỗi `status_changed` → `EnqueuePlanAuto` → job → worker (fake worker AI hoàn thành run, fake `TaskPlanWriter`) → `done`, trên Postgres 16 và MySQL 8 thật; **giao lại cùng sự kiện hai lần** → một job, một run; hai worker cùng lúc → mỗi job một owner; người bấm tay chen ngang (`GeneratePlan.Execute` thường) → một run, một Plan; hoãn khi bận (`MaxAgentRunsPerProject=1` với run `agent_proposal` giả đang chạy) rồi chạy khi giải phóng; khởi động lại giữa chừng (tạo worker mới trên cùng dữ liệu) → tiếp tục đúng; `RUN_INTERRUPTED` do sweeper → thử lại một lần.
5. Đăng ký suite `request_auto_plan` trong `run_acceptance.py`, đặt **sau** `aistub` (như `request_base_ref`: các suite dùng `claude` giả đọc mọi lời gọi) và liệt kê trong `README.md` của `orca-flows`.

## Kiểm thử

- `python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver request_auto_plan` xanh; các suite `request`, `matrix`, `plan`, `analysis`, `task`, `aistub`, `request_base_ref` không hỏng.
- `python3 -m unittest discover -s tests/stages/e2e/orca-flows -p 'test_fake_claude_cli.py'` xanh.
- `cd backend-go/services/request-service && DOCKER_HOST=... go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'PlanAuto' -count=1 -p 1` xanh trên cả hai dialect.
- Spec UI (`auto-plan-generation.web.e2e.ts`) làm ở FE-REQ-TASK-044-04.
- Chạy lại `qa-bug-task-native-flow` (stub) để chắc không hồi quy.

## Tiêu chí hoàn thành

- [ ] `APG-01` đến `APG-09` xanh với `--ai-stub`; ca nào không dựng được trên stack ghi rõ lý do và được phủ ở tầng tích hợp.
- [ ] Tầng tích hợp phủ giao lại sự kiện, hai worker, người bấm tay chen ngang, hoãn khi bận, khởi động lại giữa chừng, `RUN_INTERRUPTED`.
- [ ] `APG-10` không tự chạy; khi chạy (có đồng ý) cho thấy commit phía server giữ `satisfies`.
- [ ] Không hồi quy các suite hiện có.

## Rủi ro và lưu ý

- Công tắc env `REQUEST_PLAN_AUTO_GENERATE` đổi cần khởi động lại request-service; acceptance không làm việc đó, nên ca "env tắt" chỉ ở tầng tích hợp/wiring.
- Mã lỗi mà stub `error` thực sự tạo ra phụ thuộc `mapAIRelayError` (`adapter/grpcclient/ai_relay_errors.go`); nếu không phải `REQUEST_PLAN_AI_UNAVAILABLE` thì chỉnh bảng tạm thời hoặc kiểu lỗi stub (chưa kiểm).
- Ca OpenSpec thật cần `.tdd-agent/config.toml` trong fixture và Claude thật đăng nhập; không tự chạy khi chưa được đồng ý.
- Stack local mất Vault khi khởi động lại; chuẩn bị lại bằng `vault-init` trước khi chạy.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
