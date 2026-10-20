# TASK-REQ-045-09: Tích hợp, hợp đồng và e2e (suite `PRV-xx`) với claude giả, dữ liệu seed và ca OpenSpec thật tuỳ chọn

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `tests/e2e`, `request-service`
**File:** `tests/stages/e2e/orca-flows/suite_plan_revision.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/orca-flows/plan_documents_seed.py` (mới, + `test_plan_documents_seed.py`), `tests/stages/e2e/orca-flows/plan_revision_driver.py` (mới; dùng lại `analysis_revision_driver.py`, `analysis_revision_users.py`, `ledgerlite_fixture.py`), `tests/stages/e2e/orca-flows/fake_claude_cli/claude` (sửa chỉ khi việc 4 cho phép) + `test_fake_claude_cli.py`, `tests/stages/e2e/orca-flows/README.md` (dòng suite mới), `backend-go/services/request-service/internal/adapter/contracttest/{plan_revision_flow_contract,execution_flow_contract}.go` (mới/sửa)
**Depends on:** TASK-REQ-045-01 đến 045-08, và các task e2e/hạ tầng của BE-REQ-SOL-044 (worker commit, `fake_claude_cli` trạng thái `fail_plan_next`)
**Status:** [x] DONE (2026-10-20), trừ PRV-09/10/14/15 (không chạy, lý do ở ghi chú) và ca loại B/C

---

## Context

- Acceptance chạy qua WebSocket gateway trên stack local; `--ai-stub` đặt `claude` giả (`fake_claude_cli/claude`) trước `claude` thật; câu trả lời gắn với prompt thật của request-service (phân loại, Solution, **Plan native** qua `PLAN_MARK`, phân tích chỉ đọc). Phản hồi của người duyệt (khối `<feedback>`) được lặp lại trong câu trả lời ("revised, round N, asked: …"); `[stub:fail-once]` làm lượt đầu thất bại, `[stub:slow]` làm chậm (`FAKE_CLAUDE_SLOW_SECONDS`).
- **Fake claude không tác giả OpenSpec** (tôi đã `grep` `openspec` trong file: không có): prompt OpenSpec bị "fail loudly". Engine OpenSpec còn cần `openspec --version` trên dev server (`EngineReadinessGate`, `engine_readiness_gate.go`), kỹ năng tri thức và `.tdd-agent/config.toml` (`KNOWLEDGE_HUB_UNSUPPORTED` với `--ai-stub`, ghi chú e2e của repo). Một fixture không có tri thức phải đặt dự án về engine `native` và `solutionAgent=false` (`request.engineSettingsSet`, admin, khôi phục sau).
- Bài học TASK-REQ-041-04: (1) cổng OPA chặn approver là member thường (xử lý ở task 01); (2) dev server gắn với reporter (task 02); (3) ca dùng hai người dùng cần `docker exec orca-go-postgres` để ghi chính sách duyệt và `admin.createUser` + đăng nhập: thiếu thì `BLOCKED` kèm lý do, không `FAIL`; stack mặc định chỉ có một người dùng.
- Fake claude: CR-REQ-043 (task 06), 044 (task 09) và 046 (task 07) cũng sửa `fake_claude_cli/claude`. 044 đã chọn tách mã thất bại bằng tệp trạng thái `fail_plan_next` trong `$FAKE_CLAUDE_STATE_DIR` vì lượt tự động không có khối `<feedback>`; lượt chỉnh của CR này **có** `<feedback>` nên dùng được `[stub:fail-once]`. Hợp nhất theo hàm khi sửa, mỗi CR thêm ca của mình vào `test_fake_claude_cli.py`.
- Phân loại chi phí (kế hoạch tổng mục 4.5): A = claude giả (mỗi PR, không tốn); B = Claude thật native, repo tạm (cuối giai đoạn, khoảng 3 phút, nhỏ); C = Claude thật OpenSpec (7 đến 13 USD mỗi lượt sinh, **cần người dùng đồng ý**).

## Việc cần làm

