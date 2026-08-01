# ADR-020 — Enterprise Rollout Phases & Backward Compatibility

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-020 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | deployment.md, README (Rollout Strategy), security.md |
| **CR Ref** | CR-DS-001–005 |
| **Code Ref** | `src/main/dev-server/agent-dispatcher.ts` (dual-mode), `deploy/agent/` |
| **Feature Ref** | F22–F39 (tất cả enterprise features) |
| **Amends** | [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) Migration Path |
| **Related** | [enterprise-migration-impact-assessment.md](../enterprise-migration-impact-assessment.md) |

---

## Bối cảnh

Orca enterprise migration trải qua 3 phases rõ ràng theo HLD deployment.md và feature timeline. ADR-013 đã phác thảo migration path ở mức cao, nhưng cần ADR chi tiết hơn về:

1. **Feature flags** — bật tắt features theo phase
2. **Backward compat** — relay (v5) + agent (v6) cùng tồn tại
3. **Rollback triggers** — khi nào và cách rollback
4. **Database version gate** — ngăn chặn feature access khi migrations chưa chạy

---

## Quyết định

### Phase Structure

```
Phase 1: Web Server Baseline (v5.0a)    ← Migrations 0001–0005
Phase 2: Enterprise Features (v5.0b)   ← Migrations 0006–0010
Phase 3: Dev Server Agent v6.0         ← Agent binary upgrade
```

### 1. Feature Flags (Server-side)

```typescript
// src/main/feature-flags.ts
// Feature flags controlled by environment + migration state

class FeatureFlags {
  private migrationVersion: number  // highest completed migration

  isEnabled(feature: EnterpriseFeature): boolean {
    switch (feature) {
      // Phase 1 features — require migrations 0001–0005
      case 'WEB_SERVER_MODE':         return this.migrationVersion >= 5
      case 'MULTI_USER_AUTH':         return this.migrationVersion >= 5
      case 'ADMIN_PANEL':             return this.migrationVersion >= 5
      case 'FLEET_MONITORING':        return this.migrationVersion >= 5

      // Phase 2 features — require migrations 0006–0010
      case 'PROFILE_HIERARCHY':       return this.migrationVersion >= 6
      case 'PROJECT_BINDING':         return this.migrationVersion >= 7
      case 'AI_PROVIDER_MANAGEMENT':  return this.migrationVersion >= 8
      case 'WORKFLOW_ORCHESTRATION':  return this.migrationVersion >= 9
      case 'TASK_GRAPH':              return this.migrationVersion >= 10

      // Phase 3 features — require Agent v6.0 connected
      case 'REMOTE_GIT_UI':           return this.hasConnectedAgent()
      case 'PROJECT_WORKSPACE':       return this.hasConnectedAgent()
      case 'PER_USER_PTY_ISOLATION':  return this.hasConnectedAgent()
      case 'PROFILE_AWARE_SPAWNER':   return this.hasConnectedAgent()
    }
  }

  private hasConnectedAgent(): boolean {
    return AgentConnectionManager.hasActiveConnection()
  }
}
```

### 2. Dual-Mode AgentDispatcher (Relay + Agent Backward Compat)

```typescript
// src/main/dev-server/agent-dispatcher.ts
// Dual-mode: ưu tiên Agent (v6), fallback về relay (v5)

class AgentDispatcher {
  async call(devServerId: string, method: string, params: any): Promise<any> {
    // TRY: Agent (v6) mode
    const agentConn = AgentConnectionManager.get(devServerId)
    if (agentConn?.isAlive) {
      return agentConn.rpc(method, params)
    }

    // FALLBACK: Relay (v5) mode
    const relayConn = RelayConnectionPool.get(devServerId)
    if (relayConn?.state === 'ready') {
      // Warn admin panel: server using legacy relay
      adminEventBus.emit('fleet.legacyRelay', { devServerId })
      return relayConn.call(method, params)
    }

    throw new Error(`No connection available for ${devServerId}`)
  }

  getMode(devServerId: string): 'agent-v6' | 'relay-v5' | 'offline' {
    if (AgentConnectionManager.get(devServerId)?.isAlive) return 'agent-v6'
    if (RelayConnectionPool.get(devServerId)?.state === 'ready') return 'relay-v5'
    return 'offline'
  }
}
```

### 3. Admin Panel — Server Mode Indicator

```tsx
// Admin Panel Fleet View: hiển thị mode của mỗi Dev Server

<ServerStatusBadge devServer={server}>
  {dispatcher.getMode(server.id) === 'agent-v6' && (
    <Badge color="green">Agent v6.0</Badge>
  )}
  {dispatcher.getMode(server.id) === 'relay-v5' && (
    <Badge color="yellow" tooltip="Upgrade to Agent v6.0 for full features">
      ⚠️ Legacy Relay v5
    </Badge>
  )}
  {dispatcher.getMode(server.id) === 'offline' && (
    <Badge color="red">Offline</Badge>
  )}
</ServerStatusBadge>
```

