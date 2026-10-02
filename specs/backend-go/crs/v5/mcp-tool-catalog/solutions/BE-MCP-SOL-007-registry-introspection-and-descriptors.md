# BE-MCP-SOL-007: Introspection cho `wscompat.Registry`, `ToolSpec`, `ToolExecutor`, parity test

> **🔲 Designed — chưa implement.** Nền tảng cho BE-MCP-SOL-008/009/010; phụ thuộc BE-MCP-SOL-003 (khung `/mcp`) và gọi vào `mcp-service` (BE-MCP-SOL-002/012/013) qua gRPC.

**CR:** [CR-MCP-007](../../../../../../docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md)
**Service:** `api-gateway` (adapter mới `mcpserver/tools`, thêm accessor vào `wscompat`)
**TDD tham chiếu:** `api-gateway.md` §2, §6 (T1, T2); `arch/08` (gateway responsibilities); `arch/03` (adapter không chứa nghiệp vụ)
**CONTRACT items hiện thực:** kênh `mcp.admin.tool.list` (phần catalog) + kiểu `McpToolView` (§1) — phần `effective/effectiveSource` do `mcp-service` tính (BE-MCP-SOL-012), phần còn lại ở đây.

---

## 1. Trạng thái hiện tại (re-verify bằng mã thật, 2026-10-01)

| Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|
| `Registry` có 4 map private, không có API liệt kê | Đúng. `registry.go:54-59`: `handlers`, `streamHandlers`, `streamChannelHandlers`, `binaryStreamHandlers`. Chỉ có tra cứu theo tên: `StreamHandlerFor`, `BinaryStreamHandlerFor`, `DispatchStreamChannel`, `Dispatch`. `grep -rn "func (r \*Registry)"` không có `Channels/List/Names` | Không |
| `ChannelHandler` = `func(ctx, Identity, []json.RawMessage) (any, error)` | Đúng (`registry.go:44`) | Không |
| "một channel chỉ thuộc đúng một loại" | Đúng về thiết kế (doc `registry.go`), **và** đo thực tế không có tên nào trùng giữa 4 map (449 tên duy nhất / 449 mục) | Không |
| `Dispatch` chỉ phục vụ `handlers` | **Đúng và là điểm CR chưa nói rõ:** `Dispatch` (`registry.go:172`) chỉ tra `r.handlers`; channel loại stream/streamChannel/binary rơi vào `notImplementedHandler`. `terminal.create`, `agent.start/resume/switchAccount`, `ephemeralVm.provision`, `files.watch`, `terminal.subscribe`, `onboarding.openGhAuthTerminal` là `StreamChannelHandler` ⇒ `ToolExecutor` phải đi `DispatchStreamChannel` (xem §2.C) | **Lệch nhỏ (CR nói chung chung "gọi Dispatch")** |
| "~417 lần Register*, 58 namespace" | Grep literal `.Register*("…")` trong file không-test: 418 (402 `Register` + 12 `RegisterStream` + 8 `RegisterStreamChannel` + 2 `RegisterBinaryStreamHandler` = 424 lời gọi, 6 lời gọi dùng biến `channel`). **Số đo runtime thật: 449 channel duy nhất, 57 namespace** (xem §2.F) | **Lệch** — CR đếm thiếu ~32 (đăng ký qua helper/vòng lặp: `simpleFileOp`, `registerAccountsRelay`, `registerBrowserRelay("browser."+op)`…) |
| Handler trả `any` | Đúng, và **không đồng nhất**: một số trả `proto.Message` thô (`git.diff`, `files.read` → `ReadFileResponse.Content []byte`), số khác trả struct `…View` camelCase, số khác `map[string]any`. Envelope WS tuần tự hoá bằng `encoding/json` (không `protojson`) ⇒ proto thô ra **snake_case + `[]byte` thành base64** (xem comment `channels.go` "anchorView/annotationView") | Bổ sung: `ToolExecutor` phải tự chuẩn hoá (§2.D) |
| Tham số theo vị trí | Đúng: mọi handler `decodeArg[T](args, 0)` (arg[0] là object). Tên khoá **không nhất quán** giữa channel: `git.status/commit/history` dùng `worktree` (tiền tố `id:` bị `stripWorktreeSelectorPrefix` cắt), `git.diff` bản đầu dùng `worktreeId` (bị `registerGitDeepChannels` ghi đè bằng `worktree`), `files.*` dùng `worktreeId`, `terminal.send` dùng `terminal`/`text` ⇒ **`Args()` tay là cách duy nhất đúng** | Không (khẳng định luận điểm CR) |
| `Identity` có `Role` chỉ ở nhánh cookie | Đúng (`registry.go:19-35`). Token MCP (OAuth/PAT) phải được BE-005/006 + BE-003 chuyển thành `Identity{TenantID,UserID,Role}` với `Role` lấy từ auth-service; `Role` rỗng ⇒ mọi channel `requireAdmin` fail-closed (mong muốn) | Phụ thuộc BE-003 |
| Không có thư viện JSON Schema/MCP trong module | Đúng: `go.mod` của `api-gateway` và `go.work` không có `jsonschema`/`modelcontextprotocol` (grep). Chọn thư viện thuộc ADR D4 | (chưa xác minh phiên bản) |

