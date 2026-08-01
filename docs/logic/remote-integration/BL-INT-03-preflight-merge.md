# BL-INT-03: Preflight Status Merge (Local + Remote)

**Domain:** Remote Source Control Integrations  
**Priority:** P1  
**Actor chính:** Carlos, Alex  
**Tham chiếu:** FR-18.1, F30

---

## Mô tả

`mergePreflightStatuses()` kết hợp kết quả preflight check từ local Orca instance và remote Dev Server (qua SSH relay), cho kết quả UI thống nhất. Ưu tiên relay results; fallback sang local nếu relay không available.

## Preflight Check Sources

### Source 1: Local checks (Orca Web Server)
- Git version (local): `git --version`
- API token checks: `WebCredentialStore.get(service)` → validate token format
- Network reachability: ping provider APIs

### Source 2: Remote checks (Dev Server qua relay)
- GitHub CLI: `gh auth status` (với GH_CONFIG_DIR isolation)
- GitLab CLI: `glab auth status`
- Node.js version: `node --version`
- Disk space, port availability

## Merge Algorithm

```typescript
function mergePreflightStatuses(
  local: PreflightCheckResult[],
  relay: PreflightCheckResult[]
): PreflightCheckResult[] {
  const merged = new Map<string, PreflightCheckResult>();
  
  // 1. Seed với local results
  for (const check of local) {
    merged.set(check.id, check);
  }
  
  // 2. Relay results override local (relay is authoritative for CLI checks)
  for (const check of relay) {
    merged.set(check.id, check);
  }
  
  // 3. Nếu relay error (SSH fail) → giữ local results + thêm warning
  if (relayError) {
    merged.set('relay-connectivity', {
      id: 'relay-connectivity',
      status: 'warning',
      message: 'Cannot reach Dev Server — showing local checks only'
    });
  }
  
  return [...merged.values()];
}
```

## PreflightCheckResult Schema

```typescript
interface PreflightCheckResult {
  id: string;           // "github-cli", "git-version", "disk-space", ...
  status: 'ok' | 'warning' | 'error' | 'skip';
  message: string;      // Human-readable (vd: "gh 2.52.0 installed")
  details?: string;     // Extra info (vd: full auth status output)
  source: 'local' | 'relay'; // Which source
}
```

## UI Display

```
Preflight Checks:
  ✓ GitHub CLI (gh 2.52.0) — Logged in as @username     [relay]
  ✓ Git 2.45.0                                            [relay]
  ✓ Disk space: 120GB free                                [relay]
  ⚠ GitLab CLI: not installed (optional)                  [relay]
  ✓ Linear API token: valid                               [local]
  ⚠ Dev Server: SSH connection slow (120ms latency)       [relay]
```

## RpcMethodContext Injection

Mọi RPC method được inject context:
```typescript
interface RpcMethodContext {
  userId: string;           // From session (web mode)
  devServerManager: DevServerManager | null; // Available khi có DevServer
  // → method có thể proxy request tới relay
}
```

## Source References

- `src/main/integrations/preflight-merge.ts`
- `src/main/integrations/preflight-runner.ts` — runLocalChecks(), runRelayChecks()
- `src/main/rpc/rpc-context.ts` — RpcMethodContext
- `src/renderer/src/components/PreflightPanel.tsx`
