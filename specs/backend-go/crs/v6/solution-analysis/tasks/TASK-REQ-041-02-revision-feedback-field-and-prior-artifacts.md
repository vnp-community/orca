# TASK-REQ-041-02: `revision_feedback` trên Solution và phản hồi cũ trong prompt

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service`
**File:** `proto/orca/request/v1/solution.proto` (sửa) + mã sinh, `internal/domain/solution.go`, `internal/usecase/{list_solutions,run_solution_generation,solution_prompt}.go` (sửa), `internal/adapter/{postgres,mysql}/solution_repository.go` (sửa), `internal/adapter/grpc/{server_solution,solution_mapper}.go` (sửa)
**Depends on:** TASK-REQ-041-01
**Status:** [ ] TODO

---

## Context

- `Solution` proto dừng ở trường 17; `solutions.generation_run_id` trỏ tới `analysis_runs` có cột `feedback`.
- `priorArtifacts` không điền `PriorArtifact.Feedback`; `BuildSolutionPrompt` đã in `reviewer comment` khi có.
- Giao diện cần hiển thị ghi chú và nội dung yêu cầu chỉnh của từng phiên bản.

## Việc cần làm

1. Proto: `Solution.revision_feedback = 18`; sinh lại mã bằng quy trình của repo.
2. Repository: phương thức `RunFeedbackByIDs(ctx, runIDs []string) (map[string]string, error)` cho cả hai dialect (một truy vấn `IN`, giới hạn kích thước lô 500).
3. `ListSolutions` điền `RevisionFeedback` theo lô; `GetSolution` (nếu có) điền cho một bản.
4. `priorArtifacts`: nạp phản hồi của run từng bản, điền `PriorArtifact.Feedback` (đã bọc/cắt ở `BuildSolutionPrompt`).
5. **Giới hạn tổng** cho khối `prior_artifacts` (40000 rune): giữ các bản mới nhất, bỏ bản cũ nhất trước, thêm dòng "N earlier versions omitted". Hằng số có tên, không số ma thuật.
6. Mapper proto và JSON gateway: `revisionFeedback`.

## Kiểm thử

- Hợp đồng hai dialect: `RunFeedbackByIDs` (rỗng, nhiều, trùng, vượt lô).
- `TestListSolutions_RevisionFeedback_BatchedNoNPlusOne` (đếm truy vấn).
- Golden prompt: 3 vòng chỉnh, phản hồi cũ và mới đều có và đều nằm trong khối bọc; ca vượt 40000 rune cắt bản cũ nhất.
- Test log/span không chứa nội dung phản hồi.
- `go test ./services/request-service/... -run 'RevisionFeedback|PriorArtifacts|Prompt' -v`.

## Tiêu chí hoàn thành

- [ ] `solution.list` trả `revisionFeedback` đúng cho từng phiên bản.
- [ ] Prompt của lượt chỉnh chứa chuỗi phản hồi, trong khối bọc, trong giới hạn kích thước.
- [ ] Không N+1; không lộ nội dung vào log.

## Rủi ro và lưu ý

- Tăng trưởng prompt theo số vòng (không giới hạn vòng): giới hạn tổng ở bước 5 là bắt buộc.
- Phản hồi cũ có thể chứa nội dung nhạy cảm: giữ nguyên quy tắc `fenceUntrusted` và redaction hiện có.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