## Quyết định khác/thêm so với CR gốc

1. **Hai loại `ToolSpec`:** `Kind=Channel` (một channel unary/streamChannel, `Args()` ánh xạ trực tiếp — đa số) và `Kind=Composite` (hàm `Run` của adapter, được phép gọi nhiều channel/stream — chỉ cho `terminal_*`, `agent_*` ở BE-009 và `workflow_run_status`). Lý do: stream cần bộ đệm/con trỏ mà channel không có (xem BE-009).
2. **`excluded_channels.yaml` có thêm cờ `listedAsHardDenied`:** channel vừa bị loại vừa muốn admin thấy "khoá" trong tab Tools (FE-MCP-SOL-005) — handler `mcp.admin.tool.list` tổng hợp `McpToolView{hardDenied:true,effective:'deny',effectiveSource:'hard_deny'}` từ YAML; `tools/list` của giao thức **không bao giờ** liệt kê chúng.
3. **Số đo chính xác được in ở CI** bằng test kiểm kê (§2.F), không dùng grep.
4. **Lỗi validate input** trả `isError:true` (tool execution error) với mã `INVALID_ARGUMENTS`, không phải JSON-RPC `-32602` — để LLM tự sửa tham số (chưa xác minh khớp spec 2025-06-18; đối chiếu khi làm ADR D5).

## 2. Giải pháp

### A. `Registry.Channels()` (chỉ đọc, additive)

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/registry_channels.go` (NEW — tách file để không chạm `registry.go`)

```go
type ChannelKind string

const (
	ChannelUnary        ChannelKind = "unary"        // r.handlers
	ChannelStream       ChannelKind = "stream"       // r.streamHandlers
	ChannelStreamChannel ChannelKind = "streamChannel" // r.streamChannelHandlers
	ChannelBinaryStream ChannelKind = "binaryStream" // r.binaryStreamHandlers
)

type ChannelInfo struct {
	Name string
	Kind ChannelKind
}

// Channels returns a name-sorted snapshot. Registration happens only in the
// composition root before Serve, so no lock is needed; calling it later while
// another goroutine registers is a bug (maps are unsynchronised by design).
func (r *Registry) Channels() []ChannelInfo
```

Impact: `gitnexus impact` target `Registry` direction upstream — **chưa chạy**; lệnh: `gitnexus_impact({target:"Registry", direction:"upstream"})`. Rủi ro dự kiến LOW–MEDIUM (nhiều caller nhưng chỉ thêm method trong file mới, không đổi chữ ký). Test: `registry_channels_test.go` — đăng ký 1 channel mỗi loại, kiểm sắp xếp, kiểm bất biến "không trùng tên giữa các loại".

Thêm 2 hàm xuất để adapter ngoài package dùng được (hiện các `*Context(...)`/registry per-connection là unexported, dựng trong `handler.go:111-137`):

```go
// registry_tool_session.go (NEW)
// ToolSession replaces the per-WebSocket-connection registries (terminal
// streams, json-subscribe, binary router, provision, file-watch) with
// per-MCP-session ones, so terminal.send & friends work without a WS conn.
type ToolSession struct{ ctx context.Context; cancel context.CancelFunc /* + the 5 registries */ }
func NewToolSession(parent context.Context) *ToolSession
func (s *ToolSession) Context() context.Context
func (s *ToolSession) Close()  // cancels every AttachPty stream; does NOT kill PTYs (BE-009 does that)
```

### B. `ToolSpec` (khai báo tay, đặt cạnh domain)

**Package:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/` (T1: adapter giao thức, không nghiệp vụ). Thư viện sinh schema: `google/jsonschema-go` (cùng họ go-sdk D4) — (chưa xác minh) chốt trong ADR D4.

