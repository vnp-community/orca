# Architecture Decision Records — Orca

Tài liệu ghi lại các quyết định kiến trúc quan trọng của hệ thống Orca.

**Định dạng:** MADR (Markdown Architecture Decision Records)  
**Cập nhật:** 2026-07-30

---

## Cấu trúc thư mục

```
docs/adrs/
├── README.md                                    ← File này (root index)
├── enterprise-migration-impact-assessment.md    ← Deployment migration guide v4→v5→v6
├── v1/                                          ← ADR-001–015 (HLD v5.0/v6.0 baseline)
│   ├── README.md
│   ├── ADR-001 … ADR-015
│   └── ...
└── v2/                                          ← ADR-016–020 (HLD v6.0 additions)
    ├── README.md
    ├── ADR-016 … ADR-020
    └── ...
```

---

## ADR Directory

### v1 — Baseline Architecture (ADR-001–015)

**Phạm vi:** HLD v5.0 Enterprise + v6.0 Dev Server Agent concepts  
**Tài liệu:** [v1/README.md](./v1/README.md)

| ID | Tiêu đề | Trạng thái |
|----|---------|------------|
| ADR-001 | Platform Abstraction via IPlatformServices | ✅ Accepted |
| ADR-002 | Multi-Database via IConnectionPool | ✅ Accepted |
| ADR-003 | Per-User Process Isolation via SessionManager | ✅ Accepted |
| ADR-004 | SSH Relay Binary for Remote Dev Server | 🔄 Superseded → ADR-013 |
| ADR-005 | Agent WebSocket Binary Wire Protocol | 🔄 Superseded → ADR-014 |
| ADR-006 | WebCredentialStore — AES-256-GCM | ✅ Accepted |
| ADR-007 | 3-Layer Profile Hierarchy with Deep-Merge | 🚧 Proposed |
| ADR-008 | AI Provider Credentials on Dev Server | 🚧 Proposed |
| ADR-009 | Workflow Orchestration via DAG | 🚧 Proposed |
| ADR-010 | Task Graph as DAG with BFS | 🚧 Proposed |
| ADR-011 | Project Workspace + AgentConnectionManager | ⚠️ Amended (v6.0) |
| ADR-012 | Remote Git UI via Agent RPC | 🚧 Proposed |
| ADR-013 | Dev Server Agent Replaces Thin Relay | 🚧 Proposed (v6.0) |
| ADR-014 | Gateway–Agent JSON-RPC 2.0 Protocol v3 | 🚧 Proposed (v6.0) |
| ADR-015 | Signed Execution Context for Trust | 🚧 Proposed (v6.0) |

---

### v2 — HLD v6.0 Additions (ADR-016–020)

**Phạm vi:** Enterprise schema, Agent layer model, Rollout strategy  
**Tài liệu:** [v2/README.md](./v2/README.md)

| ID | Tiêu đề | Trạng thái |
|----|---------|------------|
| ADR-016 | Database Migrations 0006–0010 (Enterprise Schema) | 🚧 Proposed |
| ADR-017 | Dev Server Agent Layer Model (A0–A4) | 🚧 Proposed |
| ADR-018 | Control Plane / Data Plane Separation | 🚧 Proposed |
| ADR-019 | Agent Autonomous Operation & Reconnect Strategy | 🚧 Proposed |
| ADR-020 | Enterprise Rollout Phases & Backward Compatibility | 🚧 Proposed |

---

## Enterprise Migration Guide

[enterprise-migration-impact-assessment.md](./enterprise-migration-impact-assessment.md) — Hướng dẫn migration deployment từ v4 → v5 → v6:
- Breaking changes per version
- DB migration changelog
- Infrastructure changes (env vars, Docker)
- Security impact analysis
- Rollout checklist (Phase 1–3)
- Rollback plan

> Xem thêm: [Feature Impact Matrix](../flows/enterprise-migration-impact-assessment.md) — phân tích chi tiết tác động per-feature (F01–F39), risk register, và roadmap.

---

## Trạng thái Implementation Summary

| Phase | Migrations | ADRs | Status |
|-------|-----------|------|--------|
| Phase 0 (v4.x) | — | — | ✅ Done |
| Phase 1 (v5.0a) | 0001–0005 | ADR-001–003, 006 | ✅ Done |
| Phase 2 (v5.0b) | 0006–0010 | ADR-007–012, 016 | 🚧 In Progress |
| Phase 3 (v6.0) | — (agent) | ADR-013–015, 017–020 | 🚧 Proposed |

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [docs/hld/](../hld/) | HLD: C1–C4 + security + deployment |
| [docs/flows/](../flows/) | Data flow documents F22–F39 |
| [docs/features/](../features/) | Feature specifications F01–F39 |
| [docs/crs/](../crs/) | Change Requests (CR-DS-001–005) |
