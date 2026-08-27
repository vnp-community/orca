# Architecture Decision Records — v2 (HLD v6.0 Additions)

**Phạm vi:** v2 ADRs — quyết định kiến trúc bổ sung từ HLD v6.0  
**Ngày tạo:** 2026-07-30  
**Căn cứ:** `docs/hld/v1/README.md` (v6.0 Architecture Layers L0–L5 + A0–A4), `docs/hld/v1/C4-code.md` (C4.11)  
**Liên hệ:** Xem [ADR v1](../v1/README.md) cho ADR-001–015 (v5.0 baseline + v6.0 Agent concepts)

> 📌 Xem [Enterprise Migration Impact Assessment](../enterprise-migration-impact-assessment.md) cho deployment checklist và rollback plan.

> ⚠️ **Current vs. Proposed:** Các ADR v2 này và các tài liệu C4 model dưới `docs/hld/v1/` mô tả kiến trúc **v6.0 đề xuất** (proposed target state), chưa được implement — xem "Trạng thái Implementation" ở cuối mỗi ADR. Để biết kiến trúc **hiện tại đã implement thật sự** trong code, xem 3 file ở gốc `docs/hld/`: [`backend-server-architecture.md`](../../hld/backend-server-architecture.md), [`dev-server-architecture.md`](../../hld/dev-server-architecture.md), [`web-server-architecture.md`](../../hld/web-server-architecture.md) — các file này đã được rà soát và sửa lại đúng theo code thật vào tháng 8/2026.

---

## Danh sách ADR v2

| ID | Tiêu đề | Trạng thái | Amends | Features | HLD Ref | Ngày |
|----|---------|------------|--------|----------|---------|------|
| [ADR-016](./ADR-016-db-migrations-0006-0010-schema.md) | Database Migrations 0006–0010 (Enterprise Schema) | 🚧 Proposed | ADR-002 | F33–F37 | C4.3, C2 (13–16) | 2026-07-30 |
| [ADR-017](./ADR-017-dev-server-agent-layer-model.md) | Dev Server Agent Layer Model (A0–A4) | 🚧 Proposed | ADR-013 | F01–F04, F12–F14, F17–F18, F27, F35–F39 | README A0–A4, C4.11 | 2026-07-30 |
| [ADR-018](./ADR-018-control-plane-data-plane-separation.md) | Control Plane / Data Plane Separation | 🚧 Proposed | ADR-013 | F22–F25, F33–F37 | README L0–L5 + A0–A4 | 2026-07-30 |
| [ADR-019](./ADR-019-agent-autonomous-operation-reconnect.md) | Agent Autonomous Operation & Reconnect Strategy | 🚧 Proposed | ADR-013, ADR-017 | F01, F04, F27, F36, F37 | README Principle 8, C4.11 | 2026-07-30 |
| [ADR-020](./ADR-020-enterprise-rollout-phases-backward-compat.md) | Enterprise Rollout Phases & Backward Compatibility | 🚧 Proposed | ADR-013 Migration Path | F22–F39 | deployment.md | 2026-07-30 |

---

## v2 ADR Rationale — Tại sao cần ADR v2?

ADR v1 được viết dựa trên HLD v5.0 (containers, components, code C1–C4) và phác thảo v6.0 (ADR-013–015). Sau khi HLD v6.0 được finalize với:

1. **Architecture Layer model** (L0–L5 Control Plane + A0–A4 Data Plane)
2. **Dev Server Agent internal structure** (C4.11: 5 layers, 40+ modules)
3. **Full DB schema** (migrations 0006–0010 với DDL chi tiết)
4. **Deployment phases** (3 phases, feature flags, backward compat)

...cần bổ sung ADRs để capture những quyết định kiến trúc này một cách chính thức.

---

## Mối quan hệ v1 → v2

