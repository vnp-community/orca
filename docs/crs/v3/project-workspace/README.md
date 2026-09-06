# Project Workspace — Change Requests (v3)

> **Bối cảnh:** phát sinh từ investigation 1 câu hỏi user về deployment b15.openledger.vn
> (Project Workspace → project "Vnp-asm" → tab Git hiện "(no branch)"). CR-PW-001/002/003 là
> **frontend-only** — không CR nào cần đổi `backend-go` hoặc `agent/`, xem mục "Không thuộc phạm
> vi" trong từng CR để biết lý do cụ thể. CR-PW-004/005/006 là 1 phiên làm việc tiếp nối, phát
> sinh từ yêu cầu "monitor quá trình thực thi trên các dev-server thông qua agent và backend-go
> (đóng vai trò proxy)" — CR-PW-004 vẫn frontend-only, CR-PW-005 đụng `backend-go`, CR-PW-006 đụng
> cả 3 tầng (frontend + backend-go + agent, phần lớn chỉ ở mức thiết kế — xem status).

| CR | Vấn đề | Giải pháp | Status |
|----|--------|-----------|--------|
| [CR-PW-001](./CR-PW-001-git-status-shape-mismatch.md) | `GitPanel` cast nhầm shape RPC `git.status` → branch/ahead/behind sai, "(no branch)" gộp nhiều nguyên nhân khác nhau | [FE-SOL-001](../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md) | ✅ Implemented |
| [CR-PW-002](./CR-PW-002-multi-repo-workspace-repo-label.md) | 1 Project nhiều repo — Git tab không cho biết đang xem repo nào | [FE-SOL-001](../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md) (gộp cùng CR-PW-001) | ✅ Implemented |
| [CR-PW-003](./CR-PW-003-workflows-tab-wiring.md) | Tab Workflows là stub tĩnh, không gọi RPC nào dù backend đã sẵn | [FE-SOL-003](../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-003-wire-workflow-monitor-to-listExecutions.md) | ✅ Implemented |
| [CR-PW-004](./CR-PW-004-step-status-badge-cancelled-crash.md) | `StepStatusBadge` crash khi `execution.status === 'cancelled'` | [FE-SOL-004](../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-004-step-status-badge-cancelled.md) | ✅ Implemented |
| [CR-PW-005](./CR-PW-005-wscompat-missing-workflow-rpcs.md) | `backend-go`'s wscompat chỉ expose 4/11 RPC `WorkflowService` đã có sẵn | [BE-SOL-001](../../../../specs/backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md) | ✅ Implemented |
| [CR-PW-006](./CR-PW-006-execution-monitoring-architecture.md) | Kiến trúc monitor execution trên dev-server qua agent+backend-go — 2 engine song song, không có push transport, agent không có execution code | [FE-SOL-005](../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-005-execution-status-polling.md) (Phase A) + [SOL-AG-PW-001](../../../../specs/agent/crs/v3/project-workspace/solutions/SOL-AG-PW-001-execution-progress-reporting-design.md) (Phase D, design-only) | 🟡 Partially Implemented — Phase A ✅, Phase B/C/D/E 🔲 Designed |

## Thứ tự thực thi

```
CR-PW-001 + CR-PW-002 → cùng 1 solution (FE-SOL-001), cùng file GitPanel.tsx/WorkspaceContext.tsx
CR-PW-003 → độc lập, khác cụm file (WorkflowMonitor.tsx)
CR-PW-004 → độc lập, khác cụm file (StepStatusBadge.tsx/ExecutionMonitor.tsx)
CR-PW-005 → backend-go only, độc lập với frontend, nhưng là dependency của CR-PW-006 Phase A
CR-PW-006 Phase A → phụ thuộc CR-PW-005 (cần workflow.getExecution nối dây xong ở Web mode)
CR-PW-006 Phase B/C/D/E → chưa implement, xem CR doc's "Trạng thái triển khai"
```

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol sửa | Risk | Impacted count |
|---|---|---|
| `refreshGitStatus` (`WorkspaceContext.tsx`) | LOW | 4 |
| `GitPanel` (`components/workspace/git/GitPanel.tsx`) | LOW | 0 direct |
| `WorkflowMonitor` (`components/workflow/WorkflowMonitor.tsx`) | LOW | 0 direct |
| `StepStatusBadge` (`components/workflow/StepStatusBadge.tsx`) | LOW | 1 direct (`ExecutionMonitor.tsx`) |
| `registerWorkflowChannels` (`backend-go` wscompat `channels_workflow.go`) | LOW | 3 (1 direct: `cmd/server/main.go`'s `run`) |
| `useWorkflowExecution` (`hooks/useWorkflowExecution.ts`) | LOW | 1 direct (`ExecutionMonitor.tsx`) |

Không có execution flow (process) nào bị ảnh hưởng theo GitNexus ở bất kỳ symbol nào trên — an
toàn để sửa trực tiếp, không cần feature flag/rollout riêng. `detect_changes({scope:"all"})` sau
khi hoàn tất CR-PW-004/005/006: risk **low**, 42 changed symbols / 16 files, 0 affected processes.
