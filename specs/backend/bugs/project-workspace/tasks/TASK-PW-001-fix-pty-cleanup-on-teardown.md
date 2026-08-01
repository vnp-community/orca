# TASK-PW-001: Fix WorkspaceService không cleanup PTY khi teardown

**Priority:** 🔴 HIGH — PTY process leak khi workspace close  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-PW-001  
**Solution ref:** [SOLUTION-project-workspace.md](../solutions/SOLUTION-project-workspace.md)

## Bước 1 — Tìm teardown method

```bash
grep -n "teardown\|cleanup\|destroy\|dispose" src/main/workspace/WorkspaceService.ts 2>/dev/null | head -15
```

## Bước 2 — Thêm PTY cleanup

```typescript
// WorkspaceService.ts — trong teardown() hoặc close():
async teardown(workspaceId: string): Promise<void> {
  const workspace = this.activeWorkspaces.get(workspaceId)
  if (!workspace) return

  // FIX PW-001: Kill all PTY processes before closing workspace
  const terminals = await this.terminalManager.listByWorkspace(workspaceId)
  await Promise.allSettled(
    terminals.map(t => this.terminalManager.kill(t.id, 'SIGTERM'))
  )

  // Wait 2s for graceful exit, then SIGKILL:
  await new Promise(r => setTimeout(r, 2000))
  await Promise.allSettled(
    terminals.map(t => this.terminalManager.kill(t.id, 'SIGKILL').catch(() => {}))
  )

  // Remove workspace from active map:
  this.activeWorkspaces.delete(workspaceId)
  await this.repository.delete(workspaceId)
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: open workspace → create terminal → close workspace → no zombie PTY processes
# ps aux | grep pty | grep -v grep → should be empty after teardown
```
