# TASK-REQ-041-04: Đường lỗi, đồng thời, knowledge hub, gateway, CONTRACT và e2e

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service, api-gateway, tests/e2e`
**File:** `internal/usecase/{generate_solution,run_agent_readonly_analysis,openspec_solution_prompt}.go` (kiểm tra, sửa nếu cần), `internal/adapter/contracttest/solution_contract.go` (sửa), `services/api-gateway/internal/adapter/wscompat/`, `mcpserver/tools/` (sửa), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`, `tests/stages/e2e/orca-flows/suite_analysis_revision.py` (mới)
**Depends on:** TASK-REQ-041-01, 041-02, 041-03
**Status:** [x] DONE (2026-10-20) — `suite_analysis_revision` ARV-01..12 xanh trên stack (stub); ARV-13 (hotfix) BLOCKED vì stack chỉ có một người dùng (không có approver hợp lệ cho Approval của hotfix)

---

## Context

- Sau khi chỉnh, bản cũ `superseded` ngay khi lượt mới bắt đầu; nếu lượt thất bại, không còn đề xuất để duyệt.
- Đường `solutionAgent=true` với knowledge hub và `openspec_solution_prompt.go` có thể dùng prompt khác `BuildSolutionPrompt`.
- Cần chứng minh bằng test thật, không giả định.

## Việc cần làm

1. **Thất bại giữa chừng**: kịch bản agent giả trả lỗi (hết hạn, JSON sai, `READONLY_VIOLATION`) ở lượt chỉnh: Request ở `analyzing`, có `AnalysisRun` thất bại, `Regenerate` và chỉnh lại khả thi, `priorArtifacts` của lượt kế chứa bản đã bị thay thế. Nếu phát hiện Request kẹt thì sửa (ví dụ cho phép chạy lại từ `analyzing` với phản hồi).
2. **Hai người chỉnh đồng thời**: người sau nhận `REQUEST_SOLUTION_WRONG_STATE`; không sinh hai lượt.
3. **Bấm đúp**: cùng `idempotencyKey` chỉ một lượt.
4. **Knowledge hub/OpenSpec**: xác định đường nào thực sự sinh Solution khi `solutionAgent=true`; nếu prompt của đường đó không mang `Feedback` và `PriorArtifacts` thì thêm (kèm golden). Ghi kết luận vào PR.
5. **Gateway/MCP**: `solution.list` trả `revisionFeedback`; MCP đọc Solution thêm trường; `solution.generate`/`solution_generate` mô tả quyền approver; `parity_test` xanh.
6. **CONTRACT** cập nhật.
7. **E2E** (`suite_analysis_revision.py`, id `ARV-xx`) trên fixture tài chính (xem kế hoạch tổng mục 4) với claude giả: (a) `change_request` Solution: chỉnh 2 lần, 3 phiên bản (1 hiện hành, 2 superseded), Approval mới sau mỗi vòng; (b) `bug` chẩn đoán; (c) `spike` Findings; (d) `question` Answer; (e) approver không phải reporter chỉnh được, người lạ bị từ chối; (f) lượt thất bại không làm kẹt; (g) `hotfix` không có cổng nên chỉnh bị từ chối đúng lý do.

## Kiểm thử

- Hợp đồng `solution_contract` mở rộng cho (1) đến (3).
- `python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver analysis_revision --ai-stub` xanh; `... request` và `... qa` không hồi quy.
- `go test ./services/api-gateway/internal/adapter/... -run 'Solution|Parity' -v` xanh.

## Tiêu chí hoàn thành

- [x] Mọi đường lỗi để Request ở trạng thái thao tác được (không kẹt).
- [x] Phản hồi tới được agent ở mọi đường sinh phân tích đang dùng; kết luận R4 ghi trong PR. (Kết luận R4 ở ghi chú TASK-REQ-041-02; chưa có PR vì không commit.)
- [x] Suite `ARV-xx` xanh với repo fixture thật: 12 đạt, 1 chặn (ARV-13), 0 lỗi. Chạy thật phát hiện và sửa: (1) cổng OPA chặn thành viên dự án là approver (quy tắc `member_steering_rpcs` cho `GenerateSolution`/`ChooseSolutionOption`, chỉ thành viên; non-member vẫn bị chặn); (2) dev server gắn với reporter nên tra cứu kết nối và worker nền chạy dưới danh tính reporter, lượt chạy vẫn ghi cho người gọi; (3) dòng Activity đếm cả run `clarification:` hệ thống.
- [x] CONTRACT khớp mã; `parity_test` xanh.

