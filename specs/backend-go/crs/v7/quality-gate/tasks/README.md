# backend-go Tasks: Quality Gate (v7)

Task thực thi của sáu solution trong [`../solutions/`](../solutions/README.md). Tất cả `Status: [ ] TODO`, chưa chạy test nào. `NN` tăng liên tục theo từng CR (CR-085 có hai solution nên `01–13`).

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-085-quality-gate-evaluator-and-profiles | [085-01](./BE-CV-TASK-085-01-migration-0004-quality-gate-two-dialects.md) | Migration `0004_quality_gate` | P0 |
| | [085-02](./BE-CV-TASK-085-02-proto-quality-gate-service.md) | Proto `codeintel_quality_gate.proto` | P0 |
| | [085-03](./BE-CV-TASK-085-03-quality-profile-domain-and-builtin-default.md) | Domain profile + `orca-default` | P0 |
| | [085-04](./BE-CV-TASK-085-04-evaluate-gate-pure-function.md) | `EvaluateGate` + chọn run | P0 |
| | [085-05](./BE-CV-TASK-085-05-quality-profile-repositories-and-usecases.md) | Repository + Save/GetQualityProfile | P0 |
| | [085-06](./BE-CV-TASK-085-06-evaluate-quality-gate-usecase-and-get-handler.md) | `EvaluateQualityGate` + `GetQualityGate` | P0 |
| | [085-07](./BE-CV-TASK-085-07-opa-quality-actions-flag-gating-and-audit.md) | OPA, cổng cờ, audit | P0 |
| BE-CV-SOL-085-waivers-and-trend | [085-08](./BE-CV-TASK-085-08-waiver-domain-repository-and-waive-usecase.md) | Miễn trừ | P0 |
| | [085-09](./BE-CV-TASK-085-09-wire-waivers-and-dismissals-into-gate.md) | Nối waiver/dismissal vào cổng | P0 |
| | [085-10](./BE-CV-TASK-085-10-trend-point-recording-and-consumers.md) | Ghi điểm xu hướng + consumer | P0 |
| | [085-11](./BE-CV-TASK-085-11-gate-changed-outbox-event.md) | Sự kiện `gate_changed` | P1 |
| | [085-12](./BE-CV-TASK-085-12-get-quality-trend-and-maintenance.md) | `GetQualityTrend` + bảo trì | P1 |
| | [085-13](./BE-CV-TASK-085-13-tenant-isolation-contract-fixtures-and-dialect-matrix.md) | Cô lập tenant, fixture, CI hai dialect | P1 |
| BE-CV-SOL-089-agent-turn-provenance | [089-01](./BE-CV-TASK-089-01-migration-0006-agent-turns.md) | Migration `0006` | P1 |
| | [089-02](./BE-CV-TASK-089-02-proto-agent-turn.md) | Proto agent turn | P1 |
| | [089-03](./BE-CV-TASK-089-03-command-summary-claims-and-merge-domain.md) | Chuẩn hoá lệnh, claim, merge | P1 |
| | [089-04](./BE-CV-TASK-089-04-agent-turn-repositories-two-dialects.md) | Repository hai dialect | P1 |
| | [089-05](./BE-CV-TASK-089-05-record-agent-turn-usecase-and-handler.md) | `RecordAgentTurn` | P1 |
| | [089-06](./BE-CV-TASK-089-06-reconcile-claims-with-independent-runs.md) | Đối chiếu độc lập | P1 |
| | [089-07](./BE-CV-TASK-089-07-list-get-turns-maintenance-and-tests.md) | List/Get, bảo trì, fixture | P1 |
| BE-CV-SOL-090-review-report-model | [090-01](./BE-CV-TASK-090-01-proto-review-report.md) | Proto | P1 |
| | [090-02](./BE-CV-TASK-090-02-report-model-builder-and-digest.md) | Builder + digest | P1 |
| | [090-03](./BE-CV-TASK-090-03-redaction-and-mermaid-text-generation.md) | Che + Mermaid | P1 |
| | [090-04](./BE-CV-TASK-090-04-export-review-report-usecase.md) | Use case | P1 |
| | [090-05](./BE-CV-TASK-090-05-export-handler-authz-audit.md) | Handler, quyền, audit | P1 |
| | [090-06](./BE-CV-TASK-090-06-report-golden-determinism-and-fixtures.md) | Golden, tái lập | P2 |
| BE-CV-SOL-092-requirement-trace | [092-01](./BE-CV-TASK-092-01-migration-0007-requirement-trace-links.md) | Migration `0007` | P2 |
| | [092-02](./BE-CV-TASK-092-02-proto-requirement-trace.md) | Proto | P2 |
| | [092-03](./BE-CV-TASK-092-03-criteria-extractor-and-name-tokenizer.md) | Rút tiêu chí, tokenizer | P2 |
| | [092-04](./BE-CV-TASK-092-04-evidence-matcher-and-state-machine.md) | Bằng chứng + `state` | P2 |
| | [092-05](./BE-CV-TASK-092-05-requirement-providers-and-cross-service-clients.md) | Provider + client liên service | P2 |
| | [092-06](./BE-CV-TASK-092-06-trace-link-repositories-and-confirm-link-usecases.md) | Repository link + Confirm/Link | P2 |
| | [092-07](./BE-CV-TASK-092-07-get-requirement-trace-usecase.md) | `GetRequirementTrace` | P2 |
| | [092-08](./BE-CV-TASK-092-08-trace-handlers-authz-limits-and-fixtures.md) | Handler, quyền, fixture | P2 |
| BE-CV-SOL-093-ai-review-summary | [093-01](./BE-CV-TASK-093-01-infra-fleet-ai-complete-timeout-120s.md) | infra-fleet: `ai.complete` 120 s | P2 |
| | [093-02](./BE-CV-TASK-093-02-proto-ai-review.md) | Proto | P2 |
| | [093-03](./BE-CV-TASK-093-03-input-builder-redaction-and-sanitizer.md) | Đầu vào, che, làm sạch | P2 |
| | [093-04](./BE-CV-TASK-093-04-prompt-frame-and-output-validator.md) | Khung nhắc + kiểm đầu ra | P2 |
| | [093-05](./BE-CV-TASK-093-05-generate-review-summary-usecase-cache-and-quota.md) | Use case, cache, hạn mức | P2 |
| | [093-06](./BE-CV-TASK-093-06-handler-authz-audit-and-gate-isolation-test.md) | Handler, quyền, test tách cổng | P2 |
| | [093-07](./BE-CV-TASK-093-07-injection-corpus-fixtures-and-manual-evaluation.md) | Fixture injection, đánh giá tay | P2 |

