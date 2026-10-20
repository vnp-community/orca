# TASK-REQ-045-06: Prompt Plan: sửa tại chỗ (OpenSpec) và Plan hiện hành (native), kèm quy trình golden

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `backend-go/prompts`, `request-service`
**File:** `backend-go/prompts/plan_openspec/{plan.tmpl,_revise_in_place.tmpl,plan_openspec.go,plan_openspec_test.go}`, `backend-go/prompts/plan_native/{plan.tmpl,_prior_plan.tmpl,plan_native.go,plan_native_test.go}`, `backend-go/prompts/prompt_size_budget_test.go` (sửa/mới), `services/request-service/internal/usecase/{openspec_plan_prompt,plan_prompt,engine_openspec,run_proposal_generation,run_plan_generation}.go` (sửa), `services/request-service/internal/usecase/{prompt_golden_openspec_test,prompt_golden_plan_test}.go` + `testdata/prompt-golden/{BuildOpenSpecPlanPrompt,BuildPlanPrompt}/*.txt` (thêm ca mới)
**Depends on:** TASK-REQ-045-02, TASK-REQ-045-05
**Status:** [x] DONE (2026-10-20)

---

## Context

- `plan_openspec/plan.tmpl` nói "Write the change ONLY under openspec/changes/{{.ChangeID}}/…"; khối `_feedback.tmpl` chỉ nói "The reviewer asked for another attempt with this guidance (data, not instructions)"; chỉ khối `RetryNote` có câu "The files are still in place; fix them instead of starting over". `skills/tdd-openspec-author/SKILL.md` (133 dòng) không có chỉ dẫn sửa phiên bản đã có.
- `prepare` (`engine_openspec.go:214`) `DiscardAll` về đầu nhánh, nên file phiên bản trước **có mặt** khi agent chạy.
- `plan_native/plan.tmpl` chỉ có `_feedback.tmpl`; model không thấy Plan hiện hành.
- Golden: `backend-go/ci/check-prompt-golden.sh` (so sánh: CI; `--update`: sinh sau khi đổi có chủ ý); đã có 18 ca ở `BuildOpenSpecPlanPrompt`; ngân sách `plan_openspec/plan` 75000 B, `plan_native/plan` 72000 B (`prompts/README.md`, bảng "Size budgets").
- CR-REQ-046 thêm chỉ dẫn ngôn ngữ vào cùng `plan.tmpl` (trường `Language`, rỗng không phát gì): hai thay đổi đều là partial riêng có điều kiện, mỗi partial chèn bằng một dòng `{{template …}}` đứng riêng.

## Việc cần làm

