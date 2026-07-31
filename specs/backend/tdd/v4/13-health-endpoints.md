# TDD-BE-13: Health Endpoints

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/server/health-endpoint.ts`

---

## 1. Endpoints

```
GET /health
  → 200 OK { status: 'ok', version: string, uptime: number }
  → 503 Service Unavailable { status: 'degraded', ... } nếu DB unhealthy

GET /health/ready
  → 200 OK { ready: true } nếu DB ready
  → 503 { ready: false, reason: string } nếu chưa ready

GET /health/metrics
  → 200 OK { db: DbMetrics, process: ProcessMetrics }
```

---

## 2. DbMetrics

```typescript
type DbMetrics = {
  dialect:       string
  queryCount:    number
  errorCount:    number
  avgLatencyMs:  number
  lastCheckAt:   number    // Unix ms
  healthy:       boolean
}
```

---

## 3. ProcessMetrics

```typescript
type ProcessMetrics = {
  uptimeSeconds: number
  memoryMb: {
    heapUsed:  number
    heapTotal: number
    rss:       number
  }
  pid:     number
  version: string
}
```

---

## 4. Tích hợp vào HttpServer

```typescript
// Mounted trước static files (high priority)
if (options.dbMonitor) {
  app.get('/health',         (req, res) => handleHealth(req, res, options.dbMonitor!))
  app.get('/health/ready',   (req, res) => handleHealthReady(req, res, options.dbMonitor!))
  app.get('/health/metrics', (req, res) => handleHealthMetrics(req, res, options.dbMonitor!))
}
```

---

## 5. Docker healthcheck config

```yaml
# deploy/prod/docker-compose.yml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:6769/health/ready"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

---

## 6. Liveness vs Readiness

| Endpoint | Meaning | Used by |
|----------|---------|---------|
| `/health` | App process running (liveness) | Load balancer keepalive |
| `/health/ready` | DB connected, migrations done (readiness) | Docker healthcheck, k8s readiness probe |
| `/health/metrics` | Detailed stats | Monitoring systems (Prometheus, Grafana) |

---

## 7. SQLite Edge Case

Khi dialect='sqlite' (local file):
- `/health/ready` luôn `ready: true` (file-based, always available sau migration)
- `/health/metrics` vẫn track query latency
