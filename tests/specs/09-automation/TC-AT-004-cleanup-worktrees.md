# TC-AT-004 — Cleanup Worktrees Theo Policy

**BL Reference:** BL-AT-04  
**Priority:** P2

---

## TC-AT-004-01: Cleanup worktrees cũ hơn 7 ngày (merged/idle)

### Steps
1. Policy: cleanup worktrees với status='merged' sau 7 ngày
2. Run cleanup automation

### Expected Results
- Worktrees merged > 7 days ago: deleted
- Active worktrees: NOT deleted

---

## TC-AT-004-02: Cleanup worktrees uncommitted — Protected

### Steps
1. Worktree có uncommitted changes
2. Cleanup runs

### Expected Results
- Worktree với uncommitted changes: SKIPPED (protected)
- Warning logged