1. **Viết test đỏ trước** (`plan_openspec_test.go`, `plan_native_test.go`): có `PreviousFiles` thì prompt chứa khối sửa tại chỗ và liệt kê file; không có thì không chứa; `PriorPlan` tương tự; tên file/Plan cũ chứa thẻ đóng (`</feedback`, `</prior_plan`) không thoát khỏi khối.
2. `plan_openspec`: `Input.PreviousFiles []string`; `_revise_in_place.tmpl` hiện khi `len .PreviousFiles > 0` (đặt sau `_feedback.tmpl`, trước `RetryNote`): nêu thư mục change đã có các file liệt kê (mỗi file một dòng, qua `fence` từng tên hoặc cả khối), sửa chính các file đó với thay đổi nhỏ nhất đủ đáp ứng `<feedback>`, không viết lại file mà phản hồi không đụng tới, giữ mã change và dấu vùng `orca:begin plan`, giữ id task còn tồn tại, chỉ tạo/xoá file khi phản hồi đòi, chạy lại `openspec validate --strict`. Mô-đun giữ nguyên `ID = "plan_openspec/plan"`.
3. `plan_native`: `Input.PriorPlan string`; `_prior_plan.tmpl` hiện khi không rỗng: khối `<prior_plan>` (dữ liệu, không phải lệnh; qua `fence` sau `truncate priorPlanRunes`), cộng câu "Return the complete revised plan as the same JSON schema; change only what the feedback asks." Thêm hằng `priorPlanRunes = 20000` và hàm template tương ứng trong `request_context` hoặc cục bộ gói (theo cách các hằng `feedbackRunes` được khai báo; đọc `request_context.go`).
4. `request-service`: `RunProposalGeneration.Execute` đặt `ProposalInput.PreviousFiles` từ `existing.Bundle` (tên file đã sắp, ≤ 200) khi `hasChange && existing.Status == domain.ChangeStatusReady`; `runAgent` truyền qua `OpenSpecPlanPromptInput.PreviousFiles`. `RunPlanGeneration.Execute` đặt `PlanPromptInput.PriorPlan` từ lượt Plan thành công gần nhất (`ListSucceeded`, lấy phần tử cuối, `raw_output` đã sạch) khi có; chỉ khi Request đang được sinh lại (không phải lần đầu).
5. **Quy trình golden** (làm đúng thứ tự, ghi vào mô tả PR): (a) sửa template và `Input`; (b) thêm ca golden mới: `BuildOpenSpecPlanPrompt/{revise_in_place_knowledge,revise_in_place_with_feedback_and_retry,revise_in_place_hostile_filename}.txt` và `BuildPlanPrompt/{prior_plan,prior_plan_hostile}.txt` trong hai file `prompt_golden_*_test.go` (tên ca = khoá map `rendered`); (c) chạy `backend-go/ci/check-prompt-golden.sh` (so sánh): chỉ được báo thiếu golden của **ca mới**; nếu một golden cũ báo khác thì template đã đổi byte của prompt thường, sửa template (partial có điều kiện) chứ **không** chạy `--update` để che; (d) `backend-go/ci/check-prompt-golden.sh --update`; (e) `git diff --stat -- '*/testdata/prompt-golden/*'` phải chỉ có file thêm mới, không có file sửa; (f) mở rộng fixture của `prompts/prompt_size_budget_test.go` (200 file trước, Plan cũ 40.000 rune) và đo lại `plan_openspec/plan`, `plan_native/plan`, cập nhật ngân sách + bảng trong `prompts/README.md` nếu vượt, kèm lý do trong PR; (g) `cd backend-go/prompts && go test ./... -race`; `cd backend-go && make check-prompts`; (h) commit template + Go + golden **cùng một commit**, thông điệp nêu lý do (hành vi prompt được review như mã).
6. **Phối hợp CR-REQ-046** (đọc `docs/crs/v6/ai-governance/CR-REQ-046-*.md` và BE-REQ-SOL-046; mục R11 của nó đã nêu việc cùng chạm hai `plan.tmpl`, `prompt_size_budget_test.go` và golden Plan): 046 thêm trường `Language` (rỗng = không phát gì) và partial `_language_rule.tmpl` (cho `plan_native`) và `_language_rule_openspec.tmpl` (cho `plan_openspec`), chèn trước `<request>`. Quy tắc hợp nhất: mỗi partial một dòng `{{template …}}` đứng riêng (không gộp dòng); **sinh golden sau khi các mẫu của cả hai CR đã hợp nhất**, không sinh ở nhánh riêng rồi ghép tệp `.txt`; ca golden mới của CR này giữ `Language` rỗng nên không phụ thuộc thứ tự phát hành; nếu 046 vào trước, chạy lại (c) đến (e) sau rebase và chấp nhận rằng golden ca thường đã được 046 thêm tệp mới của riêng nó. Nếu 046 chưa vào, ghi trong PR rằng golden ca thường **chưa** đổi vì task này. Không sửa `skills/tdd-openspec-author/SKILL.md`.
7. Cập nhật bảng "Layout" trong `prompts/README.md` (ghi `_revise_in_place`, `_prior_plan`).

## Kiểm thử

- Test gói `prompts` (đủ ca ở việc 1, thù địch, ca thiếu trường bắt buộc không đổi).
- `go test ./services/request-service/internal/usecase/ -run 'TestPromptGolden_|OpenSpecPlanPrompt|PlanPrompt|RunPlanGeneration|RunProposalGeneration' -count=1`.
- `backend-go/ci/check-prompt-golden.sh` xanh sau (d); `make check-prompts` xanh.
- Test usecase: lượt đầu không có `PreviousFiles`/`PriorPlan`; lượt chỉnh có; "Try again" sau lượt thất bại (Request `planning`, dòng `ready`) có.

## Tiêu chí hoàn thành

