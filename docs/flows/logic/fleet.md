# Luồng Dữ liệu — Fleet Management

**Domain:** Fleet Management  
**Nghiệp vụ:** BL-FLEET-01 → BL-FLEET-04  
**Kiến trúc tham chiếu:** HLD v1 — Dev Servers Container, C3.5, C4.4, ADR-004, F27/F28/F31

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Admin Browser (Admin SPA) | UI | Fleet inventory UI, health dashboard |
| Orca Web Server | Backend | Fleet REST API, health aggregator |
| FleetManager | Business Logic | Inventory management, provisioning |
| FleetHealthMonitor | Business Logic | Periodic health checks, alerting |
| SSH (ssh2) | Transport | Connect to Dev Servers |
| Dev Server Agent | Remote | Node.js agent binary on dev server |
| Server Database | Persistence | orca_dev_servers, health records |

---

## BL-FLEET-01 — Fleet Inventory Config (YAML)

```
Admin/DevOps
    │
    ▼
Option A — YAML file:
    /etc/orca/fleet.yaml:
    ---
    servers:
      - id: dev-01
        host: 192.168.1.10
        port: 22
        user: ubuntu
        tags: [backend, prod]
        region: us-east-1
    │
    ▼
[Orca Web Server — FleetManager.loadInventory()]
    ├─ Parse YAML (js-yaml)
    ├─ Validate schema (Zod)
    ├─ UPSERT orca_dev_servers { id, host, port, user, tags, region }  ← Server DB
    └─ Trigger: FleetHealthMonitor.check(serverId)

Option B — Admin UI:
    Admin → POST /admin/api/fleet/servers
    Body: { host, port, user, sshKeyId, tags }
    ├─ requireAdmin() guard
    ├─ Validate + INSERT orca_dev_servers  ← Server DB
    └─ Test connection: ssh.connect(host) → echo 'ok'

Luồng:
YAML file → FleetManager (parse + validate) → Server DB (UPSERT)
OR: Admin → POST /admin/api/fleet/servers → Server DB → SSH test
```

---

## BL-FLEET-02 — Bulk Server Provisioning

```
Admin
    │
    ▼
[Admin SPA] Fleet → "Provision All" hoặc chọn subset
    │ POST /admin/api/fleet/provision { serverIds[], template: 'standard' }
    ▼
[Orca Web Server — FleetProvisioner.provision()]
    ├─ FOR each serverId (parallel Promise.all):
    │   ├─ SSH connect: ssh2.connect(server)
    │   ├─ Upload relay binary (SFTP):
    │   │   sftp.put(relayBinaryPath, '~/.orca/bin/orca-relay')
    │   ├─ SSH exec: chmod +x + set up systemd/launchd service
    │   │   exec('systemctl enable orca-relay && systemctl start orca-relay')
    │   ├─ Verify: SSH exec: orca-relay --version
    │   ├─ UPDATE orca_dev_servers SET status='active', relayVersion=?  ← DB
    │   └─ emit: server:provisioned { serverId }
    ├─ Collect results: { success[], failed[] }
    └─ Return provisioning report
    │
    ▼
[Admin SPA] progress table: per-server status

Luồng:
Admin → POST /admin/api/fleet/provision → FleetProvisioner
      → [SSH connect + SFTP upload + exec] × N (parallel)
      → Server DB (UPDATE status per server)
      → SSE/WebSocket progress events → Admin SPA
```

---

## BL-FLEET-03 — Fleet Health Monitoring

```
[FleetHealthMonitor] background cron: mỗi 30 giây
    │
    ▼
FOR each active server in orca_dev_servers:
    ├─ WebSocket ping: relay.call('health.get')
    │   Response: { cpu: 45%, ram: 60%, disk: 30%, agentCount: 2, latency: 12ms,
    │               ptySupported: true|false }
    │   ptySupported=false khi Dev Server Agent chưa cài node-pty → dashboard hiển
    │   thị badge "No terminal support" thay vì ẩn server khỏi danh sách
    ├─ IF timeout (5s): status = 'unreachable'
    ├─ Evaluate thresholds:
    │   cpu > 90% OR ram > 90% → status = 'warning'
    │   disk > 95% → status = 'critical'
    ├─ INSERT health_metrics { serverId, cpu, ram, disk, latency, timestamp }  ← DB
    └─ IF status changed: emit fleet:serverStatusChanged { serverId, oldStatus, newStatus }
    │
    ▼
[Admin SPA] nhận event → update health dashboard
    ├─ Color-coded status: green/yellow/red
    ├─ Badge riêng cho ptySupported (hỗ trợ / chưa hỗ trợ terminal)
    └─ Alert nếu critical

[Webhook alerts (optional)]:
    FleetHealthMonitor → POST webhookUrl { serverId, status, metrics }
    (Slack, PagerDuty, etc.)

Luồng:
Cron (30s) → FleetHealthMonitor → relay.call('health.get') × N (parallel)
           → Server DB (INSERT metrics)
           → WebSocket/SSE event → Admin SPA
           → Webhook (if configured)
```

---

## BL-FLEET-04 — Dev Server Onboarding Wizard

```
Carlos/Admin
    │
    ▼
[Admin SPA] Fleet → "Add Server" → Onboarding Wizard
    Step 1: Nhập host, port, SSH user
    Step 2: SSH key selection / upload
    Step 3: Test connection → [Test]
    │ POST /admin/api/fleet/servers/test
    ▼
[FleetManager.testConnection()]
    ├─ ssh2.connect({ host, port, username, privateKey })
    ├─ exec('echo "orca-test"') → verify response
    └─ Return { connected: true, osInfo: 'Linux 5.15 x86_64' }
    │
    ▼ [Wizard step 3 OK]
    Step 4: Deploy relay → [Deploy]
    │ POST /admin/api/fleet/servers/provision { serverId, single: true }
    ▼
[FleetProvisioner.provision(serverId)] → BL-FLEET-02 flow (single server)
    │
    ▼
    Step 5: Verify + add tags, region
    Step 6: [Finish]
    │ PATCH /admin/api/fleet/servers/:id { tags, region, notes }
    ▼
[Server DB] UPDATE orca_dev_servers → server active trong fleet

Luồng:
Wizard step-by-step:
Admin → POST /test → SSH test → response
Admin → POST /provision → SFTP + SSH exec → verify → DB UPDATE
Admin → PATCH /servers/:id → DB UPDATE (metadata)
```

---

## Sơ đồ tổng quan — Fleet Management

```
┌──────────────┐  HTTP/SSE  ┌─────────────────────────────────────┐
│  Admin SPA   │◄──────────►│  Orca Web Server                    │
│  Fleet panel │            │  FleetManager                       │
│  Health dash │            │  FleetProvisioner                   │
│  Wizard      │            │  FleetHealthMonitor (cron 30s)      │
└──────────────┘            └──────────┬──────────────────────────┘
                                       │
                            ┌──────────▼──────────────────────────┐
                            │  Server Database                     │
                            │  orca_dev_servers                   │
                            │  health_metrics                     │
                            └──────────┬──────────────────────────┘
                                       │ SSH (ssh2) + WebSocket
                       ┌───────────────┼───────────────────────────┐
                       │               │                           │
              ┌────────▼──┐   ┌────────▼──┐               ┌───────▼──┐
              │ Dev Server│   │ Dev Server│      ...       │ Dev Server│
              │  01        │   │  02        │               │  N        │
              │  relay:6799│   │  relay:6799│               │  relay:6799│
              └────────────┘   └────────────┘               └──────────┘
```