### 4. Migration Version Gate (API-level)

```typescript
// src/main/api/feature-gate-middleware.ts
// Middleware: từ chối API calls cho features chưa available

export function requireMigration(version: number): RequestHandler {
  return (req, res, next) => {
    if (migrationState.version < version) {
      res.status(503).json({
        error: 'FEATURE_NOT_AVAILABLE',
        message: `Requires migration version ${version}. Current: ${migrationState.version}`,
        runMigrationsFirst: true
      })
      return
    }
    next()
  }
}

// Usage in routes:
app.use('/api/projects', requireMigration(7), projectRouter)
app.use('/api/ai-providers', requireMigration(8), aiProviderRouter)
app.use('/api/workflows', requireMigration(9), workflowRouter)
app.use('/api/tasks', requireMigration(10), taskRouter)
```

### 5. Rollback Decision Matrix

| Scenario | Action | Data Loss |
|---|---|---|
| Migration 0006 fails | Auto-rollback (transaction), server stays on 0005 | None |
| Migration 0007 fails | Auto-rollback, 0006 remains | None |
| Phase 2 features not working | Set env `ORCA_MAX_MIGRATION=5` → Phase 1 only | Phase 2 data preserved, inaccessible |
| Agent v6 causing issues | Server auto-fallbacks to relay | None (relay still works) |
| Full rollback to v4.x | Manual DB migration down | ⚠️ See enterprise-migration-impact-assessment.md |

---

## Deployment Timeline (Recommended)

```
Week 1–2: Phase 1 Deployment
  → docker pull orca:5.0a
  → export ORCA_DB_URL=postgresql://...
  → docker run → migrations 0001–0005 auto-apply
  → Verify: /health/ready, /admin login
  → GATE: no Phase 2 features (profile/project/AI/workflow/task)

Week 3–4: Phase 2 Deployment
  → docker pull orca:5.0b  (same Docker image, includes 0006–0010)
  → Server start → migrations 0006–0010 auto-apply
  → Admin: create Company profile → create Departments → create Projects
  → Admin: register AI Provider accounts
  → GATE: Phase 3 features (remote git UI) still disabled (no Agent)

Week 5–8: Phase 3 — Agent v6.0 Rollout
  → Per Dev Server: download agent binary + install script
  → sudo systemctl enable --now orca-agent
  → Admin Panel: Dev Servers → [server] shows "Agent v6.0" badge
  → Phase 3 features auto-enable
  → Monitor: Audit Log, Event Stream

Week 9+: Decommission Legacy Relay
  → Verify ALL servers show "Agent v6.0" (not "Legacy Relay")
  → Schedule relay deprecation for v7.0
```

---

## ORCA_MAX_MIGRATION Environment Variable

Cho phép operator kiểm soát feature availability mà không cần rollback:

```bash
# Only Phase 1 features (even if 0006–0010 applied):
ORCA_MAX_MIGRATION=5

# Full features (default: use actual migration version):
ORCA_MAX_MIGRATION=10  # or unset
```

---

## Hậu quả

**Tích cực:**
- Feature flags ngăn chặn access trước khi ready (no broken UI)
- Dual-mode: relay fallback → no service interruption during agent rollout
- Admin Panel mode indicator → clear upgrade path for ops team
- Migration gate → API returns 503 gracefully (not 500)

**Tiêu cực:**
- Dual-mode AgentDispatcher thêm complexity trong hot path
- `hasConnectedAgent()` phải efficient (in-memory Map lookup, không phải DB)
- Feature flag state cần invalidate khi agent connect/disconnect

---

## Trạng thái Implementation

❌ Chưa implement  
🎯 `src/main/feature-flags.ts` — migration-gated feature flags  
🎯 `src/main/dev-server/agent-dispatcher.ts` — dual-mode dispatch  
🎯 `src/main/api/feature-gate-middleware.ts` — API gate  
🎯 Admin Panel: ServerStatusBadge + mode indicator  
🎯 `deploy/agent/install.sh` — agent install script

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [enterprise-migration-impact-assessment.md](../enterprise-migration-impact-assessment.md) | Deployment checklist + rollback plan chi tiết |
| [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) | Migration Path (phác thảo) |
| [ADR-016](./ADR-016-db-migrations-0006-0010-schema.md) | DB migrations 0006–0010 |
| [ADR-017](./ADR-017-dev-server-agent-layer-model.md) | Agent layer model |
| [ADR-018](./ADR-018-control-plane-data-plane-separation.md) | Control/Data plane |
| [ADR-019](./ADR-019-agent-autonomous-operation-reconnect.md) | Agent reconnect strategy |
| **HLD deployment.md** | Deployment diagram + Docker Compose |
| **HLD security.md** | Trust boundaries per phase |
