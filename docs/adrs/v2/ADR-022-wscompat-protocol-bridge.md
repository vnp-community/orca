# ADR-022 — `wscompat`: lớp dịch giao thức WS-RPC (browser/Electron) ↔ gRPC nội bộ (backend-go)

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-022 |
| **Trạng thái** | ✅ Accepted — implemented (mô tả code đang chạy thật, không phải đề xuất) |
| **Ngày** | 2026-09-06 |
| **HLD Ref** | [docs/hld/backend-go-architecture.md §4](../../hld/backend-go-architecture.md#4-internal-contract-grpc--api-gateways-wscompat-bridge-tới-browserelectron) |
| **Code Ref** | `backend-go/services/api-gateway/internal/adapter/wscompat/*.go` (~25 file `channels_*.go`), ví dụ `channels_workflow.go:1-46` |
| **Amends** | — (chưa từng có ADR nào mô tả cơ chế này trước đây) |
| **Không áp dụng cho** | Giao tiếp nội bộ service↔service trong `backend-go/` (thuần gRPC, không qua wscompat) |

---

## Bối cảnh

Frontend web/Electron hiện có (mô tả tại
[web-server-architecture.md §5.1](../../hld/web-server-architecture.md#51-websocketrpcclient)) đã có sẵn
một giao thức RPC riêng qua WebSocket: envelope JSON kiểu `ipcRenderer`
(`{ id, type: 'invoke', channel, args }` → `{ id, type: 'result', result }`), **không phải** JSON-RPC 2.0
và **không phải** gRPC/gRPC-Web. `IRpcClient` (`invoke(channel, ...args)` / `on(channel, handler)`) là
interface chung cho cả `web-preload-api.ts` (Web) và Electron's `ipcRenderer` preload (Desktop) — đổi
transport ở đây đồng nghĩa đổi cả 2 client.

`backend-go`'s internal contract lại là gRPC thuần, namespace theo `orca.<service>.v1`
(`specs/backend-go/tdd/architecture/08-inter-service-communication.md` §"gRPC conventions"). Cùng tài
liệu đó, ở mục "API Gateway responsibilities", còn mô tả một **target khác** cho REST edge: route REST
request qua `grpc-gateway` sinh tự động từ chính file `.proto`. Đây là kiến trúc **mục tiêu**, không phải
những gì `api-gateway` thật sự implement hôm nay cho browser/Electron.

## Quyết định

`api-gateway` implement `internal/adapter/wscompat/` — mỗi file `channels_<domain>.go` đăng ký 1 tập
handler cho đúng tên `channel` mà frontend hiện đang gọi (giữ nguyên 1:1 tên RPC namespace cũ, ví dụ
`workflow.execute`, `workflow.getExecution`, `git.status`, `tenant.project.*` — xem danh sách file trong
`wscompat/`). Mỗi handler:

1. Decode `args` (JSON) từ đúng envelope WS mà `WebSocketRpcClient`/`web-preload-api.ts` gửi.
2. Gắn tenant/user identity vào gRPC metadata (`gatewaygrpc.AttachIdentity`).
3. Gọi đúng `<Service>ServiceClient` method tương ứng qua gRPC nội bộ.
4. Encode response về lại đúng shape `{ id, type: 'result', result }` mà client cũ đã biết cách parse.

Nói cách khác: **`api-gateway` giữ nguyên giao thức ngoài (WS channel/invoke) làm hợp đồng duy nhất với
browser/Electron, và coi wscompat là lớp dịch nội bộ, một chiều, không phải một API thứ hai cần duy trì
song song "cho đúng" với gRPC-Web thật.** Frontend không cần biết — và không cần đổi bất kỳ dòng code
nào — để một domain chuyển từ được TS `backend/` xử lý sang được backend-go xử lý.

## Lý do (suy luận hợp lý từ code, không phải trích dẫn quyết định đã ghi lại từ team — xem cảnh báo)

| Lựa chọn | Đánh giá |
|---|---|
| **wscompat (đã chọn)** ✅ | Zero thay đổi ở frontend cho từng domain cắt sang backend-go — đúng tinh thần strangler-fig (`specs/backend-go/crs/v0/migration/ts-to-go-migration-strategy.md`); mỗi `channels_<domain>.go` là 1 đơn vị cutover độc lập, review/rollback theo từng domain |
| `grpc-gateway` REST (target trong `08-inter-service-communication.md`) | Đòi hỏi frontend đổi hẳn transport (WS persistent connection + push-channel hiện tại → REST/HTTP polling hoặc thêm 1 kênh WS riêng cho push) — thay đổi lớn, đồng bộ 2 phía, rủi ro cao hơn trong giai đoạn migrate dần |
| gRPC-Web/Connect trực tiếp từ browser | Cùng vấn đề trên, cộng thêm phải giữ tương thích ngược cho Electron (không chạy trong trình duyệt, không có gRPC-Web transport sẵn) |

> ⚠️ **Đây là suy luận dựa trên code + tài liệu migration, không phải trích dẫn 1 quyết định đã ghi lại
> rõ ràng từ team backend-go.** Không có ADR/CR nào trong `specs/backend-go/` giải thích trực tiếp "tại
> sao chọn wscompat thay vì grpc-gateway" — ADR này lấp khoảng trống đó ở tầng cross-cutting (HLD/ADR),
> dựa trên bằng chứng quan sát được (code thật chọn wscompat, tài liệu target vẫn nói grpc-gateway).

## Hệ quả

- ✅ Cutover từng domain độc lập, không cần đổi frontend — xác nhận qua CR-PW-005 (`channels_workflow.go`
  comment): backfill 7 RPC method còn thiếu wiring mà không đụng gì tới frontend.
- ⚠️ **Chi phí bảo trì kép**: mapping channel↔gRPC-method là code viết tay, không sinh tự động từ
  `.proto` — mỗi RPC method mới ở gRPC cần 1 lượt wiring wscompat riêng, và có thể "quên" (đúng như gap
  đã xảy ra thật với 7 method workflow.* trước CR-PW-005, khiến Web-mode 404 "unknown channel" trong khi
  gRPC server đã có method đó từ trước).
- ⚠️ **JSON shape không đảm bảo khớp `protojson`**: các handler trả raw proto message qua
  `encoding/json` thuần (struct tag `snake_case` của protoc-gen-go), không phải `protojson`
  (`camelCase`). `channels_workflow.go:31-45` tự ghi nhận đây là vấn đề thật đang tồn tại ở các channel
  cũ, **cố ý chưa sửa** vì ngoài phạm vi CR đó; `channels_tenant_project.go` và
  `channels_dev_server_access_control.go` đã tự sửa bằng cách viết 1 view struct camelCase riêng cho
  response — nghĩa là 2 pattern (raw proto vs. view struct) **cùng tồn tại song song** trong wscompat
  hôm nay, chưa thống nhất.
- ✅ Không có thay đổi nào ở giao thức nội bộ service↔service (vẫn gRPC thuần) — wscompat chỉ tồn tại ở
  biên `api-gateway`.

## Trạng thái Implementation

✅ Đã implement và đang chạy — ~25 file `channels_*.go` trong
`backend-go/services/api-gateway/internal/adapter/wscompat/`, bao phủ workflow, git, tenant/project,
credentials, browser, admin users, SCM, Jira/Linear, orchestration, automation/task, và nhiều domain
khác. **Chưa audit đầy đủ** liệu 100% RPC namespace mà frontend gọi đã có channel wscompat tương ứng hay
còn domain nào vẫn 404 như workflow.* từng bị trước CR-PW-005 — không khẳng định độ phủ đầy đủ.

## Cross-References

| Resource | Mô tả |
|---|---|
| [docs/hld/backend-go-architecture.md](../../hld/backend-go-architecture.md) | Bối cảnh đầy đủ backend-go |
| [docs/hld/web-server-architecture.md §5.1](../../hld/web-server-architecture.md) | Giao thức WS-RPC phía client, thứ wscompat phải tương thích |
| [ADR-024](./ADR-024-dual-workflow-engines-migration.md) | Hệ quả cụ thể của wscompat cho domain workflow — 2 engine song song |
| `specs/backend-go/tdd/architecture/08-inter-service-communication.md` | Target design (grpc-gateway) — khác với implementation thật (wscompat) |
