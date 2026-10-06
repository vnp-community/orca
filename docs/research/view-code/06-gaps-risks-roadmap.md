# 06 — Khoảng trống, rủi ro và lộ trình

## 1. Khoảng trống hiện tại (đã xác minh)

| # | Khoảng trống | Bằng chứng |
|---|---|---|
| G1 | Backend Go chưa gọi `tools/call` của agent | grep `tools/call` trong `backend-go`: chỉ `mcpprober`, `mcpserver` (MCP server của Orca) |
| G2 | Tool `gitnexus`/`codegraph` của agent là CLI thô, `args` tuỳ ý, không `-r` | `agent-tool-registry.ts` (đoạn `gitnexus`, `codegraph`) |
| G3 | GitNexus có 12 repo trong registry; thiếu `-r` thì lỗi "Multiple repositories indexed" | chạy `gitnexus cypher` không `-r` |
| G4 | `cypher` trả markdown chứ không phải rows JSON | `{ "markdown":…, "row_count":… }` |
| G5 | Part B (`relay-ssh`) không có `tools/*` | `relay.ts` không tham chiếu registry |
| G6 | Timeout mặc định 30 s ở Go, 60 s ở tool agent; stdout không giới hạn | `execTimeoutForMethod`, `runToolCommand` |
| G7 | Chưa có kênh `codeIntel.*` ở `wscompat`, chưa có proto/service cho code-intel | liệt kê `wscompat/channels_*.go` |
| G8 | Index có thể cũ so với HEAD (đang là việc thủ công) | `.gitnexus/meta.json` có `lastCommit`/`indexedAt`; CLAUDE.md nhắc "Index stale" |

## 2. Rủi ro và cách giảm

- **Bảo mật / thực thi lệnh**: cho phép UI gửi `args` là đưa RCE-theo-CLI (`analyze`, `clean`, `remove`). → method hẹp + whitelist + `shell:false` (đã có) + kiểm tra đường dẫn (xem 02 §4).
- **Rò rỉ mã nguồn**: `content`/`explore` trả mã. → quyền theo project, chỉ lấy theo symbol, tôn trọng `.gitignore`, ghi audit.
- **Kích thước**: Orca 247k/644k (GitNexus) và 296k/958k (CodeGraph) → không bao giờ trả cả graph (04 §2).
- **Chi phí khởi động CLI** (~1,8 s): gom nhiều truy vấn vào một lần gọi, cache ở backend, về sau dùng phiên MCP stdio.
- **Trôi lệch schema công cụ**: GitNexus `schemaVersion: 5`, CodeGraph `schema_versions`/`extractionVersion`. → agent kiểm tra phiên bản, ghi `toolVersion` vào mọi kết quả; test hợp đồng với mẫu đã lưu.
- **Đa transport**: Part A vs Part B khác nhau → chỉ hứa hỗ trợ `direct/relay-websocket` ở MVP; `relay-ssh` làm sau (hoặc dùng `gitnexus` qua SSH exec riêng).
- **Đa nền tảng / SSH** (AGENTS.md): đường dẫn, WSL, SSH — chuẩn hoá path ở backend, không nhúng giả định `/`.
- **Xung đột đọc/ghi**: CodeGraph ghi WAL khi `sync`; đọc read-only an toàn nhưng có thể thấy trạng thái trung gian. GitNexus `analyze` có thể khoá DB (đã thấy `lbug.wal.missing-shadow.*`) → `codeintel.reindex` nối tiếp, từ chối khi đang chạy.
- **Đa người dùng**: nhiều người cùng xem một dev server → cache theo `(repo, commit)` dùng chung trong tenant.

## 3. Lộ trình đề xuất

**Giai đoạn 0 — Xác minh (0,5–1 ngày, không đổi code sản phẩm)**
- Gọi thử qua `Relay(method="tools/call", name="gitnexus", args=["-r","<repo>", …])` trên một dev server thật để xác nhận đường end-to-end và đo độ trễ/kích thước.
- Chốt cú pháp Cypher cho Architecture/Flow bằng `gitnexus cypher -r orca …`; lưu mẫu output làm fixture.

**Giai đoạn 1 — Chỉ đọc, một view (MVP)**
- Agent: `codeintel.status`, `codeintel.overview`, `codeintel.subgraph` (Part A).
- Backend: `code-intel-service` (hoặc module trong mcp-service) + proto + cache; gateway `codeIntel.*`.
- UI: view Architecture + panel symbol + hiển thị `indexedAt/stale`.

**Giai đoạn 2 — Luồng và tác động**
- `codeintel.processes/process/impact/symbol/routes`; view Flow, Impact, Route.
- Hợp nhất id GitNexus↔CodeGraph.

**Giai đoạn 3 — Kiểm soát thay đổi**
- `codeintel.changeOverlay` (detect-changes + diff worktree); phủ lên mọi view.
- Push `codeIntel.changed`; làm mới index theo yêu cầu, có tiến trình và quyền.

**Giai đoạn 4 — Tối ưu**
- Phiên MCP stdio dùng lại (GitNexus), đọc SQLite read-only (CodeGraph) cho truy vấn nặng.
- Hỗ trợ `relay-ssh`, đa repo, so sánh theo commit.

## 4. Câu hỏi còn mở (cần chốt trước Giai đoạn 1)

1. ~~Đặt bộ phối hợp ở đâu?~~ Đề xuất đã nêu ở [07 D3](./07-architecture-decisions.md): service mới `code-intel-service` gọi `RelayByDevServer`; `mcp-service` chỉ dùng cho chính sách nếu cần. Chờ chốt.
2. Ai làm mới index và khi nào: người dùng bấm tay, hook sau `git pull`, hay lịch định kỳ?
3. Phạm vi repo: một Orca project ↔ một repo GitNexus? (Registry có 12 repo; cần quy tắc ánh xạ project/worktree → tên repo.)
4. Có cần coi code-intel là tool MCP của Orca (qua `AuthorizeToolCall`) để dùng chung chính sách/kill-switch không?
5. ~~Giới hạn mode cho MVP?~~ Đề xuất đã nêu ở [07 D2/D5](./07-architecture-decisions.md): chỉ `direct-websocket` (agent chủ động kết nối ra backend, lý do bảo mật). Chờ chốt.

## 5. Việc nên kiểm chứng khi bắt tay làm

- Cú pháp Cypher cụ thể trên LadybugDB cho phép gộp cạnh theo cụm (04 §2) — viết mẫu, đo thời gian trên Orca.
- `gitnexus query/trace` có `--json` hay chỉ in text; `communities` 10 252 vs 9 039.
- `codegraph impact/callees --json` shape (chỉ mới xác nhận `callers --json`, `query --json`, `status --json`).
- Mã lỗi/timeout của Go `Exec` với kết quả lớn (> 1 MiB).
