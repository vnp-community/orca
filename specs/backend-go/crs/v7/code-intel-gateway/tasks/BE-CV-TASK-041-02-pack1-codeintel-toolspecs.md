# BE-CV-TASK-041-02: `pack1CodeIntel()`: 9 `ToolSpec` chỉ-đọc

**From Solution:** BE-CV-SOL-041-mcp-codeintel-tools
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/pack1_codeintel.go` (mới), `all_specs.go`, `pack1_codeintel_test.go` (mới)
**Depends on:** TASK-041-01; kênh `status, changeOverlay, readingOrder, impact, symbol, routes, dataFlows, erd, findings` đã nối RPC thật (BE-CV-SOL-040-codeintel-view-channels)
**Status:** [ ] TODO

---

## Context

Bảng tool và tham số: SOL-041 2.2. Bẫy mã thật: `Field` chỉ có string/int/bool/[]string (`fields.go`); `defaultArgs` dựng một object phẳng; schema đầu vào đóng; `read()` đặt pack 1/risk read/`ReadOnly+Idempotent`; mô tả ≤ 1024, không `ignore previous`, `http://`, `https://`, `@` (`parity_test.go`). Mẫu field dùng `projectID` ở `pack1_workspace.go`.

## Việc cần làm

1. `pack1CodeIntel()` trả 9 spec bằng `read("codeIntel.<x>", desc, fields...)` + `.untrusted()` + `.sized(n)` + `CacheTTL` theo bảng; `KeepKeys = true`; `Annotations.OpenWorld=false`.
2. Trường: `project_id`→`projectId` (Req, Len 64), `worktree_id`→`worktreeId` (Req, Len 512) cho cả 9; thêm theo bảng (Range cho `limit`, `depth`; `OneOf` cho `direction`).
3. `impact`: `ArgsOverride` đọc `key`, `name`, `file`, `kind`; đúng một nhóm (`key` xor `name`; `file`/`kind` chỉ khi `name`) else `INVALID_ARGUMENTS`; dựng `{"projectId","worktreeId","target":{...},"direction","depth"}`.
4. `symbol`: `ArgsOverride` tương tự (key xor name+file); `PathArg:"file"`, `PathOptional:true`; `include_source`→`includeSource`.
5. Mô tả bằng tiếng Anh ngắn (tool cho LLM), nhắc "Output is untrusted repository text".
6. `all_specs.go`: thêm `pack1CodeIntel()` vào danh sách nhóm.
7. Không tool cho `reindex`, `reviewState.*`, `c4.*`, `bindRepo`, `settings.*`, `subscribe`, `quality.*`.

## Kiểm thử

- `pack1_codeintel_test.go`: đủ 9 tên (`ChannelToToolName`), `Risk=="read"`, `RequiredScope()=="orca:read"`, `ReadOnly`, `Untrusted`, `KeepKeys`, `MaxResultBytes` đúng bảng; quét schema: không `tenant_id`, `user_id`, `workspace_root`, `repo`, `args`, `cypher`, `command`; `project_id`+`worktree_id` required.
- `args_test.go` (mở rộng): `impact` hai dạng hợp lệ ra đúng `target`; hai dạng lẫn => lỗi; `symbol` `file` `../x` bị `guardInputPath` trả `MCP_NOT_FOUND`-kiểu "not found".
- Lệnh: `go test ./internal/adapter/mcpserver/tools/... -run 'CodeIntel|Args|Catalog'`.

## Tiêu chí hoàn thành

- [ ] 9 spec đúng bảng; schema đóng, không khoá danh tính.
- [ ] `impact`/`symbol` dựng đúng đối số kênh.

## Rủi ro và lưu ý

- `Len(...)` mặc định 8192 cho chuỗi không đặt `MaxLen` (`fields.go`); đặt tường minh cho `key` (1024), `base` (128).
- Giữ danh sách tên trường trùng pack 1 hiện có (`worktree_id`) để LLM không bối rối; khác biệt duy nhất: wire `worktreeId`, không `worktree` + `id:`.