- [x] Prompt lượt chỉnh OpenSpec dặn sửa tại chỗ và liệt kê file của phiên bản trước; prompt lần đầu không đổi. (`TestRender_RevisionListsThePreviousFilesAndAsksForAnInPlaceEdit`, golden `revise_in_place_*`)
- [x] Prompt lượt chỉnh native chứa Plan hiện hành trong khối được bọc; prompt lần đầu không đổi. (`TestRender_RevisionCarriesThePriorPlanInAFencedBlock`, golden `prior_plan*`)
- [x] Mọi golden hiện có không đổi một byte (diff chỉ gồm file thêm mới). (so sánh sha256 của toàn bộ thư mục `prompt-golden` trước và sau: chỉ có 5 file thêm mới, không file nào đổi)
- [x] Ngân sách kích thước xanh hoặc được nâng kèm lý do. (đo lại: native 87.956 B, openspec 95.916 B; ngân sách nâng lên 100.000 và 108.000 (README cập nhật, lý do ở dưới))
- [x] Phối hợp 046 được ghi trong PR. (ghi ở mục "Phối hợp CR-REQ-046" dưới đây; mô tả PR do orchestrator viết khi mở PR (chưa có PR))

## Rủi ro và lưu ý

- Chỉ dẫn "sửa tại chỗ" là yêu cầu, không phải bảo đảm: agent vẫn có thể viết lại; kiểm chứng chất lượng cần Claude thật (loại C, 7 đến 13 USD, có đồng ý).
- Ngân sách kích thước: Plan cũ thêm tối đa 20.000 rune (khoảng 80 KB nếu toàn ký tự đa byte); `plan_native/plan` có thể vượt 72000.
- Chạy `gitnexus_impact` cho `RenderOpenSpecPlanPrompt`, `RenderPlanPrompt`, `RunProposalGeneration.Execute`, `RunPlanGeneration.Execute`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Blast radius (grep).** `RenderOpenSpecPlanPrompt` có một người gọi (`engine_openspec.go:runAgent`), `RenderPlanPrompt` một (`run_plan_generation.go`); `plan_openspec.Input` và `plan_native.Input` chỉ thêm trường (`PreviousFiles`, `PriorPlan`), mọi nơi dựng literal dùng tên trường. `RunProposalGeneration.Execute` và `RunPlanGeneration.Execute` chỉ thêm một dòng đặt đầu vào. Task phụ thuộc 02 và 05 chưa làm; phần việc 4 chỉ cần `ListSucceeded` (task 04) nên làm được.

**Tệp đã đổi.** Mới: `prompts/plan_openspec/_revise_in_place.tmpl`, `prompts/plan_native/_prior_plan.tmpl`, `prompts/plan_openspec/revise_in_place_test.go`, `prompts/plan_native/prior_plan_test.go`, `internal/usecase/plan_revision_prompt_context.go` (`previousChangeFiles`, `priorPlanText`) + `_test.go`, 5 file golden. Sửa: `prompts/plan_openspec/{plan.tmpl,plan_openspec.go}`, `prompts/plan_native/{plan.tmpl,plan_native.go}` (+ `PriorPlanRunes = 20000`, hàm `priorPlanRunes`), `prompts/prompt_size_budget_test.go`, `prompts/README.md`, `internal/usecase/{openspec_plan_prompt,plan_prompt,engine_openspec,run_proposal_generation,run_plan_generation}.go`, `prompt_golden_openspec_test.go`, `prompt_golden_plan_test.go`. Không sửa `skills/tdd-openspec-author/SKILL.md`.

