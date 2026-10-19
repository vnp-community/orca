# TASK-REQ-042-05: Chụp golden của mọi prompt hiện tại (trước khi di chuyển)

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `request-service, task-service, git-gateway-service`
**File:** `backend-go/services/request-service/internal/usecase/testdata/prompt-golden/`, `.../adapter/grpcclient/testdata/`, `task-service/.../testdata/`, `git-gateway-service/.../testdata/` (mới) + test sinh golden; `backend-go/ci/check-prompt-golden.sh` (mới)
**Depends on:** TASK-REQ-041-02 (đổi prompt phân tích: phản hồi cũ và giới hạn `prior_artifacts`); làm trước TASK-REQ-042-06 và 07. Nếu 041-02 chưa xong thì chụp golden sau khi nó xong, không chụp hai lần.
**Status:** [x] DONE (2026-10-19)

---

## Context

- Mọi hàm dựng prompt hiện nay sinh chuỗi từ dữ liệu miền; không có golden.
- Golden là cổng an toàn của việc chuyển sang `.tmpl`: đầu ra mới phải trùng từng byte.

## Việc cần làm

1. Lập danh sách hàm dựng và bộ fixture đại diện cho từng luồng: classify; Solution (`ai.complete` và `AgentSolution`, có/không `UseKnowledge`); chẩn đoán theo `diagnosisFocus` (bug, security, performance); Findings; Answer; có/không `Feedback`, `RetryNote`, `PriorArtifacts`, `Answers`; Plan native theo `planShapeRules`/`planTypeRules` (mỗi loại); Plan OpenSpec; Solution OpenSpec; thực thi task (có/không cha, phụ thuộc, `PromptTemplate`, `{{outputs.*}}`) kèm `ContractPromptSuffix` (các tổ hợp `requirePassingCheck`, `knowledgeHub`); tách task; sinh prompt agent; commit message; agent prompt của git.
2. Test sinh file golden (cờ `-update`), một file mỗi ca, dữ liệu fixture cố định (không thời gian, không UUID ngẫu nhiên).
3. Test so khớp mặc định (không `-update`); `ci/check-prompt-golden.sh` chạy test và thất bại nếu golden thay đổi mà không có commit sửa golden cố ý.
4. Ghi vào PR số ca và hàm đã phủ; hàm nào không thể golden (phụ thuộc IO) thì nêu lý do.

## Kiểm thử

- Chạy test hai lần liên tiếp cho cùng kết quả (xác định).
- Cố ý đổi một câu trong prompt để chứng minh CI bắt được.

## Tiêu chí hoàn thành

- [x] Mỗi hàm dựng prompt có ít nhất một golden; luồng có nhánh có golden cho từng nhánh.
- [x] Golden xác định, không phụ thuộc thời gian hay ngẫu nhiên.
- [x] CI bắt thay đổi golden không chủ ý.

## Rủi ro và lưu ý

- Fixture phải tránh dữ liệu thật; không đưa bí mật vào golden.

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

**Chỉ thêm file kiểm thử, testdata và script; không sửa mã sản xuất nào.** Chụp mã hiện tại (đã gồm TASK-REQ-041-02: phản hồi của các bản cũ trong `prior_artifacts`, trần 40000 rune).

### File thêm
- Helper so khớp (một bản mỗi gói test, cùng nội dung; không dùng gói chung để không thêm mã sản xuất): `prompt_golden_assertion_test.go` trong `request-service/internal/usecase`, `request-service/internal/adapter/grpcclient`, `task-service/internal/domain`, `task-service/internal/adapter/grpcclient`, `task-service/internal/usecase`, `git-gateway-service/internal/usecase`. So từng byte với `testdata/prompt-golden/<hàm>/<ca>.txt`, báo dòng khác đầu tiên, báo file golden mồ côi (không còn ca), cờ `-update` ghi lại (và xoá file mồ côi). `request-service/internal/usecase` dùng lại cờ `updateGolden` đã có (`openspec_plan_prompt_test.go`) vì một gói không đăng ký cờ `update` hai lần.
- Test: `request-service/internal/usecase/prompt_golden_{fixtures,solution,plan,openspec,clarification}_test.go`, `request-service/internal/adapter/grpcclient/classification_prompt_golden_test.go`, `task-service/internal/domain/execution_result_prompt_golden_test.go`, `task-service/internal/adapter/grpcclient/execute_prompt_golden_test.go`, `task-service/internal/usecase/decompose_and_generation_prompt_golden_test.go`, `git-gateway-service/internal/usecase/agent_and_commit_prompt_golden_test.go`. Tên test `TestPromptGolden_<hàm>`.
- 158 file golden dưới `.../testdata/prompt-golden/` (khoảng 800 KB; hai ca trần `prior_artifacts_cap_*` mỗi ca khoảng 34 KB).
- `backend-go/ci/check-prompt-golden.sh` (so khớp, không bao giờ ghi; `--update` để cập nhật có chủ ý; thất bại nếu `-run` không khớp test nào).
- `.gitattributes`: thêm một dòng `eol=lf` cho `testdata/prompt-golden/**/*.txt` để Windows không đổi xuống dòng.

