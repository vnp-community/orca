# backend-go Solutions: Code Intel Sources (v7)

**CRs:** [docs/crs/v7/code-intel-sources](../../../../../../docs/crs/v7/code-intel-sources/README.md) (CR-CV-030…038)
**Hợp đồng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md) (§1 PQ-01…37, §8.2 tên solution, §8.3 kiểm tra chéo), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md). Khi CR và hợp đồng khác nhau thì theo hợp đồng (mục "Lệch giữa CR và hợp đồng" trong từng solution).
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

> 📋 Proposed. Chưa triển khai, chưa chạy build/test nào. Solution 030–034 do nhóm A soạn; 035–038 do nhóm B soạn (README này chỉ liệt kê, bảng task nhóm B xem `../tasks/README.md`). Mọi solution chạy trong `backend-go/services/code-intel-service` (mới, do `BE-CV-SOL-010` dựng).

## Bảng CR, Solution, Task

| CR | Solution (13) | Phần | Priority | Task | Trạng thái file |
|----|---------------|------|----------|------|-----------------|
| CR-CV-030 | [BE-CV-SOL-030-repo-file-access-gateway](./BE-CV-SOL-030-repo-file-access-gateway.md) | A | P0 | `TASK-030-01…07` | có |
| CR-CV-031 | [BE-CV-SOL-031-sql-migration-parser](./BE-CV-SOL-031-sql-migration-parser.md) | A | P0 | `TASK-031-01…06` | có |
| CR-CV-031 | [BE-CV-SOL-031-erd-model-and-access-scan](./BE-CV-SOL-031-erd-model-and-access-scan.md) | A | P0 | `TASK-031-07…12` | có |
| CR-CV-032 | [BE-CV-SOL-032-proto-and-wscompat-contract-catalog](./BE-CV-SOL-032-proto-and-wscompat-contract-catalog.md) | A | P1 | `TASK-032-01…07` | có |
| CR-CV-033 | [BE-CV-SOL-033-c4-component-view](./BE-CV-SOL-033-c4-component-view.md) | A | P1 | `TASK-033-01…06` | có |
| CR-CV-033 | [BE-CV-SOL-033-c4-overrides-yaml](./BE-CV-SOL-033-c4-overrides-yaml.md) | A | P1 | `TASK-033-07…11` | có |
| CR-CV-034 | [BE-CV-SOL-034-data-flow-model](./BE-CV-SOL-034-data-flow-model.md) | A | P1 | `TASK-034-01…09` | có |
| CR-CV-035 | [BE-CV-SOL-035-storage-map](./BE-CV-SOL-035-storage-map.md) | B | P0 | `TASK-035-01…08` | có |
| CR-CV-036 | [BE-CV-SOL-036-change-overlay-pipeline](./BE-CV-SOL-036-change-overlay-pipeline.md) | B | P0 | `TASK-036-01…05` | có |
| CR-CV-036 | [BE-CV-SOL-036-reading-order-and-risk](./BE-CV-SOL-036-reading-order-and-risk.md) | B | P0 | `TASK-036-06…10` | có |
| CR-CV-037 | [BE-CV-SOL-037-structure-findings-and-dismissals](./BE-CV-SOL-037-structure-findings-and-dismissals.md) | B | P0 | `TASK-037-01…08` | có |
| CR-CV-038 | [BE-CV-SOL-038-contract-diff](./BE-CV-SOL-038-contract-diff.md) | B | P0 | `TASK-038-01…07` | có |
| CR-CV-038 | [BE-CV-SOL-038-static-tenant-filter-rule](./BE-CV-SOL-038-static-tenant-filter-rule.md) | B | P0 | `TASK-038-08…12` | có |

(Phần AG của CR-037: `AG-CV-SOL-037-structural-facts` ở `specs/agent/crs/v7`.)

## Sơ đồ phụ thuộc

