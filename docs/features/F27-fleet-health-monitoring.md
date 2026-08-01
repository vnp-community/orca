# F27 — Fleet Health Monitoring

| Trường | Giá trị |
|--------|---------|
| **ID** | F27 |
| **Tên** | Fleet Health Monitoring |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [remote-server/CR-005](../crs/v1/remote-server/CR-005-fleet-health-monitoring.md) |
| **TDD** | [TDD-13: Dev Server Onboarding](../specs/backend/tdd/13-dev-server-onboarding.md) |
| **Phiên bản** | v3.0+ |
| **ADR References** | ADR-004 |
| **HLD References** | C3.5 |

---

## Mô tả

Orca theo dõi **sức khỏe của toàn bộ fleet dev servers** theo thời gian thực — CPU, RAM, disk, relay status, network latency. Cảnh báo khi server unhealthy. Hỗ trợ Prometheus metrics endpoint.

---

## Tính năng chi tiết

### Realtime Health Checks
- `FleetHealthMonitor` — poll mỗi server theo interval (mặc định 60s)
- SSH + relay ping để kiểm tra connectivity
- Thu thập metrics: CPU%, RAM%, disk%, latency ms
- Phân loại status: `healthy` | `degraded` | `unhealthy` | `unreachable`

### FleetHealthStore
- Lưu trữ kết quả health check (in-memory + SQLite)
- Historical data cho trend analysis

### FleetStatusService
- Aggregate status từ tất cả servers
- `getFleetStatus()` → summary: `{total, healthy, degraded, unhealthy}`
- `onStatusChange(handler)` — event callback

### Prometheus Metrics Endpoint
```
GET :6769/metrics   (hoặc :6768/metrics qua ws-transport HTTP hook)
```
Output:
```
# HELP orca_fleet_servers_total Total registered fleet servers
orca_fleet_servers_total 5
# HELP orca_fleet_healthy_servers Healthy fleet servers count
orca_fleet_healthy_servers 4
# HELP orca_server_cpu_percent CPU usage percent per server
orca_server_cpu_percent{server="b15.openledger.vn"} 23.5
```

### Webhook Alerts
- `fleetAlertWebhookUrl` trong `GlobalSettings`
- POST đến webhook khi server status thay đổi
- Payload: `{serverId, serverName, status, metrics, timestamp}`

### Fleet Dashboard UI
- `FleetHealthDashboard` React component
- Real-time status cards per server
- CPU/RAM/disk progress bars
- Latency indicator (green/yellow/red)

### Global Settings
```typescript
interface GlobalSettings {
  fleetMetricsEnabled: boolean        // enable /metrics endpoint
  fleetAlertWebhookUrl?: string       // webhook URL cho alerts
  fleetHealthCheckIntervalMs: number  // default 60000
}
```

---

## Tiêu chí chấp nhận

- [x] Health check tất cả servers theo interval
- [x] Status phân loại: healthy/degraded/unhealthy/unreachable
- [x] `FleetStatusService.getFleetStatus()` trả về aggregate
- [x] Prometheus metrics tại `/metrics`
- [x] Webhook alert khi status thay đổi
- [x] `GlobalSettings.fleetMetricsEnabled` toggle metrics
- [x] Fleet Dashboard UI hiển thị realtime

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Fleet health monitor | `src/main/ssh/fleet-health-monitor.ts` |
| Fleet health store | `src/main/ssh/fleet-health-store.ts` |
| Fleet status service | `src/main/ssh/fleet-status-service.ts` |
| Fleet remote commands | `src/main/ssh/fleet-remote-commands.ts` |
| Metrics endpoint | `src/main/runtime/rpc/ws-transport.ts` (HTTP hook) |
| Global settings | `src/shared/types.ts` |
| Dashboard UI | `src/renderer/src/components/fleet/FleetHealthDashboard.tsx` |

**Tests:** via fleet integration tests
