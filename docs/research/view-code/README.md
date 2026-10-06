# Vẽ dữ liệu GitNexus / CodeGraph từ dev server lên giao diện Orca

> Trạng thái: nghiên cứu / đề xuất (2026-10-05). Phần "hiện trạng" đã đối chiếu với `specs/agent/api/*`, code `agent/src/relay/*`, `backend-go/services/infra-fleet-service`, và chạy thật CLI `gitnexus 1.6.9` / `codegraph 1.4.1` trên repo Orca. Phần "đề xuất" chưa có code.

## Bài toán

Dev server có code và đã cài GitNexus, CodeGraph (MCP local). Cần lấy dữ liệu từ hai hệ thống này và vẽ thành hình trên giao diện Orca để kiểm soát code.

## Mục lục

| File | Nội dung |
|---|---|
| [01-agent-backend-connection.md](./01-agent-backend-connection.md) | Agent chạy trên dev server và nối tới backend thế nào (3 mode, handshake, wire, đường `Relay`) |
| [02-local-mcp-interaction.md](./02-local-mcp-interaction.md) | Agent tương tác GitNexus/CodeGraph ra sao; 4 phương án; quy tắc an toàn |
| [03-command-and-data-flow.md](./03-command-and-data-flow.md) | Luồng lệnh UI→backend→agent→MCP và luồng dữ liệu ngược lại; hợp đồng `codeintel.*` |
| [04-raw-data-and-pipeline.md](./04-raw-data-and-pipeline.md) | Dữ liệu thô (đo thật) và pipeline 5 tầng |
| [05-graph-schemas.md](./05-graph-schemas.md) | Schema graph chuẩn cho UI, hợp nhất id, ánh xạ kind |
| [06-gaps-risks-roadmap.md](./06-gaps-risks-roadmap.md) | Khoảng trống, rủi ro, lộ trình, câu hỏi mở |
| [07-architecture-decisions.md](./07-architecture-decisions.md) | Quyết định kiến trúc: dùng lại agent TS, agent chủ động kết nối, `code-intel-service` mới, mô hình kéo + đẩy nhẹ |
| [08-views-and-review-models.md](./08-views-and-review-models.md) | 5 view (cấu trúc code, C4 level 3, luồng dữ liệu, ERD, lưu trữ) và các view review nhanh R1–R8 |
| [09-external-inputs-required.md](./09-external-inputs-required.md) | Dữ liệu cần từ nguồn khác ngoài GitNexus/CodeGraph (E1–E17) để bạn bổ sung |
| [10-frontend-review-ux.md](./10-frontend-review-ux.md) | Đề xuất giao diện review sau khi agent code: điểm vào, bố cục, các lens, tính năng review nhanh, kỹ thuật frontend |
| [11-additions-for-quality-control.md](./11-additions-for-quality-control.md) | Cần bổ sung gì để kiểm soát chất lượng sau khi sinh: index bắt kịp, tín hiệu chất lượng, cổng chất lượng, nền đồ hoạ; CR đề xuất thêm |

## Tóm tắt kết luận

1. **Đường vận chuyển đã có sẵn**: UI →(WS `wscompat`)→ api-gateway →(gRPC `Relay/RelayByDevServer`)→ infra-fleet-service →(JSON-RPC)→ agent. `Relay` chuyển method nguyên văn, nên không cần đổi infra-fleet-service.
2. **Agent đã có tool `gitnexus` và `codegraph`**, nhưng chỉ là CLI thô qua `tools/call` (chỉ Part A, `args` tuỳ ý, không `-r`, backend Go chưa gọi). Dùng thử được nhưng không đủ an toàn và đầu ra là text.
3. **Nguồn dữ liệu**: GitNexus là nguồn chính (cluster, process, route, impact); `cypher` trả markdown nên phải parse. CodeGraph bổ trợ (`--json` cho `query/callers/status`; SQLite read-only cho truy vấn nặng).
4. **Thiết kế đề xuất**: agent thêm nhóm `codeintel.*` (method hẹp, whitelist, có giới hạn) → backend chuẩn hoá về một schema chuẩn + cache theo `(repo, commit)` → gateway kênh `codeIntel.*` → UI chỉ vẽ schema chuẩn.
5. **Không bao giờ vẽ cả graph** (Orca: 247k nút/644k cạnh): drill-down theo cấp cluster → module → symbol → call chain.
6. **Kiểm soát code** = Architecture, Module, Flow, Impact, Route, ChangeOverlay, IndexStatus (05).

## Việc cần chốt trước khi làm

Xem [06 §4](./06-gaps-risks-roadmap.md): vị trí bộ phối hợp (service mới hay `mcp-service`), cơ chế làm mới index, ánh xạ project/worktree → repo GitNexus, có đi qua chính sách MCP của Orca hay không, giới hạn mode cho MVP.
