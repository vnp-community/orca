# BE-CV-SOL-041-mcp-codeintel-tools: 9 tool MCP `codeIntel_*` chỉ-đọc (P2, mặc định tắt)

> **Proposed.** Chưa triển khai, chưa chạy test nào. Chỉ làm khi quyết định O2 (README v7) chuyển sang "Có" (hợp đồng O-11: tắt tới lúc đó). Không đổi `mcp-service`.

**CR:** [CR-CV-041](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-041-mcp-codeintel-tools.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver/tools`, `cmd/server`) · `docs/guides/mcp` · `tests/mcp`
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (AuthZ, input validation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "API Gateway responsibilities"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) mục 9; tool chính sách v5: `CR-MCP-007/008/012/013`

---

## Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-ui-api.md` | §9 (9 tool: `codeIntel_status, changeOverlay, readingOrder, impact, symbol, routes, dataFlows, erd, findings`; `Untrusted` cả 9; `MaxResultBytes` 8–48 KiB; không tool cho `reindex`, `reviewState.*`, `c4.*`, `bindRepo`, `settings.*`, `subscribe`, `quality.*`; cờ tắt => `CODEINTEL_DISABLED` qua `toolErr`, tool vẫn trong `tools/list`), §1 U3, U7, §2.3 |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-01, PQ-04, PQ-14, PQ-27, §8.3 điểm 2, §9 O-11, O-16 |

## Lệch giữa CR và hợp đồng

| # | CR-CV-041 | Hợp đồng / mã thật | Theo |
|---|---|---|---|
| M1 | tham số tool chỉ `worktree_id` | mọi kênh `(sel)` cần `{projectId, worktreeId}` và giải mã chặt (PQ-04, U1); thiếu `projectId` => `INVALID_PARAMS` | thêm `project_id` bắt buộc cho cả 9 tool |
| M2 | `impact.target` một chuỗi ≤ 512 | kênh nhận `target` = `{key}` xor `{name, file?, kind?}` (UI-API 3.1) | tool có `key` hoặc `name`+`file?`+`kind?`; `ArgsOverride` dựng `target` |
| M3 | list loại trừ tường minh gồm 17 kênh | còn **20 kênh `codeIntel.quality.*`** (PQ-27) | thêm một dòng `codeIntel.quality.*` |
| M4 | `limit` 1..100/50 cho tool | kênh `findings ≤ 200`, `dataFlows ≤ 100`, `routes ≤ 500` | giữ ngưỡng chặt hơn của tool (CR) |
| M5 | `KeepKeys` false | `camelizeKeys` (`redaction_rules.go:75`) camel hoá **mọi khoá**, kể cả khoá tự do (`metrics`, `incoming`) | `KeepKeys: true` (đầu ra của kênh đã camelCase) |
| M6 | "tool vẫn hiện trong tools/list khi cờ tắt" | pack 1 bật mặc định (`DefaultConfig`, `config.go`), nên chỉ thêm spec là **mở 9 tool ngay** | cổng triển khai `MCP_CODEINTEL_TOOLS_ENABLED` (mặc định false); khác với cờ tenant `CODEINTEL_DISABLED` |
| M7 | `filterPaths`/`filterMatches` "có thể áp" | chúng chỉ xử lý `obj["items"]` (`files_sensitive_guard.go`); kết quả code-intel là phong bì có `data` | `Post` riêng lọc đường dẫn nhạy cảm |

## Phụ thuộc chéo khu vực

| Hướng | Solution | Ghi chú |
|---|---|---|
| BE trước | `BE-CV-SOL-040-codeintel-channel-foundation` (+ `-view-channels` 10, 13, 11, 12, 14), `BE-CV-SOL-040-…-write-and-stream-channels` (`status` không dùng; `reindex` không có tool) | các kênh `status, changeOverlay, readingOrder, impact, symbol, routes, dataFlows, erd, findings` phải nối RPC thật |
| BE trước | `BE-CV-SOL-013-authorization-flags-and-audit` | quyền đọc thực tế, `read_source` cho `symbol`; audit phân biệt agent/người cần mở rộng `auditclient` |
| FE | không | |
| AG | không | |

Thứ tự §7.3: đợt 6, sau 040 (cả nền lẫn kênh).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `mcpserver/tools/{all_specs.go, spec.go, spec_builders.go, fields.go, executor.go (110-340, 379-421), result_normalize.go, output_schema.go, files_sensitive_guard.go, sensitive_path_rules.go, catalog.go, config.go, catalog_test.go (140-200), parity_test.go, excluded.go, excluded_channels.yaml, pack1_workspace.go (1-60)}`, `cmd/server/mcp_tools_wiring.go` (dòng 29-44, 114 qua grep), `docs/guides/mcp/` (danh sách file), `tests/mcp/` (danh sách), `tests/mcp/check_mcp_ws_channels.py` (1-40).

Xác nhận:
- `AllSpecs()` gom `pack1Workspace(), pack1Git(), pack1SCM(), pack1Trackers(), pack1Operations(), pack2*, pack3*, pack1WorkflowRunStatus(), pack4Admin()` (`all_specs.go`) (hàm nhóm nằm ở các file `pack1_*.go`, `pack2_*.go`, `pack3_exec.go`; chưa đọc từng file ngoài `pack1_workspace.go`).
- `Config` mặc định `Packs{1:true}`; `NewCatalog` giữ spec theo `cfg.Packs[s.Pack]`; `Declared` => không liệt kê/không chạy (`catalog.go`).
- Kiểu `Field` chỉ có string, int, bool, `[]string` (`fields.go`): **không có object lồng**; `defaultArgs` dựng một object phẳng `{wireKey: value}`; `ArgsOverride` (`spec.go`) cho phép dựng tuỳ ý.
- Schema đầu vào đóng (`AdditionalProperties: Not{}`), nên `tenant_id` không thể nhập (`inputSchema`).
- `Executor.CallTool`: tra spec => kiểm scope (`read` => `orca:read`) => `buildArgs` => `gate.Decide` => `run` (cache `CacheTTL`, `guardInputPath`, `ToolTimeout` 55 s) => `normalizeResult` (redact, `camelizeKeys` trừ `KeepKeys`, `Post`, `truncateObject`) => `gate.Complete` => bọc `WrapUntrusted` khối text khi `meta.UntrustedOutput` (`executor.go:163`); `WrapUntrusted` được nối ở `cmd/server/mcp_tools_wiring.go:114`.
- `mapExecError`: lỗi thường khớp `codedMessage = ^([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+):` (`executor.go:379,414`) => `toolErr(<mã>, "request failed")`; vì vậy `CODEINTEL_DISABLED` giữ mã nhưng **mất mô tả và hậu tố JSON** (đúng ý: LLM không nhận `candidates`).
- `Dispatch` của kênh dùng `Identity{TenantID, UserID, Role}` từ `Principal`, không `DeviceID` (`executor.go:135`).
- Golden `TestToolsListGolden` duyệt **mọi** spec của `AllSpecs()` bất kể pack (`catalog_test.go:161`), nên thêm 9 spec luôn đổi `testdata/tools_list.golden.json`.
- `ChannelToToolName` thay `.` bằng `_`, giữ chữ hoa (`spec.go`); `codeIntel.status` => `codeIntel_status`.
- Chưa có `tests/mcp/check_mcp_codeintel_tools.py`, `pack1_codeintel.go`; `docs/guides/mcp/` chưa có đoạn code-intel.

### Correction relative to CR-CV-041

| # | CR nói | Thực tế | Xử lý |
|---|---|---|---|
| C1 | `ToolSpec{Fields: ...}` đủ cho `impact.target` | `Field` không có object | `ArgsOverride` |
| C2 | "thêm `pack1CodeIntel()` vào danh sách nhóm" đủ | tool lập tức nằm trong catalog mặc định | cổng `Config.CodeIntelTools` (M6) |
| C3 | `PathFilter` cho `findings/impact` | không áp được (M7) | `Post` |
| C4 | `KeepKeys` false | camel hoá khoá tự do (M5) | `KeepKeys: true` |
| C5 | tên tool không dấu `-`: ok | `toolNameRE ^[a-zA-Z0-9_-]{1,64}$` (`parity_test.go`) | `codeIntel_changeOverlay` hợp lệ |

## 2. Giải pháp

### 2.1 Cổng triển khai (mới)

`Config.CodeIntelTools bool` (`tools/config.go`), đọc từ `MCP_CODEINTEL_TOOLS_ENABLED` (mặc định false) trong `ApplyEnv`; `NewCatalog` bỏ qua spec có `s.Namespace == "codeIntel"` khi false (một điều kiện, kèm test). Hệ quả: binary mới không lộ tool nào cho tới khi vận hành bật (O-11); `tools/list` và admin view không thấy tool khi tắt; khi bật nhưng tenant tắt `code_intel_enabled`, tool **vẫn liệt kê** và `tools/call` trả `CODEINTEL_DISABLED` (hợp đồng §9).

### 2.2 Tool

File `tools/pack1_codeintel.go` (mới): `pack1CodeIntel()`; thêm vào `AllSpecs()`. Trường chung: `project_id` (`Str("project_id","projectId",...,Req,Len(64))`), `worktree_id` (`Str("worktree_id","worktreeId",...,Req,Len(512))`).

| Tool | Kênh | Trường thêm (snake_case => wire) | `MaxResultBytes` | `CacheTTL` |
|---|---|---|---|---|
| `codeIntel_status` | `codeIntel.status` | — | 8 KiB | 30 s |
| `codeIntel_changeOverlay` | `codeIntel.changeOverlay` | `base?` (≤ 128) | 48 KiB | 0 |
| `codeIntel_readingOrder` | `codeIntel.readingOrder` | `base?` | 24 KiB | 0 |
| `codeIntel_impact` | `codeIntel.impact` | `key?` (≤ 1024) hoặc `name?`+`file?`+`kind?`; `direction?` upstream\|downstream; `depth?` 1..3 | 48 KiB | 0 |
| `codeIntel_symbol` | `codeIntel.symbol` | `key?` hoặc `name?`+`file?` (`PathArg:"file"`, `PathOptional`); `include_source?` | 32 KiB | 0 |
| `codeIntel_routes` | `codeIntel.routes` | `limit?` 1..100, `page_token?` | 32 KiB | 30 s |
| `codeIntel_dataFlows` | `codeIntel.dataFlows` | `limit?` 1..50, `page_token?` | 24 KiB | 0 |
| `codeIntel_erd` | `codeIntel.erd` | `service?` | 48 KiB | 30 s |
| `codeIntel_findings` | `codeIntel.findings` | `rules?` (≤ 32), `limit?` 1..100, `page_token?` | 24 KiB | 30 s |

Chung: `read(...)` (pack 1, `Risk=read`, `ReadOnly+Idempotent`), `Untrusted=true` cả 9, `OpenWorld=false`, `KeepKeys=true`, mô tả ≤ 1024 ký tự không chứa `ignore previous`, `http://`, `https://`, `@` (`parity_test.go`). Không tool nào có `tenant_id`, `user_id`, `workspace_root`, `repo`, `args`, `cypher`, `command`. `ArgsOverride` cho `impact` (và kiểm "key xor name") và cho `symbol` (key xor name+file).

### 2.3 Kết quả cho LLM

`MaxResultBytes` nhỏ hơn kênh WS (8–48 KiB) với `truncateObject` (`result_normalize.go`) cắt mảng/chuỗi và đánh dấu `truncated`. `Post` (`pack1_codeintel_result.go`, mới): (a) `symbol`: cắt `data.source.text` theo dòng (đầu + cuối) còn ≤ 32 KiB, đặt `source.truncated=true`; (b) mọi tool: đệ quy bỏ phần tử object có `filePath|file|path` thuộc `IsSensitivePath` (sau `CleanWorktreePath`); (c) không sinh gợi ý lệnh. Che secret bằng `redactValue` có sẵn. `PathArg:"file"` của `symbol` trả lỗi như "không tìm thấy" (`errFilesNotFound`).

### 2.4 Parity và loại trừ

`excluded_channels.yaml`: **thay** dòng `codeIntel.*` (SOL-040-foundation 2.8) bằng: 17 dòng kênh `codeIntel.reindex`, `.reindexStatus`, `.structure`, `.architecture`, `.dataFlow`, `.storage`, `.subgraph`, `.contractDiff`, `.dismissFinding`, `.reviewState.get`, `.reviewState.save`, `.c4.get`, `.c4.save`, `.bindRepo`, `.settings.get`, `.settings.set`, `.subscribe` + **một** dòng `codeIntel.quality.*` (M3); mỗi dòng `category` và `reason` >= 20 ký tự. 17 + 20 + 9 tool = 46. `TestChannelInventory` đỏ nếu thiếu/trùng. Golden cập nhật có chủ đích.

### 2.5 Tài liệu và e2e

`docs/guides/mcp/code-intelligence-tools.md` (mới; tên theo nội dung): 9 tool, tham số `project_id`+`worktree_id`, cờ tenant và cờ triển khai, cảnh báo nội dung không tin cậy, giới hạn kích thước, `reindex` không có. `tests/mcp/check_mcp_codeintel_tools.py` (mới) theo `mcp_check_framework.py`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Chỉ đọc, 9 tool | ghi/reindex chạy trình phân tích trên máy dev; LLM không tự điều khiển |
| D2 | Cổng `MCP_CODEINTEL_TOOLS_ENABLED` | pack 1 bật mặc định; O2 mặc định "Không" |
| D3 | Tất cả `Untrusted` + `KeepKeys` | văn bản phụ thuộc repo; khoá tự do không bị camel hoá |
| D4 | Thêm `project_id` | PQ-04/U1 |
| D5 | `Post` lọc đường dẫn nhạy cảm | `PathFilter` không áp được |
| D6 | Không sửa `mcp-service`, không hạn mức mới | mọi thứ cần đã có trong `ToolRef`; đo ở CR-071 |
| D7 | Tool vẫn hiện khi tenant tắt cờ | cùng quyết định v6 CR-REQ-017 Q3 |

## 4. Phụ thuộc và thứ tự

TASK-041-01 (cổng cấu hình) => 02 (spec) => 03 (kết quả) => 04 (loại trừ + golden) => 05 (an toàn/red-team) => 06 (e2e + hướng dẫn). 03 có thể song song với 02 sau khi có kiểu field.

## 5. Kiểm thử

`go test ./internal/adapter/mcpserver/tools/...` (parity, golden, catalog, args, redaction, executor), `go test ./cmd/server/...`; red-team `mcp-service/internal/redteam`; e2e MCP. Chi tiết trong từng task. Chạy `backend-go/ci/mcp-conformance/run-go-conformance.sh` (tier 1; chưa kiểm chứng). Chưa chạy bất kỳ test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa đọc hết `policy/orca-authz/mcp.rego`: chưa biết sau đọc không tin cậy thì tool `write_reversible` không-open-world có bị đẩy sang approval (CR-041 mục 6); red-team phải chốt.
- Taint chỉ có hiệu lực với tool open-world kế tiếp (`mcp.rego:155-176` theo CR, chưa đọc lại trong phiên này).
- Số 9 tool ảnh hưởng ngữ cảnh LLM (CR-MCP-008 khuyến nghị 60–100 tool); chưa đo.
- `CacheTTL` theo `(tenant,user,tool,args)` có thể hiện dữ liệu cũ nhẹ 30 s.
- Hạn mức đọc đồng thời của `code-intel-service` chưa đo.
- `auditclient.Append` thiếu `actor_type/target_type` (`common/auditclient/client.go:35`, theo CR): audit đọc mã nguồn kiểu "agent hay người" cần mở rộng, ngoài solution này.
- Phiên SSH/remote: tool gọi kênh WS nội bộ; không giả định thực thi cục bộ.

## 7. Câu hỏi mở

- **Q1 (O2/O-11).** Mở MCP không; nếu không, đóng solution và giữ dòng `codeIntel.*`.
- **Q2.** `project_id`: LLM lấy ở đâu (`project_list`, `worktree_list` đã có); xác nhận tên field trùng pack 1.
- **Q3.** `codeIntel_reindexStatus` có mở không (mặc định: không).
- **Q4.** `codeIntel_symbol` có cần tenant policy mặc định `require_approval` (nhạy cảm nhất)?
- **Q5.** Thêm `codeIntel_subgraph/structure` sau khi có nhu cầu.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` (§9)
- `/opt/repos/orca/docs/crs/v7/code-intel-gateway/CR-CV-041-mcp-codeintel-tools.md`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/mcpserver/tools/{spec.go,spec_builders.go,fields.go,executor.go,result_normalize.go,files_sensitive_guard.go,parity_test.go,excluded_channels.yaml,config.go,catalog.go}`
- `/opt/repos/orca/specs/backend-go/crs/v6/gateway-and-mcp/solutions/BE-REQ-SOL-017-mcp-request-tools-and-source.md` (mẫu)
