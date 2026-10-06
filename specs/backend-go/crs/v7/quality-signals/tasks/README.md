# backend-go Tasks: Quality Signals (v7)

Task thực thi của các solution trong [`../solutions/`](../solutions/README.md). Tất cả `Status: [ ] TODO`, chưa chạy test nào. Hợp đồng nguồn: [`CONTRACT-codeintel-*`](../../CONTRACT-codeintel-proto-and-data-map.md). CR-CV-081 và 084 chỉ có phần agent; CR-CV-094 không có solution.

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-080 | [080-01](./BE-CV-TASK-080-01-proto-index-basis.md) | Proto `IndexBasis` | P0 |
| | [080-02](./BE-CV-TASK-080-02-infra-fleet-status-changed-payload.md) | Payload `statusChanged` (infra-fleet) | P0 |
| | [080-03](./BE-CV-TASK-080-03-index-refresh-planner.md) | `PlanRefresh` thuần | P0 |
| | [080-04](./BE-CV-TASK-080-04-reindex-job-agent-done-slot-repository.md) | Slot debounce trên `reindex_jobs` | P0 |
| | [080-05](./BE-CV-TASK-080-05-auto-refresh-index-usecase-and-slot-dispatcher.md) | Use case + bộ phát slot | P0 |
| | [080-06](./BE-CV-TASK-080-06-agent-status-consumer-and-wiring.md) | Consumer + cấu hình + wiring | P0 |
| | [080-07](./BE-CV-TASK-080-07-replan-after-reindex-finished-and-metrics.md) | Re-plan sau `reindex.finished`, metric | P1 |
| | [080-08](./BE-CV-TASK-080-08-hint-agent-turn-finished-rpc.md) | `HintAgentTurnFinished` | P1 |
| | [080-09](./BE-CV-TASK-080-09-auto-refresh-integration-tests.md) | Tích hợp NATS + hai dialect | P1 |
| BE-CV-SOL-082 | [082-01](./BE-CV-TASK-082-01-proto-quality-run-and-finding.md) | Proto quality | P0 |
| | [082-02](./BE-CV-TASK-082-02-migration-0003-quality-runs-and-findings.md) | Migration `0003` | P0 |
| | [082-03](./BE-CV-TASK-082-03-quality-domain-and-finding-validation.md) | Domain, kiểm tra | P0 |
| | [082-04](./BE-CV-TASK-082-04-quality-repositories-postgres.md) | Repository Postgres | P0 |
| | [082-05](./BE-CV-TASK-082-05-quality-repositories-mysql.md) | Repository MySQL | P0 |
| | [082-06](./BE-CV-TASK-082-06-quality-repository-contract-suite.md) | Bộ hợp đồng hai dialect | P0 |
| | [082-07](./BE-CV-TASK-082-07-agent-quality-result-decoding.md) | Giải mã kết quả agent | P0 |
| | [082-08](./BE-CV-TASK-082-08-ingest-quality-run-usecase.md) | `IngestQualityRun` | P0 |
| | [082-09](./BE-CV-TASK-082-09-quality-maintenance-and-wiring.md) | Bảo trì + wiring | P1 |
| BE-CV-SOL-083 | [083-01](./BE-CV-TASK-083-01-proto-coverage-report.md) | Proto coverage | P1 |
| | [083-02](./BE-CV-TASK-083-02-migration-0005-coverage-reports.md) | Migration `0005` | P1 |
| | [083-03](./BE-CV-TASK-083-03-coverage-domain-and-payload-trimming.md) | Domain, cắt payload | P1 |
| | [083-04](./BE-CV-TASK-083-04-coverage-repository-postgres.md) | Repository Postgres | P1 |
| | [083-05](./BE-CV-TASK-083-05-coverage-repository-mysql.md) | Repository MySQL | P1 |
| | [083-06](./BE-CV-TASK-083-06-coverage-repository-contract-suite.md) | Bộ hợp đồng hai dialect | P1 |
| | [083-07](./BE-CV-TASK-083-07-ingest-coverage-usecase.md) | `IngestCoverage` | P1 |
| | [083-08](./BE-CV-TASK-083-08-estimated-coverage-from-change-overlay.md) | Ước lượng | P1 |
| | [083-09](./BE-CV-TASK-083-09-get-coverage-rpc-and-retention.md) | `GetCoverage` + bảo trì | P1 |
| BE-CV-SOL-086-scm-commit-checks | [086-01](./BE-CV-TASK-086-01-proto-list-commit-checks.md) | Proto `ListCommitChecks` | P1 |
| | [086-02](./BE-CV-TASK-086-02-commit-check-domain-and-rate-limit-error.md) | Domain, lỗi rate limit | P1 |
| | [086-03](./BE-CV-TASK-086-03-list-commit-checks-usecase-and-provider-port.md) | Use case + cổng | P1 |
| | [086-04](./BE-CV-TASK-086-04-github-commit-checks-adapter.md) | Adapter GitHub | P1 |
| | [086-05](./BE-CV-TASK-086-05-github-check-annotations-and-steps.md) | Annotations/steps | P1 |
| | [086-06](./BE-CV-TASK-086-06-gitlab-pipeline-jobs-adapter.md) | Adapter GitLab | P1 |
| | [086-07](./BE-CV-TASK-086-07-grpc-handler-wiring-and-capability-matrix.md) | Handler + wiring | P1 |
| BE-CV-SOL-086-ci-run-merge-and-comparison | [086-08](./BE-CV-TASK-086-08-proto-ci-comparison-and-refresh.md) | Proto CI | P1 |
| | [086-09](./BE-CV-TASK-086-09-ci-run-mapping-and-finding-rules.md) | Ánh xạ, dựng run/finding | P1 |
| | [086-10](./BE-CV-TASK-086-10-ci-local-comparison-relations.md) | `CompareLocalAndCI` | P1 |
| | [086-11](./BE-CV-TASK-086-11-scm-client-and-hosted-review-locator.md) | Client scm, định vị PR | P1 |
| | [086-12](./BE-CV-TASK-086-12-ci-run-repository-upsert-and-replace.md) | Repository run CI | P1 |
| | [086-13](./BE-CV-TASK-086-13-refresh-ci-run-usecase.md) | `RefreshCiRun` | P1 |
| | [086-14](./BE-CV-TASK-086-14-refresh-ci-run-rpc-and-comparison-service.md) | RPC + `CiComparisonService` | P1 |
| | [086-15](./BE-CV-TASK-086-15-ci-config-metrics-and-wiring.md) | Cấu hình, metric, wiring | P2 |
| | [086-16](./BE-CV-TASK-086-16-ci-merge-integration-tests.md) | Tích hợp hai dialect | P1 |
| BE-CV-SOL-091 | [091-01](./BE-CV-TASK-091-01-security-profile-visibility-policy.md) | Chính sách hiển thị | P2 |
| | [091-02](./BE-CV-TASK-091-02-profile-gate-wiring-start-and-runnable.md) | `ProfileGate` | P2 |
| | [091-03](./BE-CV-TASK-091-03-security-finding-sanitizer.md) | Sanitizer finding | P2 |
| | [091-04](./BE-CV-TASK-091-04-secret-canary-leak-tests.md) | Canary rò rỉ | P2 |
| | [091-05](./BE-CV-TASK-091-05-security-scan-audit-and-scope-permission.md) | Audit, quyền, lỗi môi trường | P2 |
| | [091-06](./BE-CV-TASK-091-06-security-flag-integration-tests.md) | Tích hợp cờ | P2 |

