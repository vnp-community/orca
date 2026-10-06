# 03 — Luồng lệnh và luồng dữ liệu

Ký hiệu: **[có]** = đã tồn tại trong repo; **[mới]** = đề xuất.

## 1. Lệnh: UI → backend → agent → MCP local

```
UI (React)                      api-gateway (wscompat)            infra-fleet-service              Agent (dev server)               MCP local
 │ invoke "codeIntel.overview"        │                                   │                              │                                 │
 │ {projectId, worktreeId, level}     │                                   │                              │                                 │
 ├───────────WS /ws──────────────────▶│ authz + tenant                    │                              │                                 │
 │                                    │ [mới] resolve project→devServer   │                              │                                 │
 │                                    │   cache hit (repo,commit,level)? ─┼─▶ trả cache                  │                                 │
 │                                    ├─gRPC Relay/RelayByDevServer [có]─▶│ Client.Exec [có]             │                                 │
 │                                    │   method="codeintel.overview"[mới]├──JSON-RPC (khung 13B) [có]──▶│ dispatch [mới] → validate       │
 │                                    │                                   │                              │ spawn gitnexus/codegraph [có]──▶│ đọc .gitnexus / .codegraph
```

Các bước:
1. **UI** gửi invoke qua kênh mới `codeIntel.*` (cùng dialect `invoke/result/push` của wscompat). Tham số là khái niệm của Orca (`projectId`, `worktreeId`, `level`, `symbolId`), không có tên lệnh CLI.
2. **api-gateway** [mới] đăng ký `codeIntel.*` trong `wscompat` (mẫu: `channels_files.go`, `channels_git.go`): kiểm tra tenant/quyền, tra repo → dev server → tên repo GitNexus/đường dẫn CodeGraph.
3. **Bộ phối hợp** [mới]: đặt trong một service riêng (đề xuất `code-intel-service`, theo mẫu `git-gateway-service` gọi `infrafleetv1.InfraFleetServiceClient.RelayByDevServer`) hoặc mở rộng `mcp-service`. Nó lo cache, chuẩn hoá, và quyết định gọi nguồn nào. Nên đặt ngoài `infra-fleet-service` để service này giữ nguyên là lớp vận chuyển.
4. **infra-fleet-service** [có] chuyển method nguyên văn (`Relay*`), không cần sửa. Chỉ cần thêm hạn mức thời gian dài hơn cho `codeintel.*` trong `execTimeoutForMethod` nếu truy vấn chậm hơn 30 s.
5. **Agent** [mới] thêm nhóm `codeintel.*` (Part A, file `agent-rpc-dispatch-codeintel.ts` theo mẫu `-misc.ts`, `-git.ts`; thêm cả vào Part B nếu cần `relay-ssh`). Mỗi method kiểm tra tham số, dựng lệnh trong whitelist (xem 02 §4), chạy, **cắt/chuẩn hoá phần thô thành bản nhẹ** và trả về.
6. **MCP local**: GitNexus/CodeGraph đọc index trên đĩa.

Đường tạm thời không cần sửa agent: `Relay(method="tools/call", params={name:"gitnexus", arguments:{args:["cypher","-r","orca","…"], cwd}})`. Dùng được để thử nghiệm nhưng không đủ an toàn và dữ liệu là text (xem 02 §2).

## 2. Dữ liệu: MCP local → agent → backend → UI

```
MCP local ──raw (markdown/JSON/text/SQL rows)──▶ Agent
Agent: parse → cắt (limit/depth) → CodeIntelChunk (JSON gọn)
Agent ──JSON-RPC result (unary ≤ vài MiB) hoặc notifyBulk (lớn)──▶ infra-fleet-service
infra-fleet-service ──Relay result_json / RelayStream frames──▶ code-intel-service
code-intel-service: chuẩn hoá về schema chuẩn (05) → lưu cache (repo+commit+level) → proto
code-intel-service ──gRPC──▶ api-gateway ──WS result / push──▶ UI
UI: dựng đồ thị (React Flow / Cytoscape), drill-down gọi lại theo cấp
```

