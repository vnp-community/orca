# TASK-REQ-044-08: Gateway (`request.retryPlan`, `autoPlan`, `planAutoGenerate`, `request.planAuto`), loại trừ MCP, authz, cờ CLI và CONTRACT

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `api-gateway`, `request-service`, `orca-cli`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_plan_auto.go` (mới) và test, `channels_request_plan.go` (sửa), `channels_request_engine.go` (sửa), `channels_request_base_ref.go` (sửa `requestFeatures()`), `request_rpc_channel_map_test.go` (sửa), `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml` (sửa), `backend-go/policy/orca-authz/request_test.rego` (sửa), `backend-go/cmd/orca-cli/internal/command/engine_cmd.go` và `engine_cmd_test.go` (sửa), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (sửa), `docs/guides/request/runbook-request-flow.md` (sửa)
**Depends on:** TASK-REQ-044-04, TASK-REQ-044-07
**Status:** [ ] TODO

---

## Context

- Mẫu: `channels_request_base_ref.go` (`registerBaseRef`, `requestFeatures()` ở `:16-19`), `channels_request_plan.go` (`planProposal`, `addPlanEngineView`), `channels_request_engine.go` (`engineSettingsView`, `request.engineSettingsSet`).
- **Cùng điểm sửa với CR-REQ-043**: `requestFeatures()` (CR-REQ-043 thêm tên tính năng của nó vào cùng hàm). Khi triển khai, cộng dồn tên, không ghi đè; xung đột nhỏ có thể xảy ra ở file này và ở `CONTRACT-request-ui-api.md`.
- MCP: mọi kênh đã đăng ký cần `ToolSpec` hoặc dòng loại trừ có lý do, nếu không `parity_test.go` đỏ (`excluded_channels.yaml`; mẫu `request.setBaseRef`, `request.planProposal`).
- `orca engine set` (`cmd/orca-cli/internal/command/engine_cmd.go:51-81`) chỉ gửi mục được nêu và gửi lại `version`; cờ hiện có `--engine`, `--solution-agent`, `--min-version`.

## Việc cần làm

1. `request.retryPlan {id}` (kênh mới, `requestChannelOpts{timeout: requestRPCTimeout}`, `GroupWrite`): gọi `RetryPlanGeneration`, trả `{autoPlan}` (camelCase). Đăng ký ở `registerPlan`. Thêm `"RetryPlanGeneration": viaChannel("request.retryPlan")` vào `request_rpc_channel_map_test.go`.
2. `request.planProposal`: `planProposalView` thêm khoá `autoPlan` (luôn có khi backend trả khối; `{enabled, state, origin, attempt, maxAttempts, reasonCode, nextAttemptAt}`; không đặt khoá khi backend cũ không trả). Khi view không có run (chỉ job), vẫn trả `runId:""`, `proposal:null` như hiện nay.
3. `request.engineSettings` và `request.engineSettingsSet` thêm `planAutoGenerate` (`*bool` ở đầu vào: thiếu khoá = giữ nguyên; kiểu sai → `invalidRequestArg`); `engineSettingsView` điền.
4. `requestFeatures()` thêm `request.planAuto`; `request.flowStatus` đã trả mảng này.
5. MCP: dòng loại trừ cho `request.retryPlan` (category `ui-only` hoặc `request_flow` theo mẫu gần nhất; lý do: "Khôi phục từ giao diện sau một lần sinh Plan thất bại; agent bắt đầu Plan qua `request_generatePlan` khi công cụ đó hết trạng thái Declared"). `parity_test` xanh.
6. `request.rego` test: thêm ca khẳng định agent không gọi được `RetryPlanGeneration` và người báo cáo (nhóm `plan`) gọi được.
7. CLI: `orca engine set --plan-auto-generate=<true|false>` (chỉ gửi khi cờ được nêu; hiển thị trong `orca engine show`); cập nhật test lệnh.
8. CONTRACT: mục mới (số kế tiếp sau 2.12 hoặc mục cuối của CR đang thêm): `request.retryPlan`, khối `autoPlan` và bảng `state`, `planAutoGenerate`, tính năng `request.planAuto`, mã `REQUEST_PLAN_AUTO_DISABLED` và `reasonCode` `REQUEST_PLAN_COMMIT_FAILED`; đánh dấu "(mới, CR-REQ-044)". Runbook `runbook-request-flow.md`: nêu hai công tắc (`REQUEST_PLAN_AUTO_GENERATE`, `plan_auto_generate`), các biến chỉnh nhịp, **chi phí OpenSpec**, và cách quay lui (`REQUEST_PLAN_AUTO_GENERATE=false`).

## Kiểm thử

- `wscompat` với fake client (mẫu `channels_request_fake_test.go`): `request.retryPlan` thành công và lỗi (`REQUEST_PLAN_AUTO_DISABLED`, `REQUEST_STATE_STALE`); `autoPlan` đủ trường và camelCase; `planAutoGenerate` thiếu khoá giữ nguyên, `false` được gửi xuống, kiểu sai bị từ chối; `request.flowStatus.features` có cả `request.baseRef` và `request.planAuto`.
- `parity_test.go` MCP, `request_rpc_channel_map_test.go`, kiểm tra OPA (`opa test`) xanh.
- CLI: `engine set --plan-auto-generate=false` gửi đúng; không nêu cờ thì không gửi khoá.
- `cd backend-go && go test ./services/api-gateway/internal/adapter/wscompat/... ./services/api-gateway/internal/adapter/mcpserver/... ./cmd/orca-cli/... -v` và `make opa-test` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] `request.retryPlan` và `autoPlan` hoạt động và có test; gateway cũ không có `request.planAuto` nên frontend giữ hành vi cũ.
- [ ] Client cũ không đổi công tắc dự án khi không gửi `planAutoGenerate`.
- [ ] `parity_test` xanh, `request.rego` chặn agent.
- [ ] CONTRACT và runbook nêu hai công tắc, chi phí và cách quay lui.

## Rủi ro và lưu ý

- Token CLI hiện chưa mang claim `role` nên `orca engine set` có thể bị từ chối (README của `orca-cli`); công tắc dự án vẫn đặt được qua phiên web admin hoặc kênh `request.engineSettingsSet`.
- Hai CR cùng sửa `requestFeatures()`: rebase cẩn thận, test `features` phải liệt kê đủ.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
