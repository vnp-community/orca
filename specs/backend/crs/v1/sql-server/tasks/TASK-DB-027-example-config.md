# TASK-DB-027: Tạo `config/orca-server.example.yaml` + cập nhật Docker Compose ✅ DONE

**Source:** SOL-DB-004 §5, SOL-DB-006 §4.6  
**Phase:** 4 | **Effort:** XS (< 20 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-003, TASK-DB-023

---

## Objective

Tạo file ví dụ về cấu hình DB và cập nhật Docker Compose health check sang `/health/ready`.

---

## Files to create

### 1. `config/orca-server.example.yaml`

```yaml
# orca-server.yaml — Orca Server Mode Database Configuration
# ============================================================
# Copy this file to your userDataPath and rename to orca-server.yaml
# Default userDataPath: ~/.orca/ (or ORCA_USER_DATA_PATH)
#
# Priority (highest to lowest):
#   1. ORCA_DB_URL env var
#   2. ORCA_DB_DIALECT + ORCA_DB_HOST + ... env vars
#   3. This config file (future support)
#   4. SQLite default (orca-server.db in userDataPath)
# ============================================================

database:
  # ── SQLite (default — no config needed) ─────────────────────────────────
  # dialect: sqlite
  # path: ./orca-server.db     # relative to userDataPath

  # ── MySQL 8.x / MariaDB ─────────────────────────────────────────────────
  # dialect: mysql
  # host: db.example.com
  # port: 3306                  # default: 3306
  # database: orca_prod
  # username: orca_user
  # password: ""                # use env var: ORCA_DB_PASSWORD
  # ssl: false                  # set true for production
  # pool:
  #   min: 2                    # minimum idle connections
  #   max: 20                   # maximum total connections
  #   acquireTimeoutMs: 5000    # 5s timeout waiting for connection
  #   idleTimeoutMs: 30000      # 30s before idle connection is closed
  #   connectionRetries: 3      # retry attempts on connection failure
  #   retryDelayMs: 500         # delay between retries

  # ── PostgreSQL 14+ ──────────────────────────────────────────────────────
  # dialect: postgresql
  # host: pg.example.com
  # port: 5432                  # default: 5432
  # database: orca_prod
  # username: orca
  # password: ""                # use env var: ORCA_DB_PASSWORD
  # schema: public              # PostgreSQL schema (default: public)
  # ssl: true

  # ── TiDB (MySQL protocol) ────────────────────────────────────────────────
  # dialect: tidb
  # host: tidb.example.com
  # port: 4000                  # TiDB default port
  # database: orca
  # username: root
  # password: ""

# ── Environment variable reference ──────────────────────────────────────────
# ORCA_DB_URL=sqlite:///data/orca/db.sqlite
# ORCA_DB_URL=mysql://user:pass@host:3306/orca?ssl=true
# ORCA_DB_URL=postgresql://user:pass@host:5432/orca
# ORCA_DB_URL=tidb://user:pass@host:4000/orca
#
# Or structured:
# ORCA_DB_DIALECT=mysql
# ORCA_DB_HOST=db.example.com
# ORCA_DB_PORT=3306
# ORCA_DB_NAME=orca_prod
# ORCA_DB_USER=orca_user
# ORCA_DB_PASSWORD=secret
# ORCA_DB_SSL=true
# ORCA_DB_POOL_MAX=20
# ORCA_DB_POOL_MIN=2
```

---

## Files to modify

### `deploy/prod/docker-compose.yml` — Update health check

Đọc file hiện tại trước (`cat deploy/prod/docker-compose.yml`), sau đó thay thế health check section:

```yaml
# OLD:
healthcheck:
  test: wget -qO- http://localhost:6769/

# NEW:
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:6769/health/ready"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 15s
```

Cũng thêm DB environment variables vào service:

```yaml
services:
  orca-server:
    # ... existing ...
    environment:
      - ORCA_PORT=6768
      - ORCA_HTTP_PORT=6769
      - ORCA_USER_DATA_PATH=/data/orca
      # Add DB config via env vars (override as needed):
      # - ORCA_DB_URL=mysql://orca_user:${DB_PASSWORD}@db:3306/orca_prod
      # - ORCA_DB_URL=postgresql://orca@db:5432/orca_prod
```

### `deploy/prod/.env.example` — Add DB variables

```bash
# Existing vars...

# Database (uncomment and fill in for non-SQLite backends)
# ORCA_DB_URL=mysql://orca_user:CHANGE_ME@localhost:3306/orca_prod
# ORCA_DB_URL=postgresql://orca_user:CHANGE_ME@localhost:5432/orca_prod

# Or structured:
# ORCA_DB_DIALECT=mysql
# ORCA_DB_HOST=localhost
# ORCA_DB_PORT=3306
# ORCA_DB_NAME=orca_prod
# ORCA_DB_USER=orca_user
# ORCA_DB_PASSWORD=CHANGE_ME
# ORCA_DB_SSL=false
# ORCA_DB_POOL_MAX=10
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Verify example config exists
ls -la config/orca-server.example.yaml

# Verify docker-compose is still valid
docker compose -f deploy/prod/docker-compose.yml config > /dev/null && echo "OK"

# Verify new health check URL in compose
grep "health/ready" deploy/prod/docker-compose.yml
```

---

## Done criteria

- [x] `config/orca-server.example.yaml` tồn tại với examples cho cả 4 dialects
- [x] Docker Compose health check dùng `/health/ready` (thay vì `wget -qO- http://...`)
- [x] `deploy/prod/.env.example` có DB env var examples
- [x] `docker compose config` vẫn valid