- **Một phát (unary)**: overview/impact/context — kết quả đã bị cắt, thường < 1 MiB. Đi `Relay`/`RelayByDevServer`.
- **Stream**: subgraph lớn hoặc reindex tiến trình. Dùng `RelayStream` (đã có cho `git.execStream`) hoặc thêm notification `codeintel.progress` và RPC streaming riêng (mẫu `StreamExecOutput`). Agent dùng `notifyBulk` để không chặn `pty.data`.
- **Cập nhật chủ động**: khi `meta.json`/commit đổi hoặc worktree thay đổi (`fs.changed` đã có), agent/backend phát `codeintel.changed {repo, commit}`; gateway đẩy `push` để UI hiện "index đã cũ/đã làm mới".

## 3. Method đề xuất trên agent (hợp đồng RPC)

Tất cả nhận `{repo, worktreePath?}` và trả `{source, commit, indexedAt, stale, …}`.

| Method | Mục đích | Nguồn chính | Tham số chính | Giới hạn |
|---|---|---|---|---|
| `codeintel.status` | index có sẵn không, mới đến đâu | `gitnexus list`+`.gitnexus/meta.json`, `codegraph status --json` | — | — |
| `codeintel.overview` | cụm (community) + cạnh giữa các cụm | GitNexus `cypher` | `topN` (mặc định 200) | ≤ 500 cụm |
| `codeintel.processes` | danh sách luồng thực thi | GitNexus `Process` | `limit`, `offset` | ≤ 100/trang |
| `codeintel.process` | các bước một luồng | GitNexus `STEP_IN_PROCESS` | `processId` | ≤ 200 bước |
| `codeintel.subgraph` | nút + cạnh quanh một cụm/tệp/symbol | GitNexus `cypher` / CodeGraph `callers/callees --json` | `center`, `depth≤3`, `kinds`, `limit` | ≤ 1 500 nút |
| `codeintel.impact` | blast radius | GitNexus `impact` | `target(uid)`, `direction`, `depth` | ≤ 300 nút |
| `codeintel.symbol` | chi tiết + mã nguồn | GitNexus `context --content` / CodeGraph `node` | `uid`/`name+file` | ≤ 200 KiB |
| `codeintel.routes` | API route → handler | GitNexus `Route`, `HANDLES_ROUTE` | `limit` | — |
| `codeintel.reindex` | làm mới (nền, tiến trình) | `gitnexus analyze` / `codegraph sync` | `mode` | một lần tại một thời điểm |

Quy ước chung: `uid` dùng id gốc của GitNexus (`Function:path:name`) hoặc id CodeGraph (`function:<hash>`); agent trả cả hai nếu khớp được (xem 05 §3). Lỗi dùng mã JSON-RPC hiện có (`AgentErrorCode`); thêm mã riêng cho `INDEX_MISSING`, `INDEX_STALE`, `TOOL_UNAVAILABLE`, `RESULT_TRUNCATED`.

## 4. Gateway/UI

Kênh `wscompat` đề xuất (đồng bộ tên với method agent): `codeIntel.status|overview|processes|process|subgraph|impact|symbol|routes|reindex`, và kênh push `codeIntel.changed`, `codeIntel.reindexProgress`. Tham số kèm `connectionId`/`devServerId` giống các kênh `fs.*`/`git.*`; không nhận lệnh CLI.

## 5. Quyền và nhật ký

- Quyền đọc code-intel tương đương quyền đọc worktree (cùng `tenant` + `project` membership). `reindex` cần quyền ghi.
- Mọi lần gọi ghi audit (đã có mô hình `channels_admin_audit.go`, và `mcp-service` có `AuthorizeToolCall`/`EvaluateToolCall`/`CompleteToolCall`): nếu muốn coi code-intel là tool MCP của Orca, gọi các hàm đó trước/sau để dùng chung chính sách và kill-switch.
- Không bao giờ trả mã nguồn của file bị `.gitignore`/ngoài worktree; `content` chỉ lấy khi UI mở symbol.
