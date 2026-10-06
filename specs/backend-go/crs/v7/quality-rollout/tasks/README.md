# quality-rollout: tasks (backend-go, v7)

> 📋 Proposed. Mọi task `Status: [ ] TODO`; chưa task nào được làm hay chạy. Đã đối chiếu với file thật của `api-gateway`, `infra-fleet-service`, `mcp-service`, `common`, `deploy`, `ci`, workflows; phần thuộc `code-intel-service` dựa trên CR và hợp đồng vì service chưa có.

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| [BE-CV-SOL-070](../solutions/BE-CV-SOL-070-collector-golden-contract.md) | [070-01](./BE-CV-TASK-070-01-agent-results-golden-layout-and-hygiene-scan.md) | Bố cục `testdata/agent-results`, manifest, quét vệ sinh | P0 | SOL-010, AG-070 |
| | [070-02](./BE-CV-TASK-070-02-collector-normalizes-agent-golden.md) | Collector chuẩn hoá tệp vàng, bỏ `perf` | P0 | 070-01, SOL-021, 020 |
| | [070-03](./BE-CV-TASK-070-03-schema-tolerance-provenance-and-symbolref-key-vectors.md) | Dung sai schema, phiên bản, vector khoá `SymbolRef` | P0 | 070-02 |
| | [070-04](./BE-CV-TASK-070-04-agent-error-golden-mapping.md) | Tệp vàng lỗi và ánh xạ `Kind`/trailer | P0 | 070-01, SOL-023 |
| | [070-05](./BE-CV-TASK-070-05-method-coverage-matrix-for-golden.md) | Ma trận bao phủ method và mã lỗi | P1 | 070-01, 070-04 |
| | [070-06](./BE-CV-TASK-070-06-code-intel-contract-workflow.md) | Workflow `code-intel-contract.yml` | P0 | 070-01..05, AG-070 |
| [BE-CV-SOL-071](../solutions/BE-CV-SOL-071-metrics-tracing-and-budgets.md) | [071-01](./BE-CV-TASK-071-01-codeintel-budgets-file-and-budget-checker.md) | `codeintel-budgets.json` và checker | P1 | SOL-010 |
| | [071-02](./BE-CV-TASK-071-02-code-intel-service-metrics-registry.md) | Registry `orca_codeintel_*`, `/metrics` | P1 | SOL-010, 020, 021 |
| | [071-03](./BE-CV-TASK-071-03-collector-perf-block-metrics-and-spans.md) | Khối `perf`: histogram, span con | P1 | 071-02, 070-02, AG-071 |
| | [071-04](./BE-CV-TASK-071-04-gateway-codeintel-metrics-and-root-span.md) | Metrics và span gốc ở gateway, ghép `/metrics` | P1 | SOL-040-foundation |
| | [071-05](./BE-CV-TASK-071-05-infra-fleet-codeintel-metrics-and-relay-span.md) | Metrics và span ở infra-fleet | P1 | SOL-023 |
| | [071-06](./BE-CV-TASK-071-06-trace-chain-and-span-attribute-guard.md) | Chuỗi trace và rào thuộc tính | P1 | 071-03..05 |
| | [071-07](./BE-CV-TASK-071-07-go-benchmarks-and-nightly-bench-workflow.md) | Benchmark Go và workflow đêm | P2 | 071-01, SOL-031 |
| | [071-08](./BE-CV-TASK-071-08-alert-rules-and-performance-guide.md) | Rule cảnh báo, hướng dẫn hiệu năng | P2 | 071-02, 04, 05 |
| | [071-09](./BE-CV-TASK-071-09-quality-gate-server-metrics-and-promotion-queries.md) | Số liệu server CR-095, truy vấn quản trị | P2 | 071-02, SOL-085, 089 |
| [BE-CV-SOL-072](../solutions/BE-CV-SOL-072-security-tests-service-gateway.md) | [072-01](./BE-CV-TASK-072-01-path-attack-vectors-and-service-path-validation.md) | Vector đường dẫn và kiểm ở service | P0 | SOL-030, 012 |
| | [072-02](./BE-CV-TASK-072-02-gateway-args-decoder-fuzz-and-size-limits.md) | Fuzz và giới hạn `decodeCodeIntelArgs` | P0 | SOL-040-foundation, 072-01 |
| | [072-03](./BE-CV-TASK-072-03-service-param-validation-fuzz-and-path-builder.md) | Fuzz tham số service, trình dựng đường dẫn | P0 | 072-01, SOL-012, 021, 030 |
| | [072-04](./BE-CV-TASK-072-04-postgres-rls-nonowner-role-tests.md) | RLS thật bằng vai trò không phải chủ sở hữu | P0 | SOL-011 |
| | [072-05](./BE-CV-TASK-072-05-tenant-scoping-both-dialects-cache-stream-relay.md) | Cô lập tenant hai dialect, cache, stream, relay | P0 | 072-04, SOL-022, 024 |
| | [072-06](./BE-CV-TASK-072-06-permission-matrix-and-uniform-denial.md) | Ma trận quyền 49 RPC, từ chối đồng nhất | P0 | SOL-013, 085, 073-03 |
| | [072-07](./BE-CV-TASK-072-07-secret-canary-pipeline-scan.md) | Quét canary secret | P0 | 073-05, SOL-030 |
| | [072-08](./BE-CV-TASK-072-08-agent-forgery-and-denial-of-service-limits.md) | Agent giả mạo và giới hạn DoS | P1 | SOL-021, 022, 013, 070-04 |
| | [072-09](./BE-CV-TASK-072-09-security-workflow-redteam-and-threat-model.md) | Workflow bảo mật, red-team MCP, threat model | P1 | 072-01..08 |
| [BE-CV-SOL-073](../solutions/BE-CV-SOL-073-settings-flag-and-rollout.md) | [073-01](./BE-CV-TASK-073-01-settings-proto-domain-and-usecases.md) | Proto, domain, use case `GetSettings`/`SetSettings` | P0 | SOL-010, 011, 013 |
| | [073-02](./BE-CV-TASK-073-02-effective-flag-reader-and-env-switches.md) | `EffectiveFlags`, cache 5 s, env | P0 | 073-01 |
| | [073-03](./BE-CV-TASK-073-03-feature-gate-grpc-interceptor.md) | Interceptor `feature_gate` | P0 | 073-02 |
| | [073-04](./BE-CV-TASK-073-04-service-wiring-check-script.md) | Script kiểm đăng ký | P0 | SOL-010 |
| | [073-05](./BE-CV-TASK-073-05-e2e-harness-fakes-and-scenario-e01.md) | Khung e2e T1, fake, E01 | P0 | 070-01, 070-02, 073-03 |
| | [073-06](./BE-CV-TASK-073-06-e2e-read-view-scenarios.md) | E02–E10 | P0 | 073-05, SOL-022, 031, 036 |
| | [073-07](./BE-CV-TASK-073-07-e2e-write-flag-and-stream-scenarios.md) | E05, E11–E14, E18, ma trận RPC | P0 | 073-05, 073-06, 072-06 |
| | [073-08](./BE-CV-TASK-073-08-e2e-ci-matrix-and-nightly-stack-workflow.md) | CI hai dialect, workflow đêm T3/T4 | P1 | 073-05..07 |
| | [073-09](./BE-CV-TASK-073-09-operations-docs-runbook-and-rollback-drill.md) | Tài liệu, runbook, diễn tập quay lui | P1 | 073-01..04, SOL-071, 072 |

