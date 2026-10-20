# TASK-REQ-046-03: Ca golden mới cho `vi`/`en`, bằng chứng golden cũ không đổi, ngân sách và CI

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `request-service`, `backend-go/prompts`
**File:** `services/request-service/internal/usecase/{prompt_golden_solution_test.go,prompt_golden_plan_test.go,prompt_golden_openspec_test.go}` (sửa: thêm ca), `.../testdata/prompt-golden/{BuildSolutionPrompt,BuildPlanPrompt,BuildOpenSpecPlanPrompt}/*.txt` (thêm tệp), `backend-go/prompts/README.md` (số đo ngân sách)
**Depends on:** TASK-REQ-046-01, 046-02
**Status:** [ ] TODO

---

## Context

- 158 tệp golden nằm ở sáu gói dịch vụ (`request-service/internal/usecase` 103 tệp: `BuildSolutionPrompt` 36, `BuildPlanPrompt` 29, `BuildOpenSpecPlanPrompt` 18, `BuildOpenSpecSolutionPrompt` 5, `BuildTaskPromptTemplate` 3, `TaskRangeFor` 1, `approvedAnalysisText` 5, `writeClarificationAnswers` 6; còn lại ở `request-service/adapter/grpcclient`, `task-service`, `git-gateway-service`). Không có `backend-go/testdata/prompt-golden`.
- `checkPromptGoldens` (`prompt_golden_assertion_test.go:16`) lỗi nếu có tệp golden không có ca; `--update` sinh lại từ render thật. Theo `prompts/README.md`: golden chỉ đổi bằng tái sinh có chủ ý và có rà soát, không bao giờ để làm CI xanh.
- Thiết kế bảo đảm ca cũ dùng `Language` rỗng nên không đổi; task này chứng minh điều đó rồi **chỉ thêm** ca mới.

## Việc cần làm

1. Trước khi thêm gì: chạy `bash backend-go/ci/check-prompt-golden.sh` (so sánh) trên nhánh đã có 046-01 và 046-02: phải xanh. Ghi vào PR.
2. Thêm ca (đặt `Language` bằng `output_language.Vietnamese`/`English` qua `mod` của ca):
   - `BuildSolutionPrompt`: `solution_ai_complete_language_vi`, `solution_ai_complete_language_en`, `solution_agent_knowledge_language_vi`, `diagnosis_bug_language_vi`, `findings_language_vi`, `answer_language_vi`.
   - `BuildPlanPrompt`: `shape_plan_phases_language_vi`, `type_bug_plan_tasks_language_vi`, `shape_plan_tasks_language_en`.
   - `BuildOpenSpecPlanPrompt`: `phases_medium_knowledge_language_vi`, `tasks_medium_no_knowledge_language_en`.
3. Sinh tệp mới: `bash backend-go/ci/check-prompt-golden.sh --update`; rồi `git status --short -- '*/testdata/prompt-golden/*'` phải chỉ có `??`/`A` (11 tệp mới, 158 → 169), **không** có `M` hay `D`. Nếu có `M`, mẫu hoặc ánh xạ sai: sửa mã, không sửa golden.
4. Đọc từng golden mới bằng mắt: dòng `Output language: …` đúng chỗ, đúng một lần, không thừa dòng trống ở phần còn lại.
5. Ghi số đo ngân sách của năm luồng phân tích, `plan_native`, `plan_openspec` (từ 046-01) vào bảng của `prompts/README.md`.
6. `make check-prompts` (trong `backend-go/`) xanh: `check-agent-relay-single-source`, `check-prompt-outside-catalog`, `check-prompt-golden`, `check-prompt-guards-selftest`.

## Kiểm thử

- `cd backend-go/services/request-service && go test ./internal/usecase -run 'TestPromptGolden_' -count=1` xanh, chạy hai lần liên tiếp cho cùng kết quả.
- `cd backend-go/prompts && go test ./... -race` xanh (ngân sách).
- Cố ý đổi một chữ của partial: `check-prompt-golden.sh` phải đỏ ở các ca `*_language_*` và **không** đỏ ở ca cũ; hoàn lại.

## Tiêu chí hoàn thành

- [ ] Golden so sánh xanh trước khi thêm ca (bằng chứng không đổi).
- [ ] 11 tệp golden mới, không tệp cũ nào đổi (`git diff --stat` chỉ có thêm).
- [ ] `make check-prompts` và test ngân sách kích thước xanh; số đo ghi trong README.

## Rủi ro và lưu ý

- Không chạy `--update` để làm một golden cũ xanh; nếu một golden cũ đổi thì đó là lỗi (rất có thể là khoảng trắng quanh dòng gọi partial).
- Job `prompt-catalog` của `.github/workflows/backend-go-request-contracts.yml` chạy khi `backend-go/prompts/**` hoặc `services/**` đổi; xác nhận nó xanh trên nhánh.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
