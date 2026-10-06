# 07 — Quyết định kiến trúc

Trạng thái: đề xuất (2026-10-05), chờ chốt. Bối cảnh: [01](./01-agent-backend-connection.md) (kết nối), [02](./02-local-mcp-interaction.md) (MCP local), [03](./03-command-and-data-flow.md) (luồng).

## D1. Dùng lại agent TypeScript, không viết agent Go mới

**Quyết định**: mở rộng `agent/src/relay/` bằng nhóm method `codeintel.*`; không tạo agent Go song song.

**Lý do**
- Agent TS đã có sẵn kết nối 3 mode, token, handshake, keepalive, reconnect, systemd, và quy trình triển khai/cập nhật trên mọi dev server. Agent mới phải làm lại tất cả và thêm một thứ để cấp token, triển khai, giám sát.
- Spec đã ghi agent có hai bề mặt RPC lệch nhau (Part A/B); thêm bề mặt thứ ba làm nặng thêm.
- Việc cần làm (chạy CLI, parse, cắt gọn) khớp mẫu `agent-rpc-dispatch-*.ts` và `runToolCommand` (`shell:false`) đã có. GitNexus là Node, CodeGraph có CLI/SQLite, Go không có lợi thế gì khi gọi chúng.

**Điều kiện xem xét lại**
- Cần giữ phiên MCP stdio dài hạn hoặc đọc SQLite ~1,2 GB thường xuyên: cân nhắc sidecar nhỏ riêng (vẫn không phải agent mới).
- Cần hỗ trợ `relay-ssh`: bổ sung handler vào `relay.ts` (Part B không có `tools/*`).

## D2. Agent chủ động kết nối ra backend (`direct-websocket`)

**Quyết định**: chỉ hỗ trợ `direct-websocket` cho tính năng này. Agent dial `wss://<gateway>/agent`; backend gửi lệnh `codeintel.*` xuống chính kết nối đó.

**Khả thi vì**: kết nối là JSON-RPC hai chiều trên cùng một WS; `git.*`, `fs.*`, `pty.*` đã chạy đúng cách này. Dev server không mở cổng vào.

**Điều kiện**
- Không dùng `relay-websocket` (backend gọi vào cổng 6799) và `relay-ssh` (backend khởi tạo) vì trái yêu cầu bảo mật.
- Dev server phải online. Khi rớt kết nối, backend xếp hàng gọi tối đa 20 s (`RECONNECT_WAIT_MS`); quá hạn trả lỗi "chưa kết nối" cho UI.
- Token một lần, tự gia hạn (`AgentTokenManager`); không cần xử lý thêm.

**Hệ quả bảo mật**: ranh giới tin cậy chuyển từ "ai vào được cổng" sang "backend bắt agent chạy gì". Do đó:
- Chỉ mở method hẹp `codeintel.*` với whitelist lệnh, không mở `args` tự do như tool `gitnexus`/`codegraph` hiện tại.
- Agent chỉ chạy trong workspace root đã đăng ký, có timeout và giới hạn kích thước.
- Cấm `analyze`, `clean`, `remove`, `publish`, `setup`; `reindex` chỉ qua method riêng có phân quyền.
- Cypher chỉ chạy mẫu có tham số, chỉ đọc.

## D3. Xử lý dữ liệu ở `code-intel-service` mới (Go)

| Việc | Nơi làm |
|---|---|
| Giữ kết nối WS, định tuyến lệnh xuống agent | `infra-fleet-service` (có sẵn, gần như không đổi) |
| Phân quyền tenant/project; ánh xạ project → dev server → repo | `code-intel-service` |
| Chuẩn hoá về schema chung ([05](./05-graph-schemas.md)), hợp nhất id GitNexus/CodeGraph | `code-intel-service` |
| Cache và lưu snapshot theo `(repo, commit, view)` | `code-intel-service` (DB riêng) |
| Chuyển kết quả cho UI, đẩy `push` khi index đổi | `api-gateway` (kênh `codeIntel.*` ở `wscompat`) |

**Lý do**: theo mẫu `git-gateway-service` gọi `InfraFleetService.RelayByDevServer`, `infra-fleet-service` giữ vai trò lớp vận chuyển. Cache, chuẩn hoá và quyền là nghiệp vụ riêng, không nhét vào đó.

**Phương án khác**: đặt trong `mcp-service` nếu muốn code-intel đi qua chính sách/kill-switch MCP của Orca. Cách nhẹ hơn: giữ service riêng và gọi `mcp-service` (`AuthorizeToolCall`/`CompleteToolCall`) khi cần dùng chính sách chung.

## D4. Mô hình dữ liệu: kéo theo yêu cầu + đẩy nhẹ từ agent

1. **Kéo (chính)**: UI mở view → `code-intel-service` kiểm tra cache → thiếu thì gọi `codeintel.*` qua `RelayByDevServer` → agent trả kết quả đã cắt gọn → chuẩn hoá và lưu cache.
2. **Đẩy nhẹ (phụ)**: khi `.gitnexus/meta.json` hoặc commit đổi, agent gửi notification `codeintel.indexChanged {repo, commit}`; backend huỷ cache và báo UI. Không đẩy cả graph (khung tối đa 16 MiB, tốn băng thông).

**Việc phải làm cho chiều đẩy**
- Chiều agent → backend hiện chỉ có consumer cho `pty.data`, `fs.changed`, `agent.hook`…. `devserveragent` cần consumer cho `codeintel.indexChanged` rồi chuyển tới `code-intel-service`, ví dụ RPC stream `StreamCodeIntelEvents` theo mẫu `StreamFileChanges`.
- Chưa kiểm tra repo có event bus dùng chung để thay cho RPC stream hay không.
- `execTimeoutForMethod` mặc định 30 s; truy vấn nặng cần ngoại lệ.

## D5. Phạm vi MVP

- Chỉ `direct-websocket` (Part A).
- Agent: `codeintel.status`, `codeintel.overview`, `codeintel.subgraph`.
- Backend: `code-intel-service` + proto + cache, kênh `codeIntel.*`.
- UI: view Architecture + panel symbol + hiển thị `indexedAt`/`stale`.
- Lộ trình đầy đủ: [06 §3](./06-gaps-risks-roadmap.md).

## Còn mở

- Ai làm mới index và khi nào (tay, hook sau `git pull`, hay lịch).
- Quy tắc ánh xạ project/worktree → tên repo GitNexus (registry có nhiều repo).
- Có đi qua chính sách MCP của Orca hay không (D3).
- Tên và vị trí event bus/stream cho `codeintel.indexChanged` (D4).
