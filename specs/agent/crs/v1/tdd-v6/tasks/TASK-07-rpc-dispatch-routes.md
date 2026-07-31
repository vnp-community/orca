# TASK-07: Add All New RPC Routes to agent-rpc-dispatch.ts

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:52

**Phase:** 4 (thực hiện SAU khi TASK-01 đến TASK-06 đã xong)
**File:** `src/relay/agent-rpc-dispatch.ts`
**Operation:** EXTEND (thêm cases vào switch)
**CR:** CR-AG-09, CR-AG-10, CR-AG-11, CR-AG-12, CR-AG-13
**Depends on:** TASK-02, TASK-03, TASK-04, TASK-05, TASK-06
**Blocked by:** Tất cả TASK-02 đến TASK-06 phải xong trước

---

## Mục tiêu

Thêm **13 case routes mới** vào `route()` function trong `agent-rpc-dispatch.ts`.

File hiện tại (266 lines) có switch ending tại line 229:
```typescript
    default:
      return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`)
  }
}
```

---

## Thay đổi cần thực hiện

### Edit — Thay thế `default:` block (lines 226-229) bằng toàn bộ đoạn sau:

```typescript
    // ── v5.0: ai.provider.deleteCredential ───────────────────────────────────
    case 'ai.provider.deleteCredential': {
      try {
        const { handleDeleteCredential } = await import('./agent-credential-store')
        return (await handleDeleteCredential(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.deleteCredential unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case 'git.pr.create': {
      try {
        const { handleGitPrCreate } = await import('./agent-git-handler')
        return (await handleGitPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.list ──────────────────────────────────────────────
    case 'git.worktree.list': {
      try {
        const { handleGitWorktreeList } = await import('./agent-git-handler')
        return (await handleGitWorktreeList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.add ───────────────────────────────────────────────
    case 'git.worktree.add': {
      try {
        const { handleGitWorktreeAdd } = await import('./agent-git-handler')
        return (await handleGitWorktreeAdd(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.remove ────────────────────────────────────────────
    case 'git.worktree.remove': {
      try {
        const { handleGitWorktreeRemove } = await import('./agent-git-handler')
        return (await handleGitWorktreeRemove(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.remove unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.stat ────────────────────────────────────────────────────────
    case 'fs.stat': {
      try {
        const { handleFsStat } = await import('./fs-agent-extensions')
        return (await handleFsStat(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.stat unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.glob ────────────────────────────────────────────────────────
    case 'fs.glob': {
      try {
        const { handleFsGlob } = await import('./fs-agent-extensions')
        return (await handleFsGlob(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.glob unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.writeFile ───────────────────────────────────────────────────
    case 'fs.writeFile': {
      try {
        const { handleFsWriteFile } = await import('./fs-agent-extensions')
        return (await handleFsWriteFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.writeFile unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.pr.create ───────────────────────────────────────────────
    case 'github.pr.create': {
      try {
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.pr.merge ────────────────────────────────────────────────
    case 'github.pr.merge': {
      try {
        const { handleGitHubPrMerge } = await import('./external-api-connector')
        return (await handleGitHubPrMerge(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.merge unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.issue.list ──────────────────────────────────────────────
    case 'github.issue.list': {
      try {
        const { handleGitHubIssueList } = await import('./external-api-connector')
        return (await handleGitHubIssueList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.mr.create ───────────────────────────────────────────────
    case 'gitlab.mr.create': {
      try {
        const { handleGitLabMrCreate } = await import('./external-api-connector')
        return (await handleGitLabMrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.pipeline.status ─────────────────────────────────────────
    case 'gitlab.pipeline.status': {
      try {
        const { handleGitLabPipelineStatus } = await import('./external-api-connector')
        return (await handleGitLabPipelineStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.pipeline.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case 'agent.spawn': {
      try {
        const { handleAgentSpawn } = await import('./agent-spawner')
        // Fire-and-forget: streaming handler sends multiple frames
        void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.kill ─────────────────────────────────────────────────────
    case 'agent.kill': {
      try {
        const { handleAgentKill } = await import('./agent-spawner')
        return (await handleAgentKill(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`)
      }
    }

    // ── Unknown method ───────────────────────────────────────────────────────
    default:
      return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`)
  }
}
```

---

## Verify

```bash
# TypeScript compile
npx tsc --noEmit -p config/tsconfig.node.json

# Count total case statements
grep -c "case '" src/relay/agent-rpc-dispatch.ts
# Expected: 22+ cases (8 existing + 15 new = 23)

# Verify all new routes exist
grep "case 'ai.provider.deleteCredential'\|case 'git.pr.create'\|case 'git.worktree'\|case 'fs.stat'\|case 'fs.glob'\|case 'fs.writeFile'\|case 'github.pr'\|case 'github.issue'\|case 'gitlab.mr'\|case 'gitlab.pipeline'\|case 'agent.spawn'\|case 'agent.kill'" \
  src/relay/agent-rpc-dispatch.ts
```

---

## Done criteria

- [ ] `ai.provider.deleteCredential` route thêm
- [ ] `git.pr.create` route thêm
- [ ] `git.worktree.list`, `git.worktree.add`, `git.worktree.remove` routes thêm
- [ ] `fs.stat`, `fs.glob`, `fs.writeFile` routes thêm
- [ ] `github.pr.create`, `github.pr.merge`, `github.issue.list` routes thêm
- [ ] `gitlab.mr.create`, `gitlab.pipeline.status` routes thêm
- [ ] `agent.spawn` (fire-and-forget), `agent.kill` routes thêm
- [ ] TypeScript compile không lỗi
- [ ] Build thành công: `pnpm run build:relay`
