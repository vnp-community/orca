# Frontend Solutions — v8/01-integrated-workspace

**CR Set:** `docs/crs/v8/01-integrated-workspace/`  
**Backend Solutions:** `specs/backend-go/crs/v8/01-integrated-workspace/solutions/`  
**TDD Base:** `specs/frontend/tdd/v5/`  
**Updated:** 2026-10-19 (v2 — aligned với backend solutions)

---

## Tổng quan

Tất cả 5 CRs đều có `layer: backend`. Frontend solutions này **hoàn toàn aligned** với backend solutions tương ứng:

- Type definitions mirror Go structs (field names, JSON tags)
- Error messages match backend error format strings chính xác
- Env var names match `.env.example` từ SOL-ORCA-004
- Execution order khớp backend dependency chain

---

## Solution Index

| Solution | CR | Backend Solution | Priority | Effort | Status |
|----------|-----|-----------------|----------|--------|--------|
| [FE-IW-SOL-001](./FE-IW-SOL-001-pis-tools-ui-integration.md) | CR-ORCA-001 | SOL-ORCA-001 | P1 | 1–2 ngày | ✅ Done |
| [FE-IW-SOL-002](./FE-IW-SOL-002-task-kg-context-ui.md) | CR-ORCA-002 | SOL-ORCA-002 | P2 | 2–3 ngày | ✅ Done |
| [FE-IW-SOL-003](./FE-IW-SOL-003-quality-gate-rejection-ui.md) | CR-ORCA-003 | SOL-ORCA-003 | P2 | 1–2 ngày | ✅ Done |
| [FE-IW-SOL-004](./FE-IW-SOL-004-agent-executor-deploy-settings.md) | CR-ORCA-004 | SOL-ORCA-004 | P1 | 1 ngày | ✅ Done |
| [FE-IW-SOL-005](./FE-IW-SOL-005-peer-invoke-connection-ui.md) | CR-ORCA-005 | SOL-ORCA-005 | P1 | 0.5–1 ngày | ✅ Done |

**Tổng effort frontend:** ~6–9 ngày

---

## Backend ↔ Frontend Alignment Map

| Backend artifact | Frontend mirror |
|-----------------|-----------------|
| `TaskKGContext` Go struct (CRSID, FeatureRef, Service, RepoTarget) | `TaskKGContext` TS type (crs_id, feature_ref, service, repo_target) snake_case |
| `TaskRejectedResponse` (task_id, status, reason, action_required) | `TaskRejection` TS type — same JSON field names |
| Error: `"pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"` | `PISTooLNotConfiguredError` — detect exact string |
| Error: `"peer-invoke not configured: set ORCA_MCP_PROXY_URL"` | `classifyError()` → "not-configured" |
| Error: `"peer-invoke: no grant for..."` (HTTP 403/401) | `classifyError()` → "auth-error" |
| `PISToolCatalog()` — 5 tools, conditional expose | `PIS_TOOL_NAMES` const array — detect availability |
| Convention error: `"%d files missing '// implements: CRS-XXX'"` | `parseConventionError()` — parse count + crsId |
| Endpoint: `/internal/v1/agent-peer/mcp/proxy` | Hard-coded hint trong `MCPProxyStatusIndicator` |
| `.env.example` — 7 env vars (incl. 3 required, 4 optional) | `ORCA_AGENT_ENV_VARS` array — 7 rows |
| Default docker-compose: `mcp-proxy:3012` | Default value trong onboarding form |
| `EnrichedTask.CRSContext` (`json.RawMessage`) | `taskEnrichments[id].crsContext` (`unknown`) |
| `BuildSystemPrompt()` convention: `// implements: CRS-XXX` | Convention hint trong `TaskKGContextPanel` |

---

## Dependency Graph (Frontend)

```
FE-IW-SOL-005 (MCPProxyStatus — P1, blocking WS client)
    ↓
FE-IW-SOL-004 (Deploy settings — P1, dùng PIS_TOOL_NAMES từ SOL-001)
    ↓
FE-IW-SOL-001 (PIS tools hook + KG renderer — P1, foundational)
    ↓
FE-IW-SOL-002 (Task KG context — P2, dùng hook + slice)
    ↓
FE-IW-SOL-003 (Quality gate rejection — P2, dùng types + slice từ SOL-002)
```

**Khớp với backend execution order (SOL-ORCA-005):**
```
CR-ORCA-005 → CR-ORCA-004 → CR-ORCA-001 → CR-ORCA-002 → CR-ORCA-003
```

---

## Files Frontend Tổng hợp

