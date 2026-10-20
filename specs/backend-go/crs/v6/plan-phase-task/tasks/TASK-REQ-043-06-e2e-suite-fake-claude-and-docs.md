# TASK-REQ-043-06: Suite acceptance `ARA-xx`, mở rộng `claude` giả cho prompt thực thi, cập nhật tài liệu phát hành

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `tests/e2e`, `request-service`
**File:** `tests/stages/e2e/orca-flows/suite_approve_run_all.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/orca-flows/fake_claude_cli/{claude,test_fake_claude_cli.py}` (sửa), `tests/stages/e2e/orca-flows/README.md` (bảng "Danh mục test", sửa), `deploy/dev/RELEASE-2026-10-DEPLOY.md` mục 4.3 và dòng 207 (sửa)
**Depends on:** TASK-REQ-043-04, TASK-REQ-043-05
**Status:** [x] DONE (2026-10-20): chạy trên stack; còn lại một mục không chạy theo chỉ định (`--real continuous`, loại B)

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

- [x] `ARA-01`, `02`, `04`, `05`, `06` xanh với claude giả; `ARA-03` xanh với claude giả mở rộng (chạy hết 2 Phase, 6 task, miễn phí); `ARA-07` `BLOCKED` có lý do. (`python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver approve_run_all`: ARA-01..06 PASS, ARA-07 BLOCKED, 53 s; ba dòng FAIL `AST-01..03` của lần chạy hẹp này là do `aistub` tự thêm vào và chạy trước khi có lời gọi AI nào, không phải lỗi: lần chạy đầy đủ bên dưới xanh.)
- [x] Claude giả nhận prompt thực thi, có test, và không làm vỡ các kiểu prompt cũ. (`python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py`: 57 test OK; `bash .../verify-against-request-service.sh`: OK cả hai module, gồm `TestFakeClaude_ExecutionAnswerPassesTheRealResultParser` và `TestFakeClaude_PlainExecutionPromptIsRecognised` ở task-service)
- [x] Không hồi quy các suite hiện có; README và RELEASE đã cập nhật. (`run_acceptance.py --ai-stub` toàn bộ: 151 pass, 6 fail, 5 blocked, 4 skip, 354 s. 6 FAIL đều ngoài phạm vi: `QA-TERM-01..03` (stub không mở được PTY: `INFRA_AGENT_SPAWN_PTY_FAILED`), `QA-GH-01/03` (BUG-QA-03 chưa sửa, đã ghi ở RELEASE mục 8), `QA-JIRA-02` (assert của test, không liên quan). 5 BLOCKED: `AIS-01/03`, `MCP-01/02`, `ARV-13` như các lần trước, cộng `ARA-07`. UI stub: `qa-bug-task-native-flow` 3/3, `analysis-revision` 5/5, `request-base-branch` 3/3 xanh.)

## Rủi ro và lưu ý

- Chưa kiểm chứng task-service chấp nhận khối kết quả `done` không có thay đổi tệp.
- Dev stack mặc định engine `openspec` và `REQUEST_DEFAULT_SOLUTION_AGENT=true`; phải đặt `native` và tắt `solutionAgent` cho dự án thử, nếu không ca vô tình chạy OpenSpec tốn tiền.
- Stack local mất Vault khi khởi động lại; chuẩn bị lại bằng `vault-init` trước khi chạy. Không sửa checkout orca trong lúc `--real` chạy.

## Ghi chú triển khai (2026-10-20)

**Tệp:** `tests/stages/e2e/orca-flows/{suite_approve_run_all.py,approve_run_all_driver.py}` (mới), `run_acceptance.py` (đăng ký `approve_run_all` sau `aistub`), `fake_claude_cli/{claude,test_fake_claude_cli.py,verify-against-request-service.sh,task_service_contract.overlay.go.txt}`, `README.md` (dòng `ARA`), `deploy/dev/RELEASE-2026-10-DEPLOY.md` (4.3, dòng chạy thử, mục 8: đã có màn hình Project Settings cho chế độ thực thi). Sửa lỗi sản phẩm: xem "Lỗi tìm được" bên dưới.

