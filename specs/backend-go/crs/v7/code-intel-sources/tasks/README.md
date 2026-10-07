# backend-go Tasks: Code Intel Sources (v7)

Task thực thi của 13 solution trong [`../solutions/`](../solutions/README.md). Tất cả `Status: [x] DONE`. Mỗi task nêu file, lệnh test, tiêu chí; số dòng code trích dẫn từ lần đọc 2026-10-06. Hợp đồng: [`../../CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md).

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

## Bảng Solution, Task (nhóm B)

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-035-storage-map | [035-01](./BE-CV-TASK-035-01-reverify-storage-sources-and-reader-contract.md) | Re-verify nguồn lưu trữ, chữ ký `RepoSourceReader`, fixture gốc | P0 |
| | [035-02](./BE-CV-TASK-035-02-proto-storage-map.md) | Proto `codeintel_storage.proto` + `GetStorageMap` | P0 |
| | [035-03](./BE-CV-TASK-035-03-domain-storage-map-and-store-identity.md) | Domain `storagemap`, `Store.id`, thứ tự chuẩn, hợp nhất | P0 |
| | [035-04](./BE-CV-TASK-035-04-secretmasking-env-values-and-connection-urls.md) | Gói `secretmasking`, đường dẫn cấm, URL kết nối | P0 |
| | [035-05](./BE-CV-TASK-035-05-secretmasking-final-leak-scan.md) | Quét cuối `storage_map_leak_scan`, `CODEINTEL_SECRET_LEAK_BLOCKED` | P0 |
| | [035-06](./BE-CV-TASK-035-06-compose-and-config-store-binding-extractors.md) | Trích Store & Binding từ compose, init script, `config.go`, adapter | P0 |
| | [035-07](./BE-CV-TASK-035-07-nats-topic-literal-extractor.md) | Trích Topic NATS từ mã Go bằng `go/parser` | P1 |
| | [035-08](./BE-CV-TASK-035-08-get-storage-map-usecase-cache-and-grpc.md) | `GetStorageMap` use case, cache snapshot, gRPC handler, canary | P0 |
| BE-CV-SOL-036-change-overlay-pipeline | [036-01](./BE-CV-TASK-036-01-reverify-agent-contract-and-overlay-fixtures.md) | Re-verify hợp đồng agent `detectChanges`/`impact`, fixture overlay | P0 |
| | [036-02](./BE-CV-TASK-036-02-proto-change-overlay.md) | Proto `codeintel_change_overlay.proto` | P0 |
| | [036-03](./BE-CV-TASK-036-03-domain-change-set-mapping-and-freshness.md) | Domain `changeoverlay`, ánh xạ agent, `IndexFreshness` | P0 |
| | [036-04](./BE-CV-TASK-036-04-impact-selection-ports-and-deadline-fanout.md) | Chọn K symbol, cổng làm giàu mềm, fan-out `impact` | P0 |
| | [036-05](./BE-CV-TASK-036-05-get-change-overlay-usecase-cache-and-grpc.md) | Use case `GetChangeOverlay`, cache snapshot, handler gRPC | P0 |
| BE-CV-SOL-036-reading-order-and-risk | [036-06](./BE-CV-TASK-036-06-reading-graph-and-strongly-connected-components.md) | Đồ thị đọc từ `impact` độ sâu 1, Tarjan SCC lặp | P0 |
| | [036-07](./BE-CV-TASK-036-07-dependency-ordering-and-reading-steps.md) | Kahn ưu tiên, gộp bước theo tệp, `stepKey`, `reason` | P0 |
| | [036-08](./BE-CV-TASK-036-08-risk-rules-and-scoring.md) | Bảng quy tắc điểm rủi ro và `ScoreRisk` | P0 |
| | [036-09](./BE-CV-TASK-036-09-component-grouping-and-overlay-limits.md) | Gom component và `ApplyLimits` (đếm trước khi cắt) | P0 |
| | [036-10](./BE-CV-TASK-036-10-get-reading-order-rpc-and-determinism-suite.md) | RPC `GetReadingOrder`, bộ test xác định, cô lập tenant | P0 |
| BE-CV-SOL-037-structure-findings-and-dismissals | [037-01](./BE-CV-TASK-037-01-reverify-structural-facts-contract-and-fixtures.md) | Re-verify `structuralFacts`, git log, CODEOWNERS, fixture | P0 |
| | [037-02](./BE-CV-TASK-037-02-proto-findings.md) | Proto `codeintel_findings.proto` (`Finding`, `ListFindings`, `DismissFinding`) | P0 |
| | [037-03](./BE-CV-TASK-037-03-layer-and-cycle-finding-rules.md) | Domain `structurefinding`, `finding_key`, quy tắc lớp, vòng import | P0 |
| | [037-04](./BE-CV-TASK-037-04-hotspot-scoring-and-git-log-parsing.md) | Hotspot: parse `git log`, `churn`, proxy độ phức tạp, `centrality` | P0 |
| | [037-05](./BE-CV-TASK-037-05-dead-export-and-owner-resolution.md) | Mã chết (`dead.unused-export`) và owner resolution | P0 |
| | [037-06](./BE-CV-TASK-037-06-detector-interface-and-list-findings-usecase.md) | Giao diện `Detector`, `ListFindings`, cache snapshot, `origin` | P0 |
| | [037-07](./BE-CV-TASK-037-07-finding-dismissal-repositories-and-dismiss-usecase.md) | Repository `finding_dismissals` (hai dialect), `DismissFinding` | P0 |
| | [037-08](./BE-CV-TASK-037-08-findings-grpc-handler-and-end-to-end-suite.md) | Handler gRPC, đăng ký và bộ test đầu-cuối | P0 |
| BE-CV-SOL-038-contract-diff | [038-01](./BE-CV-TASK-038-01-reverify-contract-parsers-and-diff-fixtures.md) | Re-verify parser, route HTTP, dựng fixture so sánh hợp đồng | P0 |
| | [038-02](./BE-CV-TASK-038-02-proto-contract-diff.md) | Proto `codeintel_contract_diff.proto` (`ContractDiff`, `GetContractDiff`) | P0 |
| | [038-03](./BE-CV-TASK-038-03-contract-file-selection-and-two-version-loader.md) | Chọn tệp hợp đồng đã đổi, nạp 2 phiên bản qua `RepoSourceReader` | P0 |
| | [038-04](./BE-CV-TASK-038-04-proto-contract-diff-rules.md) | Quy tắc so sánh proto (`proto.*`) bám `buf breaking` | P0 |
| | [038-05](./BE-CV-TASK-038-05-wscompat-and-route-diff-rules.md) | Quy tắc so sánh kênh `wscompat` (`ws.*`) và route HTTP (`route.*`) | P0 |
| | [038-06](./BE-CV-TASK-038-06-migration-statement-rules-and-table-impact.md) | Phân loại câu lệnh migration hai dialect và `TableImpact` | P0 |
| | [038-07](./BE-CV-TASK-038-07-get-contract-diff-usecase-cache-and-grpc.md) | Use case `GetContractDiff`, giới hạn, cache, handler gRPC | P0 |
| BE-CV-SOL-038-static-tenant-filter-rule | [038-08](./BE-CV-TASK-038-08-reverify-tenant-queries-and-golden-set-scaffold.md) | Re-verify truy vấn thiếu `tenant_id`, khung bộ vàng 100 truy vấn | P0 |
| | [038-09](./BE-CV-TASK-038-09-go-ast-sql-string-extraction.md) | Trích chuỗi SQL từ mã Go bằng `go/parser` | P0 |
| | [038-10](./BE-CV-TASK-038-10-sql-statement-analysis-and-tenant-rule.md) | Phân tích SQL nhẹ, điều kiện báo thiếu `tenant_id` (hai dialect) | P0 |
| | [038-11](./BE-CV-TASK-038-11-confidence-tiers-and-tenant-finding-key.md) | Tầng độ tin cậy, heuristic RLS, khoá toàn cục, `finding_key` | P0 |
| | [038-12](./BE-CV-TASK-038-12-tenant-filter-detector-wiring-and-golden-gate.md) | Detector `sql.*` cắm vào `ListFindings`, cổng bộ vàng | P0 |

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

## Thứ tự phụ thuộc (nhóm B)

```
035-01 ─▶ 035-03 ─▶ 035-04 ─▶ 035-05 ─▶ 035-06 ─┬─▶ 035-08
                                       035-07 ─┘
035-02 ─────────────────────────────────────────┘
036-01 ─▶ 036-03 ─▶ 036-04 ─▶ 036-05
036-02 ─────────────────────┘
036-04 ─▶ 036-06 ─▶ 036-07 ─▶ 036-08 ─▶ 036-09 ─▶ 036-10
037-01 ─▶ 037-03 ─┬─▶ 037-06 ─▶ 037-08
037-02 ───────────┤
037-04, 037-05 ───┤
037-07 ───────────┘
038-01 ─▶ 038-03 ─▶ 038-04 ─┬─▶ 038-07
038-02 ─────────────────────┤
038-05, 038-06 ─────────────┘
038-08 ─▶ 038-09 ─▶ 038-10 ─▶ 038-11 ─▶ 038-12
```
Song song được: 035-02 với 035-03…07; 036-02 với 036-01/03/04; 037-02/04/05 với 037-01/03; 038-02 với 038-01/03/04; 038-05/06 với 038-04.

## Ghi chú

- **Proto trước:** các task proto (031-07, 033-02/08, 034-02, 032-02) cần `codeintel_common.proto` và `codeintel.proto` (G0, BE-CV-SOL-010/020); chạy `buf lint` và `buf breaking` trực tiếp (không `make proto-lint` có `|| true`).
- **Hai dialect:** parser/miền thuần test cả hai dialect ở mức fixture; phần DB (snapshot, `c4_overrides`) dùng ma trận CI `dialect: [postgres, mysql]`, Postgres bằng role `NOSUPERUSER NOBYPASSRLS` (dev compose dùng superuser nên RLS không có hiệu lực ở dev).
- **Cô lập tenant:** mọi cache/snapshot/override có `tenant_id` và có test hai tenant.
- **Số field proto** do chủ sở hữu CR gán: các task ghi "đề xuất" khi hợp đồng chưa có số; sau merge `buf breaking` khoá.
- **Git/SSH/GitLab:** cổng đọc chỉ dùng lệnh git nền 2.25, chịu độ trễ SSH (cache theo oid); không giả định GitHub.
