# TASK-REQ-042-08: Kiểm soát CI, tài liệu và hồi quy cuối

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P2
**Service:** `backend-go/ci`
**File:** `backend-go/ci/{check-prompt-golden.sh,check-agent-relay-single-source.sh}` (mới/sửa), `.github/workflows/` (sửa), `backend-go/prompts/README.md`, `specs/backend-go/crs/v6/ai-governance/` (cập nhật trạng thái), `deploy/agent/README.md`
**Depends on:** TASK-REQ-042-02, 03, 04, 06, 07
**Status:** [x] DONE (2026-10-20)

---

## Context

- Sau khi gom, cần ngăn quay lại bản sao: một kiểm tra tìm chuỗi `"agent.execPrompt"`/`"ai.complete"` dùng làm method của `Relay`/`RelayByDevServer` ngoài `common/agentrelay`.
- Cần ngăn prompt mới nằm ngoài catalog.

## Việc cần làm

1. `check-agent-relay-single-source.sh`: thất bại nếu có `Method: "agent.execPrompt"` hoặc `"ai.complete"` ngoài `common/agentrelay` (cho phép trong test và `infra-fleet-service`).
2. Kiểm tra "prompt mới phải vào catalog": thất bại nếu có `strings.Builder` dựng chuỗi chứa mẫu `<feedback>`/`<request>` ngoài `backend-go/prompts` (heuristic, ghi rõ giới hạn).
3. Kiểm tra kích thước prompt: test đo prompt lớn nhất của mỗi luồng với fixture lớn và khẳng định dưới ngưỡng (ghi ngưỡng trong README).
4. Workflow CI chạy các kiểm tra và golden.
5. `backend-go/prompts/README.md`: cách thêm luồng/mẫu, cách sinh golden, quy tắc `fence`/`redact`, ID gói.
6. Hồi quy cuối: acceptance toàn bộ (`run_acceptance.py` bộ không dùng AI thật), UI stub, `--real native-execute`; so số liệu trước/sau.
7. Cập nhật trạng thái các CR và README thư mục.

## Kiểm thử

- Hai script chạy ở CI và thất bại đúng khi cố ý vi phạm (thêm một lời gọi `Method: "agent.execPrompt"` giả ở nhánh thử).
- Bộ hồi quy cuối xanh.

## Tiêu chí hoàn thành

- [x] CI chặn quay lại bản sao relay và prompt ngoài catalog. (hai script + golden + kiểm kích thước chạy trong job `prompt-catalog` qua `make check-prompts`; tự kiểm chứng bằng `check-prompt-guards_test.sh`)
- [x] README catalog và `deploy/agent/README.md` cập nhật.
- [x] Hồi quy cuối xanh. Acceptance `--ai-stub platform devserver request matrix exits ai plan analysis task notfound aistub request_base_ref analysis_revision` (2026-10-20, sau khi triển khai lại request/task/git-gateway/orchestration): 121 đạt, 0 lỗi, 3 chặn (AIS-xx cần AI thật, MCP-01/02 có sẵn), 2 bỏ qua; chạy lại tập nhỏ hơn sau lần triển khai cuối: 102 đạt, 0 lỗi. UI `qa-bug-task-native-flow` 3/3; `--real native-execute` (Claude thật, repo tạm) 2/2 đạt trong 11,8 phút. Số liệu "trước" không còn bản ghi cùng tập suite; các lần chạy wave 2 (trước khi chuyển prompt) cũng 0 lỗi.

## Rủi ro và lưu ý

- Heuristic "prompt ngoài catalog" có thể báo nhầm; cho phép danh sách loại trừ có lý do.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Ngày:** 2026-10-20. Trạng thái: PARTIAL. Các việc 1 đến 5 và 7 xong; việc 6 (hồi quy cuối) do người điều phối chạy, **chưa chạy ở đây**. GitNexus không dùng được nên không chạy `impact`/`detect_changes`; không sửa symbol Go sẵn có (chỉ thêm script, một test, tài liệu, Makefile, workflow).

### File

