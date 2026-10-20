# TASK-REQ-045-08: Gateway, REST, CLI, loại trừ MCP và CONTRACT cho xem tài liệu và chỉnh Plan

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `api-gateway`, `cmd/orca-cli`, `request-service`
**File:** `services/api-gateway/internal/adapter/wscompat/{channels_request_plan.go,testdata/channel_inventory.json}` (sửa), `services/api-gateway/internal/adapter/httpgateway/request_routes_flow.go` (sửa), `services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml` (sửa), `cmd/orca-cli/internal/{apiclient/request.go,command/plan_cmd.go}` (sửa), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (sửa), test của từng phần, bảng ánh xạ mã lỗi gateway nếu có
**Depends on:** TASK-REQ-045-05, TASK-REQ-045-07
**Status:** [ ] TODO

---

## Context

- `request.generatePlan` giải mã `{id, mode, feedback, proposal, rawAiResponse}` (`channels_request_plan.go`); `request.planProposal {id, runId}`; `request.planBundleFile {id, path}`. Các kênh `request.planProposal`/`request.planBundleFile` nằm trong loại trừ MCP (`excluded_channels.yaml:645-651`).
- REST: `GET /{id}/plan/bundle` → `request.planProposal`, `GET /{id}/plan/bundle/file` → `request.planBundleFile` (`httpgateway/request_routes_flow.go:41-42`).
- CLI: `orca plan generate --feedback`, `orca plan proposal`, `orca plan bundle --file` (`plan_cmd.go`); `plan generate` làm propose → chờ → commit từ CLI (chuyển sang commit server khi có CR-REQ-044).
- BE-REQ-SOL-044 (task 08 của nó) thêm `request.retryPlan`, `autoPlan` trong `request.planProposal` và cờ `request.planAuto` vào cùng `channels_request_plan.go`, `request_routes_flow.go`, `CONTRACT-request-ui-api.md`, `excluded_channels.yaml`: hợp nhất theo kênh, không ghi đè.
- Quyết định: không thêm công cụ MCP đọc/viết (CR mục 2.10); mọi kênh mới phải có dòng loại trừ có lý do.
- Mã lỗi mới ở task 02 và 05: `REQUEST_PLAN_REVISION_IN_PROGRESS`, `REQUEST_PLAN_REVISION_NOT_APPLICABLE`, `REQUEST_PLAN_REVISION_NO_CHANGE`, `REQUEST_PLAN_REVISION_NOT_FOUND`, `REQUEST_PLAN_REVISION_TEXT_PRUNED`.

## Việc cần làm

1. `request.generatePlan`: nhận thêm `idempotencyKey` (chuỗi) và chuyển vào `GeneratePlanRequest.IdempotencyKey` ở nhánh `propose`; `commit` không đổi.
2. `request.planProposal`: nhận `revision` (số nguyên ≥ 0); chuyển vào `GetPlanProposalRequest.Revision`; trường `bundle.files[]` thêm `sha256`, `change` (qua `protoJSON` đã có nên chỉ cần proto); thiếu trường ở gateway cũ không hỏng giao diện.
3. `request.planBundleFile`: nhận `revision`; trả `{path, content, omitted}` như cũ.
4. `request.planRevisions` (mới): nhóm `read` ở catalog, nhận `{id, pageSize?, pageToken?}` trả `{revisions: [...], nextPageToken}` (`protoJSON` hoặc struct tường minh; tên trường camelCase: `revision`, `runId`, `status`, `engine`, `revisionFeedback`, `actorId`, `generatedAt`, `commit`, `filesChanged`, `fileCount`, `textKept`, `stats`). Đăng ký trong `registerPlan`; cập nhật `channel_inventory.json`; `TestChannelInventory`, `TestRequestRPCChannelParity` xanh.
5. Ánh xạ lỗi: năm mã ở trên đi qua `apperrors` → kind đúng (xem `Err*` ở task 02, 05); kiểm bảng ánh xạ lỗi của `wscompat` để mã tới giao diện nguyên vẹn dạng `CODE: message`.
6. REST: `GET /{id}/plan/bundle/file` đọc `revision` từ query; thêm `GET /{id}/plan/revisions`; `GET /{id}/plan/bundle` đọc `revision` từ query.
7. CLI: `orca plan bundle <id> --revision N` (với `--file` lẫn danh sách), `orca plan revisions <id>` (bảng: số bản, trạng thái, thời điểm, số file đổi, 80 ký tự đầu của phản hồi); `apiclient/request.go` thêm hàm tương ứng.
8. `excluded_channels.yaml`: thêm `request.planRevisions` (category `request_flow`, lý do: "Lists the versions of a Plan with the reviewer's change requests and the authored files' hashes; the text of the documents is agent-written from Request content, so no MCP tool is decided; add a ToolSpec after a design review of untrusted-text handling") và cập nhật lý do hai dòng hiện có để nhắc `revision`.
9. `CONTRACT-request-ui-api.md`: mục mới cho `request.planRevisions`, tham số `revision`, `idempotencyKey`, năm mã lỗi, cờ tính năng (nếu gateway có cơ chế `features` của `request.flowStatus`, thêm `request.planRevisions` như TASK-REQ-039-05 đã làm cho `request.baseRef`; giao diện cần cờ để biết gateway cũ).
10. Nếu `request.flowStatus` có danh sách `features`, thêm `request.planRevisions`; cập nhật test.

## Kiểm thử

- `wscompat`: `request.generatePlan` có `idempotencyKey`; `request.planRevisions` (trang đầu, trang tiếp, rỗng); `request.planProposal` và `request.planBundleFile` có và không `revision`; lỗi tới giao diện đúng mã.
- `go test ./services/api-gateway/internal/adapter/... -run 'Plan|Parity|ChannelInventory|Mcp' -count=1`.
- REST: route mới và tham số `revision`.
- CLI: test lệnh (`go test ./cmd/orca-cli/...`).
- `parity_test` MCP: `request.planRevisions` có dòng loại trừ.

## Tiêu chí hoàn thành

- [ ] Giao diện gọi được `request.planRevisions`, `request.planProposal`/`request.planBundleFile` với `revision`, `request.generatePlan` với `idempotencyKey`.
- [ ] Mọi kênh mới có `ToolSpec` hoặc dòng loại trừ có lý do; `parity_test` xanh; `channel_inventory.json` khớp.
- [ ] REST và CLI có tham số `revision` và lệnh `plan revisions`.
- [ ] CONTRACT khớp mã.

## Rủi ro và lưu ý

- Gateway cũ bỏ qua tham số lạ: giao diện không được tự cho rằng `revision` đã được áp dụng; dựa vào cờ `features` hoặc vào việc `planRevisions` trả `Unimplemented`/`NOT_FOUND` để ẩn tính năng.
- CLI token chỉ sống khoảng 10 phút (ghi chú stack local); không ảnh hưởng thiết kế.
- CONTRACT nằm ngoài phạm vi sửa của người soạn tài liệu này; task này là nơi sửa.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
