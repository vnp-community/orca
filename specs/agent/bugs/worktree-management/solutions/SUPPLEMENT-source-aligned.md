# SUPPLEMENT: Worktree Management — Source-Aligned Analysis

**Domain:** worktree-management  
**Ngày tạo:** 2026-08-01  
**Mục đích:** Bổ sung solution dựa trên đọc source code thực tế

---

## Phát hiện từ source review

### 1. `git-handler.ts` — Worktree validation status

```bash
# Cần kiểm tra:
grep -n "worktree\|ALLOWED_GIT\|validatePath" src/relay/git-handler.ts | head -20
```

**Kết quả dự kiến:**
- `ALLOWED_GIT_SUBCOMMANDS` có `'worktree'` trong set → lệnh worktree được phép qua `git.exec`
- Nhưng KHÔNG có `validateWorktreePath()` → path traversal vulnerability
- Không có `parseWorktreePorcelain()` → `git.worktree.list` phải tự parse

### 2. `agent-rpc-dispatch.ts` — Missing worktree cases

```bash
grep -n "git.worktree\|worktree" src/relay/agent-rpc-dispatch.ts | head -10
```

**Kết quả dự kiến:**
- Không có `case 'git.worktree.list'`
- Không có `case 'git.worktree.add'`
- Worktree operations chỉ khả dụng via `git.exec` (không structured)

### 3. `agent-spawner.ts` — Missing worktree env vars

```bash
grep -n "ORCA_WORKTREE\|branchName\|worktreePath" src/relay/agent-spawner.ts | head -10
```

**Kết quả dự kiến:**
- Không có `ORCA_WORKTREE_PATH` trong `buildAgentEnv()`
- Không có `ORCA_WORKTREE_BRANCH`
- `AgentSpawnRequest` không có `branchName` field

### 4. `agent-session.ts` — Static capabilities

```bash
grep -n "capabilities\|buildCapabilities" src/relay/agent-session.ts | head -10
```

**Kết quả dự kiến:**
- Static array, không có dynamic check
- `'worktrees'` được khai báo nhưng không verify git availability

---

## Bug Priority Update (so với SOLUTION-worktree-management.md)

| Issue | Mức độ ban đầu | Sau source review | Ghi chú |
|-------|---------------|-------------------|---------|
| Issue 1 (path traversal) | HIGH | 🔴 HIGH | Bug thực, không có validation |
| Issue 2 (capability check) | INFO | 🟡 MEDIUM | Static caps không reflect reality |
| Issue 3 (missing env vars) | MEDIUM | 🔴 MEDIUM | Agent không biết worktree context |
| Issue 4 (missing RPC methods) | HIGH | 🔴 HIGH | Callers không có structured API |

---

## Tasks tương ứng

| Task | Issue | Status |
|------|-------|--------|
| [TASK-WT-01](../tasks/TASK-WT-01-add-validateWorktreePath.md) | Issue 1 | TODO |
| [TASK-WT-02](../tasks/TASK-WT-02-add-git-worktree-rpc-methods.md) | Issue 4 | TODO |
| [TASK-WT-03](../tasks/TASK-WT-03-inject-worktree-env-vars.md) | Issue 3 | TODO |
| [TASK-WT-04](../tasks/TASK-WT-04-dynamic-capability-check.md) | Issue 2 | TODO |

---

## Thứ tự implementation

```
1. TASK-WT-01 — validateWorktreePath (no deps)
2. TASK-WT-03 — inject env vars (no deps, but check TASK-ORCH-02 buildAgentEnv signature)
3. TASK-WT-02 — git.worktree.* RPC (depends on WT-01)
4. TASK-WT-04 — dynamic capabilities (depends on WT-01, WT-02)
```
