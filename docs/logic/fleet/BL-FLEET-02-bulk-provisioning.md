# BL-FLEET-02: Bulk Server Provisioning

**Domain:** Fleet Management  
**Priority:** P1  
**Actor chính:** Admin  
**Tham chiếu:** FR-15.2, F27, F28

---

## Mô tả

Provision nhiều dev servers cùng lúc: deploy relay binary, bootstrap tools (Node.js, Git), kiểm tra prerequisites.

## Flow

```
orca fleet provision [--project <name>] [--concurrency 5]

1. Load server list từ fleet store (filter by project nếu có)
2. For each server (parallel, max concurrency):
   a. SSH connect (resolve identity từ fleet config)
   b. Check prerequisites: Node.js version, Git version, disk space
   c. Deploy relay binary (SFTP upload nếu outdated/missing)
   d. Verify relay binary hash
   e. Start relay process
   f. Run health check
   g. Update server status trong fleet store
3. Print summary: { success: N, failed: M, skipped: K }
```

## Bootstrap Steps (per server)

| Step | Command | Success Condition |
|------|---------|-------------------|
| Check Node.js | `node --version` | `>= 22.0.0` |
| Check Git | `git --version` | `>= 2.25.0` |
| Check Disk | `df -h ~/.orca` | `>= 5GB free` |
| Deploy Relay | SFTP put `orca-relay` to `~/.local/bin/` | SHA256 matches |
| Start Relay | `~/.local/bin/orca-relay --daemon` | PID file created |
| Health Check | `curl http://localhost:<relayPort>/health` | `200 OK` |

## Error Handling

- SSH connect failure → log + continue với server khác
- Prerequisites không đủ → log specific error, mark server `degraded`
- Relay deploy failure → retry 3 lần, sau đó mark `unhealthy`
- Partial provision → idempotent: có thể chạy lại mà không bị lỗi

## Source References

- `src/main/fleet/fleet-provisioner.ts` — provisionServer(), bulkProvision()
- `src/main/ssh/ssh-connection.ts` — SSH connect
- `src/main/ssh/relay-deploy.ts` — deployRelay()