## Thứ tự phụ thuộc

```
SOL-070:  070-01 ─▶ 070-02 ─▶ 070-03 ; 070-01 ─▶ 070-04 ─▶ 070-05 ; (01..05) ─▶ 070-06
SOL-073:  073-01 ─▶ 073-02 ─▶ 073-03 ──────────────┐   073-04 (độc lập, sau SOL-010)
                                                   ▼
          070-01,02 ─▶ 073-05 ─▶ 073-06 ─▶ 073-07 ─▶ 073-08 ─▶ 073-09
SOL-071:  071-01 ; 071-02 ─▶ 071-03 ; 071-04, 071-05 ─▶ 071-06 ; 071-02,04,05 ─▶ 071-08 ; 071-02 ─▶ 071-09 ; 071-01 ─▶ 071-07
SOL-072:  072-01 ─▶ 072-02, 072-03 ; 072-04 ─▶ 072-05 ; 073-03 ─▶ 072-06 ; 073-05 ─▶ 072-07 ; 070-04 ─▶ 072-08 ; (01..08) ─▶ 072-09
```

## Ghi chú

- Làm sớm: 073-01..04 (cờ và đăng ký; đợt 3), 071-01 và 073-04 không cần code sản phẩm. Tệp vàng (070) cần `AG-CV-SOL-070` sinh tệp trước.
- Trước khi sửa `healthAndMetricsMux`/`main.go` của `api-gateway` (071-04), `main.go` của `infra-fleet-service` (071-05) hoặc `handler.go` (072-02, `SetReadLimit`), chạy `gitnexus_impact` và báo blast radius (CLAUDE.md của repo).
- Lệnh: `cd backend-go/services/code-intel-service && go test ./... && go test -tags=integration ./internal/adapter/... && go test -tags=e2e ./e2e/...` (cần Docker); gateway: `cd backend-go/services/api-gateway && go test ./internal/adapter/wscompat/... ./cmd/server/...`. **Chưa chạy.**
- Số ngày, ngưỡng, ngân sách, hạn mức là giả định chưa đo (hợp đồng O-15). Mọi test chạm DB có ma trận `dialect: [postgres, mysql]` và test cô lập `tenant_id`. Không `max-lines` disable (AGENTS.md); SSH và GitLab/provider khác: không có phần đặc thù provider ở feature này.
