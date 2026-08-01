# BUG-BE-FLEET-002: `FleetHealthMonitor` chỉ check SSH connection state — không gọi `relay.call('health.get')` để lấy CPU/RAM/Disk metrics

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-FLEET-001  
**Note:** FleetHealthMonitor.runHealthCheck() polls connection state + webhook alerts  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-FLEET-03) mô tả:
```
FleetHealthMonitor cron mỗi 30 giây:
    FOR each active server:
        WebSocket ping: relay.call('health.get')
        Response: { cpu: 45%, ram: 60%, disk: 30%, agentCount: 2, latency: 12ms }
        INSERT health_metrics { serverId, cpu, ram, disk, latency, timestamp }
        cpu > 90% OR ram > 90% → status = 'warning'
        disk > 95% → status = 'critical'
```

Nhưng `fleet-health-monitor.ts` thực tế chỉ:
1. Lấy SSH connection state (status: connected/disconnected)
2. Record vào `fleetHealthStore` (connection state only)
3. **Không gọi `relay.call('health.get')`**
4. **Không có CPU/RAM/Disk metrics**
5. **Không có threshold alerting**

```typescript
// fleet-health-monitor.ts:50-60
const state = this.getConnectionState(target.id)
const status = state?.status ?? 'disconnected'
fleetHealthStore.recordConnectionState({
  targetId, status, error: state?.error,
  reconnectAttempt: 0
  // ← THIẾU: cpu, ram, disk, latency, agentCount
})
```

## File liên quan

- [`src/main/ssh/fleet-health-monitor.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/ssh/fleet-health-monitor.ts) — Lines 46-75

## Ảnh hưởng

1. Admin dashboard không có CPU/RAM/Disk data — chỉ có connected/disconnected.
2. Threshold alerting (cpu > 90%) không hoạt động.
3. `INSERT health_metrics` với proper metrics không được gọi.
4. Webhook alerts (Slack, PagerDuty) không trigger vì không có metric data.
5. Cron interval là **60 seconds** thay vì **30 seconds** như HLD quy định.

## Sai khác cụ thể

| HLD | Code thực tế |
|-----|-------------|
| Cron mỗi **30s** | `DEFAULT_PING_INTERVAL_MS = 60_000` (60s) |
| `relay.call('health.get')` | Không có — chỉ `getConnectionState()` |
| `{ cpu, ram, disk, latency }` | `{ status, error }` |
| `cpu > 90% → 'warning'` | Không có threshold logic |
| `disk > 95% → 'critical'` | Không có |

## Liên quan đến luồng

- **BL-FLEET-03**: Fleet health monitoring — metrics không collect.
