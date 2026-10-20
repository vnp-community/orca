# TASK-REQ-043-04: Kênh `approval.approve`, route REST, CLI và cờ tính năng `approval.runAll`

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `api-gateway` · `orca-cli`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_approval.go` (sửa), file giữ `requestFeatures()` (hiện `wscompat/channels_request_base_ref.go`, sửa), `.../httpgateway/request_routes.go` (sửa), `backend-go/cmd/orca-cli/internal/{command/approval_cmd.go,apiclient/request.go}` (sửa) và test tương ứng
**Depends on:** TASK-REQ-043-01, TASK-REQ-043-02
**Status:** [ ] TODO

---

## Context

- `approval.approve` (`channels_approval.go:85-97`) dựng `ApproveRequest{Id, Comment, ExpectedDigest, ExpectedVersion}`; `decodeDecision` bắt buộc `expectedVersion >= 1` và `expectedDigest` (`:139-147`).
- REST `decide(channel)` (`request_routes.go:130-146`) dùng chung cho approve và reject, chỉ truyền `version`, `digest`, `comment`.
- `requestFeatures()` trả danh sách tính năng của `request.flowStatus`; gateway cũ không có tên mới và bỏ qua tham số lạ, nên frontend phải đọc cờ này trước khi mời lựa chọn.
- MCP: `approval.approve` đã nằm trong `mcpserver/tools/excluded_channels.yaml:601` (agent không được duyệt); không thêm ToolSpec.
- CLI: `newApprovalDecideCmd` (`approval_cmd.go:75-110`) dùng chung approve và reject.
- CONTRACT (`specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` dòng 198, 408, bảng lỗi) do người điều phối cập nhật; task này **không** sửa CONTRACT.

## Việc cần làm

1. `approvalDecisionArgs.ExecutionMode string `json:"executionMode"``; `approval.approve` truyền vào `ApproveRequest.ExecutionMode` nguyên văn (không kiểm giá trị ở gateway); `approval.reject` trả `REQUEST_EXECUTION_MODE_INVALID: executionMode applies to approve only` (qua `requestInputError`, như `request.engineSettingsSet` làm với `REQUEST_FORBIDDEN`) khi khoá có mặt và khác rỗng, không gọi service.
2. `approvalDecisionResult.ExecutionMode string `json:"executionMode,omitempty"`` lấy từ `ApproveResponse.GetExecutionMode()`; `reject` không có.
3. `requestFeatureApproveRunAll = "approval.runAll"` thêm vào `requestFeatures()` cạnh `request.baseRef` (không bỏ tên cũ).
4. REST `decide`: body thêm `executionMode`; đặt `"executionMode"` vào args của cả hai route khi giá trị khác rỗng (kênh `reject` sẽ từ chối).
5. CLI: cờ `--execution-mode` trên `orca approval approve` (chỉ chấp nhận `continuous`; mặc định rỗng); `orca approval reject --execution-mode` bị từ chối bằng lỗi dùng sai; `apiclient.DecideApproval` nhận `body` đã có chỗ cho trường, `Document{"executionMode": v}` chỉ khi khác rỗng; in ra chế độ trả về nếu có. Cập nhật mô tả `README` của `cmd/orca-cli` nếu liệt kê cờ.
6. Kiểm `mcpserver` `parity_test` và `excluded_channels.yaml` vẫn khớp (không thêm kênh).

## Kiểm thử

- `wscompat`: `approval.approve` chuyển `executionMode` xuống client giả (kiểm đúng giá trị nhận), kết quả có `executionMode`; không kèm thì không có khoá; `approval.reject` kèm `executionMode` bị từ chối và client giả không nhận lời gọi nào; `request.flowStatus` trả `features` gồm cả `request.baseRef` và `approval.runAll`; `request_rpc_channel_map_test.go` không đỏ.
- `httpgateway`: body có/không `executionMode`; route `reject` kèm `executionMode` trả lỗi tham số.
- CLI: cờ hợp lệ, cờ sai, `reject` + cờ; kiểm in kết quả.
- `go test ./services/api-gateway/... ./cmd/orca-cli/...` (đúng gói đã đổi) và `parity_test` MCP.

## Tiêu chí hoàn thành

- [ ] Duyệt qua WS, REST và CLI đều mang được `executionMode`; kết quả trả chế độ đang ghim.
- [ ] `request.flowStatus.features` có `approval.runAll`; tên cũ còn nguyên.
- [ ] Không thêm ToolSpec MCP; `parity_test` xanh.
- [ ] Ghi lại danh sách thay đổi CONTRACT cần làm (mục 8 của CR) trong mô tả PR cho người điều phối.

## Rủi ro và lưu ý

- Gateway không kiểm giá trị nên mọi chuỗi đi xuống request-service; lỗi `REQUEST_EXECUTION_MODE_INVALID` của service là nguồn sự thật.
- Chạy `gitnexus_impact` cho `approvalDecisionResult`, `requestFeatures`, `requestRoutes.decide`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
