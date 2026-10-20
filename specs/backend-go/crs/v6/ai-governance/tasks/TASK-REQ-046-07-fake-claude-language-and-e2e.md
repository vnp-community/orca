# TASK-REQ-046-07: Claude giả hiểu chỉ thị ngôn ngữ, suite `OLG`, spec UI và kiểm `--real` một lượt

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `tests/e2e`
**File:** `tests/stages/e2e/orca-flows/fake_claude_cli/{claude,test_fake_claude_cli.py,request_service_contract.overlay.go.txt}` (sửa), `tests/stages/e2e/orca-flows/suite_output_language.py` (mới, đăng ký trong `run_acceptance.py`), `ui-live/analysis-output-language.web.e2e.ts` (mới, stub), `ui-live/analysis-output-language-real.web.e2e.ts` (mới, `--real`), `run_ui_flow.py` (nhóm `language`), `README.md` của `orca-flows` (danh mục test)
**Depends on:** TASK-REQ-046-02, 046-03, 046-06 (stack phải chạy bản `request-service` mới); 046-05 cho ghi chú OpenSpec
**Status:** [x] DONE (2026-10-20): trừ kiểm đột biến `en` (không được phép đổi biến môi trường của dịch vụ đang chạy) và lần so token với `en`

---

## Context

- Claude giả (`fake_claude_cli/claude`) nhận prompt bằng câu mở đầu và loại đọc-chỉ bằng `"kind":"<loại>"` trước `<request>`; ghi mỗi lời gọi vào `calls.jsonl`; suite đọc lại bằng `_stub(ctx).calls()` (`suite_ai_stub_login.py`). Marker `[revised, round N, asked: …]` do suite `ARV` đọc (`analysis_revision_driver.py`): không đổi.
- Mặc định compose là `vi` (task 06): mọi suite `--ai-stub` hiện có từ nay nhận chỉ thị `Output language: Vietnamese.`. Đã `grep` `tests/` không thấy suite hay spec nào khẳng định trên chuỗi tiếng Anh do claude giả sinh (ngoài `"fake-claude" in reason` của phân loại, không đổi).
- Lớp chi phí: A = stub, miễn phí; B = một lượt phân tích chỉ đọc thật (loại `question`), chi phí chưa đo, opt-in; run OpenSpec thật (7 đến 13 USD) ngoài phạm vi và cần đồng ý riêng. Quy tắc `--real`: không sửa checkout orca trong lúc chạy (guard quiet-checkout); thăm dò (đăng nhập `claude`, dev server nối) trước khi tốn tiền.

## Việc cần làm

1. **Claude giả (thay đổi tối thiểu):**
   - `language_directive(prompt)`: lấy phần trước `\n<request>\n`, tìm dòng `^Output language: (\w+)\.$`, ánh xạ `Vietnamese→vi`, `English→en`, không có → `""`.
   - `record(...)` ở `run_print_stdin` và `run_print_arg` thêm `"outputLanguage": language_directive(prompt)` vào `extra`.
   - `solution_answer`, `plan_answer`, `readonly_answer`: khi mã `vi`, các chuỗi cho người đọc (`title`, `summary`, `approach`, `pros`, `cons`, `rollback`, `statement`, `answer_markdown`, `description`, `notes`) lấy từ bảng `VI` (khoảng 15 chuỗi có đủ dấu, mỗi chuỗi dưới giới hạn rune); id, enum, nhãn, khoá giữ nguyên; `revision_note` giữ nguyên chữ. Cập nhật docstring đầu tệp (một đoạn).