## Rủi ro và lưu ý

- Ca Claude thật (chất lượng phản hồi) là opt-in, tốn tiền; không tự chạy.
- Nếu R4 cho thấy cần sửa prompt OpenSpec, phối hợp với CR-REQ-042 (golden) để không xung đột.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.


---

## Ghi chú triển khai

**Ngày:** 2026-10-20.

### Kết luận chính

1. **Request không kẹt sau lượt chỉnh thất bại.** Lượt chỉnh đã chuyển Request sang `analyzing` ngay khi bắt đầu, bản cũ `superseded`, Approval cũ bị huỷ; khi lượt thất bại, nháp bị xoá, Request ở `analyzing`, có `AnalysisRun` `failed` (hiện ở `solution.list.runs`). `Regenerate` (không phản hồi) và gửi phản hồi mới từ `analyzing` đều chạy được (`checkAnalysisState` cho `analyzing` mọi trường hợp), bản `superseded` nằm trong `prior_artifacts` của lượt sau.
2. **Hai lỗi thật tìm thấy và đã sửa** (không phải kẹt, nhưng mất phản hồi):
   - *Phản hồi của lượt thất bại bị mất khi Regenerate không kèm chữ.* Nháp của lượt thất bại bị xoá nên phản hồi không còn trong `prior_artifacts`. Sửa nhỏ nhất: `start` kế thừa `feedback` của lượt **gần nhất cùng loại** nếu lượt đó `failed` (`inheritedFeedback`, `usecase/solution_revision_recovery.go`). Gửi phản hồi mới thì phản hồi mới thay hẳn phản hồi cũ (không gộp); lượt gần nhất đã `succeeded` thì không kế thừa gì; lỗi đọc lịch sử lượt trả về chứ không nuốt.
   - *Người chỉnh thứ hai bị trả lại lượt của người thứ nhất như thể phản hồi của họ đã được áp.* `StartRun` trả lượt đang chạy (`Created=false`) không lỗi, và vì Request đã `analyzing` nên `checkAnalysisState` cho qua. Sửa: yêu cầu có `feedback` mà `StartRun` trả lượt của người khác (khác `idempotencyKey`) thì trả `REQUEST_SOLUTION_WRONG_STATE` (`refuseSecondRevision`, lỗi `ErrSolutionRevisionInProgress` ở `domain/solution_revision_errors.go`, cùng mã để client cũ vẫn hiểu, thông điệp nói rõ lý do). Cùng `idempotencyKey` (bấm đúp, thử lại) vẫn trả đúng lượt đó, `Regenerate` không phản hồi vẫn trả lượt đang chạy, đường hệ thống (làm rõ xong tự sinh lại) giữ cách tái dùng im lặng.
3. **Giới hạn đã biết, ghi lại:** người duyệt chỉ chỉnh được khi Request đang ở cổng (`awaiting_analysis_approval`, họ còn là approver của Approval đang chờ). Sau khi lượt chỉnh bắt đầu, Approval đã huỷ nên họ không còn quyền; chỉ người báo cáo hoặc `admin|lead|owner` chạy lại hay gửi phản hồi từ `analyzing`. Người duyệt đến sau lượt thắng nhận `REQUEST_SOLUTION_FORBIDDEN`, không phải `WRONG_STATE` (quyền kiểm trước trạng thái). Hai người chỉnh thật sự đồng thời (cùng đọc `awaiting`) thì người sau nhận `WRONG_STATE`.
4. **Gateway/MCP:** `GenerateSolution` chuyển nguyên `feedback` và `idempotencyKey` (không có kiểm quyền vai trò ở gateway; quyền do request-service quyết định). `SolutionView` đã mang `seq` và `revisionFeedback` từ task 02/03; thêm test. MCP `solution_list` trả cùng trường (kênh dùng `keepKeys`, không đổi khoá); mô tả `solution_generate` và `solution_list` nêu quyền approver, khi nào bản cũ `superseded`, `WRONG_STATE` và hai trường mới.

### Tệp đã đổi