```
BE-CV-SOL-010 → 011 → 012 → 013     (feature code-intel-service-foundation)
BE-CV-SOL-020 (SymbolRef, ResultMeta, WorktreeSelector) ─┐
BE-CV-SOL-021/023 (collector, AgentRelay client)          │
                                                          ▼
SOL-030 (RepoSourceReader, AgentRelay port)
   ├─▶ SOL-031-parser ─▶ SOL-031-erd ─────────────┐
   ├─▶ SOL-032 (ContractCatalog) ─┬─▶ SOL-033-view ◀─ SOL-033-overrides
   │                              └─▶ SOL-034 ◀─ SOL-033-view, SOL-031-erd
   │                                   SOL-035 (cần cấu hình; P2)
   └─▶ SOL-036 (cần AG-005, 020, 021) ─▶ SOL-037 ─▶ SOL-038
```

Thứ tự đợt (hợp đồng §7.3): đợt 2 gồm 030, 031; đợt 3 gồm 036; đợt 4 gồm 032, 033, 034; đợt 5 gồm 037, 038; đợt 6 gồm 035. Dãy `NN` của task liên tục **theo CR** (033 hai solution dùng chung dãy 01–11).

## Quyết định chung (nhóm A, theo hợp đồng)

| # | Quyết định | Nguồn |
|---|-----------|-------|
| F1 | Mọi đọc file qua `RepoSourceReader` (SOL-030); 031–038 không gọi `fs.*`/`git.*` trực tiếp | S1 README feature |
| F2 | Parser SQL/proto tự viết, không dependency mới (`CGO_ENABLED=0`); kiểm chéo bằng DB/descriptor thật ở CI | O-7 |
| F3 | Request nhận `WorktreeSelector selector`; `repo_binding_id` chỉ trong bảng/response | PQ-04 |
| F4 | Proto: một file mỗi chủ đề `codeintel_<chủ đề>.proto`, RPC vào `CodeIntelService` duy nhất; `ContractCatalog` chỉ message nội bộ | PQ-07, PQ-29 |
| F5 | Kết quả phẳng `ResultMeta` + `etag`/`not_modified`; ≤ 2 MiB; cache `graph_snapshots` khoá `head_commit + params_hash` | PQ-12, 14, 15 |
| F6 | Lỗi `CODEINTEL_*` đứng đầu message, hậu tố ` \| {json}`; offline = `CODEINTEL_DEV_SERVER_OFFLINE` (`Unavailable`) | PQ-02, 03 |
| F7 | Biến môi trường `CODEINTEL_*` | PQ-23 |
| F8 | Mọi truy vấn có `tenant_id`; nhánh hai dialect có test hai dialect; Postgres test bằng role `NOBYPASSRLS` | §8.3, H10 |
| F9 | Suy luận có `origin`/`confidence`; `unknown/degraded/partial` là trạng thái thật | H7 |
| F10 | Không mã nguồn trong DB/log/lỗi | H8 |

## Điểm hợp đồng thiếu/mâu thuẫn nhóm A phát hiện (báo chủ hợp đồng)

1. `fs.*`/`git.*` qua infra-fleet: CR-023 chỉ phủ lỗi `codeintel.*`/`quality.*`; lỗi file/git không có mã (SOL-030 G1). Chủ sở hữu port `AgentRelay` chưa giao (G2).
2. `GetErdResponse` §3.1 thiếu `services[]` cho trường hợp không có `service` (UI §3.1); `warnings` đặt ở response vs `ErdModel.warnings` ở UI (SOL-031-erd G1–G3).
3. Số field proto của `GetArchitecture`, `Get/SaveC4Overrides`, `ListDataFlows`, `GetDataFlow`, `ErdChange` chưa có trong hợp đồng; kiểu `version` của `c4` (bigint DB, chuỗi ở view, `expectedVersion` ở UI).
4. Không có subject/audit chuẩn cho `c4.saved`.
5. Đếm trong CR lệch mã thật: `DO $$` ở 7 file (CR: 4); `logical FK` 16 Postgres + 11 MySQL (CR: 17 Postgres); `var _ usecase.` MySQL 23 (CR: 29).

## Điều còn mở

- O-7 (thư viện parse; precision `sql.missing-tenant-filter`), O-12 (schema `c4.yaml`, seed), O-16 (gói `secretmasking`/`pathsafety` dùng chung), Q1 CR-030 (khoá `workDir` ở agent), tên schema MySQL cho ERD (SOL-031-parser O-S1), quy tắc hai cài đặt cổng khi dựng luồng (034 Q1).