2. Test của claude giả: mở rộng `fake_claude_cli/test_fake_claude_cli.py` (unittest, không cần stack; dùng `run_stub`): không chỉ thị → văn bản tiếng Anh như cũ; `Output language: Vietnamese.` trước `<request>` → chuỗi có dấu, JSON vẫn qua `json.loads`, `opt-1`/`severity` nguyên; `calls.jsonl` có `outputLanguage`; marker `[revised…]` còn nguyên. Mở rộng `fake_claude_cli/request_service_contract.overlay.go.txt` (chạy bằng `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh`, dựng prompt bằng **bộ dựng thật** rồi đưa đầu ra claude giả qua **parser và validator thật**): thêm ca với `Language: Vietnamese` cho Solution, Plan native, chẩn đoán, Findings, Answer, chứng minh chuỗi tiếng Việt của claude giả qua `SolutionOptions.Validate`, `ValidatePlanProposal`, `ValidateByKind`.
3. `suite_output_language.py` (tiền tố `OLG`, `--ai-stub` only, đăng ký sau `analysis_revision` trong `SUITES` và `AI_USING`):
   - `OLG-01` mọi lời gọi `promptKind` `solution`, `plan`, `readonly:*` trong `calls.jsonl` sau khi chạy các ca dưới có `outputLanguage == "vi"`.
   - `OLG-02` Request `change_request` tiếng Anh (fixture LedgerLite, `solutionAgent=false`) → Solution lưu: chuỗi tiếng Việt có dấu trong `summary`; `opt-1`, `A-1`, `severity`, `size` nguyên; run `succeeded`.
   - `OLG-03` Plan native: tiêu đề tiếng Việt, nhãn `test:regression` nguyên, qua validator, duyệt được.
   - `OLG-04` Request `bug` (chẩn đoán) và `question` (Answer) chỉ đọc: tiếng Việt, schema hợp lệ.
   - `OLG-05` Request thân tiếng Việt và thân tiếng Anh cùng nhận chỉ thị `vi` (không phát hiện ngôn ngữ).
   - `OLG-06` chỉ chạy khi `ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en` (stack đã đặt biến, suite không đổi được môi trường dịch vụ): chỉ thị `English`, đầu ra giả tiếng Anh; không thì `SKIP` có lý do.
