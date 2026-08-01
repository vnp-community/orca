# TC-WT-005 — Merge Worktree Thắng

**BL Reference:** BL-WT-05  
**Flow Reference:** docs/flows/logic/worktree-management.md#BL-WT-05  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya

---

## TC-WT-005-01: Merge strategy=merge — Happy Path

**Priority:** P1

### Preconditions
- Worktree `wt-winner` với branch `feature/winner`
- Branch `feature/winner` là fast-forward từ `main` (no conflict)

### Steps
1. RPC: `worktree.merge { worktreeId: 'wt-winner', strategy: 'merge', cleanup: true }`

### Expected Results
- `relay.call('git.exec', { args: ['merge-base', '--is-ancestor', branch, 'main'] })` check
- `relay.call('git.exec', { args: ['merge', 'feature/winner'] })` 
- DB: `orca_worktrees SET status='merged'`
- Cleanup: BL-WT-03 cho các worktrees còn lại
- WS event: `worktree:merged`

### Assertions
```
await rpc.call('worktree.merge', { worktreeId, strategy: 'merge', cleanup: true })
assert spyRelay.calledWith('git.exec', { args: ['merge', 'feature/winner'] })
assert db.worktrees.find({ id: worktreeId }).status === 'merged'
event = await wsEvents.next('worktree:merged')
assert event.worktreeId === worktreeId
```

---

## TC-WT-005-02: Merge strategy=squash

**Priority:** P1

### Steps
1. `worktree.merge { strategy: 'squash', ... }`

### Expected Results
- `relay.call('git.exec', { args: ['merge', '--squash', branch] })`

### Assertions
```
await rpc.call('worktree.merge', { strategy: 'squash', ... })
assert spyRelay.calledWith('git.exec', { args: include('--squash') })
```

---

## TC-WT-005-03: Merge strategy=rebase

**Priority:** P1

### Steps
1. `worktree.merge { strategy: 'rebase', ... }`

### Expected Results
- `relay.call('git.exec', { args: ['rebase', branch] })`

---

## TC-WT-005-04: Merge — Conflict detection

**Priority:** P1

### Preconditions
- Branch có conflict với main

### Steps
1. Mock `git.exec merge-base` → có conflict
2. `worktree.merge { ... }`

### Expected Results
- Error: `{ code: 'MERGE_CONFLICT', conflictFiles: ['file.ts', 'other.ts'] }`
- Không execute merge

---

## TC-WT-005-05: Merge với cleanup=true — Xóa các worktrees còn lại

**Priority:** P1

### Preconditions
- 3 worktrees: wt-A, wt-B (winner), wt-C

### Steps
1. `worktree.merge { worktreeId: 'wt-B', cleanup: true }`

### Expected Results
- wt-B: status='merged'
- wt-A và wt-C: bị delete (BL-WT-03 x2)
- DB: 1 worktree với status='merged', 2 worktrees deleted

### Assertions
```
await rpc.call('worktree.merge', { worktreeId: 'wt-B', cleanup: true })
assert db.worktrees.find({ id: 'wt-B' }).status === 'merged'
assert db.worktrees.find({ id: 'wt-A' }) === null
assert db.worktrees.find({ id: 'wt-C' }) === null
```

---

## TC-WT-005-06: Merge với cleanup=false — Giữ các worktrees khác

**Priority:** P1

### Steps
1. `worktree.merge { worktreeId: 'wt-B', cleanup: false }`

### Expected Results
- wt-B: status='merged'
- wt-A và wt-C: vẫn tồn tại với status='ready'

---

*TC-WT-005 — Orca v5.0 — 2026-08-01*