- `backend-go/services/request-service/internal/usecase/generate_solution.go` (sửa `start`: kế thừa phản hồi, từ chối chỉnh thứ hai), `usecase/solution_revision_recovery.go` (mới), `domain/solution_revision_errors.go` (mới).
- Test: `usecase/solution_revision_failure_test.go` (mới); `adapter/contracttest/solution_revision_failure_contract.go` (mới, 2 kịch bản `FailedRevisionIsRecoverable`, `ConcurrentRevisionsStartOneRun`) và `solution_contract.go` (đăng ký hai dòng; `solCompleter` thêm `hold` để giữ lượt đang chạy).
- Gateway (`backend-go/services/api-gateway`): `internal/adapter/mcpserver/tools/pack_request_flow.go` (mô tả), `tools/request_flow_revision_test.go` (mới), `tools/request_flow_fixture_test.go` (`GenerateSolution` giả), `wscompat/channels_request_fake_test.go` và `channels_request_test.go` (assert `seq`, `revisionFeedback`, `feedback`, `idempotencyKey`).
- `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` mục 2.2: đoạn "Chỉnh phân tích bằng phản hồi (CR-REQ-041)" và cột Quyền của `solution.generate`.
- e2e (`tests/stages/e2e/orca-flows/`): `suite_analysis_revision.py`, `analysis_revision_driver.py`, `analysis_revision_users.py`, `test_analysis_revision_driver.py` (mới); `run_acceptance.py` (đăng ký suite `analysis_revision`, thêm vào `AI_USING`); `fake_claude_cli/claude` (xem dưới), `fake_claude_cli/test_fake_claude_cli.py`, `fake_claude_cli/request_service_contract.overlay.go.txt`.

### Fake claude (mở rộng nhỏ, xác định)

- Prompt có khối `<feedback>` sau `</request>`: câu trả lời thêm `[revised, round N, asked: <phản hồi một dòng, cắt 60 ký tự>]` (N = số bản trong `<prior_artifacts>` + 1) vào `summary` của option/diagnosis/findings hoặc `answer_markdown`. Nhờ đó vòng 2 khác vòng 3 và e2e thấy phản hồi cùng các bản trước đã tới model. Không có phản hồi: câu trả lời không đổi.
- `[stub:fail-once]` trong phản hồi: lần đầu thất bại (Solution: `is_error`; chỉ-đọc: exit 1), các lần sau cùng phản hồi thành công (marker theo băm phản hồi trong thư mục trạng thái). `[stub:slow]`: trả lời sau `FAKE_CLAUDE_SLOW_SECONDS` giây (mặc định 6; agent xoá biến `FAKE_*` nên thực tế là 6).
- Kiểm: `fake_claude_cli/test_fake_claude_cli.py` (35 test) và `verify-against-request-service.sh` (thêm `TestFakeClaude_RevisionRoundsAnswerDifferentlyAndStayValid`: bốn loại tài liệu qua validator thật).

### Blast radius (grep; GitNexus không dùng được)

- `GenerateSolution.start`: gọi từ `execute` (RPC `GenerateSolution`), `PrepareForSystem`/`ExecuteForSystem` (`SolutionRegenerator`, làm rõ xong tự sinh lại). Hành vi đổi: (a) lượt không phản hồi từ `analyzing` mang theo phản hồi của lượt thất bại gần nhất (chỉ khi lượt đó `failed`); (b) gọi không phải hệ thống, có `feedback`, gặp lượt đang chạy của khoá khác nay là lỗi thay vì tái dùng im lặng. Không đổi proto, schema, migration, gateway.
- `solCompleter` (contract test): thêm trường `hold`; mọi kịch bản cũ giữ nguyên hành vi (chạy lại toàn bộ hợp đồng hai dialect, xem dưới).
- Rủi ro: thấp-vừa. Client hiện hữu: UI gọi `solution.generate` có phản hồi chỉ từ cổng; nếu hai tab cùng chỉnh, tab sau nhận `WRONG_STATE` (đã được xử lý như mọi `WRONG_STATE`: tải lại).

### Lệnh đã chạy và kết quả

