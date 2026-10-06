# backend-go Solutions: Quality Gate (v7)

**CRs:** [docs/crs/v7/quality-gate](../../../../../../docs/crs/v7/quality-gate/README.md)
**Hợp đồng (nguồn sự thật):** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md), [CONTRACT-codeintel-agent-rpc.md](../../CONTRACT-codeintel-agent-rpc.md), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md); README v7 mục 8 thắng mục 3, **hợp đồng thắng CR**
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

> 📋 Proposed. Chưa triển khai, chưa chạy build/test/analyze nào. `code-intel-service` và proto `codeintel` **chưa tồn tại** (đã `ls` `backend-go/services`, `backend-go/proto/orca`).

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-CV-085](../../../../../../docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md) | [BE-CV-SOL-085-quality-gate-evaluator-and-profiles](./BE-CV-SOL-085-quality-gate-evaluator-and-profiles.md) | `code-intel-service`, `codeintel_quality_gate.proto`, OPA | Large | `BE-CV-TASK-085-01` đến `-07` |
| CR-CV-085 | [BE-CV-SOL-085-waivers-and-trend](./BE-CV-SOL-085-waivers-and-trend.md) | `code-intel-service` (miễn trừ, xu hướng, `gate_changed`) | | `BE-CV-TASK-085-08` đến `-13` |
| [CR-CV-089](../../../../../../docs/crs/v7/quality-gate/CR-CV-089-agent-provenance-and-claim-reconciliation.md) | [BE-CV-SOL-089-agent-turn-provenance](./BE-CV-SOL-089-agent-turn-provenance.md) | `code-intel-service` (`agent_turns`, đối chiếu) | Large | `BE-CV-TASK-089-01` đến `-07` |
| [CR-CV-090](../../../../../../docs/crs/v7/quality-gate/CR-CV-090-exportable-review-report.md) | [BE-CV-SOL-090-review-report-model](./BE-CV-SOL-090-review-report-model.md) | `code-intel-service` (mô hình báo cáo) | Medium | `BE-CV-TASK-090-01` đến `-06` |
| [CR-CV-092](../../../../../../docs/crs/v7/quality-gate/CR-CV-092-requirement-traceability.md) | [BE-CV-SOL-092-requirement-trace](./BE-CV-SOL-092-requirement-trace.md) | `code-intel-service` + client `project`/`task`-service | Large | `BE-CV-TASK-092-01` đến `-08` |
| [CR-CV-093](../../../../../../docs/crs/v7/quality-gate/CR-CV-093-ai-review-summary.md) | [BE-CV-SOL-093-ai-review-summary](./BE-CV-SOL-093-ai-review-summary.md) | `code-intel-service` + `infra-fleet-service` (timeout `ai.complete` 120 s) | Medium | `BE-CV-TASK-093-01` đến `-07` |
| [CR-CV-095](../../../../../../docs/crs/v7/quality-gate/CR-CV-095-review-quality-telemetry.md) | **Phần BE nằm ở `BE-CV-SOL-071-metrics-tracing-and-budgets`** (hợp đồng §8.2); feature này không có solution BE cho 095 | — | Small | — |

Đối ứng FE (hợp đồng §8.2): `FE-CV-SOL-085-source-control-quality-notice`, `FE-CV-SOL-089-agent-turn-recorder`, `FE-CV-SOL-090-review-report-export`, `FE-CV-SOL-092-requirement-trace-view`, `FE-CV-SOL-093-ai-summary-panel`, `FE-CV-SOL-095-review-telemetry`; scorecard `FE-CV-SOL-087-*` (feature `quality-visualization`). AG: không có solution cho feature này.

## Re-verify (đối chiếu CR/hợp đồng với mã thật, 2026-10-06)

