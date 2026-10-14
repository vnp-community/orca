# Tasks backend: solution-engines (v6)

> 🔧 **Đã hiện thực một phần (2026-10-14) theo [SOL-TDD-002](../../../v8/tdd-agent-integration/solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md)**: xem bảng "Trạng thái hiện thực" bên dưới và [CONTRACTS.md](../../../v8/tdd-agent-integration/CONTRACTS.md). Mỗi task 0,5 đến 2 ngày. Đường dẫn tương đối tới `backend-go/services/request-service/` (mới) trừ khi ghi khác. Tất cả `Status: [ ] TODO`, chưa chạy test nào.

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

## Trạng thái hiện thực (2026-10-14)

| Task | Trạng thái | Ghi chú |
|---|---|---|
| 026-01 domain | xong | như kiểm toán 2026-10-07 |
| 026-02 migration, repository | xong | `0007`, `0094`, `0095`, `0096`; repository Postgres + MySQL có contract test chạy thật trên cả hai |
| 026-03 parser, renderer `tasks.md` | xong, **hợp đồng khác bản gốc** | `depends` tương đối trong container, phụ thuộc phase trước thành `DependsOnPhaseIndices`, một vùng duy nhất, bốn dòng `Serves/Code/Verify/Done when` (CONTRACTS.md mục 3) |
| 026-04 readiness gate | xong, mở rộng | thêm agent, skill, `.tdd-agent`, cache 10 phút, tách blocker và gap tri thức |
| 026-05 giao diện engine, ghim | xong | `engine_override` từ client chỉ nhận `native` |
| 026-06 `openspecEngine.GenerateAnalysis` | **không làm theo hướng này** | SOL-TDD-002: Solution không dùng engine `openspec`; dùng agent chỉ đọc có KG (cờ `solution_agent`). Phần `ProposalWorkspace`, kiểm phạm vi ghi và `GenerateProposal` (Plan) đã xong |
| 026-07 `GeneratePlan`, `TasksMdSyncer`, archive | xong | `GeneratePlan` do agent viết change (T2.4 đến T2.8); `TasksMdSyncer` quét 30 s tick `tasks.md`, archive thật khi `completed`, thử lại 5 lần rồi `failed` (T3.3, xem CONTRACTS.md mục 5b của v8) |
| 026-08 RPC, metric, audit | xong | RPC (admin, audit bền), kênh và REST, bốn series metric, tích hợp hai dialect |

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
