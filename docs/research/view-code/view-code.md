# Vẽ dữ liệu GitNexus / CodeGraph từ dev server lên giao diện Orca

> Trạng thái: nghiên cứu / đề xuất. Chưa đọc sâu code Orca để xác nhận; các tên trong repo (`McpEvent`, `ListDevServers`, `wscompat/channels_mcp_events.go`) lấy từ gợi ý của CodeGraph và cần kiểm chứng trước khi thiết kế chi tiết.

## Bài toán

Trên các dev server có code và đã cài GitNexus và CodeGraph (chạy dạng MCP local). Muốn lấy dữ liệu từ hai hệ thống này và vẽ lại thành hình trên giao diện Orca.

## Nguyên tắc

Frontend **không gọi thẳng MCP trên dev server**. Thêm một lớp collector chuẩn hoá dữ liệu về một graph schema chung, UI chỉ vẽ schema đó.

## 1. Chọn nguồn dữ liệu

| Nguồn | Dùng cho | Ghi chú |
|---|---|---|
| GitNexus `cypher`, `list_repos`, resource `clusters` và `processes` | Nguồn chính, trả về dữ liệu có cấu trúc (node, edge, cluster, execution flow) | Tạo được 3 loại hình: kiến trúc theo cluster, luồng thực thi, call graph/impact của một symbol |
| GitNexus `impact`, `context`, `route_map`, `api_impact` | Hình theo yêu cầu: blast radius, API route → handler | Gọi khi người dùng bấm vào một node |
| CodeGraph | Nguồn bổ trợ | `codegraph_explore` chỉ trả text/source, không có JSON, nên không parse để vẽ. Nếu cần edge của CodeGraph, collector đọc trực tiếp SQLite `.codegraph/` ở chế độ read-only |

## 2. Kiến trúc

```
Dev server                         Orca backend                    Orca UI
┌──────────────────────┐   ┌────────────────────────────┐   ┌───────────────────┐
│ gitnexus MCP (stdio) │   │ code-graph collector        │   │ Graph view        │
│ codegraph MCP/SQLite │──▶│ - gọi tool qua kênh dev     │──▶│ React Flow /      │
│ (qua SSH/relay)      │   │   server hiện có            │   │ Cytoscape / Sigma │
└──────────────────────┘   │ - chuẩn hoá → {nodes,edges, │   │ drill-down        │
                           │   clusters, commit, at}     │   └───────────────────┘
                           │ - cache theo repo+commit    │
                           │ - api-gateway REST/WS       │
                           └────────────────────────────┘
```

1. **Kết nối**: dùng đúng đường Orca đã dùng để nói chuyện với dev server (SSH/relay, `infra-fleet-service`, mcp service). Collector khởi chạy hoặc tái dùng MCP stdio của GitNexus trên server đó. Theo AGENTS.md phải tính cả trường hợp SSH, không giả định chạy local.
2. **Chuẩn hoá**: schema trung lập, ví dụ `GraphNode{id, kind, name, file, line, cluster}`, `GraphEdge{from, to, type}`, kèm `repo`, `commitSha`, `indexedAt`. Khai báo bằng proto để khớp các service khác.
3. **Cache**: theo `repo + commit`. Index GitNexus có thể cũ hơn code, nên UI hiển thị `indexedAt` và có nút "re-analyze".
4. **API**: api-gateway cung cấp endpoint kiểu `GET /code-graph?project=&level=clusters|flows|symbol&id=`, có thể đẩy cập nhật qua WS tương tự `channels_mcp_events`.
5. **Vẽ**: React Flow (node-edge có layout), hoặc Cytoscape.js / Sigma.js nếu graph lớn. Luồng thực thi dùng Mermaid sequence/flow là đủ.

## 3. Điểm cần chú ý

- **Không vẽ toàn bộ graph**: repo Orca có ~250k symbol và ~640k quan hệ, vẽ hết sẽ treo UI. Drill-down theo cấp: cluster → file/module → symbol → call chain (depth 2–3). Collector chịu trách nhiệm cắt và giới hạn.
- **Bảo mật**: chỉ expose tool đọc (cypher chỉ-đọc, có whitelist), không đưa nguyên MCP ra frontend. Cần xác thực và phân quyền theo project.
- **Tương thích Windows/WSL/SSH**: đường dẫn file trong graph phải map về worktree mà Orca đang mở.
- **Giao diện**: theo `docs/STYLEGUIDE.md` và token trong `main.css`.

## 4. Lộ trình gợi ý

1. MVP: view "Architecture" vẽ cluster và quan hệ giữa chúng (GitNexus `clusters` + `cypher`).
2. Thêm view "Flow" (execution process) và "Impact" (khi chọn symbol).
3. Sau cùng: đồng bộ real-time và so sánh theo commit.

## Câu hỏi còn mở

- Loại hình ưu tiên làm trước: kiến trúc tổng thể, call graph/impact, hay luồng thực thi?
- Cách mcp service và `infra-fleet-service` hiện gọi tool trên dev server (cần đọc code để chốt proto, service, component UI).
- Collector đặt ở service mới hay mở rộng mcp service hiện có?