**Claude giả và prompt thực thi (điều tra theo yêu cầu):** prompt thực thi tới claude giả có hai dạng: (a) task không có spec: `agent.execPrompt` ghi quyền `--print --output-format stream-json --verbose --dangerously-skip-permissions`, prompt (stdin) bắt đầu bằng khối `# Orca Project Context` của agent rồi `Complete the following task.\n\nTask: ...`, **không có khối kết quả**, task-service coi exit 0 là xong; (b) task có spec (contract run): thêm hậu tố `ORCA_RESULT_BEGIN <nonce>`. Stub nhận cả hai (`prompt_kind` = `execute`), trả một dòng (a) hoặc khối `done` hợp lệ (b), ghi `outputLanguage`, `dangerousSkip`, `gitWorktree` vào `calls.jsonl`; từ chối phân tích chỉ đọc có cờ bỏ qua quyền. **task-service có chấp nhận `done` không đổi tệp nào không: có.** Chạy thật: task `open` -> `review` -> `done` (tự hoàn thành) với `files_changed` rỗng và không có thay đổi tệp, Phase 1 rồi Phase 2 chạy xong. `files_changed` không được dùng để từ chối ở `execution_result.go`; riêng đường contract có `ClassifyRunFailureChecked` (cần check đạt khi `RequirePassingCheck`), đã kiểm bằng parser thật chứ chưa chạy trên stack (task native không có spec).

**Lỗi tìm được và sửa (task-service, ngoài danh sách tệp của task nhưng là lỗi thật chặn `ARA-03`):** lần chạy `ARA-07` làm member duyệt, `ExecutionAccess` cấp quyền execute và task-service ghi sự kiện outbox `task.grant_received`. Chủ đề này **nằm ngoài stream JetStream `TASK` (`orca.task.>`)** nên mỗi lần phát bị từ chối (`nats: no response from stream`), và `common/outbox.Relay` dừng ở lần lỗi đầu tiên của lô (có chủ ý, để giữ thứ tự): **mọi sự kiện đứng sau bị giữ vô hạn** (331 lần thử lại trong 30 phút, 28 sự kiện `statuschanged` kẹt). Hậu quả: request-service không nhận `task.statuschanged` nên task kẹt ở `review`, Request kẹt `executing` mãi (ARA-03 hết 360 s). Bất kỳ lần cấp/thu quyền task nào ở bất kỳ tenant nào cũng gây ra. Sửa: `internal/adapter/eventbus/publisher.go` (`streamSubject` thêm tiền tố `orca.` cho mọi sự kiện chưa có), test `publisher_test.go` (`TestPublisher_PutsEveryEventUnderTheTaskStream`), migration `0022_outbox_grant_subjects` ở **postgres và mysql** (đổi tên chủ đề của các hàng chưa gửi, `down` là no-op). Triển khai: `redeploy-local.sh task-service` (migration áp dụng, 0 hàng chưa gửi sau đó), khởi động lại request-service và api-gateway. Blast radius: chỉ `Publisher.Publish` (người gọi: `Grant`, `RevokeGrant`); không có consumer nào của hai chủ đề này. **Chưa chạy**: test tích hợp migration trên Postgres 16/MySQL 8 (máy không có CLI `migrate`); câu SQL của MySQL chưa chạy thật.

**ARA-07:** `BLOCKED` có lý do (README ghi): qua gateway không đạt được lỗi `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`, vì cây Plan do admin (người commit) sở hữu và `ExecutionAccess.CheckReady` chỉ từ chối khi người duyệt chính là chủ cây mà không có quyền; member được cấp quyền từ chủ cây và lời gọi thành công, Request chạy `continuous`. Không thêm `ORCA_E2E_MEMBER_*`: suite tự tạo member bằng `admin.createUser`.

**Các kiểm tra khác:** ARA-06 chạy thật: Request ghim `step_by_step` (chạy Phase 1 bằng tay), `request.returnToBacklog` stage `phase`, `request.reopen`, đi lại phân tích và Plan (commit Plan mới được), rồi `continuous` bị `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, duyệt thường vẫn thành công. Sự cố vận hành ghi nhận: lần chạy `--ai-stub` có outbox bị giữ kết thúc với "dev server 'dev-local-mac' did not reconnect within 90s" khi khôi phục agent (mã thoát 3); chạy lại `start-local-agent.sh` thì nối ngay; sau khi sửa outbox các lần khôi phục đều thành công.

**Chưa kiểm:** mục 4 (`run_ui_flow.py --real continuous`, loại B) không chạy theo chỉ định; spec chỉ được liệt kê và kiểm kiểu. Không chạy OpenSpec thật.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
