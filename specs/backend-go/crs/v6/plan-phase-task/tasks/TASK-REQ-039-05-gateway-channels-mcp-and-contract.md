# TASK-REQ-039-05: Gateway, MCP, OPA và CONTRACT cho `base_ref`

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_request.go` + `request_rpc_channel_map_test.go` (sửa), `.../mcpserver/tools/` (sửa), `backend-go/policy/` (OPA `agent_rpcs`), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (mục 2.11 mới)
**Depends on:** TASK-REQ-039-02
**Status:** [x] DONE (2026-10-19)

---

## Context

- Mọi RPC của `request-service` tự kiểm quyền; gateway định tuyến và chuẩn hoá tham số.
- Kênh `request.*` dùng tham số `id`.
- Mọi kênh mới cần `ToolSpec` MCP hoặc dòng loại trừ có lý do; `parity_test` bắt thiếu.

## Việc cần làm

1. `request.create` nhận `baseRef` (chuỗi, tuỳ chọn); trả `request.baseRef`, `baseRefLocked`, `baseRefLockReason`, `baseRefUnverified`.
2. `request.setBaseRef {id, baseRef, expectedVersion}` → `{request}`; `GroupWrite`; thêm vào bảng ánh xạ kênh→RPC và test.
3. `request.get`/`list` trả ba trường mới (luôn có mặt, kể cả `baseRef: ""`, không `omitempty`).
3b. `request.flowStatus` trả thêm `features: ["request.baseRef"]` (gateway công bố; request-service không cần biết); test kiểm có mặt. Frontend dựa vào cờ này để ẩn ô (FE-REQ-TASK-040-01).
4. MCP: `request_create` nhận `baseRef`; tool `request_setBaseRef` (hoặc dòng loại trừ có lý do); mô tả tool nêu quy tắc khoá. Giới hạn tạo theo giờ (`RequestCreatePerHour`) không đổi.
5. OPA: nếu agent được gọi `setBaseRef` thì thêm vào `agent_rpcs` và test chính sách; nếu không thì ghi rõ trong CONTRACT.
6. CONTRACT mục 2.11: kênh, tham số, ba mã lỗi, trường trả về, hành vi fail-open của kiểm ref.

## Kiểm thử

- `wscompat`: ca `request.create` có/không `baseRef`, `request.setBaseRef` thành công, khoá, xung đột phiên bản, quyền, tham số sai.
- `parity_test` MCP xanh; test chính sách OPA.
- `go test ./services/api-gateway/internal/adapter/... -run 'Request|Parity|BaseRef' -v`.

## Tiêu chí hoàn thành

- [x] Kênh và tool mới có test; `parity_test` xanh.
- [x] CONTRACT cập nhật và khớp mã.
- [x] Frontend cũ (không gửi `baseRef`) chạy như trước.

## Rủi ro và lưu ý

- Gateway cũ + frontend mới: frontend phải ẩn ô khi thiếu capability (FE-REQ-TASK-040-01).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

### Tệp đã đổi (tất cả dưới `backend-go/services/api-gateway/` trừ khi ghi khác)

- `internal/adapter/wscompat/channels_request_base_ref.go` (mới): kênh `request.setBaseRef`, `withBaseRefState`, cờ `features`, và `baseRefErrorSuffix` (đuôi JSON của lỗi).
- `channels_request.go` (`create` nhận `baseRef`; `get` gắn trạng thái khoá; đăng ký kênh), `channels_request_views.go` (`RequestView.BaseRef` không `omitempty`, ba con trỏ tuỳ chọn; `RequestViews` cho `list`), `channels_request_flow.go` (`flowStatus` trả `features`), `channels_request_errors.go` (một dòng nối đuôi).
- Test: `channels_request_base_ref_test.go` (mới), `channels_request_test.go` (một ca bảng + danh sách kênh), `request_rpc_channel_map_test.go` (`SetRequestBaseRef` -> `request.setBaseRef`), `testdata/channel_inventory.json` (sinh bằng `UPDATE_GOLDEN=1`).
- MCP: `mcpserver/tools/pack_request_flow.go` (`request_create` thêm `base_ref`, mô tả nêu quy tắc khoá), `excluded_channels.yaml` (dòng loại trừ `request.setBaseRef`, `human_gate`), `testdata/tools_list.golden.json` (sửa tay đúng một dòng `props`; `-update` sẽ định dạng lại cả file nên không dùng), `pack_request_flow_test.go` (`request_setBaseRef` là tool không tồn tại), `request_flow_executor_test.go` (`base_ref` đi tới service, rỗng giữ rỗng, 256 byte bị chặn trước service).
- `backend-go/policy/orca-authz/request_test.rego`: thêm `SetRequestBaseRef` vào ca "agent bị từ chối kể cả admin". `request.rego` **không đổi**.
- `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`: mục 2.11 mới, cộng vào `RequestView`, bảng 2.1 (`flowStatus`, `create`, dòng `setBaseRef`) và bảng mã lỗi mục 5.
- `request.proto` **không đổi**: `flowStatus` do gateway lắp (`GetRequestFlowSettings` chỉ trả `enabled`), nên `features` thêm ở gateway, không cần protoc.

### Quyết định

1. **Không có tool `request_setBaseRef`; dùng dòng loại trừ.** Đổi nền worktree quyết định agent sẽ build trên cái gì; RPC đã `AgentAllowed=false` và `agent_rpcs` không liệt kê nó, nên một tool sẽ luôn bị từ chối ở service. Dòng loại trừ (có lý do, `human_gate`) là mẫu sẵn có của `request.confirmType`, `request.cancel`. Agent vẫn đặt được `base_ref` một lần ở `request_create` (CR cho phép). `TestToolParity`, `TestChannelInventory` xanh.
2. **Trường khoá chỉ ở nơi service tính ra.** Ghi chú task 02: `base_ref_locked` chỉ tin cậy ở `GetRequest`, `ListRequests`, `SetRequestBaseRef`. Vì vậy `baseRefLocked`/`baseRefLockReason` là con trỏ: có ở `get`, `list`, `create`, `setBaseRef`, **vắng** ở `changeType`, `cancel`, `classify`... (một `false` ở đó có thể sai). `baseRef` thì có ở mọi `RequestView`. Task nói "luôn có ở get/list": đúng.
3. **`baseRefUnverified` nằm trong `request`** (cùng ba trường kia) cho cả `create` và `setBaseRef`, để kết quả `setBaseRef` vẫn là `{request}` như task ghi.
4. **`setBaseRef` bắt buộc khoá `baseRef`** (`*string`): thiếu khoá hay `null` là `INVALID_ARGUMENT`, không gọi service; `""` hợp lệ (về mặc định). Lý do: gõ nhầm `base_ref` không được lặng lẽ đặt lại ref.
5. **`details` của lỗi**: `apperrors` chỉ mang `CODE: message`, `request-service` thêm `ErrorInfo` ở status. Gateway đọc nó và nối đuôi ` {json}` (cùng dạng `retryAfterMs`): `{"reason":...}` cho LOCKED, `{"suggestions":[...]}` (tối đa 5) cho NOT_FOUND. Chỉ nối khi `ErrorInfo.Reason` khớp mã trong thông báo. Không lặp lại `ref`.
6. "`GroupWrite`" trong task: gateway không có khái niệm nhóm; nhóm thật là `GroupTriage` + `RateWrite` ở `request-service` (rpc_catalog). Kênh dùng deadline 8s như các kênh ghi khác.

### Blast radius (không có GitNexus; grep)

- `RequestViewOf` (10 chỗ trong `channels_request.go`) và `RequestViews`: mọi kênh `request.*` trả `RequestView` thêm khoá `baseRef` (cộng, chuỗi rỗng khi không có). Không có nơi khác trong api-gateway gọi chúng (HTTP route đi qua cùng registry nên nhận cùng dạng).
- `requestErrorWith`: dùng cho mọi kênh `request.*`, `solution.*`, `approval.*`, `clarification.*`; đuôi chỉ thêm khi status có `ErrorInfo` của hai mã `REQUEST_BASE_REF_*`, nên các lỗi khác giữ nguyên byte (các test lỗi cũ xanh).
- `request.flowStatus`: thêm khoá `features`; không ai trong repo ràng buộc dạng cũ (toàn bộ test api-gateway xanh).

### Lệnh đã chạy (overlay của quy tắc chung cho mọi lệnh)

- `go build -overlay $OV ./...`, `go vet -overlay $OV ./...`: xanh.
- `go test -overlay $OV ./... -count=1` toàn `services/api-gateway`: xanh tất cả gói (không có lỗi có sẵn).
- `go test -overlay $OV -race ./internal/adapter/wscompat/ ./internal/adapter/mcpserver/tools/ -run 'Request|Parity|BaseRef|Inventory|Golden' -count=1 -v`: PASS, gồm `TestRequestCreate_BaseRefIsPassedAndReturned`, `TestRequestCreate_WithoutBaseRefKeepsOldClientsWorking`, `TestRequestGetAndList_AlwaysCarryBaseRefFields`, `TestRequestViews_OtherChannelsOmitLockState`, `TestRequestSetBaseRef_{Success,EmptyRefResetsToDefault,BadParameters,ServiceErrors}`, `TestRequestCreate_BaseRefErrorsKeepTheirCodes`, `TestRequestFlowStatus_PublishesFeatures`, `TestRequestRPCChannelParity`, `TestChannelInventoryMatchesGolden`, `TestToolParity`, `TestChannelInventory`, `TestToolsListGolden`, `TestRequestCreate_BaseRefIsPassedOnAndBounded`.
- Chính sách: không có `opa` trong môi trường. Dùng test Go tạm trong `request-service/internal/adapter/opaclient` (nạp `policy/orca-authz` thật, đã xoá sau khi chạy): agent bị từ chối `triage/SetRequestBaseRef` (cả chủ dự án lẫn admin), người dùng chủ dự án và người báo được, thành viên thường bị từ chối. `go test ./internal/adapter/opaclient/ -run Matrix` xanh. Ca mới trong `request_test.rego` chưa chạy bằng `opa test` (CI chạy bước đó).

### Chưa kiểm chứng / ngoài phạm vi

- Không có route HTTP `POST /v1/requests/{id}/base-ref` (`httpgateway/request_routes_flow.go` không thuộc tệp được giao); CLI chưa đặt được ref. Thêm một dòng vào `requestFlowRoutes` nếu cần.
- `frontend/src/shared/request-rpc-methods.ts`, `request-types.ts`, bộ phân tích wire chưa biết kênh và trường mới (FE-REQ-TASK-040).
- `request-service/.../opaclient/request_policy_test.go` chưa có hàng cho `SetRequestBaseRef` (không thuộc tệp được giao).
- Gợi ý (`suggestions`) request-service nối bằng dấu phẩy; tên ref có dấu phẩy (git cho phép) sẽ bị tách sai ở đuôi gợi ý. Chỉ ảnh hưởng gợi ý, không ảnh hưởng ref được lưu.
- Lý do khoá `execution` do agent khác thêm; gateway chuyển chuỗi nguyên văn, CONTRACT nói frontend phải chịu lý do lạ.
- Chưa chạy qua gateway thật + request-service thật (task 06); `golangci-lint` không chạy được trong môi trường.