```
v1 ADR               v2 ADR (extends/details)
──────────────────────────────────────────────
ADR-002 (Multi-DB)   → ADR-016 (Schema 0006–0010)
ADR-013 (Agent)      → ADR-017 (Layer Model A0–A4)
ADR-013 (Agent)      → ADR-018 (Control/Data Plane)
ADR-013 (Agent)      → ADR-019 (Autonomous Operation)
ADR-013 Migration    → ADR-020 (Rollout Phases)
```

---

## ADR v2 → HLD Mapping

| ADR | HLD Section |
|-----|------------|
| ADR-016 | `v1/README.md` Architecture Layers L5, `v1/C4-code.md` §C4.3 MigrationRunner |
| ADR-017 | `v1/README.md` Architecture Layers A0–A4, `v1/C4-code.md` §C4.11 |
| ADR-018 | `v1/README.md` Architecture Layers overview, `v1/C1-system-context.md`, `v1/C2-containers.md` |
| ADR-019 | `v1/README.md` Principle 8 "Agent Autonomous", `v1/C4-code.md` §C4.11 |
| ADR-020 | `v1/deployment.md`, `v1/security.md` Trust Boundaries |

---

## Gaps Addressed in v2 (từ ADR v1 Gap Register)

| v1 Gap | Addressed By |
|--------|-------------|
| G6: `ORCA_AI_CREDENTIAL_KEY` rotation | ADR-016 §Migration 0008 notes; ADR-020 Rollback |
| G8: Clock skew Gateway↔Agent | ADR-019 §Open Questions |
| G11: Buffer 1000 events overflow | ADR-019 §Event Buffer & Replay |
| G12: Backward compat relay + agent | ADR-020 §Dual-Mode AgentDispatcher |

---

## Gaps Remaining (v2)

| # | Gap | ADR | Priority |
|---|-----|-----|----------|
| G13 | `orca_provider_usage` aggregation: monthly rollup cần scheduled job | ADR-016 | Low |
| G14 | `orca_task_edges` BFS cycle detect: O(n²) cho large trees | ADR-016 | Medium |
| G15 | Agent layer boundary enforcement: ESLint import rules chưa config | ADR-017 | Medium |
| G16 | Feature flag invalidation khi agent connect/disconnect | ADR-020 | High |
| G17 | `ORCA_MAX_MIGRATION` env: cần validation (không cho set > actual migration) | ADR-020 | Low |
| G18 | Phase 2 → 1 rollback: `ALTER TABLE DROP COLUMN` không support trên SQLite | ADR-016 | Medium |

---

## Architectural Principles (v2 additions)

### 9. Layer Boundary Enforcement
> *"Mỗi layer trong Agent (A0–A4) chỉ được import từ layer thấp hơn. A0 không import A1. A3 không import A1/A2."*  
> — ADR-017: Layer Model, ESLint import rules

### 10. Migration Version Gate
> *"API endpoints cho Phase 2+ features trả về 503 nếu migration chưa đến version required — không để UI broken."*  
> — ADR-020: `requireMigration(n)` middleware

### 11. Dual-Mode Dispatch During Migration
> *"Trong thời gian migrate từ relay sang agent, Gateway phải support cả 2 transparently — không downtime."*  
> — ADR-020: AgentDispatcher dual-mode với relay fallback

### 12. Event Priority in Buffer
> *"Critical events (agent.complete, git.push.done) không bao giờ bị drop khi buffer overflow — chỉ non-critical events bị sacrifice."*  
> — ADR-019: Event Buffer overflow strategy

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR v1 README](../v1/README.md) | ADR-001–015 (v5.0/v6.0 baseline) |
| [enterprise-migration-impact-assessment.md](../enterprise-migration-impact-assessment.md) | Deployment guide + rollback plan |
| [HLD README](../../hld/v1/README.md) | v6.0 Architecture Layers |
| [HLD C4-code.md](../../hld/v1/C4-code.md) | C4.11: Dev Server Agent modules |
| [HLD deployment.md](../../hld/v1/deployment.md) | Docker Compose + deployment diagram |
| [flows/README.md](../../flows/README.md) | Data flow documents F22–F39 |
