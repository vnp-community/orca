# TASK-REQ-043-06: Suite acceptance `ARA-xx`, mở rộng `claude` giả cho prompt thực thi, cập nhật tài liệu phát hành

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `tests/e2e`, `request-service`
**File:** `tests/stages/e2e/orca-flows/suite_approve_run_all.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/orca-flows/fake_claude_cli/{claude,test_fake_claude_cli.py}` (sửa), `tests/stages/e2e/orca-flows/README.md` (bảng "Danh mục test", sửa), `deploy/dev/RELEASE-2026-10-DEPLOY.md` mục 4.3 và dòng 207 (sửa)
**Depends on:** TASK-REQ-043-04, TASK-REQ-043-05
**Status:** [ ] TODO

---

## Context

- Bộ acceptance chạy qua WebSocket gateway trên stack local, một phiên admin (`orca_stack_config.py`); dev server là agent thật. `--ai-stub` thay `claude` bằng `fake_claude_cli/claude`. Id ca viết dạng `PLN-01`, `BRF-01`; chọn tiền tố **`ARA`** (không trùng tiền tố nào).
- Khung `request.event` và `admin.queryAuditLog` không đưa được `execution_mode` của payload ra ngoài (CR mục 2.5), nên payload chỉ kiểm ở contract test (task 05). Suite dùng phản hồi `executionMode` và `request.get`.
- `fake_claude_cli/claude` chỉ nhận bốn loại prompt (`CLASSIFY_MARK`, `SOLUTION_MARK`, `PLAN_MARK`, `READONLY_MARK`, dòng 124-127; `prompt_kind` dòng 484); prompt thực thi task (`backend-go/prompts/task_execution/execute.tmpl` cộng `result_block_suffix.tmpl`) là `unknown` và thất bại. Spec UI `qa-bug-task-native-flow` vì vậy dừng ở duyệt Plan.
- Thực thi task tốn lượt AI và chạy mã trên dev server nên bị chặn bởi `ORCA_E2E_EXECUTE_AGENT=1` (`allow_agent_execution`).
- Dự án thử dùng engine `native` (mặc định dev stack là `openspec`): dựng bằng helper của `base_ref_stack.register_project` hoặc `configureProject`.

## Việc cần làm

1. Mở rộng `claude` giả: nhận prompt thực thi bằng đặc trưng của template thật (dòng `ORCA_RESULT_BEGIN <nonce>` trong phần "end your reply with exactly one result block" của `result_block_suffix.tmpl`), trích nonce, in một khối kết quả `status:"done"` hợp lệ (`summary` cố định, `files_changed` rỗng, `checks_run` rỗng trừ khi prompt đòi `RequirePassingCheck`) giữa hai dòng đánh dấu, thoát 0; ghi vào `calls.jsonl` loại `execute`. Thêm ca vào `test_fake_claude_cli.py` (nhận đúng nonce; prompt lạ vẫn thất bại). **Kiểm** task-service có chấp nhận `done` không có thay đổi tệp không; nếu không, cho stub tạo một tệp nhỏ trong thư mục làm việc và liệt kê vào `files_changed`.
2. `suite_approve_run_all.py`, dùng `request_flow_driver` và `base_ref_stack` để đưa một `change_request` (hoặc `bug` cỡ L) có Phase tới `awaiting_plan_approval` bằng claude giả:
   - `ARA-01` dự án `step_by_step`, duyệt Plan thường: `request.get.executionMode == step_by_step`, có Approval `phase` đang chờ, chưa Phase nào bắt đầu; duyệt Phase rồi `request.startPhase` thủ công vẫn chạy (luồng cũ không đổi).
   - `ARA-02` duyệt kèm `executionMode:"continuous"` ở dự án `step_by_step`: phản hồi `requestStatus == executing` và `executionMode == continuous`; `request.get.executionMode == continuous`; không có Approval `phase` (với `ORCA_E2E_EXECUTE_AGENT=1`: `request.activity` có `phase.started` do hệ thống/`auto` cho Phase đầu).
   - `ARA-03` (cần `ORCA_E2E_EXECUTE_AGENT=1` và mục 1): cả các Phase chạy hết tới `completed` hoặc tới điểm dừng hợp lệ mà không có lần bấm nào thêm; kiểm số `phase.started` bằng số Phase.
   - `ARA-04` `executionMode:"step_by_step"` và `"bogus"` → `REQUEST_EXECUTION_MODE_INVALID`; kèm `approval.reject` → lỗi; Approval `solution` kèm chế độ → `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`; mọi ca Approval vẫn `pending`.
   - `ARA-05` nhấp đúp: hai lời gọi liên tiếp cùng tham số, cả hai thành công, cùng `requestStatus` và `executionMode`.
   - `ARA-06` ghim sẵn `step_by_step` (chạy một lần rồi đưa về backlog và mở lại, hoặc bằng cách khác nếu không khả thi, ghi lý do) rồi duyệt kèm `continuous` → `REQUEST_EXECUTION_MODE_ALREADY_PINNED`.
   - `ARA-07` người duyệt thiếu quyền execute → `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`: cần tài khoản thứ hai không có quyền execute; chạy khi có `ORCA_E2E_MEMBER_EMAIL` và `ORCA_E2E_MEMBER_PASSWORD` (đề xuất tên; thêm vào `orca_stack_config.py` và README), ngược lại `BLOCKED` kèm lý do (đã có unit và contract).
   - Mọi ca dọn Request, project, worktree như `suite_request_base_ref.py` (đọc để theo mẫu `cancel_when_idle`, dọn).
