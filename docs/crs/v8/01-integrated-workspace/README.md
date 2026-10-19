# Change Request Specifications — v8/01-integrated-workspace

**Status Tổng thể:** ✅ **5/5 CRs Implemented & Verified**  
**Ngày hoàn thành:** 2026-10-19  
**Feature Reference:** DS-005 (Integrated Dev Knowledge & AI Agent Workspace)  
**Phạm vi:** Backend (`orca-agent`, `deploy/orca-agent`) & Frontend (`frontend/src/renderer/src/`)

---

## 1. Danh sách Change Requests

| CR ID | Tiêu đề | Layer | Priority | Trạng thái | Implementation Solution |
|-------|---------|-------|----------|------------|-------------------------|
| [CR-ORCA-001](./CR-ORCA-001-pis-tools-wiring.md) | orca-agent: Expose pis_* Tools + Wire PISTool | Backend, Frontend | P1 | ✅ Implemented | [SOL-ORCA-001](../../../specs/backend-go/crs/v8/01-integrated-workspace/solutions/CR-ORCA-001-solution.md) / [FE-IW-SOL-001](../../../specs/frontend/crs/v8/01-integrated-workspace/solutions/FE-IW-SOL-001-pis-tools-ui-integration.md) |
| [CR-ORCA-002](./CR-ORCA-002-task-proto-kg-context.md) | orca: Task Proto Extension + Auto-Context Injection | Backend, Frontend | P2 | ✅ Implemented | [SOL-ORCA-002](../../../specs/backend-go/crs/v8/01-integrated-workspace/solutions/CR-ORCA-002-solution.md) / [FE-IW-SOL-002](../../../specs/frontend/crs/v8/01-integrated-workspace/solutions/FE-IW-SOL-002-task-kg-context-ui.md) |
| [CR-ORCA-003](./CR-ORCA-003-quality-gate-crs-status.md) | orca: Quality Gate — CRS Status Update + Convention Verify | Backend, Frontend | P2 | ✅ Implemented | [SOL-ORCA-003](../../../specs/backend-go/crs/v8/01-integrated-workspace/solutions/CR-ORCA-003-solution.md) / [FE-IW-SOL-003](../../../specs/frontend/crs/v8/01-integrated-workspace/solutions/FE-IW-SOL-003-quality-gate-rejection-ui.md) |
| [CR-ORCA-004](./CR-ORCA-004-orca-agent-executor-deploy-kit.md) | orca-agent: Config Env Override + Executor Extension + Deploy Kit | Backend, Frontend | P1 | ✅ Implemented | [SOL-ORCA-004](../../../specs/backend-go/crs/v8/01-integrated-workspace/solutions/CR-ORCA-004-solution.md) / [FE-IW-SOL-004](../../../specs/frontend/crs/v8/01-integrated-workspace/solutions/FE-IW-SOL-004-agent-executor-deploy-settings.md) |
| [CR-ORCA-005](./CR-ORCA-005-orca-ws-client-call-peer-invoke.md) | orca-agent: ws.Client.Call() Peer-Invoke Method | Backend, Frontend | P1 | ✅ Implemented | [SOL-ORCA-005](../../../specs/backend-go/crs/v8/01-integrated-workspace/solutions/CR-ORCA-005-solution.md) / [FE-IW-SOL-005](../../../specs/frontend/crs/v8/01-integrated-workspace/solutions/FE-IW-SOL-005-peer-invoke-connection-ui.md) |

---

## 2. Dependency Graph & Thứ tự thực thi

```
Phase 1 (CR-ORCA-005): ws.Client.Call() qua HTTP mcp-proxy (peer-invoke unblock)
  │
  ▼
Phase 2 (CR-ORCA-004): Config env override + Executor routing pis_*
  │
  ▼
Phase 3 (CR-ORCA-001): 5 pis_* tools catalog + PISTool wiring
  │
  ▼
Phase 4 (CR-ORCA-002): TaskKGContext schema + Auto-context injection UI & Handler
  │
  ▼
Phase 5 (CR-ORCA-003): Quality gate verification + Convention check + CRS Status update
```

---

## 3. Tổng hợp kết quả thực thi (Implementation Results)

### Frontend (21 files mới, 3 files cập nhật, 4 test suites):
- **Types & Model**: `mcp-proxy-types.ts`, `task-types.ts`.
- **State & Storage**: `store/slices/task-enrichment.ts`, `store/selectors/task-selectors.ts`, `store/types.ts`, `store/index.ts`.
- **Hooks & Events**: `use-mcp-proxy-status.ts`, `use-agent-tool-capabilities.ts`, `use-task-enrichment.ts`, `useIpcEvents.ts`.
- **UI Components**:
  - MCP Proxy & Fleet: `mcp-proxy-status-indicator.tsx`, `mcp-proxy-connection-test.tsx`, `agent-bridge-config-display.tsx`, `agent-pis-health-check.tsx`, `agent-env-config-display.tsx`, `pikb-bridge-step.tsx`, `pikb-integration-settings.tsx`.
  - Agent & KG: `kg-query-result-renderer.tsx`, `agent-capability-badge.tsx`, `pis-tool-not-configured-error.tsx`.
  - Task & Quality Gate: `task-kg-context-panel.tsx`, `task-form-kg-fields.tsx`, `task-peer-invoke-error.tsx`, `task-rejection-banner.tsx`.

### Kiểm thử & Chất lượng mã nguồn:
- **Vitest Unit Tests**: `23/23 tests PASS` (4/4 suites):
  - `use-mcp-proxy-status.test.ts`: 7 tests PASS
  - `use-agent-tool-capabilities.test.ts`: 5 tests PASS
  - `task-enrichment.test.ts`: 6 tests PASS
  - `task-rejection-banner.test.ts`: 5 tests PASS
- **TypeScript Verification**: `0 error TS` trên toàn bộ các tệp liên quan đến CRs.
- **Backend Go Verification**: Tất cả các unit tests của `orca-agent` (`audit`, `config`, `mcp`, `task`, `ws`) đã được kiểm thử và xác nhận đạt yêu cầu.
