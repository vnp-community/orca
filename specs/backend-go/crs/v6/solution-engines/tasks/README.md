# Tasks backend: solution-engines (v6)

> 📋 Proposed. Mỗi task 0,5 đến 2 ngày. Đường dẫn tương đối tới `backend-go/services/request-service/` (mới) trừ khi ghi khác. Tất cả `Status: [ ] TODO`, chưa chạy test nào.

## Bảng Solution, Task

| Solution | Task | Nội dung | Ưu tiên |
|----------|------|----------|---------|
| [BE-REQ-SOL-026](../solutions/BE-REQ-SOL-026-openspec-solution-engine.md) | [TASK-REQ-026-01](./TASK-REQ-026-01-engine-domain-profile-and-change-id.md) | Domain `EngineName`, `OpenSpecProfileFor`, `EffectiveEngine`, `NewChangeID`, mã lỗi | P1 |
| | [TASK-REQ-026-02](./TASK-REQ-026-02-engine-migration-and-repositories.md) | Migration `NNNN_solution_engines`, repository `project_engine_settings`, `openspec_changes`, `requests.solution_engine` | P1 |
| | [TASK-REQ-026-03](./TASK-REQ-026-03-tasks-md-parser-and-renderer.md) | Parser và renderer vùng `plan` của `tasks.md`, `TickTask` | P1 |
| | [TASK-REQ-026-04](./TASK-REQ-026-04-engine-readiness-gate.md) | `EngineReadinessGate`, `DevServerExecutor` (`agent.exec`, `agent.execPrompt`) | P1 |
| | [TASK-REQ-026-05](./TASK-REQ-026-05-solution-engine-interface-native-and-pinning.md) | Giao diện `SolutionEngine`, `nativeEngine`, ghim engine, `engine_override` | P1 |
| | [TASK-REQ-026-06](./TASK-REQ-026-06-openspec-engine-generate-analysis.md) | `openspecEngine.GenerateAnalysis`, `ProposalWorkspace`, kiểm phạm vi ghi | P1 |
| | [TASK-REQ-026-07](./TASK-REQ-026-07-openspec-plan-tasks-sync-and-archive.md) | `GeneratePlan`, `TasksMdSyncer`, archive sau `request.completed` | P1 |
| | [TASK-REQ-026-08](./TASK-REQ-026-08-engine-settings-rpc-metrics-and-integration.md) | RPC `Get/SetProjectEngineSettings`, metric, audit, tích hợp hai dialect | P1 |

## Thứ tự phụ thuộc

```
TASK-REQ-026-01 ─┬─▶ 026-02 ─────────────────────┐
                 ├─▶ 026-03 (cần TASK-REQ-012-03) │
                 └─▶ 026-04 (cần 007-04, 008-02)  │
                          │                       ▼
   026-02 + 026-04 ───────┴──▶ 026-05 (cần 007-05, 012-05, 005-05) ─▶ 026-06 (cần 008-01, 027-04) ─▶ 026-07 (cần 026-03, 013-06) ─▶ 026-08
```

Làm song song được: 026-01 với 026-02; 026-03 với 026-04; 026-02 với 026-03.

## Ghi chú

- **Số migration:** 026-02 ghi `NNNN` và bắt buộc đọc `request-service/migrations/*` lúc làm; nếu migration `analysis_runs` của TASK-REQ-007-01 chưa merge thì gộp hai thay đổi trên `analysis_runs` vào đó.
- **Hồi quy:** 026-05 di chuyển mã của SOL-007 và SOL-012 sau giao diện; toàn bộ test của hai solution phải xanh không đổi kỳ vọng. Khuyến nghị thêm điểm nối `SolutionEngine` ngay từ TASK-REQ-007-05 và 012-05 để khỏi sửa lại.
- **OpenSpec chưa cài và chưa chạy thử:** mọi cờ CLI (`--version`, `validate`, `archive --yes`), cấu trúc thư mục và định dạng `tasks.md` là hiểu biết chung. Cờ `REQUEST_OPENSPEC_ENABLED` mặc định tắt; chạy thử trên dev server mẫu là điều kiện trước khi bật.
- **git-gateway:** `CreateWorktreeRequest` cần `repo_id` mà `ResolveConnection` không trả (026-06); `Discard` có dọn tệp `untracked` hay không chưa rõ. Cả hai cần kiểm trên dịch vụ thật.
- **027:** 026-03 dùng `Violation` và 026-06 dùng `RenderArtifact`, parser `orca-json` của SOL-027 (task 027-03, 027-04); nếu SOL-027 chưa xong, dùng định nghĩa tạm ghi trong từng task.
- **Frontend:** hiển thị engine và nhãn "`tasks.md` chưa đồng bộ" thuộc solution frontend; kênh `request.engineGet`, `request.engineSet` do CR-REQ-016.
