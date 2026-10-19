# Tasks — v8/01-integrated-workspace (Frontend)

**Source:** `solutions/` (FE-IW-SOL-001 … FE-IW-SOL-005)  
**Date:** 2026-10-19  
**Total tasks:** 19

---

## Phases

| Phase | Mục tiêu | Tasks | Effort |
|-------|----------|-------|--------|
| [Phase 1](./phase-1-types-and-mcp-proxy.md) | Types + MCP Proxy status (blocking P1) | T01–T04 | 0.5 ngày |
| [Phase 2](./phase-2-pis-hook-and-fleet-ui.md) | PIS tool hook + Fleet/Settings UI (P1) | T05–T10 | 1–1.5 ngày |
| [Phase 3](./phase-3-task-enrichment-store.md) | Task enrichment Zustand slice + hook (P2) | T11–T13 | 1 ngày |
| [Phase 4](./phase-4-task-ui-panels.md) | Task KG context panel + form fields (P2) | T14–T15 | 1 ngày |
| [Phase 5](./phase-5-quality-gate-ipc.md) | Quality gate IPC + rejection banner (P2) | T16–T17 | 0.5–1 ngày |
| [Phase 6](./phase-6-tests.md) | Tests toàn bộ (Vitest) | T18–T19 | 2–3 ngày |

**Dependency order:** Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6

---

## Execution Order (backend dependency chain)

Khớp với: `CR-ORCA-005 → CR-ORCA-004 → CR-ORCA-001 → CR-ORCA-002 → CR-ORCA-003`

```
Phase 1 (T01–T04): MCPProxyTypes + useMCPProxyStatus
    ↓
Phase 2 (T05–T10): PIS_TOOL_NAMES + hooks + fleet/settings UI
    ↓
Phase 3 (T11–T13): task-enrichment slice + useTaskEnrichment
    ↓
Phase 4 (T14–T15): TaskKGContextPanel + TaskFormKGFields
    ↓
Phase 5 (T16–T17): useIpcEvents patch + TaskRejectionBanner
    ↓
Phase 6 (T18–T19): Unit tests + integration tests
```

---

## Quy ước cho AI executor

- Mỗi task có: `file`, `action` (NEW/EDIT), `source_section` (link solution), code đầy đủ
- Task độc lập trong phase — có thể chạy song song trong cùng phase
- Dependencies liên-phase phải hoàn thành trước khi bắt đầu phase tiếp theo
- Kiểm tra compile sau mỗi phase: `cd src && npx tsc --noEmit`
