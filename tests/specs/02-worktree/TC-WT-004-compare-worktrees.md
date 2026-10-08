# TC-WT-004 — So sánh Kết quả Giữa Worktrees

**BL Reference:** BL-WT-04  
**Flow Reference:** docs/flows/logic/worktree-management.md#BL-WT-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya (participant)

---

## TC-WT-004-01: So sánh 2 worktrees — Happy Path

**Priority:** P1

### Preconditions
- 2 worktrees đã tồn tại: `wt-A` (branch feat-A), `wt-B` (branch feat-B)
- Cả 2 đều branch từ `main`

### Steps
1. RPC: `worktree.compare { projectId, worktreeIds: ['wt-A', 'wt-B'], baseRef: 'main' }`
2. Kiểm tra response

### Expected Results
- Relay calls: `git.exec(diff --stat)` × 2 (parallel)
- Response: comparison data per worktree:
  ```json
  [
    { worktreeId: 'wt-A', branch: 'feat-A', filesChanged: 5, insertions: 120, deletions: 30 },
    { worktreeId: 'wt-B', branch: 'feat-B', filesChanged: 3, insertions: 80, deletions: 10 }
  ]
  ```

### Assertions
```
result = await rpc.call('worktree.compare', { worktreeIds: ['wt-A', 'wt-B'], baseRef: 'main' })
assert result.length === 2
result.forEach(item => {
  assert item.worktreeId !== undefined
  assert typeof item.filesChanged === 'number'
})
assert spyRelay.callCount('git.exec') >= 2 // one diff per worktree
```

---

## TC-WT-004-02: So sánh — Bao gồm session summary

**Priority:** P1

### Steps
1. Run agent trong `wt-A`, generate session summary
2. `worktree.compare { ... }`
3. Kiểm tra summary được include

### Expected Results
- Response includes `agentSummary` per worktree
- Summary từ `orca_task_sessions`

---

## TC-WT-004-03: So sánh 5 worktrees — Parallel

**Priority:** P1

### Steps
1. Tạo fan-out với N=5
2. `worktree.compare { worktreeIds: [wt1, wt2, wt3, wt4, wt5], ... }`
3. Verify parallel execution

### Expected Results
- 5x `git.exec` calls (parallel via Promise.all)
- Response trong reasonable time

### Assertions
```
result = await rpc.call('worktree.compare', { worktreeIds: fiveWorktreeIds, ... })
assert result.length === 5
// All calls in parallel (timing)
assert spyRelay.callCount('git.exec') === 5
```

---

## TC-WT-004-04: So sánh — worktreeId không tồn tại

**Priority:** P1

### Steps
1. `worktree.compare { worktreeIds: ['wt-real', 'wt-nonexistent'], ... }`

### Expected Results
- Error: `{ code: 'WORKTREE_NOT_FOUND', worktreeId: 'wt-nonexistent' }`
- Hoặc: partial result với error flag cho worktree không tồn tại

---

*TC-WT-004 — Orca v5.0 — 2026-08-01*