1. Suite `suite_plan_revision.py` (id `PRV-xx`) trên repo LedgerLite với `--ai-stub`, engine `native` (loại A). Mỗi ca ghi `BLOCKED` thay vì `FAIL` khi thiếu điều kiện hạ tầng:
   - PRV-01: `bug` native đi tới `awaiting_plan_approval`; `request.planRevisions` trả 1 bản `current`, `revisionFeedback` rỗng; `request.planProposal {revision:1}` trả đề xuất đủ `summary`, Phase/task.
   - PRV-02: chỉnh hai lần (`request.generatePlan {mode:"propose", feedback, idempotencyKey}`, **không** gọi `commit`: worker của 044 commit bằng job `revision`; chờ Request về `awaiting_plan_approval`, tối đa vài lần `PollInterval` 5 giây): mỗi vòng Approval cũ có lý do `revision_requested`, đúng một Approval `pending` mới khác Approval cũ, danh sách có 3 bản (1 `current`, 2 `superseded`), `revisionFeedback` đúng chữ, dấu `[revised, round N` trong đề xuất.
   - PRV-03: Activity có đúng 2 dòng `plan.revision_requested`, không có chữ phản hồi ở bất kỳ dòng nào; không có dòng `analysis.revision_requested` cho lượt Plan.
   - PRV-04: phản hồi 2000 ký tự được; 2001 bị `REQUEST_PLAN_FEEDBACK_TOO_LONG`; không đổi gì.
   - PRV-05: approver là member dự án (không phải reporter/admin) chỉnh được và được ghi là tác nhân; người lạ (không phải member) bị chặn; member không phải approver bị `REQUEST_PLAN_FORBIDDEN`; Approval và bản hiện tại còn nguyên. Dùng `analysis_revision_users.py`.
   - PRV-06: bấm đúp cùng `idempotencyKey` = một lượt; chỉnh thứ hai khi lượt đang chạy (`[stub:slow]`) = `REQUEST_PLAN_REVISION_IN_PROGRESS`.
   - PRV-07: lượt chỉnh thất bại (`[stub:fail-once]`): Request ở `planning`, không còn Approval, job `revision` ở `gave_up`; `request.generatePlan` không chữ chạy lại **với cùng phản hồi** (kế thừa) và thành công, job `revision` mới commit, Approval mới mở. Biến thể: người thử lại là approver đã yêu cầu chỉnh (qua; `BLOCKED` nếu stack một người dùng) và approver khác (bị `REQUEST_PLAN_FORBIDDEN`).
   - PRV-08: `hotfix` chỉnh bị `REQUEST_PLAN_REVISION_NOT_APPLICABLE` (`BLOCKED` nếu stack một người dùng không tạo được approver hợp lệ cho Approval của hotfix, như ARV-13).
   - PRV-08b: tab đóng giữa chừng: sau khi gửi `propose` không có client nào gọi `commit`; Request vẫn về `awaiting_plan_approval` với Approval mới (bằng chứng cho job `revision`).
   - PRV-09: dev server offline: `REQUEST_PLAN_AI_UNAVAILABLE`, Approval và bản hiện tại còn nguyên (nếu suite có công cụ ngắt dev server như `suite_dev_server_agent.py`; không thì `SKIP` ghi lý do).
   - PRV-10: 11 vòng chỉnh liên tiếp (chỉ khi `ORCA_E2E_PLAN_REVISION_LONG=1`; stub nên không tốn): `request.planRevisions` có 12 bản; bản 1 `textKept=false`, bản 2..12 còn; đọc `request.planBundleFile {revision:1}` báo `TEXT_PRUNED` chỉ áp cho openspec (với native bỏ ca này).
