# BL-FLEET-03: Fleet Health Monitoring

**Domain:** Fleet Management  
**Priority:** P1  
**Actor chính:** Admin, DevOps  
**Tham chiếu:** FR-15.3, UR-122, F27

---

## Mô tả

`FleetHealthMonitor` poll trạng thái sức khỏe của tất cả dev servers theo interval định kỳ, expose metrics qua Prometheus và gửi webhook alert khi status thay đổi.

## Status Model

| Status | Điều kiện |
|--------|-----------|
| `healthy` | SSH + relay reachable, CPU < 80%, RAM < 85% |
| `degraded` | Relay reachable nhưng CPU > 80% hoặc RAM > 85% |
| `unhealthy` | Relay không reachable nhưng SSH còn |
| `unreachable` | SSH connect timeout/fail |

## Poll Flow (per server)

```
Interval: 60s (configurable via FLEET_POLL_INTERVAL_SEC)

1. SSH connect (timeout: 5s)
2. Relay health check: GET http://127.0.0.1:<relayPort>/health (via SSH tunnel)
3. Collect metrics:
   - CPU: cat /proc/stat hoặc top -bn1
   - RAM: free -b
   - Disk: df -P ~/.orca
   - SSH latency: time(connect)
4. Compare với previous status → if changed → emit 'status_change' event
5. Write metrics to in-memory store
6. Update server record: last_checked_at, status, metrics
```

## Prometheus Metrics

Endpoint: `GET /health/metrics` (Prometheus text format)

```
# HELP orca_fleet_server_status Fleet server health status (1=healthy, 0=not)
orca_fleet_server_status{server="dev1.example.com"} 1

# HELP orca_fleet_cpu_percent CPU usage percent per server
orca_fleet_cpu_percent{server="dev1.example.com"} 45.2

# HELP orca_fleet_ram_percent RAM usage percent per server
orca_fleet_ram_percent{server="dev1.example.com"} 67.8

# HELP orca_fleet_ssh_latency_ms SSH connect latency
orca_fleet_ssh_latency_ms{server="dev1.example.com"} 120
```

## Webhook Alerts

```json
POST <webhookUrl>
{
  "event": "fleet.server.status_change",
  "server": "dev1.example.com",
  "from": "healthy",
  "to": "unhealthy",
  "timestamp": "2026-07-28T07:00:00Z",
  "metrics": { "cpu": 0, "ram": 0 }
}
```

Config: `FLEET_WEBHOOK_URL` env hoặc per-server trong fleet YAML.

## Source References

- `src/main/fleet/fleet-health-monitor.ts` — FleetHealthMonitor class
- `src/main/server/health-router.ts` — /health/metrics endpoint
- `src/main/fleet/fleet-alerts.ts` — webhook delivery