```go
type Risk string // read | write_reversible | exec | destructive | admin   (khớp CONTRACT McpRisk)

type ToolSpec struct {
	Name        string   // D7: ChannelToToolName(Channel); override chỉ khi Kind=Composite hoặc có NameReason
	Channel     string   // khoá nối Registry (Kind=Channel); rỗng với Composite
	UsesChannels []string // Composite: các channel nó gọi (để parity test tính là "đã phủ")
	Kind        SpecKind // Channel | Composite
	Namespace   string   // = phần trước dấu '.' đầu tiên của channel; Composite khai tay
	Pack        int      // 1..4 (BE-008) → McpToolView.pack
	Title, Description string
	Risk        Risk
	Scope       string   // suy ra từ Risk qua riskToScope(); khai tay chỉ để nới (không bao giờ thu hẹp)
	Annotations Annotations // ReadOnly, Destructive, Idempotent, OpenWorld
	Input, Output *jsonschema.Schema // sinh từ struct qua schemaOf[T]()
	MaxResultBytes int   // 0 = mặc định 65536
	Args func(in json.RawMessage, id wscompat.Identity) ([]json.RawMessage, error)
	Run  CompositeFunc    // chỉ Kind=Composite
	Redact []RedactionRule // bổ sung cho bộ luật chung (§2.D)
}
```

`riskToScope`: `read→orca:read`, `write_reversible→orca:write`, `exec→orca:exec`, `destructive|admin→orca:admin` (khớp CONTRACT `McpScopeId`). Quy tắc nhất quán `Annotations`: `ReadOnly ⇒ Risk==read`; `Destructive ⇒ Risk∈{destructive,admin}` — parity test kiểm.

`Args()` là nơi **duy nhất** ép trường định danh: không bao giờ copy `userId`/`tenantId` từ input vào args (vd `agent.start` có `agentStartArgs.UserID` đọc từ args — `Args()` ghi đè bằng `id.UserID`, và schema đầu vào **không có** trường này). Mỗi spec có test bảng `TestArgs_<tool>` (input JSON → `[]json.RawMessage` đúng khoá/ thứ tự thật của handler, đối chiếu bằng cách chạy chính handler với client gRPC giả). Ví dụ ở BE-008 §3.

Danh mục: `catalog.go` — `type Catalog struct{ byName map[string]*ToolSpec; byChannel map[string][]*ToolSpec }`, dựng một lần ở `main.go` từ `AllSpecs()` (`packs_register.go`), bất biến sau đó. `VisibleFor(ctx, tenant, user, scopes) []*ToolSpec` = lọc theo scope của token **và** quyết định hiệu lực từ `PolicyGate.ResolveEffective` (cache TTL 30s, vô hiệu bằng sự kiện §2.G); `deny`/hard-deny bị ẩn, `require_approval` vẫn hiện (client biết sẽ có hộp thoại).

### C. `ToolExecutor` — pipeline (T2: quyết định luôn ở `mcp-service`)