- `backend-go/ci/check-agent-relay-single-source.sh` (mới). Quét mọi `.go` không phải test, trừ `proto/`, `common/agentrelay/`, `services/infra-fleet-service/`. Luật 1: `Method: "agent.execPrompt"|"ai.complete"` (hoặc `agentrelay.MethodExecPrompt|MethodComplete` làm `Method:`) là lỗi, không có allowlist. Luật 2: literal có nháy của hai tên ngoài allowlist là lỗi; dòng chú thích bị bỏ qua. Allowlist còn đúng 4 file, mỗi file một lý do: `solution_artifact_recorder.go` (nhãn provenance), `agent_route_selection.go` (tên feature agent công bố), `contracttest/solution_artifact_contract.go` (assert provenance), `e2e/stubs/agent_script.go` (agent giả nhận method). Mục allowlist không còn khớp gì cũng làm script thất bại, nên danh sách chỉ co lại. Trên cây hiện tại ngoài `agentrelay` và `infra-fleet-service` không còn lời gọi Relay nào dùng hai tên này.
- `backend-go/ci/check-prompt-outside-catalog.sh` (mới, script anh em). Thất bại khi `.go` không-test ngoài `backend-go/prompts` có một trong `<feedback> <request> <prior_artifacts> <clarifications> <project> <approved_decision>` (mở hoặc đóng) trên dòng không phải chú thích. Loại trừ có lý do: `request-service/internal/adapter/contracttest/` (khung hợp đồng chỉ assert vị trí văn bản trong prompt đã dựng). Giới hạn ghi ở đầu script và `prompts/README.md`: không thấy prompt không dùng các thẻ này, thẻ ghép lúc chạy, hay prompt không phải `.go`; có thể báo nhầm chuỗi định dạng như `req:<request>:<task>`.
- `backend-go/ci/check-prompt-guards_test.sh` (mới): trên cây thật cả hai script pass; trên bản sao tạm có 14 ca, 10 ca buộc phải thất bại đúng thông báo (Method literal, `ai.complete`, `agentrelay.MethodExecPrompt`, raw string, Method trong file allowlist, mục allowlist cũ, `<feedback>`, `<request>` trong raw string, `<prior_artifacts>`/`<clarifications>`, mục loại trừ cũ) và 4 ca buộc phải pass (chỉ file allowlist; test, `infra-fleet-service`, `common/agentrelay`, chú thích, `prompts/`). Chạy được với `/bin/bash` 3.2.57 của macOS; không dùng cờ GNU-only.
- `backend-go/prompts/prompt_size_budget_test.go` (mới, package `prompts_test`): dựng fixture lớn cho cả 19 prompt ID và khẳng định kích thước dưới ngân sách `maxPromptBytes`; thiếu ngân sách hoặc thiếu ca thì test đỏ. Chạy 2,6 giây với `-race`.
- `backend-go/Makefile`: thêm `check-agent-relay-single-source`, `check-prompt-outside-catalog`, `check-prompt-guards-selftest`, `check-prompts` (gộp ba cái cộng `check-prompt-golden`).
- `.github/workflows/backend-go-request-contracts.yml`: job `prompt-catalog` (đã có từ 042-05/06) nay chạy `make check-prompts` thay cho bước golden riêng. Path filter đã có `backend-go/prompts/**`, `backend-go/common/**` (gồm `agentrelay`), `backend-go/ci/**`, `backend-go/services/**`; không cần thêm.
- `backend-go/prompts/README.md`: thêm bảng ID gói, hàm `fence`/`redact` và thứ tự cắt, redact, fence, cách sinh golden và khi nào được đổi, bảng ngân sách kích thước, bảng kiểm tra CI kèm giới hạn.
- `deploy/agent/README.md`: thêm con trỏ tới catalog và mục "Đã kiểm chứng ở đâu" (macOS có; Linux và Windows chưa). Phần stdin, `ORCA_AGENT_PROMPT_VIA_ARGV=1` đã có từ 042-04.

### Số đo kích thước (byte, fixture lớn, 2026-10-20)

classify 29.843 (trần 34.000); analysis solution 174.137, agent_solution 175.563, diagnosis 173.840, findings 173.635, answer 173.498 (trần 200.000 mỗi cái); clarification_answers 81.749 (95.000); plan_native 62.971 (72.000); plan_openspec 66.050 (75.000); task_prompt_template 26.462 (30.000); solution_openspec 15.040 (17.000); task_execution 434.634 (490.000); result_block_suffix 1.235 (2.000); task_decompose 212.110 (240.000); task_agent_prompt 145.860 (165.000); git commit message 497.209 (560.000); stats_only 334.067 (380.000); git pull request 484.620 (545.000); git agent prompt 208.573 (235.000). Hệ số dư 10 đến 20 phần trăm.

### Phát hiện khi đo (chưa sửa, ngoài phạm vi task)

- Số câu trả lời làm rõ (`clarification_answers`), số bình luận issue (`git_agent_prompt`), số tiêu chí của `task_prompt_template`, và văn bản Solution đã duyệt đưa vào `plan_*` không bị giới hạn bởi mẫu; ngân sách của chúng là kích thước của ca xấu nhất thực tế, không phải bảo đảm. Nếu muốn bảo đảm, thêm trần theo số lượng ở mẫu và cập nhật golden theo quy trình có duyệt.
- Task, mô tả task và diff được gửi nguyên văn (không cắt), đúng hành vi trước khi gom.

### Đã chạy

- `bash backend-go/ci/check-agent-relay-single-source.sh` -> exit 0.
- `bash backend-go/ci/check-prompt-outside-catalog.sh` -> exit 0.
- `bash backend-go/ci/check-prompt-guards_test.sh` (bash 5 và `/bin/bash` 3.2) -> 16 dòng ok, exit 0.
- `cd backend-go && make check-prompts` -> exit 0 (cả 6 gói golden ok).
- `cd backend-go/prompts && go vet ./... && gofmt -l . && go test ./... -race -count=1` -> xanh, 18 gói.
- `bash backend-go/ci/check-prompt-golden.sh` -> 6/6 ok.
- `ruby -ryaml` parse `backend-go-request-contracts.yml` -> ok.

### Chưa làm

- Hồi quy cuối (acceptance toàn bộ không dùng AI thật, UI stub, `--real native-execute`) và số liệu trước/sau: người điều phối chạy.
- README thư mục `tasks/` không liệt kê trạng thái từng task nên không sửa.
- Chưa chạy workflow trên GitHub (chỉ parse YAML và chạy cùng lệnh cục bộ).
