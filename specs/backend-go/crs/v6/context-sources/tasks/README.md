# context-sources: tasks (backend-go, v6)

> 🚧 1/8 task xong: 031-06 `[x] DONE` (2026-10-07). Các task khác `[ ] TODO`. Phần `request-service` dựa trên SOL-001, 002 (service chưa có); phần `mcp-service` và `api-gateway` đã đối chiếu với file thật.

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc | Trạng thái |
|---|---|---|---|---|---|
| [BE-REQ-SOL-031](../solutions/BE-REQ-SOL-031-source-registry-and-context-pack.md) | [TASK-REQ-031-01](./TASK-REQ-031-01-context-tables-migration-and-repositories.md) | Migration và repository `context_sources`, `context_packs`, `evidence` | P1 | SOL-001, 002 | 📋 TODO |
| | [TASK-REQ-031-02](./TASK-REQ-031-02-context-domain-ranking-redaction-evidence.md) | Domain thuần: nguồn, xếp hạng, cắt, Evidence, bộ che PII | P1 | 031-01 (kiểu), 035-01 | 📋 TODO |
| | [TASK-REQ-031-03](./TASK-REQ-031-03-repo-backed-source-adapters.md) | Adapter nguồn đọc repo qua Relay | P1 | 031-02 | 📋 TODO |
| | [TASK-REQ-031-04](./TASK-REQ-031-04-orca-data-source-adapters.md) | Adapter `request_origin`, `history`, `ownership`, `dev_server_profile` | P1 | 031-02 | 📋 TODO |
| | [TASK-REQ-031-05](./TASK-REQ-031-05-build-context-pack-usecase-and-rpc.md) | `BuildContextPack`, quản trị nguồn, RPC Preview | P1 | 031-01..04 | 📋 TODO |
| | [TASK-REQ-031-06](./TASK-REQ-031-06-mcp-service-call-external-tool-and-read-resource.md) | `mcp-service`: `CallExternalTool`, `ReadExternalResource` | P1 (đợt 2) | không | ✅ DONE 2026-10-07 |
| | [TASK-REQ-031-07](./TASK-REQ-031-07-request-service-mcp-source-client.md) | Client `mcp-service` và adapter nguồn `mcp` | P1 (đợt 2) | 031-05, 031-06 | ✅ DONE 2026-10-07 |
| | [TASK-REQ-031-08](./TASK-REQ-031-08-gateway-orca-resources-and-source-tools.md) | Gateway: `orca://...` và `source_*` | P1 | 031-05, BE-REQ-SOL-016 | 📋 TODO |

## Thứ tự phụ thuộc

```
SOL-001, 002 ─▶ 031-01 ─▶ 031-02 ─┬─▶ 031-03 ─┐
                  ▲               └─▶ 031-04 ─┤
 035-01 (secretscan) ─────────────┘            ▼
                                            031-05 ─▶ 031-08 (gateway, sau SOL-016)
 031-06 (mcp-service, độc lập) ─▶ 031-07 ─────▲
```

## Ghi chú

- Đợt 1 (nguồn nội bộ): 01 đến 05 và 08. Đợt 2 (MCP ngoài): 06, 07. Task 06 không cần `request-service`, làm sớm được.
- Số migration `NNNN` ở task 01: đọc `ls backend-go/services/request-service/migrations/{postgres,mysql}` lúc làm; hai dialect cùng số.
- Trước khi sửa `registry_server.go`, `planFor`, `Parse`: chạy `gitnexus_impact` (quy ước repo).
- Lệnh chung: `cd backend-go && go test ./services/request-service/... ./services/mcp-service/... ./services/api-gateway/internal/adapter/mcpserver/...`; tích hợp: `go test -tags=integration ...` (cần Docker). Chưa chạy.
- Việc dữ liệu không phải mã (không có task): `CODEOWNERS`, danh mục service sinh tự động, ví dụ vàng, từ điển thuật ngữ, chỉ mục ADR/CR (CR mục 2.9).
- Số liệu ngân sách token, xếp hạng, TTL là đề xuất chưa đo.
