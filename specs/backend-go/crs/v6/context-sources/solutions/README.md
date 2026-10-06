# backend-go Solutions: Context Sources (v6)

**CRs:** [docs/crs/v6/context-sources](../../../../../../docs/crs/v6/context-sources/README.md)
**Hợp đồng chung:** [docs/crs/v6/README.md](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/06`](../../../../tdd/architecture/06-secrets-vault-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

> 📋 Proposed. Chưa triển khai, chưa chạy test nào. `request-service` chưa tồn tại trên đĩa (do series v6 tạo), nên mọi file của nó là "(mới)".

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-REQ-031](../../../../../../docs/crs/v6/context-sources/CR-REQ-031-source-registry-and-context-pack.md) | [BE-REQ-SOL-031](./BE-REQ-SOL-031-source-registry-and-context-pack.md) | `request-service`, `mcp-service`, `api-gateway`, `proto` | Large | `TASK-REQ-031-01` đến `-08` |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| Backend Orca chưa có `tools/call`, `resources/read` ra máy chủ ngoài | Đúng: `mcpprober.Prober` chỉ có `ListTools`; `session.call` đủ chung để mở rộng | Không |
| `mcp-service` chỉ Postgres | Đúng (Postgres-only, D2 của v5) | Không |
| `ToolInfo` không có chú thích chỉ-đọc | Đúng (`Name`, `Description`, `InputSchema`) | Không |
| Timeout RPC mới 20 giây | `TotalTimeout=15s` hiện cho probe | Thêm hằng `CallTimeout` riêng (SOL-031 C2) |
| `seq` cấp bằng CAS `version` | CAS tăng `version`, va chạm chuyển trạng thái | **Lệch** ⇒ khoá hàng `FOR UPDATE` (SOL-031 C1) |
| `secretscan` có sẵn | Chưa có (`secret_redactor.go` nằm trong `internal/` của `mcp-service`) | Phụ thuộc TASK-REQ-035-01 |
| `GetDevServerCapabilities` ở `infra-fleet-service` | Chưa có trong proto (CR-REQ-033 đề xuất) | Adapter đặt sau port, `missing` khi chưa có |
| `fs.readFile` đọc trong repo | Agent nhận **đường dẫn tuyệt đối**, không chống `..` | Backend phải chống (`SafeRelPath`, task 03) |
| Cột `key` của `context_sources` | `KEY` là từ khoá MySQL | Đặt tên `source_key` cả hai dialect |

## Thứ tự thực thi và phụ thuộc

```
BE-REQ-SOL-001, 002 ─▶ SOL-031 (01..05, 08)   đợt 1: nguồn nội bộ, Builder, Evidence, orca://
BE-REQ-SOL-035-01 (secretscan) ─▶ 031-02 (Redactor thật)
SOL-031 (06 mcp-service, 07 client)           đợt 2: MCP ngoài (06 độc lập, làm song song được)
CR 005, 007, 008, 012  ──gọi──▶ BuildContextPack (CR 005/007/008/012 phải sửa, xem mục 9 của CR)
```

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| S1 | Registry ở `request-service`, máy chủ MCP ở `mcp-service` (`server_ref`) | `mcp-service` chỉ Postgres |
| S2 | Nguồn nội bộ biên dịch trong mã, bảng chỉ ghi đè | Hoạt động khi bảng rỗng |
| S3 | Repo đọc qua Relay, đường dẫn tương đối, chống `..` ở backend | SSH, remote; agent không chống |
| S4 | Pack lắp trước khi gọi AI; xếp hạng bằng quy tắc `cp/1`, cắt xác định | Tái lập |
| S5 | `Evidence` giữ `excerpt` và `digest`, `seq` bằng khoá hàng | Không lưu toàn văn, tránh va chạm CAS |
| S6 | `trust=low` và mọi `mcp` bọc `<untrusted>`; v1 chỉ đọc | Chống chèn chỉ dẫn |
| S7 | Hai RPC mới ở `mcp-service` đặt trong `registryInternalMethods` | Chỉ `request-service` gọi |

## Điều còn mở

- Kênh `contextSource.*`, `context.get`, `evidence.get`, `source.*` cần ghi vào CR-REQ-016 và `CONTRACT-request-ui-api.md` (người điều phối).
- Q8 của CR (egress loại mảnh `low` và `mcp`) quyết ở BE-REQ-SOL-034 task 04.
- Chưa thử với máy chủ MCP thật nào; `code_graph` qua CLI chưa kiểm chứng.
