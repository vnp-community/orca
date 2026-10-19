# TASK-REQ-039-05: Gateway, MCP, OPA và CONTRACT cho `base_ref`

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_request.go` + `request_rpc_channel_map_test.go` (sửa), `.../mcpserver/tools/` (sửa), `backend-go/policy/` (OPA `agent_rpcs`), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (mục 2.11 mới)
**Depends on:** TASK-REQ-039-02
**Status:** [ ] TODO

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

- [ ] Kênh và tool mới có test; `parity_test` xanh.
- [ ] CONTRACT cập nhật và khớp mã.
- [ ] Frontend cũ (không gửi `baseRef`) chạy như trước.

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
