# backend-go Tasks: Code Intel Service Foundation (v7)

Task thực thi của bảy solution trong [`../solutions/`](../solutions/README.md). Tất cả `Status: [ ] TODO`, chưa chạy test nào. Số dòng trích dẫn từ lần đọc code 2026-10-06. `NN` tăng liên tục trong từng CR; mỗi task ghi solution mẹ.

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-010 | [010-01](./BE-CV-TASK-010-01-reverify-buf-empty-service-and-wiring-baseline.md) | Re-verify `buf lint` service rỗng, stream, wiring | P0 |
| | [010-02](./BE-CV-TASK-010-02-apperrors-resource-exhausted-and-unavailable-kinds.md) | `common/apperrors`: 2 Kind mới | P0 |
| | [010-03](./BE-CV-TASK-010-03-module-config-and-workspace-registration.md) | `go.mod`, `config`, `go.work`, `Makefile` | P0 |
| | [010-04](./BE-CV-TASK-010-04-proto-codeintel-service-skeleton.md) | `codeintel.proto` chỉ `service`, stub | P0 |
| | [010-05](./BE-CV-TASK-010-05-migration-0001-init-two-dialects.md) | Migration `0001` hai dialect, RLS | P0 |
| | [010-06](./BE-CV-TASK-010-06-outbox-and-tenant-tx-adapters.md) | `withTenantTx/withRelayTx`, outbox, test AST | P0 |
| | [010-07](./BE-CV-TASK-010-07-main-grpc-server-and-tenant-stream-interceptor.md) | `main.go`, interceptor stream, health, relay | P0 |
| | [010-08](./BE-CV-TASK-010-08-deploy-dev-wiring-and-ci-workflow.md) | Dockerfile, compose, init DB, `migrate.sh`, CI | P1 |
| BE-CV-SOL-011-data | [011-01](./BE-CV-TASK-011-01-reverify-reserved-words-and-mysql-baseline.md) | Re-verify từ khoá SQL, MySQL, khoá | P0 |
| | [011-02](./BE-CV-TASK-011-02-migration-0002-postgres-core-tables-rls.md) | `0002` Postgres, RLS, chính sách bảo trì | P0 |
| | [011-03](./BE-CV-TASK-011-03-migration-0002-mysql-core-tables.md) | `0002` MySQL | P0 |
| | [011-04](./BE-CV-TASK-011-04-domain-entities-limits-and-errors.md) | Domain, giới hạn, mã lỗi | P0 |
| | [011-05](./BE-CV-TASK-011-05-schema-contract-test-two-dialects.md) | Test hợp đồng schema | P1 |
| BE-CV-SOL-011-repos | [011-06](./BE-CV-TASK-011-06-repository-ports-and-snapshot-limits.md) | Cổng repository, `RetentionTask` | P0 |
| | [011-07](./BE-CV-TASK-011-07-postgres-core-repositories.md) | Postgres: settings, binding, review, dismissal, C4 | P0 |
| | [011-08](./BE-CV-TASK-011-08-postgres-snapshot-and-reindex-repositories.md) | Postgres: snapshot, reindex | P0 |
| | [011-09](./BE-CV-TASK-011-09-mysql-core-repositories.md) | MySQL: settings, binding, review, dismissal, C4 | P0 |
| | [011-10](./BE-CV-TASK-011-10-mysql-snapshot-and-reindex-repositories.md) | MySQL: snapshot, reindex | P0 |
| | [011-11](./BE-CV-TASK-011-11-repository-contract-suite-and-tenant-isolation.md) | Bộ hợp đồng hai dialect, cô lập tenant | P0 |
| | [011-12](./BE-CV-TASK-011-12-maintenance-repositories-two-dialects.md) | Repository bảo trì | P1 |
| | [011-13](./BE-CV-TASK-011-13-maintenance-job-and-retention-registry.md) | Job bảo trì, registry | P1 |
| BE-CV-SOL-012-target | [012-01](./BE-CV-TASK-012-01-reverify-upstream-project-fleet-gitgateway-behaviors.md) | Re-verify hạ nguồn | P0 |
| | [012-02](./BE-CV-TASK-012-02-proto-binding-and-rpc-registration.md) | `codeintel_binding.proto`, đăng ký RPC | P0 |
| | [012-03](./BE-CV-TASK-012-03-worktree-ref-parsing-and-workspace-path-normalization.md) | Parse ref, chuẩn hoá đường dẫn | P0 |
| | [012-04](./BE-CV-TASK-012-04-grpc-clients-with-identity-forwarding.md) | Client gRPC + chuyển danh tính | P0 |
| | [012-05](./BE-CV-TASK-012-05-resolve-target-usecase.md) | `ResolveTarget` | P0 |
| | [012-06](./BE-CV-TASK-012-06-bind-repo-and-list-bindings-handlers.md) | `BindRepo`, `ListRepoBindings` | P1 |
| | [012-07](./BE-CV-TASK-012-07-worktree-deleted-consumer-with-dedup.md) | Consumer `worktree.deleted` | P1 |
| BE-CV-SOL-012-status | [012-08](./BE-CV-TASK-012-08-agent-status-decoder-and-fixtures.md) | Giải mã `codeintel.status`, fixture | P0 |
| | [012-09](./BE-CV-TASK-012-09-compute-overall-state-table.md) | `ComputeOverall` (9 trạng thái) | P0 |
| | [012-10](./BE-CV-TASK-012-10-status-cache-singleflight-and-last-status-persistence.md) | Cache, singleflight, lưu `last_status` | P0 |
| | [012-11](./BE-CV-TASK-012-11-get-index-status-usecase-and-rpc.md) | `GetIndexStatus` | P0 |
| BE-CV-SOL-013-authz | [013-01](./BE-CV-TASK-013-01-reverify-listmembers-auditclient-and-opa-tooling.md) | Re-verify `ListMembers`, role, audit, `opa` | P0 |
| | [013-02](./BE-CV-TASK-013-02-code-intel-rego-and-tests.md) | `code_intel.rego` + test | P0 |
| | [013-03](./BE-CV-TASK-013-03-internal-caller-guard-coverage-from-service-desc.md) | `internalcaller` phủ mọi RPC | P0 |
| | [013-04](./BE-CV-TASK-013-04-project-authorizer-getproject-listmembers-opa.md) | `ProjectAuthorizer` | P0 |
| | [013-05](./BE-CV-TASK-013-05-tenant-flag-reader-effective-flags.md) | `FlagReader` | P0 |
| | [013-06](./BE-CV-TASK-013-06-audit-recorder-async-queue.md) | `AuditRecorder` async | P1 |
| | [013-07](./BE-CV-TASK-013-07-rpc-policy-table-interceptor-and-status-mapping.md) | Bảng chính sách, pipeline, `toStatus` | P0 |
| BE-CV-SOL-013-gate | [013-08](./BE-CV-TASK-013-08-secret-redactor-and-sensitive-path-blocklist.md) | Bộ che, đường dẫn nhạy cảm | P0 |
| | [013-09](./BE-CV-TASK-013-09-agent-call-gate-concurrency-and-method-classes.md) | `AgentCallGate`, lớp method | P0 |
| | [013-10](./BE-CV-TASK-013-10-user-rate-limiter-and-limit-error-data.md) | L1, `CodedData` | P1 |
| | [013-11](./BE-CV-TASK-013-11-reindex-admission.md) | `ReindexAdmission` | P1 |
| | [013-12](./BE-CV-TASK-013-12-gated-agent-relay-and-bypass-guard-test.md) | `GatedAgentRelay`, test bỏ qua cổng | P0 |

