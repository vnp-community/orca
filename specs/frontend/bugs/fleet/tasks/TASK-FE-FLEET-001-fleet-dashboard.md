# TASK-FE-FLEET-001: Implement Fleet Dashboard UI

**Priority:** 🟠 HIGH  
**Effort:** ~120 phút  
****Status:** ✅ DONE — Implemented  
**Bug refs:** BUG-FE-FLEET-001  
**Solution ref:** [SOL-FE-FLEET-001](../solutions/SOL-FE-FLEET-001-fleet-dashboard-implementation.md)

## Mục tiêu

Implement Fleet Dashboard hiển thị tất cả Dev Servers, health status, và controls (connect/disconnect/remove).

## Files cần tạo / sửa

Xem chi tiết trong [SOL-FE-FLEET-001](../solutions/SOL-FE-FLEET-001-fleet-dashboard-implementation.md).

Key components:
- `src/renderer/src/components/fleet/FleetDashboard.tsx` (NEW)
- `src/renderer/src/components/fleet/ServerCard.tsx` (NEW)
- `src/renderer/src/hooks/useFleet.ts` (NEW hoặc mở rộng)

## Pattern

```typescript
// FleetDashboard.tsx
export function FleetDashboard() {
  const { servers, loading } = useFleet()

  return (
    <div className="fleet-dashboard">
      <h2>Dev Servers ({servers.length})</h2>
      {loading ? <Spinner /> : (
        <div className="server-grid">
          {servers.map(s => <ServerCard key={s.id} server={s} />)}
        </div>
      )}
    </div>
  )
}

// ServerCard.tsx
function ServerCard({ server }: { server: DevServer }) {
  const statusColor = {
    connected: 'green', connecting: 'yellow',
    disconnected: 'gray', error: 'red'
  }[server.status]

  return (
    <div className={`server-card status-${server.status}`}>
      <span className="status-dot" style={{ background: statusColor }} />
      <h3>{server.name}</h3>
      <p>{server.mode} — {server.status}</p>
      {server.health && <p>Latency: {server.health.latencyMs}ms</p>}
      <button onClick={() => rpc.call('devServer.connect', { id: server.id })}>
        {server.status === 'connected' ? 'Disconnect' : 'Connect'}
      </button>
    </div>
  )
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: Fleet Dashboard hiển thị đúng số lượng và status của servers
```