4. `ui-live/analysis-output-language.web.e2e.ts` (stub, theo mẫu `qa-bug-task-native-flow.web.e2e.ts` và fixture `fixture-repo-ledgerlite.ts`): tạo Request, đợi tab Analysis có Solution, khẳng định chuỗi tiếng Việt có dấu hiển thị nguyên vẹn (`getByText` một chuỗi có `ệ`, `ơ`, `đ`), không bị cắt hay đổi hoa/thường, nhãn mục là nhãn giao diện tiếng Anh; chụp ảnh vào artifact.
5. `ui-live/analysis-output-language-real.web.e2e.ts` + nhóm `"language": ["/analysis-output-language-real"]` trong `run_ui_flow.py` (chuỗi là regex trên đường dẫn tệp, xem chú thích ở `run_ui_flow.py:40`); theo mẫu `readonly-real.web.e2e.ts` (`runRealFlow`, repo tạm, guard): Request `question` bằng tiếng Anh; chờ Answer thật; khẳng định run `succeeded`, tài liệu JSON hợp lệ, khoá tiếng Anh, `answer_markdown` chủ yếu tiếng Việt theo heuristic (tỷ lệ chữ cái có dấu ≥ 15%, ≥ 3 từ trong {và, của, không, là, các, một, được, cho, trong}, từ dừng tiếng Anh < 10%). Ghi `Usage` của run (`RunStats`) vào output để so với một lần chạy `en` (đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en` rồi `redeploy-local.sh request-service`), ghi tỷ lệ token và xem có chạm giới hạn ký tự/bị cắt không vào PR. Kèm đọc tay một đầu ra. Điều kiện chạy như `readonly-real` (`ORCA_E2E_LIVE=1`, `ORCA_E2E_READONLY_ANALYSIS=1`); thiếu thì `skipped`.
6. `README.md` của `orca-flows`: thêm dòng `OLG` vào "Danh mục test", lệnh chạy `run_acceptance.py --ai-stub output_language` (tên suite đăng ký là `output_language`), biến `ORCA_E2E_EXPECT_OUTPUT_LANGUAGE`, và dòng `--real language`.

## Kiểm thử

- `python3 -m unittest tests/stages/e2e/orca-flows/fake_claude_cli/test_fake_claude_cli.py` xanh; `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh` xanh (cần Go); `py_compile` cho Python mới.
- Redeploy `request-service` bản mới (`redeploy-local.sh request-service`; nhớ thứ tự khởi động lại sau redeploy và rằng gateway có thể sập sau redeploy: xem ghi chú stack local), rồi `python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub output_language` xanh (`OLG-06` `SKIP`); sau đó chạy lại `request`, `matrix`, `analysis`, `plan`, `analysis_revision`, `aistub` để chắc claude giả đổi không làm hồi quy (đặc biệt `ARV`, `AST-03` không prompt lạ).
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub` chạy spec `analysis-output-language` và `qa-bug-task-native-flow` xanh.
- Kiểm đột biến: đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en` rồi chạy `ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en … output_language`: `OLG-01/02` đỏ vì không còn `vi`, `OLG-06` xanh; trả lại `vi`.
- `--real language` (lớp chi phí B): chỉ khi được đồng ý chạy tốn AI; thăm dò đăng nhập trước. Nếu không chạy, ghi "chưa kiểm chứng với mô hình thật" vào trạng thái CR-REQ-046 và PR.

## Tiêu chí hoàn thành

- [x] Claude giả ghi `outputLanguage` và sinh tiếng Việt khi chỉ thị là `vi`; test đơn vị và `verify-against-request-service.sh` xanh (đầu ra tiếng Việt qua validator thật); `ARV` và `AST` không hồi quy. (`python3 -m unittest .../test_fake_claude_cli.py`: 57 OK; `verify-against-request-service.sh`: OK, thêm `TestFakeClaude_VietnameseSolution|Plan|ReadonlyAnalysisPassesTheRealValidators`; `run_acceptance.py --ai-stub` toàn bộ: `ARV-01..12` và `AST-01..06` xanh, `ARV-13` BLOCKED như trước)
- [x] `OLG-01..05` xanh với `--ai-stub`; `OLG-06` `SKIP` có lý do hoặc xanh khi stack đặt `en`. (`run_acceptance.py --ai-stub devserver approve_run_all output_language`: OLG-01..05 PASS, OLG-06 SKIP; lặp lại trong lần chạy toàn bộ 151 pass.)
- [x] Spec UI stub xanh: chữ có dấu hiển thị đúng. (`run_ui_flow.py --ai-stub -- analysis-output-language`: passed; ảnh `scratchpad/e2e-screens/stub-vietnamese/07-solution-vietnamese.png`: "Phương án 1: mở rộng mô-đun hiện có", pros/cons có dấu, tab và nút vẫn tiếng Anh)
- [x] Có kết quả `--real language` (token so với `en`, có bị cắt không, mẫu đầu ra đọc tay) hoặc ghi rõ chưa chạy. (Đã chạy một lần, xem bên dưới; **không có lần so với `en`**.)

## Rủi ro và lưu ý

- Chỉ thị không được chứa chuỗi `"kind":"<loại>"` (claude giả dùng nó để nhận loại đọc-chỉ); nếu suite `AST-03` báo prompt lạ sau thay đổi mẫu, kiểm điều này trước.
- Heuristic tiếng Việt của spec thật có thể sai với câu trả lời rất ngắn hoặc nhiều đoạn mã; dùng một câu hỏi buộc trả lời đủ dài và đọc tay một đầu ra.
- Suite không đổi được biến môi trường dịch vụ: ca `en` là thủ công.
- CR-REQ-043 (TASK-REQ-043-06) và CR-REQ-044 (TASK-REQ-044-09) cũng sửa `fake_claude_cli/claude`, `test_fake_claude_cli.py` và `run_acceptance.py`: hợp nhất theo từng hàm riêng (`language_directive`, bảng `VI`), không viết lại tệp.
- Claude giả chỉ áp dụng cho POSIX (macOS, Linux); trên Windows dev server agent giải `claude.cmd` nên suite `OLG` không chạy ở đó.

## Ghi chú triển khai (2026-10-20)

**Tệp:** `fake_claude_cli/{claude,test_fake_claude_cli.py,request_service_contract.overlay.go.txt}` (hàm `language_directive`, bảng `VI`, `tr`; đầu ra UTF-8 thô, không escape `\u`, để agent thấy đúng byte như CLI thật), `suite_output_language.py` (mới, đăng ký `output_language` sau `analysis_revision` và `approve_run_all`), `ui-live/analysis-output-language.web.e2e.ts` (stub), `ui-live/analysis-output-language-real.web.e2e.ts` + `live-vietnamese-text-check.{ts,test.ts}` (heuristic tách riêng, 4 test `node --test` xanh), `run_ui_flow.py` (nhóm `language`), `README.md`. `ARV` giữ nguyên marker `[revised, round N, asked: ...]`. Khác đặc tả: bảng `VI` còn dịch `risks[].description`, `recommendation.reason`, `assumptions[].text`, câu hỏi chặn và chẩn đoán (đều là chữ người đọc).

**Kiểm đột biến `en`: không chạy.** Cần đổi `REQUEST_DEFAULT_OUTPUT_LANGUAGE` của `request-service` đang chạy; thao tác này bị chặn (sửa tài nguyên dùng chung) và không nằm trong phần được uỷ quyền, nên `OLG-06` và nhánh `expect_en` của suite chưa được chứng minh trên stack (chỉ kiểm cú pháp).

**Hạn chế đã biết:** `calls.jsonl` ghi 6000 ký tự đầu của prompt; OLG-05 nhận ra Request qua tiêu đề nằm trong phạm vi đó (xanh).

### Kết quả `--real language` (lớp B, một lượt, 2026-10-20)

`ORCA_E2E_GUARD_IGNORE=... python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real language`: 1 passed, 2,6 phút, `0 orphaned`. Request `question` bằng tiếng Anh ("What does the max_of function ... which edge cases ... which test covers it? Explain it in detail."), repo tạm có hook chặn; claude thật đọc repo (Grep 1, Read 2, 4 lượt), run `succeeded`.
- **JSON hợp lệ, không bị cắt:** đủ khoá `schema_version`, `kind: answer`, `answer_markdown`, `confidence` (khoá tiếng Anh nguyên), qua validator thật; `answer_markdown` 1633 ký tự, kết thúc đúng câu, không có ký tự thay thế U+FFFD.
- **Chỉ thị hoạt động:** 27,7% chữ cái có dấu (ngưỡng 15%), 8 từ dừng tiếng Việt khác nhau (ngưỡng 3), từ dừng tiếng Anh 1,2% (ngưỡng 10%). Định danh, đường dẫn và mã giữ tiếng Anh (`max_of`, `src/calc.py:6-8`, `return a if a > b else b`) đúng quy tắc; đọc tay đầu ra: "## `max_of` làm gì ... trả về số lớn hơn trong hai số ... ## Các edge case ..." (cụm "edge case" giữ tiếng Anh).
- **Mức dùng (`RunStats` của run):** `inputTokens` 4, `outputTokens` 1960, `cacheReadTokens` 21447, `cacheCreationTokens` 123583, `turns` 4, `durationMs` 26607, `costUsd` 1,2956 (phần lớn là ngữ cảnh khởi tạo của claude, không phải tiếng Việt). **Không đo được tỷ lệ so với `en`**: cần thêm một lượt ở stack `en` (khoảng 1,3 USD và đổi biến môi trường dịch vụ). 1960 token cho 1633 ký tự (khoảng 1,2 token/ký tự) chỉ là số tuyệt đối. Không chạm trần ký tự nào.
- Phân loại (`ai.complete`, không mang chỉ thị ngôn ngữ) cũng chạy bằng Claude thật trong lượt này; không ảnh hưởng kết quả.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