CR-CV-095 phần BE: `BE-CV-SOL-071-metrics-tracing-and-budgets` (feature `quality-rollout`), không có task ở đây.

## Thứ tự phụ thuộc

```
085-01, 085-02 ─▶ 085-03 ─▶ 085-04 ─▶ 085-06 ◀─ 085-05 (cần 085-01, 085-03)
085-02 ─▶ 085-07 (cần SOL-013)
085-06 + 085-07 ─▶ 085-08 ─▶ 085-09 ─▶ 085-10 ─▶ 085-11, 085-12 ─▶ 085-13
085-02 ─▶ 089-02, 090-01, 092-02, 093-02 (proto, song song)
089-01 ─▶ 089-04 ◀─ 089-03 ◀─ 089-02 ; 089-04 + 085-07 ─▶ 089-05 ─▶ 089-06 ─▶ 089-07
090-01 ─▶ 090-02 ─▶ 090-03 ─▶ 090-04 (cần 085-06) ─▶ 090-05 ─▶ 090-06
092-01 ; 092-02 ─▶ 092-03 ─▶ 092-04 ; 092-03 ─▶ 092-05 ; 092-01+092-05 ─▶ 092-06 ; 092-04+05+06 ─▶ 092-07 ─▶ 092-08
093-01 (độc lập) ; 093-02 ─▶ 093-03 ─▶ 093-04 ─▶ 093-05 (cần 093-01, 085-06) ─▶ 093-06 ─▶ 093-07
```
Song song được: 085-01 với 085-02; 085-03 với 085-05 (sau 085-01); toàn bộ solution 089/090/092/093 sau 085-02/085-07; 093-01 bất cứ lúc nào.

## Ghi chú

- **Số migration** (`0004`, `0006`, `0007`) có thể dịch khi merge: chạy `ls migrations/postgres` trước.
- **RLS:** Postgres thật theo mẫu `mcp-service` (`withTenantTx`); test dùng role `NOSUPERUSER NOBYPASSRLS` (dev compose dùng superuser nên RLS vô hiệu); MySQL lọc `tenant_id` ở mọi truy vấn, CHECK cần ≥ 8.0.16.
- **`buf`:** gọi trực tiếp `buf lint`/`buf breaking`; không dùng `make proto-lint` (có `|| true`).
- **Không** `max-lines` disable; tên file theo khái niệm; SSH/GitLab/Git 2.25 theo `AGENTS.md` (solution này không thêm lệnh git; nguồn trailer commit chờ quyết định hợp đồng).
- **Phụ thuộc chéo:** BE-CV-SOL-010/011/013 (nền), 036/037/082/083/086 (nguồn dữ liệu), 040-quality-channels (kênh gateway), 024 (đẩy push `gateChanged`).