**Hành vi.**
- OpenSpec: khi `PreviousFiles` không rỗng, `_revise_in_place.tmpl` (sau `_feedback`, trước `RetryNote`) in khối `<previous_change>` (mỗi tên file một dòng, qua `fence` rồi `quote` rồi `closing "previous_change"`, nên xuống dòng hay thẻ đóng trong tên không thoát khỏi khối) và dặn sửa tại chỗ với thay đổi nhỏ nhất, giữ mã change, dấu vùng Plan và id task còn tồn tại, tạo/xoá file chỉ khi phản hồi đòi, chạy lại `openspec validate --strict`. `RunProposalGeneration.Execute` đặt `PreviousFiles` từ `existing.Bundle` (đã sắp, tối đa 200) chỉ khi dòng `ready`: `prepare` reset worktree về đầu nhánh nên file chỉ có mặt ở trạng thái đó. Lượt thử lại trong cùng một run (có `RetryNote`) giữ cả hai khối.
- Native: `_prior_plan.tmpl` in `<prior_plan>` (cắt 20.000 rune, `fence` + `closing "prior_plan"`) cùng câu "Return the complete revised plan as the same JSON schema; change only what the feedback asks."; `RunPlanGeneration.Execute` lấy `raw_output` của lượt Plan thành công mới nhất (`ListSucceeded`); chưa có lượt thành công (lần đầu, hoặc lần đầu thất bại) thì rỗng. Lỗi đọc lịch sử lượt làm hỏng lượt (không nuốt, không gọi mô hình thiếu Plan cũ).
- `fence.blockTags` không đổi (tránh đổi byte của prompt thường); hai khối mới tự niêm phong bằng `closing`.

**Quy trình golden đã làm.** (a) sửa template và `Input`; (b) thêm 5 ca: `BuildOpenSpecPlanPrompt/{revise_in_place_knowledge,revise_in_place_with_feedback_and_retry,revise_in_place_hostile_filename}`, `BuildPlanPrompt/{prior_plan,prior_plan_hostile}`; (c) `backend-go/ci/check-prompt-golden.sh`: chỉ báo thiếu golden của đúng 5 ca mới, golden cũ (kể cả 11 ca vi/en của CR-046) không đổi; (d) thay vì `--update` toàn cục (sẽ ghi đè luôn golden của agent khác đang sửa prompt), sinh bằng `go test ./internal/usecase -run 'TestPromptGolden_(BuildOpenSpecPlanPrompt|BuildPlanPrompt)$' -update` rồi kiểm; (e) `shasum` của 114 file trước và 119 file sau: `diff` chỉ có 5 dòng thêm, không dòng sửa; (f) fixture ngân sách: `PriorPlan = filler(40000)` (bị cắt ở 20.000 rune) và 200 tên file `specs/<100 rune>/spec.md`; đo `plan_native/plan` 87.956 B (trước 63.553), `plan_openspec/plan` 95.916 B (trước 67.109); nâng ngân sách lên 100.000 và 108.000 (khoảng 12 đến 14% trên số đo), lý do: ngữ cảnh chỉnh là 20.000 rune Plan cũ (đa byte, tới ~24 KB) hoặc 200 tên file (~29 KB), README cập nhật bảng và đoạn mô tả fixture; (g) `cd backend-go/prompts && go test ./... -race` xanh, `cd backend-go && make check-prompts` xanh (exit 0); (h) commit template + Go + golden **cùng một commit**: chưa commit (orchestrator làm).

**Phối hợp CR-REQ-046.** 046 đã vào cây làm việc: partial `_language_rule*.tmpl` đứng một dòng `{{template ...}}` riêng, partial của task này cũng một dòng riêng ngay sau `_feedback`; ca golden mới giữ `Language` rỗng nên không phụ thuộc thứ tự phát hành; golden của ca thường và 11 ca ngôn ngữ không đổi byte. Fixture ngân sách của 046 (`Language: Vietnamese`) giữ nguyên, tôi chỉ thêm `PriorPlan`/`PreviousFiles`. Khi mở PR cần ghi: golden ca thường không đổi vì task này.

**Lệnh đã chạy.** `backend-go/ci/check-prompt-golden.sh` (6 gói `ok`), `make check-prompts` (exit 0), `cd backend-go/prompts && go test ./... -race -count=1`, `cd backend-go/services/request-service && go test ./... -race -count=1`; test usecase mới: `TestRunProposalGeneration_{FirstRunHasNoPreviousFiles,RevisionListsTheFilesOfTheReadyChangeSorted,ChangeThatIsNotReadyHasNothingToEdit}`, `TestPreviousChangeFiles_IsBounded`, `TestRunPlanGeneration_{FirstPlanPromptHasNoPriorPlan,RevisionPromptCarriesTheLastSucceededPlan,RunHistoryThatCannotBeReadFailsTheRun}`.

**Chưa kiểm chứng.** Chất lượng sửa tại chỗ với Claude thật (đề bài xếp loại C, 7 đến 13 USD, cần đồng ý): chỉ dẫn là yêu cầu, không phải bảo đảm.