2. `plan_documents_seed.py` (loại A, ca OpenSpec bằng dữ liệu seed): dùng cơ chế `docker exec orca-go-postgres psql` như `ARV-07`: lấy một Request `change_request` native đã ở `awaiting_plan_approval`, đặt `requests.solution_engine = 'openspec'`, chèn dòng `request.openspec_changes` (`status='ready'`, `bundle_json` có `proposal.md`, `design.md`, `specs/ledger/spec.md`, `tasks.md` có vùng `orca:begin plan`, một file là dấu `<omitted size=300000 sha256=…>`, một file chứa bí mật giả cắm sẵn **chưa** làm sạch để kiểm che khi đọc) và dòng bản tương ứng. Ca: PRV-11 `request.planProposal` trả `bundle.files` đủ, `size` thật cho file bị lược; PRV-12 `request.planBundleFile` trả văn bản, file bị lược `omitted=true`, bí mật bị che; PRV-13 `revision` cũ còn chữ / bị lược chữ. Seed phải dọn sau khi chạy (xoá dòng, trả `solution_engine`); in cảnh báo rằng đây là dữ liệu giả, không dùng ở stack dùng chung.
3. Hợp đồng: `plan_revision_flow_contract.go` chạy cả hai dialect với fake task service: ba vòng chỉnh qua `GeneratePlan` → worker giả → `finish` → bản; kiểm Approval cũ bị huỷ, số bản, lược chữ ở bản thứ 11, redaction, `ListPlanRevisions` phân trang, nhánh Activity; `execution_flow_contract.go` giữ xanh.
4. **Quyết định khả thi fake claude tác giả OpenSpec** (làm sau việc 1 đến 3, có hộp thời gian nửa ngày): đọc `engine_readiness_gate.go`, `openspec_bundle_checker.go`, `fake_claude_cli/claude` và `DevServerCapabilityReader`; liệt kê những gì cần để một lượt OpenSpec với claude giả chạy qua: shim `openspec` (chỉ `--version`, `validate <id> --strict --json`), prompt `plan_openspec` nhận dạng bằng câu đầu "You are a senior engineer turning an approved decision into an OpenSpec change", ghi `openspec/changes/<id>/{proposal.md,design.md,specs/x/spec.md,tasks.md}` đúng ngữ pháp `ParsePlanRegion` và khối kết quả theo `result_block.tmpl`, vượt qua `KNOWLEDGE_HUB_UNSUPPORTED`. Nếu làm được trong hộp thời gian: thêm chế độ vào `fake_claude_cli/claude` (kèm test trong `test_fake_claude_cli.py`, `verify-against-request-service.sh`), thêm PRV-14 (OpenSpec: xem tài liệu, chỉnh một lần, lượt chỉnh có khối "sửa tại chỗ" trong prompt qua `calls.jsonl`, file trong bản mới đổi đúng một file) và PRV-15 (bí mật cắm trong yêu cầu Request được che trong `design.md` đã lưu). Nếu không làm được: ghi kết luận và chặn cụ thể vào phần "Ghi chú triển khai" của task, và PRV-14/15 thành ca loại C (việc 5).
5. Ca loại C (OpenSpec thật, **7 đến 13 USD mỗi lượt sinh, mỗi vòng chỉnh là thêm một lượt cùng bậc, chưa đo riêng; chỉ chạy khi người dùng đồng ý**, `ORCA_E2E_OPENSPEC=1`): kịch bản 3 của fixture (`change_request`, phí chuyển khoản, size M) trên `release/1.x`: Plan OpenSpec → xem đủ bốn nhóm tài liệu → chỉnh một lần ("Add a task for the fee rounding tests") → so sánh hai bản: chỉ các file cần đổi có `change=changed`; không bí mật. Trước khi chạy: `workspace_probe.py` trên worktree proposal thử (bài học "fakes passed while six live-only defects failed"), kiểm `AGENT_WORK_DIR`, và không sửa checkout trong lúc chạy.
6. Ca loại B (Claude thật, native, repo tạm, nhỏ): `run_ui_flow.py --real` một vòng chỉnh `bug` native (đăng ký ở FE-REQ-TASK-045-08); không phải của suite này, nhưng suite ghi điều kiện chạy.
7. Đăng ký `plan_revision` trong `run_acceptance.py` (`SUITES`, `AI_USING`); đặt **sau** `aistub` như `analysis_revision` để các lời gọi `claude` không làm `AST-02/03` đếm nhầm; cập nhật `README.md` (bảng "Danh mục test").
8. Cập nhật tài liệu: dòng ghi nhận trong ghi chú triển khai của task này, danh sách lệnh chạy cho người điều phối.

## Kiểm thử

- `python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver plan_revision` xanh (ca `BLOCKED` ghi lý do, không `FAIL`); `... request`, `... qa`, `... analysis_revision`, `... request_base_ref` không hồi quy.
- `python3 -m unittest discover -s tests/stages/e2e/orca-flows -p 'test_*.py'` xanh (gồm `test_plan_documents_seed.py`, `test_fake_claude_cli.py`).
- `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh` xanh.
- `go test ./services/request-service/internal/adapter/contracttest/... -run 'PlanRevision|Execution' -v` xanh trên hai dialect.
- `python3 tests/stages/e2e/orca-flows/run_acceptance.py --list` liệt kê `plan_revision`.
- Mô phỏng suite bằng gateway trong bộ nhớ chỉ kiểm lỗi Python và trình tự, không thay chạy thật.

## Tiêu chí hoàn thành

