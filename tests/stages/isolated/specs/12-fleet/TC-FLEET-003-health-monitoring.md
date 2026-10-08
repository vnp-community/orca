# TC-FLEET-003 — Fleet Health Monitoring

**BL Reference:** BL-FLEET-03  
**Priority:** P1

---

## TC-FLEET-003-01: Health status — healthy

### Steps
1. Server: CPU 20%, RAM 40%, disk 50%, latency 10ms
2. `fleet.getHealth { serverId }`

### Expected Results
- Status: 'healthy'

---

## TC-FLEET-003-02: Health status thresholds

| Condition | Status |
|-----------|--------|
| CPU < 70%, RAM < 80%, disk < 90% | healthy |
| CPU 70-90% OR RAM 80-90% | degraded |
| CPU > 90% OR RAM > 90% OR disk > 90% | unhealthy |
| Unreachable | unreachable |

---

## TC-FLEET-003-03: Prometheus metrics endpoint

### Steps
1. `GET /metrics` (server endpoint)

### Expected Results
- Prometheus format:
  ```
  orca_server_cpu_usage{server="srv-1"} 20.5
  orca_server_ram_usage{server="srv-1"} 41.2
  ```

---

## TC-FLEET-003-04: Webhook alert khi status thay đổi

### Steps
1. Server changes from 'healthy' to 'unhealthy'
2. Verify webhook sent

### Expected Results
- POST to `fleetAlertWebhookUrl`
- Payload: `{ server: 'srv-1', oldStatus: 'healthy', newStatus: 'unhealthy' }`

---

## TC-FLEET-003-05: Poll interval — 60s default

### Steps
1. Verify poll frequency

### Expected Results
- FleetHealthMonitor polls mỗi 60s