## Thứ tự phụ thuộc

```
080-01 ─▶ 080-03 ─▶ 080-04 ─▶ 080-05 ─▶ 080-06 ─▶ 080-07 ─▶ 080-09 ; 080-02 độc lập; 080-08 sau 080-05
080-01 ─▶ 082-01 ─▶ 082-02 ─▶ 082-04 ┐
          082-03 ───────────────────┼─▶ 082-06 ; 082-05 ┘ ; 082-07 ─▶ 082-08 ─▶ 082-09
082-01 ─▶ 083-01 ; 082-02 ─▶ 083-02 ─▶ 083-04/05 ─▶ 083-06 ; 083-03 ; 082-08 + 083-0x ─▶ 083-07 ─▶ 083-09 ; 083-08 ─▶ 083-09
086-01 ─▶ 086-03 ─▶ 086-04 ─▶ 086-05 ; 086-02 ─▶ 086-03 ; 086-06 ; (086-01,03,04,06) ─▶ 086-07
082-04/05 ─▶ 086-12 ; 086-09 ─▶ 086-10 ; (086-09,10,11,12) ─▶ 086-13 ─▶ 086-14 ─▶ 086-15 ─▶ 086-16
082-08 ─▶ 091-03 ─▶ 091-04 ; 091-01 ─▶ 091-02 (cần SOL-085) ─▶ 091-05 ─▶ 091-06
```

## Ghi chú

- **Số migration:** `0003` (SOL-082) và `0005` (SOL-083) là số dự kiến của C-DM §4.2; chạy `ls migrations/postgres` trước khi đặt số.
- **SOL-085:** dòng `rpc` của `GetCoverage`, `RefreshCiRun` và điểm cắm `ProfileGate`/`CiComparisonService` cần `BE-CV-SOL-085`; task ghi rõ.
- **Test hai dialect:** ma trận CI `dialect: [postgres, mysql]` của `backend-go-code-intel-service.yml` (SOL-010); Postgres RLS kiểm với role `NOSUPERUSER NOBYPASSRLS`.
- **scm-integration-service:** workflow `backend-go-scm-integration-service.yml` đã có ma trận hai dialect; SOL-086-scm không thêm DB.
