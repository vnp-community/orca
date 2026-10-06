# backend-go Tasks: Code Intel Sources (v7)

Task thực thi của 13 solution trong [`../solutions/`](../solutions/README.md). Tất cả `Status: [ ] TODO`, chưa chạy test nào. Mỗi task nêu file, lệnh test, tiêu chí; số dòng code trích dẫn từ lần đọc 2026-10-06. Hợp đồng: [`../../CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md).

## Bảng Solution, Task (nhóm A)

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-030 | [030-01](./BE-CV-TASK-030-01-agent-fs-git-sample-recordings.md) | Re-verify, ghi mẫu JSON-RPC `fs.*`/`git.*` | P0 |
| | [030-02](./BE-CV-TASK-030-02-repo-path-and-source-policy.md) | `CleanRel`, allowlist/deny-list | P0 |
| | [030-03](./BE-CV-TASK-030-03-repofs-limits-and-reader-ports.md) | Cổng `RepoSourceReader`, `AgentRelay`, `CODEINTEL_REPOFS_*` | P0 |
| | [030-04](./BE-CV-TASK-030-04-infrafleet-relay-client-and-error-mapping.md) | Client `RelayByDevServer`, ánh xạ lỗi, whitelist | P0 |
| | [030-05](./BE-CV-TASK-030-05-list-dir-and-read-file.md) | `ListDir`/`ReadFile(s)`, cache nội dung | P0 |
| | [030-06](./BE-CV-TASK-030-06-git-read-operations.md) | `ResolveHead`, `BlobIDs`, `DirtyPaths`, `ChangedFiles`, `FileDiff`, `Log` | P0 |
| | [030-07](./BE-CV-TASK-030-07-reader-contract-and-tenant-isolation-suite.md) | Suite hợp đồng, cô lập tenant | P0 |
| BE-CV-SOL-031-sql-migration-parser | [031-01](./BE-CV-TASK-031-01-migration-corpus-inventory-and-fixtures.md) | Re-verify corpus, fixture | P0 |
| | [031-02](./BE-CV-TASK-031-02-sql-tokenizer-and-statement-splitter.md) | Tokenizer, bộ tách câu | P0 |
| | [031-03](./BE-CV-TASK-031-03-erd-catalog-domain-and-postgres-ddl.md) | `Catalog` + DDL Postgres | P0 |
| | [031-04](./BE-CV-TASK-031-04-mysql-ddl-branch.md) | Nhánh MySQL | P0 |
| | [031-05](./BE-CV-TASK-031-05-rls-do-block-idiom.md) | Thành ngữ RLS `DO $$` | P0 |
| | [031-06](./BE-CV-TASK-031-06-migration-ordering-and-corpus-suite.md) | Thứ tự, `AsOfMigration`, suite corpus | P0 |
| BE-CV-SOL-031-erd-model-and-access-scan | [031-07](./BE-CV-TASK-031-07-erd-proto-and-getErd-rpc.md) | Proto ERD + `GetErd` | P0 |
| | [031-08](./BE-CV-TASK-031-08-erd-relations-and-dialect-drift.md) | Quan hệ, `DIALECT_DRIFT` | P0 |
| | [031-09](./BE-CV-TASK-031-09-go-sql-literal-access-scan.md) | `accessedBy` quét literal | P1 |
| | [031-10](./BE-CV-TASK-031-10-build-erd-usecase-changes-and-cache.md) | `BuildErd`, `changes[]`, cache | P0 |
| | [031-11](./BE-CV-TASK-031-11-getErd-grpc-handler.md) | Handler `GetErd` | P0 |
| | [031-12](./BE-CV-TASK-031-12-erd-golden-dual-dialect-and-tenant-suite.md) | Golden, hai dialect, tenant | P1 |
| BE-CV-SOL-032 | [032-01](./BE-CV-TASK-032-01-reverify-proto-and-wscompat-inventory.md) | Re-verify, fixture | P0 |
| | [032-02](./BE-CV-TASK-032-02-contract-domain-and-proto-messages.md) | Miền + proto message | P0 |
| | [032-03](./BE-CV-TASK-032-03-protoschema-parser.md) | Parser `.proto` | P0 |
| | [032-04](./BE-CV-TASK-032-04-server-registration-and-unimplemented-detection.md) | `Register*Server`, `Unimplemented` | P0 |
| | [032-05](./BE-CV-TASK-032-05-client-edges-t2-t3.md) | Cạnh client T2/T3 | P0 |
| | [032-06](./BE-CV-TASK-032-06-wscompat-channel-catalog-extraction.md) | Danh mục kênh `wscompat` | P0 |
| | [032-07](./BE-CV-TASK-032-07-build-contract-catalog-usecase-and-crosscheck.md) | `BuildContractCatalog`, cache, kiểm chéo | P0 |
| BE-CV-SOL-033-c4-component-view | [033-01](./BE-CV-TASK-033-01-reverify-hexagonal-layout-and-port-evidence.md) | Re-verify hexagonal, fixture | P0 |
| | [033-02](./BE-CV-TASK-033-02-c4-proto-and-getArchitecture-rpc.md) | Proto C4 + `GetArchitecture` | P0 |
| | [033-03](./BE-CV-TASK-033-03-go-package-graph-scan.md) | Quét package Go | P0 |
| | [033-04](./BE-CV-TASK-033-04-derive-components-ids-and-externals.md) | Component, id, external | P0 |
| | [033-05](./BE-CV-TASK-033-05-c4-relations-and-layer-rules.md) | Quan hệ, luật lớp | P0 |
| | [033-06](./BE-CV-TASK-033-06-get-architecture-usecase-handler-and-cache.md) | `GetArchitecture` use case/handler | P0 |
| BE-CV-SOL-033-c4-overrides-yaml | [033-07](./BE-CV-TASK-033-07-c4-overrides-schema-and-strict-validator.md) | Schema + validator | P0 |
| | [033-08](./BE-CV-TASK-033-08-merge-c4-overrides-pure-function.md) | `MergeC4Overrides`, proto Get/Save | P0 |
| | [033-09](./BE-CV-TASK-033-09-c4-overrides-usecases-seed-and-cas.md) | Use case, seed, CAS | P0 |
| | [033-10](./BE-CV-TASK-033-10-c4-overrides-grpc-handlers-and-authz.md) | Handler + `c4_write` | P0 |
| | [033-11](./BE-CV-TASK-033-11-c4-overrides-integration-dual-dialect-and-tenant-suite.md) | Tích hợp hai dialect, tenant, golden | P1 |
| BE-CV-SOL-034 | [034-01](./BE-CV-TASK-034-01-reverify-channel-to-agent-chain.md) | Re-verify chuỗi, chọn golden | P0 |
| | [034-02](./BE-CV-TASK-034-02-dataflow-proto-and-rpcs.md) | Proto dataflow + 2 RPC | P0 |
| | [034-03](./BE-CV-TASK-034-03-dataflow-domain-and-limits.md) | Miền, mã gap, giới hạn | P0 |
| | [034-04](./BE-CV-TASK-034-04-go-call-index.md) | `CallIndex` | P0 |
| | [034-05](./BE-CV-TASK-034-05-build-data-flow-traversal.md) | Bộ duyệt dựng luồng | P0 |
| | [034-06](./BE-CV-TASK-034-06-list-data-flows-candidates.md) | `ListDataFlows` | P1 |
| | [034-07](./BE-CV-TASK-034-07-process-enrichment-optional.md) | Làm giàu `Process` (tuỳ chọn) | P2 |
| | [034-08](./BE-CV-TASK-034-08-export-sequence-and-dfd-models.md) | Sequence/DFD | P0 |
| | [034-09](./BE-CV-TASK-034-09-data-flow-handlers-cache-and-golden.md) | Handler, cache, golden, tenant | P0 |

## Bảng Solution, Task (nhóm B — do nhóm B soạn; cập nhật theo file có trên đĩa lúc ghi)

| Solution | Task đã có | Ghi chú |
|----------|-----------|---------|
| BE-CV-SOL-035-storage-map | `TASK-035-01…08` | có |
| BE-CV-SOL-036-change-overlay-pipeline | `TASK-036-01…05` | có |
| BE-CV-SOL-036-reading-order-and-risk | `TASK-036-06…10` | có |
| BE-CV-SOL-037-structure-findings-and-dismissals | `TASK-037-01…04` (đang soạn thêm) | chưa đủ |
| BE-CV-SOL-038-contract-diff | chưa có | do nhóm B soạn |
| BE-CV-SOL-038-static-tenant-filter-rule | chưa có | do nhóm B soạn |

## Thứ tự phụ thuộc (nhóm A)

```
030-01 ─▶ 030-02 ─▶ 030-03 ─▶ 030-04 ─▶ 030-05 ─┬─▶ 030-07
                              └──────▶ 030-06 ─┘
030-05 ─▶ 031-01 ─▶ 031-02 ─▶ 031-03 ─┬─▶ 031-04 ─┐
                                      └─▶ 031-05 ─┴─▶ 031-06 ─▶ 031-08 ─┐
031-06 ─▶ 031-09 ──────────────────────────────────────────────────────┤
031-07 (proto, cần G0) ─────────────────────────────────────────────────┴▶ 031-10 ─▶ 031-11 ─▶ 031-12
030-05 ─▶ 032-01 ─▶ 032-02 ─▶ 032-03 ─▶ 032-04 ─▶ 032-05 ─▶ 032-06 ─▶ 032-07
033-01 ─▶ 033-03 ─▶ 033-04 ─▶ 033-05 ─▶ 033-06;  033-02 ─▶ 033-06;  033-07 ─▶ 033-08 ─▶ 033-09 ─▶ 033-10 ─▶ 033-11
(032-07 + 031-09 ─▶ 033-04/05; 033-08 ─▶ 033-06)
034-01 ─▶ 034-03 ─▶ 034-04 ─▶ 034-05 ─▶ 034-09;  034-02 ─▶ 034-03;  034-06, 034-08 ─▶ 034-09;  034-07 (tuỳ chọn)
(032-07 + 033-05 + 031-10 ─▶ 034-05)
```
Song song được: 030-05 với 030-06; 031-04 với 031-05; 032 với 031-07…12; 033-07/08 với 033-03…05; 034-06 với 034-08.

## Ghi chú

- **Proto trước:** các task proto (031-07, 033-02/08, 034-02, 032-02) cần `codeintel_common.proto` và `codeintel.proto` (G0, BE-CV-SOL-010/020); chạy `buf lint` và `buf breaking` trực tiếp (không `make proto-lint` có `|| true`).
- **Hai dialect:** parser/miền thuần test cả hai dialect ở mức fixture; phần DB (snapshot, `c4_overrides`) dùng ma trận CI `dialect: [postgres, mysql]`, Postgres bằng role `NOSUPERUSER NOBYPASSRLS` (dev compose dùng superuser nên RLS không có hiệu lực ở dev).
- **Cô lập tenant:** mọi cache/snapshot/override có `tenant_id` và có test hai tenant.
- **Số field proto** do chủ sở hữu CR gán: các task ghi "đề xuất" khi hợp đồng chưa có số; sau merge `buf breaking` khoá.
- **Git/SSH/GitLab:** cổng đọc chỉ dùng lệnh git nền 2.25, chịu độ trễ SSH (cache theo oid); không giả định GitHub.
