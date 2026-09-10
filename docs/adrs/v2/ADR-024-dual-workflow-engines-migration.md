# ADR-024 — Hai workflow engine song song, không tương thích nhau, trong giai đoạn migration TS → Go

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-024 |
| **Trạng thái** | ⚠️ Ghi nhận hiện trạng (Accepted as a documented, intentional-for-now consequence — không phải "quyết định thiết kế lý tưởng", mà là hệ quả thật cần theo dõi) |
| **Ngày** | 2026-09-06 |
| **HLD Ref** | [docs/hld/backend-go-architecture.md §5](../../hld/backend-go-architecture.md#5-hai-workflow-engine-song-song-không-tương-thích-nhau) |
| **Code Ref** | `backend/src/main/workflow/`, `desktop/src/main/workflow/` (engine TS) · `backend-go/services/workflow-service/`, `backend-go/proto/orca/workflow/v1/workflow.proto:91-97` (engine Go) · `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go:23-29` (bằng chứng 2 con đường tách biệt) |
| **Amends** | [ADR-009](../v1/ADR-009-workflow-dag-orchestration.md) (mô tả engine TS) — ADR này KHÔNG thay thế ADR-009, mà ghi nhận engine Go tồn tại song song, chưa hợp nhất |
| **Không áp dụng cho** | Bất kỳ domain nào khác ngoài `workflow.*` — đây là tình huống cụ thể của domain workflow, không phải mô tả chung cho toàn bộ migration TS→Go (các domain khác có thể migrate sạch hơn, chưa audit riêng từng domain) |

---

## Bối cảnh

Theo chiến lược migration strangler-fig
(`specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md`), từng domain được cắt sang
backend-go độc lập, không big-bang. Với domain `workflow`, [ADR-022](./ADR-022-wscompat-protocol-bridge.md)
mô tả cơ chế wscompat cho phép Web-mode client gọi `workflow.*` RPC và được dịch sang gRPC
`workflow-service` (Go) — nhưng Electron/local-runtime client vẫn gọi thẳng vào TS dispatcher
(`backend/src/main/workflow/`), **không đi qua backend-go**.

Bằng chứng trực tiếp trong code (`channels_workflow.go:23-29`, comment gốc của CR-PW-005):

> *"CR-PW-005: this registers the remaining 7 of 11 WorkflowServiceClient RPCs that already existed in
> the proto/gRPC server but had no wscompat channel — Web-mode clients calling e.g.
> workflow.getExecution or workflow.template.list ... 404'd with 'unknown channel' until now, even
> though Electron/local mode already worked (it talks to the legacy TS workflow-rpc-handler directly)."*

Nghĩa là **2 engine cùng phục vụ cùng 1 tên RPC namespace (`workflow.*`) nhưng cho 2 nhóm client khác
nhau, với 2 model dữ liệu và 2 database khác nhau** — không phải giả thuyết, là hành vi runtime thật đã
được chính người viết code xác nhận trong doc comment.

## Hiện trạng (không phải "quyết định" theo nghĩa thiết kế chủ động — ghi lại vì đây là sự thật kiến trúc quan trọng, dễ gây ngạc nhiên nếu không tài liệu hoá)

| | Legacy TS engine (Electron/local) | Go `workflow-service` (Web-mode, qua wscompat) |
|---|---|---|
| Model dữ liệu | DAG/wave-based đầy đủ (`WorkflowOrchestrator`, `DAGBuilder`, step data) | Phẳng — `WorkflowExecution{id, template_id, status, root_trace_id, project_id}` (`workflow.proto:91-97`); bảng `workflow.step_executions` tồn tại ở DB nhưng **chưa expose qua gRPC response hiện tại** |
| Database | Postgres/SQLite chung của TS system (`orca_workflow_executions`) | Postgres riêng của `workflow-service` (database-per-service, [ADR-023](./ADR-023-postgres-per-service-vault-dynamic-credentials.md)) |
| Client nào gọi | Electron Desktop, local-runtime | Browser (Web-mode) qua `api-gateway`'s wscompat |
| Interop giữa 2 engine | ❌ Không có — 2 database tách biệt hoàn toàn, không đồng bộ 1 chiều hay 2 chiều |

**Hệ quả cụ thể, quan sát được:** một workflow execution tạo bằng Electron không xuất hiện khi cùng
project đó được mở qua trình duyệt (Web-mode), và ngược lại. Đây không phải bug đơn lẻ — là hệ quả cấu
trúc của việc 2 client nhóm đi 2 con đường hoàn toàn khác nhau cho cùng 1 namespace RPC.

## Vì sao vẫn được coi là "chấp nhận được, có ghi lại" thay vì "cần sửa ngay"

- Đúng tinh thần strangler-fig: từng domain cắt độc lập, có bounded window vận hành song song trước khi
  hoàn tất chuyển đổi — `specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md` mô tả rõ pha
  "dual-write period" cho các domain khác (`usage-service`, ...); domain `workflow` hiện đang ở giữa quá
  trình đó nhưng theo hướng "dual-READ tách biệt" (2 nguồn dữ liệu độc lập) chứ chưa có cơ chế dual-write
  hay backfill nào được xác nhận có trong code cho riêng domain workflow.
- 🚧 **Chưa xác nhận** trong phiên rà soát này có kế hoạch/CR nào đang chủ động đóng khoảng cách này
  (backfill dữ liệu từ TS sang Go, hoặc route Electron sang Go luôn) — không suy đoán có hay không, chỉ
  ghi nhận là chưa thấy bằng chứng trong code đọc được.

## Rủi ro cần theo dõi (flag cho người vận hành/PM, không phải khuyến nghị kỹ thuật cụ thể)

1. **Trải nghiệm người dùng không nhất quán** giữa Desktop và Web cho cùng 1 project — nếu người dùng
   chuyển qua lại 2 client, họ sẽ thấy lịch sử workflow khác nhau mà không có cảnh báo UI nào giải thích
   lý do (chưa xác nhận UI có warning gì cho tình huống này — 🚧 chưa audit).
2. **`workflow.step_executions` không expose qua gRPC** (Go engine) nghĩa là bất kỳ tính năng Web-mode
   nào cần hiển thị step/DAG chi tiết (ví dụ `ExecutionMonitor`/`DAGPreview` mô tả tại
   [web-server-architecture.md §10.6](../../hld/web-server-architecture.md)) **có khả năng không hoạt
   động đầy đủ khi chạy qua backend-go** — 🚧 chưa xác nhận trực tiếp hành vi UI thật khi trỏ vào
   Go engine, chỉ xác nhận proto response thiếu field này.
3. Domain khác cắt sang backend-go sau này (theo `00-service-catalog.md`'s cột "Migration phase") có
   nguy cơ lặp lại đúng pattern này nếu không chủ động thiết kế data-migration/backfill trước khi mở
   wscompat channel cho domain đó — khuyến nghị (không phải quyết định đã có) là mỗi domain migrate cần
   trả lời câu hỏi "dữ liệu cũ (TS) có cần nhìn thấy được từ engine mới (Go) không" trước khi bật channel
   Web-mode, học từ đúng gap này ở domain workflow.

## Trạng thái Implementation

⚠️ Đây không phải một tính năng "implement xong/chưa xong" — là một **tình trạng runtime hiện tại**, tồn
tại kể từ khi CR-PW-005 mở wscompat channel cho `workflow.*` mà chưa có cơ chế hợp nhất dữ liệu giữa 2
engine. Không có timeline khắc phục nào được xác nhận trong phiên rà soát này.

## Cross-References

| Resource | Mô tả |
|---|---|
| [docs/hld/backend-go-architecture.md §5](../../hld/backend-go-architecture.md) | Mô tả đầy đủ, bảng so sánh 2 engine |
| [ADR-022](./ADR-022-wscompat-protocol-bridge.md) | Cơ chế wscompat khiến 2 con đường này cùng tồn tại dưới 1 tên namespace |
| [ADR-009 (v1)](../v1/ADR-009-workflow-dag-orchestration.md) | Engine TS gốc — vẫn là "system of record" cho Electron/local |
| [docs/hld/web-server-architecture.md §10.6](../../hld/web-server-architecture.md) | Workflow Builder UI (F36) — client tiêu thụ 1 trong 2 engine tuỳ runtime mode |