3. Đăng ký suite `approve_run_all` trong `run_acceptance.py` (`SUITES`, và `AI_USING` nếu dùng AI); thêm dòng vào bảng "Danh mục test" của `README.md`; thêm vào chế độ `weekly` của `ci/run-orca-flows-ci.sh` nếu các suite khác ở đó.
4. Loại B: cập nhật `ui-live/continuous-execute-real.web.e2e.ts` (do task FE-043-06 làm phần giao diện) để dùng "Approve and run all steps"; task này chỉ xác nhận lệnh `run_ui_flow.py --real continuous` còn chạy được với backend mới.
5. Tài liệu phát hành: sửa `deploy/dev/RELEASE-2026-10-DEPLOY.md` mục 4.3 và dòng 207 (đã có màn hình Project Settings và lựa chọn lúc duyệt; kênh WS vẫn dùng được).

## Kiểm thử

- `python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py`.
- `python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver approve_run_all` (và với `ORCA_E2E_EXECUTE_AGENT=1` cho ARA-02/03); chạy lại `request`, `qa` để chắc không hồi quy; UI `qa-bug-task-native-flow` (stub) vẫn xanh.
- Loại B: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous` một lần. **Không** chạy ca OpenSpec thật (7 đến 13 USD mỗi Plan) khi chưa được đồng ý.

## Tiêu chí hoàn thành

- [ ] `ARA-01`, `02`, `04`, `05`, `06` xanh với claude giả; `ARA-03` xanh với claude giả mở rộng hoặc được ghi rõ chỉ chạy ở loại B kèm lý do; `ARA-07` xanh khi có tài khoản thứ hai, ngược lại `BLOCKED` có lý do.
- [ ] Claude giả nhận prompt thực thi, có test, và không làm vỡ các kiểu prompt cũ.
- [ ] Không hồi quy các suite hiện có; README và RELEASE đã cập nhật.

## Rủi ro và lưu ý

- Chưa kiểm chứng task-service chấp nhận khối kết quả `done` không có thay đổi tệp.
- Dev stack mặc định engine `openspec` và `REQUEST_DEFAULT_SOLUTION_AGENT=true`; phải đặt `native` và tắt `solutionAgent` cho dự án thử, nếu không ca vô tình chạy OpenSpec tốn tiền.
- Stack local mất Vault khi khởi động lại; chuẩn bị lại bằng `vault-init` trước khi chạy. Không sửa checkout orca trong lúc `--real` chạy.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