### Số ca theo hàm
| Hàm | Ca |
|---|---|
| `BuildSolutionPrompt` | 36 (ai.complete 8, agent 4, diagnosis bug/security/performance/hotfix/không focus + retry 6, findings 2, answer 2, feedback/retry 5, prior artifacts 7 gồm 3 vòng revision và hai ca vượt trần 40000, answers 2) |
| `BuildPlanPrompt` | 29 (11 loại Request cho `planTypeRules`, 5 shape, phases + performance, feedback, retry, analysis, giới hạn tuỳ chỉnh, bản gộp) |
| `approvedAnalysisText` (đầu vào Analysis của Plan) | 5 |
| `BuildOpenSpecPlanPrompt` | 18 (3 cỡ S/M/L, shape, knowledge có/không, feedback, retry, cắt, giới hạn chặn khoảng task) |
| `TaskRangeFor` | 1 (bảng 4 cỡ x 2 giới hạn) |
| `BuildOpenSpecSolutionPrompt` (chưa có nơi gọi) | 5 |
| `BuildTaskPromptTemplate` (prompt mẫu mỗi task OpenSpec) | 3 |
| `writeClarificationAnswers` | 6 (gồm bí mật bị che, thẻ đóng trong mọi trường, cắt) |
| `BuildClassificationPrompt` | 6 |
| `buildExecutePrompt` | 12 (có/không PromptTemplate, cha, phụ thuộc, `{{outputs.*}}`, `.stdout`, token lạ giữ nguyên) |
| `ContractPromptSuffix` | 5 (4 tổ hợp requirePassingCheck x knowledgeHub + nonce rỗng) |
| `buildDecomposePrompt` | 10 |
| `buildPromptGenerationPrompt` | 5 |
| git-gateway `buildAgentPrompt` | 7 |
| git-gateway `buildCommitMessagePrompt` | 7 |
| git-gateway `statsOnlySummary` | 2 |
| git-gateway `prFieldsPromptPrefix` | 1 |

Tổng 158 ca.

### Phân loại và hạn chế
- `buildAgentPrompt` (git-gateway) là prompt tiêm vào phiên agent của worktree tạo từ issue (`SpawnAndInject`): đi vào catalog. `buildCommitMessagePrompt` là prompt gửi qua relay `ai.complete`: đi vào catalog. `statsOnlySummary` là phần thân của prompt đó.
- Prompt PR (`generate_pull_request_fields.go`) không có hàm dựng: `Execute` nối hằng `prFieldsPromptPrefix` với diff. Chỉ chụp tiền tố cộng một diff cố định; phần nối nằm ở `Execute` (cần IO), task 07 phải giữ thứ tự nối.
- `BuildClassificationPrompt` có phần ngẫu nhiên duy nhất là ranh giới `<<<REQUEST_DATA_<16 hex>>>` (`crypto/rand` trong `newBoundary`, không sửa được). Test khẳng định đúng hai lần xuất hiện, cùng giá trị, rồi thay bằng `<<<REQUEST_DATA_BOUNDARY>>>` trước khi so; đổi định dạng ranh giới vẫn làm hỏng golden.
- Chưa chụp (ngoài danh sách của task): `formatReviewPrompt`/`ComposeReviewFeedbackPrompt` trong `api-gateway/.../wscompat/channels_annotation_send.go` (prompt phản hồi review tiêm vào terminal; api-gateway không biên dịch trên main nên không đưa vào gate), 5 mẫu MCP `.tmpl` (task 07 đã nêu ngoài phạm vi).
- Phát hiện (không sửa): `sanitize` của git-gateway cắt theo byte (`truncate`), có thể cắt giữa một ký tự nhiều byte. Golden chỉ dùng ASCII để không ghim UTF-8 hỏng; nếu task 07 đổi sang cắt theo rune thì đó là thay đổi hành vi cần quyết định riêng.
- Golden không bắt được thay đổi nằm ngoài chuỗi (thứ tự gọi, điều kiện chọn mẫu, bước ánh xạ miền sang đầu vào); test ánh xạ của task 06/07 bù phần đó.
- Script chưa được gắn vào `Makefile` hay workflow CI (ngoài tệp được giao). Đề xuất: thêm target `check-prompt-golden` vào `backend-go/Makefile` cạnh `check-agent-capability-golden` và gọi trong `backend-go-request-contracts.yml` (hoặc workflow chạy `task-service`/`git-gateway-service`).

### Blast radius
Không sửa symbol sẵn có nào (chỉ thêm `_test.go`, testdata, script, một dòng `.gitattributes`), nên không có người gọi bị ảnh hưởng. Các hàm dựng prompt được chụp nguyên trạng; đã kiểm `git diff` các file chứa chúng không đổi so với HEAD (các thay đổi chưa commit của tác tử khác trong `simple_executor.go`, `generate_solution.go` không chạm `buildExecutePrompt`/`BuildSolutionPrompt`).

### Lệnh đã chạy
- Sinh: `backend-go/ci/check-prompt-golden.sh --update` (và `go test -run TestPromptGolden_ ... -update` lúc phát triển).
- So khớp: `backend-go/ci/check-prompt-golden.sh` -> 6 gói `ok`, exit 0, chạy hai lần liên tiếp.
- Tính xác định: băm sha256 của 158 file trước và sau hai lần `--update` liên tiếp và hai lần so khớp: giống hệt.
- Chứng minh cổng cắn: sao lưu một golden (`BuildPlanPrompt/type_bug_plan_tasks.txt`), đổi `regression test` thành `regresion test` trong file golden (mô phỏng prompt đổi một chữ), chạy script: thất bại, exit 1, in dòng khác đầu tiên và hướng dẫn; khôi phục từ bản sao lưu, sha256 khớp, script xanh lại. Thêm file `stray.txt` vào một thư mục golden: thất bại với "has no test case"; đã xoá.
- `go vet ./internal/...` cho request-service, task-service, git-gateway-service: sạch.
- `go test -race -count=1` cho 6 gói trên (toàn bộ test của gói, không chỉ golden): xanh.

### Không kiểm chứng
- `golangci-lint` (không chạy được trên máy này). Không chạy trên macOS ngoài máy này, Linux hay Windows; script là bash (Windows cần Git Bash/WSL như các script ci khác).