```
src/renderer/src/
├── types/
│   ├── task-types.ts                              ← EDIT (SOL-002, SOL-003)
│   └── mcp-proxy-types.ts                        ← NEW (SOL-005)
├── hooks/
│   ├── use-agent-tool-capabilities.ts             ← NEW (SOL-001: PIS_TOOL_NAMES const)
│   ├── use-task-enrichment.ts                     ← NEW (SOL-002: concurrent fetch, 5min cache)
│   ├── use-mcp-proxy-status.ts                    ← NEW (SOL-005: 5 status types)
│   └── useIpcEvents.ts                            ← EDIT (SOL-003: rejection toast 10s, CRS update)
├── store/
│   ├── index.ts                                   ← EDIT (SOL-002: add createTaskEnrichmentSlice)
│   ├── slices/
│   │   └── task-enrichment.ts                     ← NEW (SOL-002+003: updateEnrichmentCRSStatus)
│   └── selectors/
│       └── task-selectors.ts                      ← NEW/EDIT (SOL-003: selectTaskIdsByCRSId)
└── components/
    ├── agent/
    │   ├── kg-query-result-renderer.tsx            ← NEW (SOL-001: 5 tool cases)
    │   ├── agent-capability-badge.tsx              ← NEW (SOL-001)
    │   └── pis-tool-not-configured-error.tsx       ← NEW (SOL-001: exact error string detect)
    ├── tasks/
    │   ├── task-kg-context-panel.tsx               ← NEW (SOL-002: convention hint, best-effort)
    │   ├── task-form-kg-fields.tsx                 ← NEW (SOL-002: 4 snake_case fields)
    │   ├── task-rejection-banner.tsx               ← NEW (SOL-003: parse convention error)
    │   └── task-peer-invoke-error.tsx              ← NEW (SOL-005: 4 error types)
    ├── fleet/
    │   ├── agent-bridge-config-display.tsx         ← NEW (SOL-004: 2 bridge IDs)
    │   ├── agent-pis-health-check.tsx              ← NEW (SOL-004: 5 tools fixed order)
    │   ├── agent-env-config-display.tsx            ← NEW (SOL-004: 7 env vars)
    │   └── mcp-proxy-status-indicator.tsx          ← NEW (SOL-005: latencyMs, endpoint hint)
    ├── onboarding/
    │   └── pikb-bridge-step.tsx                   ← NEW (SOL-004: 4 fields, default mcp-proxy:3012)
    └── settings/
        ├── pikb-integration-settings.tsx           ← NEW (SOL-001: list 5 pis tools)
        └── mcp-proxy-connection-test.tsx           ← NEW (SOL-005)
```

---

## Quy ước

- **snake_case** cho `TaskKGContext` type fields (match API JSON từ Go `json:"field_name"`)
- **Exact error string matching** cho error classification — không dùng error codes
- **`PIS_TOOL_NAMES`** const — single source of truth cho 5 tool names cố định
- **Sensitive vars** (tokens) mask `●●●●●●`, không log, type=password trong form
- **Best-effort** everywhere — KG fetch fail không crash UI
- **WS status ≠ Proxy status** — hai indicators riêng biệt (SOL-ORCA-005)
- Frontend KHÔNG POST trực tiếp đến mcp-proxy — luôn qua `callRuntimeRpc` → orca-agent

---

## Implementation Results (2026-10-19)

**Status: ✅ ALL 5 SOLUTIONS IMPLEMENTED**

### Verification

| Check | Result |
|-------|--------|
| TypeScript compile (`tsc --noEmit`) | ✅ 0 errors |
| Unit tests | ✅ 23/23 passed |
| Files created | ✅ 21 new files |
| Files edited | ✅ 3 files patched |

### Files Delivered

| Solution | Files |
|----------|-------|
| SOL-001 (PIS Tools) | `hooks/use-agent-tool-capabilities.ts`, `components/agent/{kg-query-result-renderer, agent-capability-badge, pis-tool-not-configured-error}.tsx`, `components/settings/pikb-integration-settings.tsx` |
| SOL-002 (Task KG) | `types/task-types.ts`, `store/slices/task-enrichment.ts`, `store/selectors/task-selectors.ts`, `hooks/use-task-enrichment.ts`, `components/tasks/{task-kg-context-panel, task-form-kg-fields}.tsx` |
| SOL-003 (Quality Gate) | `components/tasks/task-rejection-banner.tsx`, `hooks/useIpcEvents.ts` (patched: onTaskStatusChanged + onCRSStatusUpdated) |
| SOL-004 (Deploy Settings) | `components/fleet/{agent-bridge-config-display, agent-pis-health-check, agent-env-config-display}.tsx`, `components/onboarding/pikb-bridge-step.tsx` |
| SOL-005 (Peer-Invoke UI) | `types/mcp-proxy-types.ts`, `hooks/use-mcp-proxy-status.ts`, `components/fleet/mcp-proxy-status-indicator.tsx`, `components/settings/mcp-proxy-connection-test.tsx` |

### Test Coverage

| Test File | Tests | Result |
|-----------|-------|--------|
| `hooks/__tests__/use-mcp-proxy-status.test.ts` | 7 | ✅ PASS |
| `hooks/__tests__/use-agent-tool-capabilities.test.ts` | 5 | ✅ PASS |
| `store/slices/__tests__/task-enrichment.test.ts` | 6 | ✅ PASS |
| `components/tasks/__tests__/task-rejection-banner.test.ts` | 5 | ✅ PASS |
| **Total** | **23** | **✅ 23/23** |