```
tools/call(name, arguments)                         [mcpserver, BE-003]
 1 Catalog.Lookup(name)                  → không có/ẩn ⇒ -32602 "unknown tool"
 2 Validate(arguments, spec.Input)       → isError INVALID_ARGUMENTS (không chạm handler)
 3 args := spec.Args(arguments, id)      → ErrArgsMapping ⇒ INVALID_ARGUMENTS
 4 paramsHash := sha256(canonicalJSON(name, arguments))
 5 dec := PolicyGate.Authorize(ctx, ...) → mcp-service gRPC (BE-012): allow | deny | require_approval(approvalId)
      deny ⇒ isError MCP_POLICY_HARD_DENY / "not permitted"; require_approval ⇒ ErrApprovalRequired{ID} (BE-013 quyết wait/elicitation)
 6 execute:  Kind=Channel & unary          ⇒ Registry.Dispatch(sessCtx, id, channel, args)      (timeout nội bộ 60s, registry.go dispatchRPCTimeout)
             Kind=Channel & streamChannel  ⇒ Registry.DispatchStreamChannel(...) lấy `ack`, huỷ `events` ngay (chỉ dùng ack)
             Kind=Composite                ⇒ spec.Run(ctx, id, input, deps)
 7 normalizeResult(raw) → bytes/JSON → Redact → Truncate(spec.MaxResultBytes) → structuredContent + content[text]
 8 PolicyGate.RecordResult(...)          → audit qua outbox của mcp-service (BE-013); gateway KHÔNG ghi DB
```

`tool timeout` tổng: `MCP_TOOL_TIMEOUT` (mặc định 55s, nhỏ hơn 60s của `Dispatch` để lỗi do ta chủ động), vượt ⇒ `isError` kèm gợi ý dùng mẫu BE-009. Ánh xạ lỗi (không rò chuỗi nội bộ): `codes.InvalidArgument` ⇒ giữ thông điệp ngắn; `NotFound/PermissionDenied` ⇒ `"not found or not permitted"` (không phân biệt, khớp `MCP_NOT_FOUND`); `Unavailable/DeadlineExceeded` ⇒ `"temporarily unavailable, retry"`; còn lại ⇒ `"internal error (trace_id=…)"`. Lỗi `fmt.Errorf` kiểu `"CODE: …"` của wscompat (vd `BROWSER_MISSING_ARGS`) chỉ giữ phần `CODE`.

### D. Chuẩn hoá kết quả

`result_normalize.go`: (1) `proto.Message` ⇒ `protojson.MarshalOptions{UseProtoNames:false}` (camelCase, `bytes` ⇒ base64 mặc định ⇒ spec cần `Output` riêng; với `files_readPreview` Composite giải mã UTF-8, nhị phân ⇒ `{binary:true,size}` chứ không trả base64); (2) còn lại `json.Marshal`; (3) bộ che chung `redaction_rules.go`: URL có userinfo `://[^/\s:@]+(:[^@\s/]*)?@` ⇒ `://***@`; `ghp_…/github_pat_…/glpat-…/xox[bap]-…/AKIA…/Bearer …/-----BEGIN … PRIVATE KEY-----`; khoá JSON tên `token|secret|password|apiKey|credential*` ⇒ `"***"`; đường dẫn tuyệt đối của máy khác ⇒ giữ nguyên chỉ khi nằm dưới gốc worktree người gọi (chi tiết BE-008 §4). (4) Cắt: `MaxResultBytes` mặc định 64 KiB, cắt tại ranh giới rune/phần tử mảng, thêm `{"truncated":true,"hint":"use cursor/limit"}`; `structuredContent` luôn là JSON hợp lệ (cắt mảng, không cắt giữa chuỗi).

### E. Parity test + `excluded_channels.yaml`

**File:** `mcpserver/tools/excluded_channels.yaml`, `mcpserver/tools/parity_test.go`

```yaml
# channel | reason (bắt buộc, ≥ 20 ký tự) | listedAsHardDenied | category
- pattern: "credentials.*"
  reason: "Đọc/ghi secret tenant — secret không bao giờ ra khỏi server (D6, C6)"
  category: secret
  listedAsHardDenied: true
- pattern: "mobile.*"
  reason: "Yêu cầu Identity.DeviceID (registry.go:22-28); không có ngữ nghĩa với agent"
  category: device-bound
```

