# 02 — Cách agent tương tác với GitNexus / CodeGraph (MCP local)

Đo trên máy này (`/opt/repos/orca`): gitnexus 1.6.9, codegraph 1.4.1.

## 1. Hai công cụ lưu dữ liệu ở đâu

| | GitNexus | CodeGraph |
|---|---|---|
| Thư mục | `<repo>/.gitnexus/` + **registry toàn cục** (đang có 12 repo: orca, vnp-*, llm-proxy…) | `<repo>/.codegraph/` |
| Lưu trữ | LadybugDB (`lbug`), graph + FTS; `meta.json` | SQLite `codegraph.db` (WAL, ~1.2 GB cho Orca) |
| Chọn repo | **Phải truyền `-r <name>`** khi registry có >1 repo; chạy trong thư mục repo không tự chọn (đã thử: lỗi "Multiple repositories indexed") | Theo `-p <path>` hoặc cwd |
| Giao diện | CLI, MCP stdio, `mcp --http` (bearer token), `serve` (HTTP 127.0.0.1:4747) | CLI, MCP stdio, daemon (`daemon.sock`) |
| Mỗi lần gọi CLI | ~1,8 s lạnh (mở DB) | nhanh hơn, vẫn spawn process |

## 2. Agent hiện đang làm gì

`agent/src/relay/agent-tool-registry.ts` đã khai báo hai tool:
- `gitnexus` — `args: string[]`, `cwd?`; chạy `runToolCommand('gitnexus', args, {cwd, timeout 60s, env: toolEnv})`.
- `codegraph` — tương tự.

`runToolCommand` dùng `spawn(..., shell:false)` (không có shell injection), gom stdout/stderr vào bộ nhớ, trả `{stdout, stderr, exitCode}`. Qua `tools/call` kết quả bọc thành MCP: `{content:[{type:'text', text}], isError, exitCode}`.

Giới hạn của hiện trạng (cần xử lý trước khi dùng cho UI):
1. **Chỉ là CLI, không phải MCP thật**: không có phiên MCP, không có `cypher`-level whitelist; `args` là mảng tuỳ ý (chạy được cả `gitnexus analyze`, `clean`, `remove`!).
2. Không truyền `-r`; `cwd` không đủ để chọn repo GitNexus khi registry nhiều repo.
3. Đầu ra là **text** (xem 03/04): `cypher` trả markdown, `codegraph explore/node` trả mã nguồn có số dòng; chỉ một phần lệnh có JSON.
4. Timeout cứng 60 s, không giới hạn kích thước stdout (trong khi khung RPC tối đa 16 MiB).
5. Backend Go chưa gọi `tools/call`; Part B (`relay-ssh`) không có tool registry.

## 3. Bốn cách tương tác (so sánh)

| Cách | Mô tả | Ưu | Nhược |
|---|---|---|---|
| **A. CLI qua agent (mở rộng cái đang có)** | Handler mới `codeintel.*` trên agent spawn `gitnexus … ` / `codegraph … --json` | Đơn giản, không daemon, không mở cổng; tái dùng `runToolCommand` | ~1–2 s/lần; phải parse markdown của `cypher` |
| **B. MCP stdio do agent giữ phiên** | Agent spawn `gitnexus mcp` / `codegraph` MCP một lần, nói JSON-RPC MCP (`tools/call`) | Dữ liệu có cấu trúc (`cypher` trả đúng object), không trả phí khởi động mỗi lần | Cần MCP client trong agent, quản lý vòng đời process |
| **C. Đọc thẳng kho dữ liệu** | CodeGraph: SQLite read-only (bảng `nodes/edges/files`); GitNexus: không nên đọc `lbug` trực tiếp | CodeGraph nhanh, schema ổn định (xem 05) | Phụ thuộc schema nội bộ (có `schema_versions`); cần driver SQLite trên agent; WAL đang ghi |
| **D. HTTP loopback** | `gitnexus mcp --http` hoặc `serve` trên 127.0.0.1, agent proxy | Một daemon dùng chung | Thêm process/cổng; bắt buộc `--auth-token` nếu không loopback |

**Khuyến nghị**
- MVP: **A cho cả hai**, ưu tiên các lệnh có JSON (`codegraph … --json`, `gitnexus context/impact`), cộng `gitnexus cypher` với truy vấn dựng sẵn.
- Giai đoạn sau: chuyển GitNexus sang **B** (stdio MCP, một phiên cho mỗi dev server) để bỏ chi phí khởi động và parse markdown; CodeGraph có thể chuyển sang **C** (read-only) cho truy vấn nặng (liệt kê edge theo file).
- Không dùng D trừ khi cần chia sẻ phiên giữa nhiều agent.

## 4. Quy tắc an toàn khi agent chạy công cụ

- Thay `tools/call name=gitnexus args=[...]` tuỳ ý bằng các **method hẹp** (`codeintel.overview`, `codeintel.subgraph`…); agent tự dựng câu lệnh, backend/UI không gửi `args`.
- Whitelist lệnh: chỉ `query, context, impact, trace, cypher (chỉ-đọc), list, status` cho GitNexus; `query, callers, callees, impact, files, status, node` cho CodeGraph. Cấm `analyze/clean/remove/uninstall/publish/setup`.
- Với `cypher`: chỉ cho truy vấn dựng sẵn từ mẫu có tham số, từ chối `CREATE|MERGE|DELETE|SET|CALL` ghi.
- `repo` không nhận đường dẫn tự do: ánh xạ từ `connection/worktree` → tên repo GitNexus (`gitnexus list`) hoặc đường dẫn nằm dưới workspace root đã đăng ký. Lưu ý (kiểm tra 2026-10-05): agent hiện **không** có cơ chế chặn đường dẫn dùng chung cho thao tác đọc — chỉ `fs.writeFile` có kiểm tra "SecureFs" (`agent/src/relay/fs-agent-write-extensions.ts:43`) — nên việc giới hạn đường dẫn phải do `codeintel.*` và backend tự làm, không thể dựa vào `fs.*`.
- Đặt giới hạn: timeout, số dòng/nút, kích thước stdout (vd. 8 MiB, dưới trần 16 MiB).
- Làm mới chỉ khi người dùng có quyền và qua method riêng (`codeintel.reindex`) chạy nền với tiến trình; chạy `gitnexus analyze` Orca mất nhiều phút.
