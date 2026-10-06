# TASK-REQ-017-04: Kết quả không tin cậy, che dữ liệu, ca `Declared` và golden `tools/list`

**From Solution:** BE-REQ-SOL-017
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/redaction_test.go`, `.../catalog_test.go`, `.../testdata/tools_list.golden.json`, `.../executor_guards_test.go`
**Depends on:** TASK-REQ-017-01, TASK-REQ-017-02
**Status:** `[ ] TODO`

---

## Context

- `ToolSpec.Untrusted` (`spec.go:68`) kích hoạt `Guards.WrapUntrusted` (`executor.go:58-60`) và `UntrustedOutput` trong `ToolMeta` (`spec.go:115`) cho OPA.
- `redaction_test.go`, `redaction_rules.go`: bộ che hiện có; `pii_mask.go`.
- `catalog_test.go`: kiểm tên, risk, scope, annotation; golden `testdata/tools_list.golden.json`.
- Tool `Declared` không được liệt kê hay chạy (`catalog.go:91`).
- Khi cờ `request_flow_enabled` tắt, `request-service` trả `REQUEST_FLOW_DISABLED` (CR-REQ-025); gateway chỉ chuyển lỗi.

## Việc cần làm

1. Mở rộng `redaction_test.go`: kết quả của `request_get`, `request_list`, `request_typeHistory`, `solution_list`, `backlog_requests` được bọc `WrapUntrusted`; chuỗi giống khoá API trong `body` Request bị che theo `redaction_rules.go`.
2. Mở rộng `catalog_test.go`: tên đúng `ChannelToToolName`; `request_create` `RiskWriteReversible`, scope `orca:write`; tool đọc scope `orca:read` và `readOnlyHint`; `request_startPhase` `Declared` thì `Lookup` trả không tìm thấy.
3. `executor_guards_test.go`: tool ghi khi RPC trả `REQUEST_FLOW_DISABLED` thì tool trả lỗi giữ mã đó, không "internal error"; `request_flowStatus` trả `{enabled:false}`.
4. Cập nhật golden bằng đúng cách đã dùng ở repo; xem diff: chỉ thêm 17 tool, mô tả và schema khớp mục 2.1 của SOL-017.
5. Kiểm tra độ dài: ghi kích thước golden trước và sau vào mô tả PR.

## Kiểm thử

- `go test ./internal/adapter/mcpserver/tools/... -run 'Redaction|Catalog|Guards'`; toàn gói `go test ./internal/adapter/mcpserver/...`.

## Tiêu chí hoàn thành

- [ ] Bốn đến năm tool đọc mang cờ không tin cậy trong kết quả (test).
- [ ] Golden khớp; không tool nào ngoài 17.
- [ ] Lỗi `REQUEST_FLOW_DISABLED` đi qua nguyên mã.

## Rủi ro và lưu ý

- Chưa đọc quy tắc OPA `session.untrusted_read`; test chỉ khẳng định cờ `UntrustedOutput`, không khẳng định hành vi OPA (ghi vào mô tả PR).