Test (`go test ./internal/adapter/mcpserver/tools/ -run TestToolParity`), khẳng định:
1. Mọi `Registry.Channels()` thuộc đúng một trong: có ≥1 `ToolSpec` (qua `Channel` hoặc `UsesChannels`) **hoặc** khớp một `pattern` loại trừ (glob `*`); thiếu ⇒ **đỏ**, in danh sách.
2. Mỗi pattern loại trừ khớp ≥1 channel (chống mục chết) và không channel nào vừa có spec vừa bị loại trừ.
3. Tên tool: duy nhất, khớp `^[a-zA-Z0-9_-]{1,64}$`, bằng `ChannelToToolName` trừ `NameReason`; mô tả không rỗng, ≤ 1024 ký tự, không chứa chuỗi cấm (`ignore previous`, URL, email — chống prompt-injection bề mặt mô tả); schema compile được; có `Scope`+`Annotations`; ràng buộc nhất quán risk/annotation (§2.B).
4. `Kind=Channel`: `Registry.Channels()` có channel đó và `Kind` ∈ {unary, streamChannel}; loại `stream`/`binaryStream` không được có `ToolSpec.Kind=Channel`.
5. In dòng đo đếm cố định (xem F) để theo dõi tiến độ.

Registry dùng trong test: `wscompat.RegisterProductionChannels(r, ChannelDeps{…nil-able…})` — **refactor kèm theo**: gom 5 lời gọi ở `cmd/server/main.go:332-396` (`RegisterRealChannels`, `RegisterPushChannels`, `RegisterClientStateChannels`, `RegisterTaskActivityStreamChannel`, `RegisterWorkspaceSubscribeChannel`, `RegisterMobileChannels`) vào một hàm xuất để production và test dùng chung. Không thể tái dùng chỉ `RegisterRealChannels` vì 12 channel stream/mobile/clientState nằm ngoài nó.

### F. Số đo kênh (đã đo bằng phương pháp thật)

**Phương pháp:** chạy thật `NewRegistry()` + các hàm đăng ký của `main.go` với client `nil` (đăng ký chỉ tạo closure, không gọi gRPC), rồi đếm 4 map bằng một test tạm chèn qua `go test -overlay` (không sửa repo):

```bash
cd backend-go/services/api-gateway
go test -overlay overlay.json -run TestZZCount -count=1 ./internal/adapter/wscompat/ -v
```

Kết quả (2026-10-01, cây làm việc hiện tại): **449 channel duy nhất = 427 unary + 12 stream + 8 streamChannel + 2 binaryStream; 57 namespace; không tên trùng giữa các loại.** Chỉ `RegisterRealChannels` cho 419+5+8+2 = 434 (52 namespace). Top: `git` 43, `github` 33, `files` 21, `task` 19, `repo` 19, `linear` 19, `jira` 19, `browser` 19, `terminal` 18, `worktree` 15, `devServer` 14, `admin` 14, `workflow` 13. `git.diff` được đăng ký hai lần (map ghi đè, bản `registerGitDeepChannels` thắng) nên grep literal đếm dư. Test CI vĩnh viễn `TestChannelInventory` (cùng file parity) in:
`MCP_CHANNEL_INVENTORY total=449 unary=427 stream=12 streamChannel=8 binary=2 covered=<n> excluded=<n> uncovered=<n>` và fail nếu `uncovered>0`.

### G. `tools/list_changed`

Gateway subscribe ephemeral (mỗi replica nhận đủ — cơ chế `Consumer.SubscribeEphemeral(ctx, stream, subject, fn)` có thật ở `common/eventbus`, tạo consumer JetStream không bền; **lưu ý T5 ghi "core-NATS" nhưng mã thật là JetStream ephemeral consumer**, `InactiveThreshold`) các subject do `mcp-service` publish qua outbox: `orca.mcp.policy.changed`, `orca.mcp.killswitch.changed`, `orca.mcp.settings.changed` (đổi `enabled`/rollout pack). Handler: xoá cache `ResolveEffective` của tenant, rồi gọi notifier phiên của BE-MCP-SOL-004 gửi `notifications/tools/list_changed` tới mọi phiên SSE của tenant (qua `orca.ephemeral.mcp.session.<id>`, T5, cho phiên nằm ở replica khác). Debounce 500ms để batch nhiều policy đổi liền nhau. Danh mục tool tĩnh (đổi theo release) không phát sự kiện — client gặp lại khi `initialize`.

