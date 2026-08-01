# TASK-TG-001: Fix ProfileAwareAgentSpawner params mismatch với relay agent.exec

**Priority:** 🔴 CRITICAL — Task graph hoàn toàn không hoạt động  
**Effort:** ~10 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TG-001  
**Solution ref:** [SOLUTION-task-graph-exact.md](../solutions/SOLUTION-task-graph-exact.md)

---

## Mục tiêu

Sửa `relay.call('agent.exec', ...)` trong `ProfileAwareAgentSpawner.spawn()` để dùng đúng params `{ binary, args, cwd }` thay vì `{ command, workdir }`.

## File cần sửa

```
src/main/project/ProfileAwareAgentSpawner.ts
```

## Thay đổi cụ thể

### Lines 104–110 — Sửa relay.call params:

**TRƯỚC (buggy params):**
```typescript
// 6. Get relay and send agent.exec
const relay = await this.router.getRelayForProject(projectId, userId)
const result = await relay.call('agent.exec', {
  command,
  workdir: workdir ?? project.repoPath,
  env: profileEnv,
})
```

**SAU (correct params matching agent-rpc-dispatch.ts:506–516):**
```typescript
// 6. Get relay and send agent.exec
const relay = await this.router.getRelayForProject(projectId, userId)

// Parse command string into binary + args (as expected by agent.exec handler)
// agent-rpc-dispatch.ts:506: const binary = typeof p.binary === 'string' ? p.binary : ''
// agent-rpc-dispatch.ts:507: const args = Array.isArray(p.args) ? ... : []
const commandParts = command.trim().split(/\s+/).filter(Boolean)
const binary = commandParts[0] ?? ''
const args   = commandParts.slice(1)

const result = await relay.call('agent.exec', {
  binary,                              // ← required (was: "command")
  args,                                // ← array (was: missing)
  cwd: workdir ?? project.repoPath,    // ← "cwd" (was: "workdir")
  env: profileEnv,
  timeoutMs: 5 * 60 * 1000,           // 5 minutes
})
```

## Verification

```bash
pnpm tsc --noEmit

# Verify: agent.exec handler expects binary+args+cwd:
grep -n "p.binary\|p.args\|p.cwd\|p.workdir" src/relay/agent-rpc-dispatch.ts | head -10
# Expected: p.binary, p.args, p.cwd (NOT p.command, p.workdir)

# Run tests nếu có:
pnpm vitest run src/main/project/__tests__/ 2>/dev/null || true
```

## Dependency

Không có dependency — fix này là independent.