## Thứ tự phụ thuộc

```
010-01 ─▶ 010-03 ─▶ 010-04 ─┐
010-02 ─────────────────────┤
010-03 ─▶ 010-05 ─▶ 010-06 ─┴─▶ 010-07 ─▶ 010-08
010-03 ─▶ 011-04 ─▶ 011-06 ; 010-05 ─▶ 011-02 ┐ 011-01 ─▶ 011-02/011-03
                                  011-03 ┘ ─▶ 011-05
011-02+011-04+011-06 ─▶ 011-07, 011-08 (Postgres) ; 011-03+011-04+011-06 ─▶ 011-09, 011-10 (MySQL)
011-07..011-10 ─▶ 011-11 ─▶ 011-12 ─▶ 011-13

012-01 ─▶ 012-04 ; 012-02 (cần G0 SOL-020) ; 012-03 ; 011-06 ─▶ 012-05 ─▶ 012-06 ; 011-07/09 ─▶ 012-07
012-08 ─▶ 012-09 ; 012-04+012-08 ─▶ 012-10 ─▶ 012-11 (cần 012-05, 012-09)

013-01 ─▶ 013-04 ; 013-02 (song song) ─▶ 013-04 ; 010-07 ─▶ 013-03
011-06/07/09 ─▶ 013-05 ; 012-04 ─▶ 013-06 ; 013-03+013-04+013-05+013-06 ─▶ 013-07
013-08 (độc lập) ; 010-02+010-03 ─▶ 013-09 ─▶ 013-10, 013-12 ; 011-08/10+013-09+013-06 ─▶ 013-11
013-12 ─▶ tích hợp vào 012-10
```

Có thể song song: `010-02` với `010-03`; `010-04` với `010-05`; `011-02` với `011-03`; `011-07` với `011-09`; `012-03` với `012-04`; `013-02` và `013-08` với mọi task.

## Ghi chú

- **Số migration:** `0001` (010), `0002` (011). Trước khi thêm file, `ls migrations/postgres`; CR sau (`0003+`) đặt số sau.
- **RLS:** test cô lập tenant bằng role `NOSUPERUSER NOBYPASSRLS` (compose dev dùng superuser `orca`).
- **MySQL:** không `RETURNING`, không RLS; mọi truy vấn có `tenant_id`; `` `trigger` `` có nháy ngược; `CHECK` cần 8.0.16.
- **Proto:** `buf lint`/`buf breaking` gọi trực tiếp (không `make proto-lint`); `012-02` cần SOL-020 (G0).
- **Quy tắc dự án:** chạy `gitnexus_impact` trước khi sửa symbol có sẵn (chỉ `010-02` sửa mã có sẵn: `common/apperrors`); `detect_changes` trước commit; không `max-lines` disable; tên file cụ thể (không `helpers|utils|common|misc`).
- **Sau feature này:** `code-intel-graph-pipeline` (SOL-020…024) và `code-intel-gateway` (SOL-040) tiếp nối.