- `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh.
- `go test ./internal/usecase/ -run 'RevisionFailure|TestRevision_' -count=1 -v`: 23 test và test con PASS (3 chế độ lỗi ai.complete, 9 chế độ lỗi chỉ-đọc cho bug/spike/question, phản hồi mới từ `analyzing`, lượt đầu thất bại không mang gì, chỉnh thứ hai bị từ chối, bấm đúp, Regenerate khi đang chạy, đường hệ thống, lịch sử không đọc được làm start lỗi). Đã thử đột biến: bỏ `refuseSecondRevision` hoặc `inheritedFeedback` thì các test tương ứng đỏ.
- `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run Solution -count=1 -p 1` ba lần liên tiếp: xanh cả hai dialect (`FailedRevisionIsRecoverable`, `ConcurrentRevisionsStartOneRun` với 6 người chỉnh đồng thời: đúng một thắng, 5 `WRONG_STATE`, đúng hai lượt trong lịch sử, một Approval đang chờ ở bản mới). Toàn bộ hợp đồng hai dialect: MySQL `ok`; Postgres chỉ đỏ `TestTwoRelaysConcurrentlyNoLoss` (flake đã biết, `nats: no servers available`), xanh khi chạy riêng.
- Gateway: `go test -overlay <gw-overlay.json> ./internal/adapter/... -count=1` xanh; `-run 'Solution|Parity'` xanh (gồm `TestToolParity`, `TestRequestRPCChannelParity`, `TestChannelInventory`).
- `python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py`: 35 test OK. `python3 -m unittest discover -s tests/stages/e2e/orca-flows -p 'test_*.py'`: 44 OK. `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh`: ok.
- `python3 -m py_compile` các tệp suite và `python3 tests/stages/e2e/orca-flows/run_acceptance.py --list`: liệt kê `analysis_revision`.
- Mô phỏng trong bộ nhớ của kênh WebSocket (tệp tạm ngoài repo, không commit) chạy cả 13 ca ARV: bắt được hai lỗi của chính suite (hai ca dùng chung một phản hồi `fail-once`; lọc activity), đã sửa. Đây chỉ kiểm trình tự của suite và lỗi Python, **không** thay cho chạy thật.

### E2E: lệnh cho người điều phối

```
python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver analysis_revision
```
(`--ai-stub` tự thêm suite `aistub` chạy trước; suite này nằm cuối bảng để không làm `AST-02/03` đếm nhầm prompt.) Rồi `... --ai-stub devserver request` và `... --ai-stub devserver qa` để kiểm không hồi quy.

| ID | Kiểm |
|---|---|
| ARV-01 | change_request chỉnh 2 lần: 3 phiên bản (2 superseded, 1 proposed), Approval mới sau mỗi vòng, `revisionFeedback`, dấu `[revised, round N` trong tài liệu |
| ARV-02 | activity: đúng 2 dòng `analysis.revision_requested`, không có chữ của phản hồi ở bất kỳ dòng nào |
| ARV-03..05 | bug Diagnosis, spike Findings, question Answer: mỗi loại chỉnh 1 lần |
| ARV-06 | phản hồi > 2000 ký tự bị từ chối; Regenerate không phản hồi ở cổng là `WRONG_STATE`; không đổi gì |
| ARV-07/08 | approver (không phải người báo cáo) chỉnh được và được ghi là tác nhân; người lạ bị từ chối. Cần `docker exec orca-go-postgres` để ghi chính sách duyệt (chưa có kênh gateway, như `pilot_register.py`) và `admin.createUser` + đăng nhập; thiếu thì `BLOCKED` kèm lý do, không `FAIL` |
| ARV-09/10 | bấm đúp cùng khoá = một lượt; chỉnh thứ hai khi lượt đang chạy = `WRONG_STATE` (dùng `[stub:slow]`, cần `--ai-stub`) |
| ARV-11/12 | lượt chỉnh thất bại (`[stub:fail-once]`) cho change_request (ai.complete) và question (agent chỉ-đọc): Request ở `analyzing`, không còn gì để duyệt, Regenerate không chữ chạy lại với cùng phản hồi và thành công |
| ARV-13 | hotfix không có cổng: chỉnh bị `REQUEST_SOLUTION_WRONG_STATE`; nếu phân tích hotfix không xong trong môi trường thì `BLOCKED` |

Với Claude thật các ca này `SKIP` (tốn tiền, không có dấu echo) trừ khi đặt `ORCA_E2E_ANALYSIS_REVISION=1`; khi đó ca dùng trigger (ARV-09..12) vẫn `SKIP`.

### Chưa kiểm chứng

- **Chưa chạy `suite_analysis_revision.py` trên stack thật** (đề bài cấm): chưa biết `admin.createUser` + `/auth/local` của người dùng mới có đăng nhập được ngay không, chưa biết gateway có chặn người dùng không phải thành viên dự án `private` trước khi tới request-service không (ARV-08 chấp nhận `REQUEST_SOLUTION_FORBIDDEN`, `PERMISSION_DENIED`, `FORBIDDEN`, `NOT_FOUND`; ARV-07 sẽ `FAIL` nếu approver bị chặn ở gateway, đó là một phát hiện thật), chưa biết phân tích hotfix có hoàn tất trong stack mỗi dev, và chưa biết `seq` có được lập ở production (suite chỉ kiểm thứ tự khi mọi bản đều có `seq`).
- Chưa chạy với Claude thật; chất lượng phản hồi là opt-in.
- Tài liệu `README.md` của `orca-flows` chưa nhắc suite mới (ngoài phạm vi tệp được giao).