### H. Kênh `mcp.admin.tool.list` (CONTRACT §2.2)

**File:** `wscompat/channels_mcp.go` (BE-MCP-SOL-003/012 tạo file; ở đây thêm `registerMcpToolCatalogChannel`). Quy tắc: `Identity.Role!="admin"` ⇒ `MCP_NOT_ADMIN`; `MCP_ENABLED=false` ⇒ `MCP_DISABLED`. Tham số `{namespace?, risk?}` (arg[0] object). Trả `McpToolView[]` = (mọi spec thuộc catalog **kể cả** tool bị policy `deny`) + (mục YAML `listedAsHardDenied`). `effective/effectiveSource` lấy từ `PolicyGate.ResolveEffective` cho tenant của admin (mặc định, không theo user). Sắp xếp `namespace, name`. `annotations` map `ReadOnly→readOnly`, … camelCase. Không phân trang (catalog ≤ ~200 mục, mỗi mục ~300 byte).

## Hợp đồng với frontend

| Kênh | Ghi chú |
|---|---|
| `mcp.admin.tool.list` | trả `McpToolView[]` đúng CONTRACT §1; lỗi `MCP_NOT_ADMIN`, `MCP_DISABLED`. Tiêu thụ: FE-MCP-SOL-005 |
| `pack` | lấy từ `ToolSpec.Pack` (BE-008) |

## Sửa TDD kèm theo
- **T1** `api-gateway.md` §3/§6: thêm adapter `mcpserver/tools` (dịch giao thức + gọi gRPC; không quy tắc nghiệp vụ).
- **T2** `api-gateway.md` §6 + `arch/08`: mô tả `wscompat.Registry` là lớp dịch tới gRPC client; `ToolExecutor` là adapter, quyết định allow/deny/approval luôn của `mcp-service`.
- Ghi chú T5: sửa mô tả "core-NATS" thành "JetStream ephemeral consumer cho fan-out mỗi replica (đã có), core-NATS chỉ cho `orca.ephemeral.mcp.session.*`" — cần xác minh khi cài.

## Kiểm thử
`cd backend-go/services/api-gateway && go test ./internal/adapter/wscompat/ ./internal/adapter/mcpserver/tools/...` (toàn bộ test `wscompat` hiện có phải xanh); golden `testdata/tools_list.golden.json` (tên+schema+scope); `TestArgs_*` mỗi tool; fuzz `FuzzNormalizeResult` (không panic, luôn JSON hợp lệ); test che secret với bảng mẫu (`https://u:ghp_x@github.com/o/r.git`). Integration (`//go:build integration`): MCP Inspector/`mcp-go` client gọi 1 tool read, 1 write, 1 destructive (CR AC).

## Rủi ro & phụ thuộc
~450 mô tả viết tay (chia pack BE-008); mô tả là bề mặt prompt-injection ⇒ review + test cấm chuỗi. `wscompat` là lớp "legacy": nếu thay, chỉ `ToolSpec.Channel`+`Args` đổi. Phụ thuộc: BE-003 (khung `/mcp`, `Identity` từ token), BE-004 (notifier phiên), BE-012/013 (`PolicyGate`, audit). Tên RPC của `PolicyGate` do BE-012 chốt — ở đây chỉ giả định hình `{decision, approvalId, source, reasons}`.

## Không thuộc phạm vi
Nội dung pack (BE-008), terminal/agent stream (BE-009), policy engine & approval (BE-012/013), UI (FE-MCP-SOL-005).

## Liên quan
`wscompat/registry.go`, `wscompat/handler.go:111-137`, `cmd/server/main.go:331-398`, `common/eventbus` (`SubscribeEphemeral`).
