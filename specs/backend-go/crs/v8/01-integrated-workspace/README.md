# CRS v8 — 01-integrated-workspace: Backend Solutions

Thư mục này chứa **giải pháp kỹ thuật backend (Go)** cho 5 Change Requests trong batch `01-integrated-workspace`.

| CR | Title | Priority | Effort | Status |
|----|-------|---------|--------|--------|
| [CR-ORCA-001](solutions/CR-ORCA-001-solution.md) | orca-agent: Expose pis_* Tools + Wire PISTool | P1 | 2–3 ngày | 🔴 Pending |
| [CR-ORCA-002](solutions/CR-ORCA-002-solution.md) | orca: Task Proto Extension + Auto-Context Injection | P2 | 3–4 ngày | 🔴 Pending |
| [CR-ORCA-003](solutions/CR-ORCA-003-solution.md) | orca: Quality Gate — CRS Status Update + Convention Verify | P2 | 3–4 ngày | 🔴 Pending |
| [CR-ORCA-004](solutions/CR-ORCA-004-solution.md) | orca-agent: Config Env Override + Executor Extension + Deploy Kit | P1 | 2 ngày | 🔴 Pending |
| [CR-ORCA-005](solutions/CR-ORCA-005-solution.md) | orca-agent: ws.Client.Call() Peer-Invoke Method | P1 | 1 ngày | 🔴 Pending |

---

## Execution Order (dependency graph)

```
CR-ORCA-005  ← START ĐÂY (unblocks compile)
  → CR-ORCA-004  (config env override + executor restructure)
    → CR-ORCA-001  (pis_* catalog + executor wiring)
      → CR-ORCA-002  (task KGContext + auto-inject)
        → CR-ORCA-003  (quality gate + CRS status update)
```

**Total estimated effort**: ~11–14 ngày

---

## Source CRs

- Docs: [`docs/crs/v8/01-integrated-workspace/`](../../../../docs/crs/v8/01-integrated-workspace/)
- TDD reference: [`specs/backend-go/tdd/`](../../tdd/)
