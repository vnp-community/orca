# Solutions backend: solution-analysis (v6)

> ✅ Đã triển khai (kiểm chứng 2026-10-08): 11/11 task xong, ghi chú hợp nhất ở [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md). Phạm vi backend của CR-REQ-007 và CR-REQ-008 trong `request-service` (service mới, chưa có mã lúc soạn). Hợp đồng chung: [README v6](../../../../../../docs/crs/v6/README.md), mục 8 thắng mục 3.

## Bảng CR, Solution, Task

| CR | Solution | Task | Trạng thái | Ghi chú |
|----|----------|------|------------|---------|
| [CR-REQ-007](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md) | [BE-REQ-SOL-007](./BE-REQ-SOL-007-solution-generation-options-and-selection.md) | TASK-REQ-007-01 đến 06 | ✅ 6/6 | `analysis_runs`, `GenerateSolution`, `ChooseSolutionOption`, handler `solution` |
| [CR-REQ-008](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md) | [BE-REQ-SOL-008](./BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) | TASK-REQ-008-01 đến 05 | ✅ 5/5 | `AgentReadonlyRunner`; phần agent do CR-REQ-033 |

## Thứ tự phụ thuộc

```
CR-REQ-002, 003 ─▶ SOL-009 (Approval) ─┐
                                        ├─▶ SOL-007 ─▶ SOL-008
CR-REQ-005 (loại đã xác nhận) ──────────┘
```
SOL-007 có thể bắt đầu phần domain, migration, repository, adapter AI (task 01 đến 04) trước khi SOL-009 xong; task 05 và 06 cần `OpenApproval` và `SubjectHandler` (hoặc cổng no-op).

## Quyết định chung

| # | Quyết định | Lý do |
|---|-----------|-------|
| A1 | Một RPC sinh `GenerateSolution`; `kind` suy ra từ registry CR-REQ-003 | README 3.6 không có RPC riêng |
| A2 | Mọi lần sinh là một dòng `analysis_runs` có lease 90s, heartbeat 30s, quét 30s | Tránh mất việc khi restart (CR-TG-008) |
| A3 | Digest Approval tính trên JSON chuẩn tắc (không dựa văn bản JSONB/JSON) | Hai DB lưu JSON khác nhau |
| A4 | `solution` dùng `ai.complete`; ba `kind` còn lại dùng `agent.execPrompt` trên `repo_path`, `trustPreset="default"` | Solution là thiết kế văn bản; còn lại cần đọc code |
| A5 | Kiểm "repo bị sửa" chỉ khi `ResolveConnection` trả `worktree_id`; so `(branch, files)`, chưa so `HEAD` | `GetStatus` nhận `worktree_id` và không trả `HEAD` (đã đọc proto) |
| A6 | Lỗi không tự về backlog; hotfix tự duyệt có `auto=true` | Quyền trả backlog thuộc người |

## Số migration

Bảng `analysis_runs` ở `0006` (R1a, đã đánh số lại). Phần còn thiếu của solution này nằm ở `0030_solution_analysis` (dải `0030`..`0039` cấp cho CR-007/008/026), cùng số hai dialect. SOL-008 không có migration riêng.

## Mâu thuẫn và điểm chưa kiểm chứng

- `GetStatus` không có `HEAD` (CR-REQ-008 giả định có thể có): xem SOL-008 mục 1.
- `options` mặc định `'[]'` ở CR-REQ-002, tài liệu là đối tượng; `min_options` của `refactor`; `content_ref`: xem câu hỏi mở của SOL-007.
- `ai.complete` và `agent.execPrompt` thật chưa chạy; chế độ chỉ đọc chưa ép được.
