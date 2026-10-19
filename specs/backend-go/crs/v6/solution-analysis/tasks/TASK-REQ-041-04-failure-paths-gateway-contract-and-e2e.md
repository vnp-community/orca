# TASK-REQ-041-04: Đường lỗi, đồng thời, knowledge hub, gateway, CONTRACT và e2e

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service, api-gateway, tests/e2e`
**File:** `internal/usecase/{generate_solution,run_agent_readonly_analysis,openspec_solution_prompt}.go` (kiểm tra, sửa nếu cần), `internal/adapter/contracttest/solution_contract.go` (sửa), `services/api-gateway/internal/adapter/wscompat/`, `mcpserver/tools/` (sửa), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`, `tests/stages/e2e/orca-flows/suite_analysis_revision.py` (mới)
**Depends on:** TASK-REQ-041-01, 041-02, 041-03
**Status:** [ ] TODO

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

- [ ] Mọi đường lỗi để Request ở trạng thái thao tác được (không kẹt).
- [ ] Phản hồi tới được agent ở mọi đường sinh phân tích đang dùng; kết luận R4 ghi trong PR.
- [ ] Suite `ARV-xx` xanh với repo fixture thật.
- [ ] CONTRACT khớp mã; `parity_test` xanh.

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