| Khẳng định | Kết quả đọc | Lệch? |
|---|---|---|
| `code-intel-service`, proto `codeintel`, `policy/orca-authz/code_intel.rego` có sẵn | Không (`ls`) | Mọi thứ là "(mới)", phụ thuộc BE-CV-SOL-010/011/013 |
| `GetTask` kiểm grant | `usecase/get_task.go` chỉ kiểm tenant; `ResolvePermission` có kiểm | Task 092-05: `ResolvePermission` trước `GetTask` |
| `project-service.GetWorktree` an toàn theo id | Không lọc tenant (`get_worktree.go`, `worktree_repository.go:114`) | Dùng `ListWorktrees(project_id)` (SOL-092 L1) |
| `execTimeoutForMethod` chỉ ngoại lệ `agent.execPrompt` | Đúng (`client.go:412`) | SOL-093 thêm `ai.complete` 120 s |
| `FindTaskBySource` | Không có | Nhánh `derived` rỗng (SOL-092 L6) |
| `request-service`, `GetRequestCoverage` | Không có | Chỉ khung + fake (SOL-092) |
| Agent có method liệt kê thông điệp commit trong hợp đồng codeintel | Không; `git.history` có ở agent nhưng ngoài §4–5 | SOL-092 L2/Q1 |
| `common/apperrors` đủ Kind | Thiếu `ResourceExhausted`, `Unavailable` (thuộc CR-010) | Không đụng ở feature này |

## Thứ tự thực thi và phụ thuộc

```
BE-CV-SOL-010/011/013 (nền) ─ 036, 037, 082 ─▶ SOL-085-evaluator ─▶ SOL-085-waivers-and-trend
SOL-085-evaluator ─┬▶ SOL-089 (cần `agent_turn.recorded` ↔ SOL-085-waivers consumer)
                   ├▶ SOL-090
                   ├▶ SOL-092
                   └▶ SOL-093 (+ sửa infra-fleet, độc lập)
```
Nền của cả feature là SOL-085-evaluator (proto `codeintel_quality_gate.proto`, `QualityGateServer`, OPA/cờ, `0004`). SOL-089/090/092/093 độc lập nhau.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Một `QualityGateServer` (struct), phương thức ở file riêng theo CR | Một `Register`; solution độc lập nhau |
| F2 | Dòng `rpc` trong `codeintel_quality_gate.proto` chỉ thêm khi message của CR đã có | Hợp đồng §2 "RPC chưa có message thì không khai báo" |
| F3 | Chế độ chỉ báo; `mode=block` chưa lưu được | O9, PQ |
| F4 | `unknown` là trạng thái thật, ưu tiên cao hơn `warn` | H7 |
| F5 | Lời agent và tóm tắt AI **không bao giờ** vào cổng | O13 |
| F6 | Port hoá mọi nguồn dữ liệu của service/CR khác, test bằng fake | CR liên quan còn là đề xuất |
| F7 | Mọi truy vấn có `tenant_id`, test AST + test cô lập hai dialect | H10, §8.3 |
| F8 | Ngưỡng/hạn mức là "giá trị khởi điểm chưa hiệu chỉnh" | Chưa có dữ liệu thật |

## Điểm hợp đồng thiếu/mâu thuẫn (cần người duyệt; **không** tự sửa hợp đồng)

1. Ai sở hữu `RunnableProfileLister` (danh sách `runnable_profiles`) và thông điệp commit cho truy vết (không có method agent trong §4–5).
2. `QualityProfileDefinition` thiếu điều kiện theo tệp đổi (đề xuất `whenChangedPaths`).
3. `ExportReviewReport` dưới `QUALITY_GATE_DISABLED` làm mất xuất báo cáo một phần (CR Q4).
4. Mô hình `ReviewReportModel` không có chỗ cho `include_people`, `include_waiver_reasons`; bảng mã `warnings[]` chưa có.
5. Kiểu `before` của `ListAgentTurns`; `delta` của `GetQualityTrend`; consumer `index.changed` của 089 không rõ mục đích.
6. Biến `CODEINTEL_WAIVER_MAX_DAYS`, `CODEINTEL_AGENT_TURN_*_DAYS`, `CODEINTEL_AI_REVIEW_MODEL_ALLOWLIST`, `CODEINTEL_REQUIREMENT_SOURCE_REQUEST_ENABLED` chưa nằm trong bảng env §6.2.
7. Thêm `source` vào khoá `quality_trend_points` (O-10) chờ duyệt.
