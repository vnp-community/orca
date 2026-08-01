# BL-FLEET-01: Fleet Inventory Config (YAML)

**Domain:** Fleet Management  
**Priority:** P1  
**Actor chính:** Admin, DevOps  
**Tham chiếu:** FR-15.1, UR-120, F27, F28

---

## Mô tả

Khai báo danh sách dev servers dưới dạng YAML file (infrastructure-as-code). Import vào Orca để batch-provision và manage.

## YAML Schema

```yaml
# deploy/dev/orca-fleet.yaml
defaults:
  relayGracePeriodSec: 30
  nodeVersion: "22"

projects:
  - name: backend
    tags: [production, api]
  - name: frontend
    tags: [staging]

servers:
  - hostname: dev1.example.com
    user: ubuntu
    identityFile: ~/.ssh/dev_key
    project: backend
    tags: [primary, gpu]
    port: 22
  - hostname: dev2.example.com
    user: ubuntu
    identityFile: ~/.ssh/dev_key
    project: backend
    tags: [secondary]
  - hostname: fe-dev.example.com
    user: ubuntu
    identityFile: ~/.ssh/fe_key
    project: frontend
```

## Import Flow

```
orca fleet import --file orca-fleet.yaml [--dry-run]

1. Read và parse YAML file
2. Validate với Zod schema (required fields, hostname format, etc.)
3. --dry-run: chỉ in summary, không lưu
4. Upsert servers vào SSH targets store (INSERT OR UPDATE by hostname+user)
5. Tag servers theo project + tags
6. Return: { imported: N, updated: M, skipped: K, errors: [] }
```

## Validation Rules

- `hostname`: valid DNS name hoặc IP
- `user`: non-empty string
- `port`: 1-65535 (default 22)
- `identityFile`: path tồn tại (expand `~`)
- `project`: phải có trong `projects[]` list nếu khai báo

## CLI Commands

```bash
orca fleet import --file fleet.yaml      # Import servers
orca fleet list                          # List tất cả servers
orca fleet list --project backend        # Filter by project
orca fleet status                        # Health status của tất cả servers
```

## Source References

- `src/main/fleet/fleet-inventory.ts` — parseFleetYaml(), importFleet()
- `src/shared/fleet-types.ts` — FleetConfig, FleetServer Zod schemas
