# backend-go Solutions: code-intel-gateway (v7)

**CRs:** [docs/crs/v7/code-intel-gateway](../../../../../../docs/crs/v7/code-intel-gateway/README.md)
**Hợp đồng:** [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) (chính), [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md); khi CR và hợp đồng khác nhau theo hợp đồng.
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md)

> Proposed. Chưa triển khai, chưa chạy test nào. Đọc lại code liên quan trước khi sửa.

## Bảng CR, Solution, Task

| CR | Solution | Phạm vi | Task |
|---|---|---|---|
| [CR-CV-040](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md) | [BE-CV-SOL-040-codeintel-channel-foundation](./BE-CV-SOL-040-codeintel-channel-foundation.md) | wiring, `pathsafety`, giải mã chặt, lỗi, encoder, runner, catalog 46 kênh, `SetReadLimit`, parity | `040-01` đến `040-09` |
| | [BE-CV-SOL-040-codeintel-view-channels](./BE-CV-SOL-040-codeintel-view-channels.md) | 15 kênh đọc | `040-10` đến `040-15` |
| | [BE-CV-SOL-040-codeintel-write-and-stream-channels](./BE-CV-SOL-040-codeintel-write-and-stream-channels.md) | 10 kênh ghi/trạng thái + `subscribe` | `040-16` đến `040-22` |
| | [BE-CV-SOL-040-codeintel-quality-channels](./BE-CV-SOL-040-codeintel-quality-channels.md) | 20 kênh `quality.*` | `040-23` đến `040-30` |
| [CR-CV-041](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-041-mcp-codeintel-tools.md) | [BE-CV-SOL-041-mcp-codeintel-tools](./BE-CV-SOL-041-mcp-codeintel-tools.md) | 9 tool MCP chỉ-đọc (P2, tắt mặc định) | `041-01` đến `041-06` |

Bảng "kênh -> solution -> task" đủ 46 kênh: [tasks/README.md](../tasks/README.md).

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| Không có `channels_codeintel*.go` / client `codeintelv1` | Đúng; không có `proto/orca/codeintel` | Không |
| Dial theo mẫu MCP khi địa chỉ khác rỗng; thêm vào `OtherServiceAddrs` | `OtherServiceAddrs` bị dial hết, không điều kiện (`main.go:162-260`) | **Lệch**: `CodeIntelConfig` riêng (foundation C1) |
| Không `SetReadLimit` | Đúng (grep) | Không |
| `MaxCallRecvMsgSize` mặc định 4 MiB | `dial.go` không đặt; mặc định là kiến thức thư viện | Đặt tường minh |
| Stream: ack, lỗi quyền ở ack, 1 subscribe/kết nối | `handleSubscribe` ack ngay sau `sh()`; không có registry theo kết nối | **Bổ sung**: `Header()` handshake + registry (write-and-stream C1, C2) |
| Service chặn qua token nội bộ | proto-and-data-map §3 mở đầu; CR-040 không nêu | **Bổ sung**: gateway dial với `internalcaller` token |
| `PathFilter`/`KeepKeys` đủ cho tool MCP | `filterSensitiveItems` chỉ `obj["items"]`; `camelizeKeys` camel hoá khoá tự do | **Lệch** (SOL-041 M5, M7) |
| Thêm `pack1CodeIntel()` là xong | pack 1 bật mặc định, tool lộ ngay | **Bổ sung**: cổng triển khai (SOL-041 M6) |
| `NotFound|PermissionDenied` => `CODEINTEL_NOT_FOUND` | hợp đồng: `NOT_AUTHORIZED` | theo hợp đồng |

## Thứ tự thực thi và phụ thuộc

```
CR-010 + CR-020 (proto stub, G0) ─▶ SOL-040-foundation (G3: 46 kênh đã đăng ký)
      ├─▶ SOL-040-view-channels        (nối dần theo CR-012/020/021/031/033-038)
      ├─▶ SOL-040-write-and-stream     (CR-011/012/021/024/033/037/073)
      └─▶ SOL-040-quality-channels     (CR-082/083/085/086/089/090/092/093)
SOL-040-foundation + view-channels ─▶ SOL-041-mcp-codeintel-tools (chỉ khi O2 = Có)
```

Đợt (§7.3): foundation + kênh đọc đợt 2; `changeOverlay` đợt 3; `architecture`/`dataFlows` đợt 4; ghi + stream + `findings` đợt 5; 041 đợt 6; `quality.*` đợt 8–9. FE dùng kênh thật sau G3 (FE-CV-SOL-050).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Catalog 46 kênh là dữ liệu; một runner generic; placeholder `CODEINTEL_UNAVAILABLE` cho kênh chưa nối | G3, parity xanh từng PR |
| F2 | Gateway không giữ cờ, không kiểm quyền nghiệp vụ, không cache, không retry | một điểm thi hành ở service (G5, G2) |
| F3 | Danh tính chỉ từ `Identity`; 10 khoá cấm bị từ chối nêu tên trường; `DeviceID` bị từ chối trừ `settings.get` | U3, U8 |
| F4 | Lỗi `CODE: message[ \| {json}]` một dòng, không mã nguồn/đường dẫn/tham số; thông điệp ánh xạ là hằng | PQ-02, G6 |
| F5 | Encoder `protoreflect` do gateway sở hữu | hợp đồng thiếu quy tắc ra JSON (Q1 foundation) |
| F6 | `SetReadLimit(320 KiB)` toàn `/ws`; trần theo kênh dưới đó | PQ-14, O-2 |
| F7 | Thời gian: 8 s ghi/trạng thái, 20 s đọc, 24 s `quality.summary` | PQ-13, UI-API 3.2 |
| F8 | MCP: 9 tool đọc, `Untrusted`, `KeepKeys`, cổng triển khai tắt mặc định | O2/O-11, CR-041 |

## Điều còn mở

- **Quy tắc `proto -> JSON`** (enum, `int64`, `optional`, nullable): encoder gateway hay service trả JSON (foundation Q1). Không có trong hợp đồng.
- **O-2:** duyệt `SetReadLimit(320 KiB)` toàn `/ws`.
- **Token nội bộ gateway -> service:** tên biến và nơi đặt (foundation Q2); hợp đồng §6.2 không liệt kê.
- **`resync` toàn luồng:** `projectId/worktreeId` rỗng (write-and-stream Q1); khoá `payload_json` (Q3).
- **Kiểu proto** của `readingProgress/notes/turnMarkers`, `definition` của profile (JSON-in-proto hay message).
- **Trần không có trong hợp đồng:** `structure.limit`, độ dài `flowId`/`profile`/`turnKey`/`promptExcerpt` (giá trị tạm của solution, ghi rõ).
- **`quality.waive` revoke** có cần `reason`/`expiresAt` (quality Q1).
- **MCP:** `project_id` bắt buộc cho tool (hợp đồng §9 không nêu tham số); cổng `MCP_CODEINTEL_TOOLS_ENABLED` là đề xuất; chốt O2/O-11; kết luận taint với tool `write_reversible` (chưa đọc `mcp.rego`).
- Chưa kiểm chứng: giới hạn đọc mặc định của `coder/websocket` v1.8.15; hành vi `Header()` của grpc-go trong repo này.