- [x] PRV-01..08b xanh (PRV-09 SKIP có lý do) (hoặc `BLOCKED` có lý do hạ tầng cụ thể) trên stack local với claude giả và repo LedgerLite thật.
- [x] PRV-11..13 (seed) xanh: file bị lược, che bí mật khi đọc, bản cũ bị lược chữ.
- [x] Kết luận về fake claude OpenSpec được ghi (làm được hay chặn cụ thể); nếu làm được PRV-14/15 xanh.
- [x] Hợp đồng hai dialect xanh; không hồi quy `request`, `qa`, `analysis_revision`, `request_base_ref`.
- [x] Ca loại C có lệnh chạy, điều kiện và ước lượng chi phí ghi rõ; không tự chạy.

## Rủi ro và lưu ý

- Stack local một người dùng: PRV-05/08 cần `admin.createUser` và đăng nhập; chưa biết có đăng nhập được ngay không (bài học ARV-07).
- Stack mất Vault khi khởi động lại: chuẩn bị lại bằng `vault-init` trước khi chạy; sau `redeploy-local.sh` khởi động lại các dịch vụ theo thứ tự infra-fleet → git-gateway → task-service → request-service → api-gateway.
- `--real` không cho sửa checkout orca trong lúc chạy (quy tắc quiet-checkout 90 giây).
- Seed SQL là hạ tầng thử, không đưa vào mã sản phẩm.

## Ghi chú triển khai (2026-10-20, chạy trên stack)

**Tệp:** `tests/stages/e2e/orca-flows/{suite_plan_revision.py,plan_revision_driver.py,plan_documents_seed.py,test_plan_documents_seed.py}` (mới; suite `plan_revision` đăng ký sau `aistub`), `fake_claude_cli/{claude,test_fake_claude_cli.py}` (lượt chỉnh Plan: ghi chú Plan lặp `[revised, round N, asked: ...]` đếm vòng từ khối `<prior_plan>`; `[stub:fail-once]` và `[stub:slow]` áp cả cho Plan), `README.md`, `contracttest/plan_revision_flow_contract.go` + test tích hợp hai dialect (mới).

**Kết quả:** `run_acceptance.py --ai-stub devserver plan_revision`: PRV-01..08b, 11..13 xanh; PRV-09 (dev server offline: suite không có công cụ ngắt), PRV-10 (cần `ORCA_E2E_PLAN_REVISION_LONG=1`), PRV-14/15 `SKIP` có lý do. Hợp đồng: `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run PlanRevisionFlow` xanh (ba vòng chỉnh qua worker, Approval cũ bị huỷ `revision_requested`, 4 bản, phân trang, đọc từng bản).

**Lỗi sản phẩm tìm được và sửa (PRV-08):** hotfix không ghi `planTaskId` lên Request (task sửa không phải Plan) nên `checkPlanRevisable` (chỉ chạy khi `isPlanRevision`, đòi `PlanTaskID != ""`) bị bỏ qua: `request.generatePlan {feedback}` ở cổng của hotfix HUỶ Approval `pre_deploy` của task sửa, đưa Request về `planning` và chạy một run Plan không job nào commit (Request kẹt). Test đơn vị cũ đặt `PlanTaskID` nên không thấy. Sửa: `usecase/generate_plan.go` `checkPlanRevisable` coi mọi ask ở `awaiting_plan_approval` là lượt chỉnh; test mới `TestGeneratePlan_HotfixWithoutARecordedPlanIsNotRevisable`. Đã triển khai lại request-service; PRV-08 xanh.

**Quyết định claude giả tác giả OpenSpec (việc 4):** không làm. Cần shim `openspec` (`--version`, `validate`), vượt `KNOWLEDGE_HUB_UNSUPPORTED` của `--ai-stub`, worktree đề xuất do git-gateway dựng và ghi `proposal/design/specs/tasks` đúng ngữ pháp `ParsePlanRegion` cùng khối kết quả: vượt hộp thời gian nửa ngày. Thay bằng dữ liệu gieo: `plan_documents_seed.py` (xem README) chạy PRV-11..13 và spec UI `plan-documents` (b). Seed dùng `docker exec orca-go-postgres`, dữ liệu GIẢ, dọn bằng `clean`.

**Chưa kiểm:** PRV-09, PRV-10, PRV-14/15; ca loại C (`ORCA_E2E_OPENSPEC=1`, 7-13 USD mỗi lượt sinh cộng một lượt chỉnh) không chạy; ca loại B `run_ui_flow.py --real plan-revision` KHÔNG chạy do hộp thời gian.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
