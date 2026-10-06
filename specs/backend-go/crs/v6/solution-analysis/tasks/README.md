# Tasks backend: solution-analysis (v6)

> 📋 Proposed. Mỗi task 0,5 đến 2 ngày. Đường dẫn tương đối tới `backend-go/services/request-service/` (mới) trừ khi ghi khác.

## Bảng Solution, Task

| Solution | Task | Nội dung | Ưu tiên |
|----------|------|----------|---------|
| [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) | [TASK-REQ-007-01](./TASK-REQ-007-01-analysis-runs-migration.md) | Migration `analysis_runs` hai dialect, domain `AnalysisRun` | P0 |
| | [TASK-REQ-007-02](./TASK-REQ-007-02-solution-domain-options-validation-digest.md) | `Solution`, `SolutionOptions.Validate`, trích JSON, digest chuẩn tắc | P0 |
| | [TASK-REQ-007-03](./TASK-REQ-007-03-analysis-run-and-solution-repositories.md) | Repository run và solution, lease, `SKIP LOCKED` | P0 |
| | [TASK-REQ-007-04](./TASK-REQ-007-04-ai-completion-relay-and-prompt.md) | Adapter `ai.complete`, ngữ cảnh dự án, prompt | P0 |
| | [TASK-REQ-007-05](./TASK-REQ-007-05-generate-solution-usecase-and-worker.md) | `GenerateSolution`, worker, phục hồi | P0 |
| | [TASK-REQ-007-06](./TASK-REQ-007-06-choose-option-approval-handler-and-grpc.md) | Chọn phương án, handler Approval, proto, gRPC | P0 |
| [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) | [TASK-REQ-008-01](./TASK-REQ-008-01-analysis-document-domain-and-redaction.md) | Ba tài liệu, kiểm schema, che bí mật | P1 |
| | [TASK-REQ-008-02](./TASK-REQ-008-02-agent-prompt-relay-and-repo-probe.md) | Adapter `agent.execPrompt`, `RepoStateProbe` | P1 |
| | [TASK-REQ-008-03](./TASK-REQ-008-03-run-agent-readonly-analysis-usecase.md) | `RunAgentReadonlyAnalysis`, giới hạn đồng thời, prompt | P1 |
| | [TASK-REQ-008-04](./TASK-REQ-008-04-findings-answer-approval-handlers.md) | Handler `findings`, `answer`, hoàn tất sau phân tích | P1 |
| | [TASK-REQ-008-05](./TASK-REQ-008-05-readonly-analysis-integration-and-agent-interface.md) | Test tích hợp, README giới hạn, giao diện agent | P1 |

## Thứ tự phụ thuộc

```
TASK-REQ-007-01 ──▶ 007-03 ──▶ 007-05 ──▶ 007-06
TASK-REQ-007-02 ──┬─▶ 007-03        ▲        ▲
                  └─▶ 007-04 ───────┘        │
                                    (cần TASK-REQ-009-04, 009-06)
007-02 ─▶ 008-01 ─┐
007-04 ─▶ 008-02 ─┼─▶ 008-03 (cần 007-05) ─▶ 008-04 (cần 007-06, 009-06) ─▶ 008-05
```
Làm song song được: 007-01 với 007-02; 007-04 với 007-03; 008-01 với 008-02.

## Ghi chú

- Mọi task migration phải đọc số thật ở `request-service/migrations/*` (service chưa có lúc soạn).
- SOL-008 nhận kết quả của SOL-007 (bảng run, `GenerateSolution`); không bắt đầu 008-03 trước 007-05.
- Kiểm thử thật `ai.complete` và `agent.execPrompt` (chưa chạy) thuộc CR-REQ-025.
- Tham chiếu tiến (không phụ thuộc): CR-REQ-028 (Clarification) và CR-REQ-033 (agent chỉ đọc).
