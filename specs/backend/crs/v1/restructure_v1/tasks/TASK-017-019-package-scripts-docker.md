# TASK-017 + TASK-018 + TASK-019: Package scripts, Dockerfile, Docker Compose

**Source:** SOL-BE-005  
**Phase:** 3 | **Effort:** S (45–60 min tổng)  
**Depends on:** TASK-015, TASK-016

---

## TASK-017: Cập nhật `package.json` scripts

### File to modify: `package.json`

Thêm các scripts sau vào `scripts` section:

```json
{
  "scripts": {
    "build:backend": "vite build -c vite.server.config.ts",
    "build:frontend:web": "vite build -c vite.web-spa.config.ts",
    "build:server": "pnpm build:relay && pnpm build:backend && pnpm build:frontend:web",
    "dev:web": "vite --config vite.web-spa.config.ts"
  }
}
```

**Lưu ý:** Chỉ **thêm** scripts mới, KHÔNG xóa scripts hiện có (`build:relay`, `build`, `build:web`, v.v.).

### Verification TASK-017

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Kiểm tra scripts đã được thêm
node -e "
const pkg = require('./package.json')
const required = ['build:backend','build:frontend:web','build:server','dev:web']
for (const s of required) {
  console.log(s, pkg.scripts[s] ? '✅' : '❌ MISSING')
}
"

# Test build:backend
pnpm build:backend 2>&1 | tail -5
```

---

## TASK-018: Tạo `deploy/prod/Dockerfile`

### File to create: `deploy/prod/Dockerfile`

```dockerfile
# Multi-stage build for Orca Server production image
# Stage 1: Build (with Node.js and pnpm)
FROM node:22-alpine AS builder

WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy package files
COPY package.json pnpm-lock.yaml ./

# Install ALL deps (including devDeps needed for build)
RUN pnpm install --frozen-lockfile

# Copy source code
COPY . .

# Build backend + web bundle
RUN pnpm build:server

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2: Runtime (minimal)
FROM node:22-alpine AS runtime

WORKDIR /app

# Install pnpm for prod dependency installation
RUN corepack enable && corepack prepare pnpm@latest --activate

# Install native dependencies needed at runtime
# node-pty requires python3 and build tools during install
RUN apk add --no-cache \
    python3 \
    make \
    g++ \
    git \
    openssh-client \
    bash

# Copy package files and install PRODUCTION deps only
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --prod

# Rebuild native modules for Linux
RUN pnpm rebuild node-pty better-sqlite3 2>/dev/null || true

# Copy built outputs from builder stage
COPY --from=builder /app/out/server ./out/server
COPY --from=builder /app/out/web ./out/web
COPY --from=builder /app/out/relay ./out/relay

# Copy runtime configuration
COPY deploy/prod/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Environment defaults
ENV ORCA_PORT=6768
ENV ORCA_HTTP_PORT=6769
ENV NODE_ENV=production

# Ports
EXPOSE 6768
EXPOSE 6769

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:${ORCA_HTTP_PORT}/ > /dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "out/server/index.js"]
```

### File to create: `deploy/prod/entrypoint.sh`

```bash
#!/bin/sh
set -e

echo "=== Orca Server ==="
echo "Version: ${ORCA_VERSION:-unknown}"
echo "Port: ${ORCA_PORT:-6768}"
echo "HTTP Port: ${ORCA_HTTP_PORT:-6769}"
echo "Data Dir: ${ORCA_USER_DATA_PATH:-~/.orca}"

# Create data directory if needed
if [ -n "$ORCA_USER_DATA_PATH" ]; then
  mkdir -p "$ORCA_USER_DATA_PATH"
fi

# Execute the command
exec "$@"
```

---

## TASK-019: Tạo `deploy/prod/docker-compose.yml`

### File to create: `deploy/prod/docker-compose.yml`

```yaml
# Orca Server — Production Docker Compose
# Usage: docker compose -f deploy/prod/docker-compose.yml up -d
version: '3.9'

services:
  orca-server:
    image: ${ORCA_IMAGE:-vnpblc/orca-server}:${ORCA_VERSION:-latest}
    build:
      context: ../..
      dockerfile: deploy/prod/Dockerfile
    container_name: orca-server
    restart: unless-stopped

    environment:
      - ORCA_PORT=6768
      - ORCA_HTTP_PORT=6769
      - ORCA_VERSION=${ORCA_VERSION:-0.0.0}
      - ORCA_USER_DATA_PATH=/data/orca
      - NODE_ENV=production

    volumes:
      # Persistent data (SQLite, crypto keys, relay binaries, etc.)
      - orca-data:/data/orca

    ports:
      # WebSocket/RPC port — expose to gateway
      - "${ORCA_RPC_PORT:-6768}:6768"
      # HTTP port (web UI) — expose to gateway
      - "${ORCA_WEB_PORT:-6769}:6769"

    networks:
      - orca-net

    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6769/"]
      interval: 30s
      timeout: 10s
      start_period: 15s
      retries: 3

    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "5"

volumes:
  orca-data:
    driver: local

networks:
  orca-net:
    driver: bridge
```

### File to create: `deploy/prod/.env.example`

```bash
# Orca Server Production — Environment Variables

# Docker image tag (usually set by CI/CD)
ORCA_VERSION=latest
ORCA_IMAGE=vnpblc/orca-server

# Host ports exposed to outside (adjust if needed)
ORCA_RPC_PORT=6768    # WebSocket/RPC
ORCA_WEB_PORT=6769    # HTTP (web UI)
```

### File to create: `deploy/prod/scripts/deploy.sh`

```bash
#!/bin/bash
# Deploy Orca Server to production server
# Usage: bash deploy/prod/scripts/deploy.sh [version]

set -euo pipefail

ORCA_VERSION=${1:-$(node -p "require('./package.json').version")}
REMOTE_HOST=${ORCA_REMOTE_HOST:-ubuntu@172.20.2.39}
DEPLOY_DIR=${ORCA_DEPLOY_DIR:-~/orca-deploy}

echo "=== Deploying Orca Server v${ORCA_VERSION} ==="

# 1. Build production image
docker buildx build \
  --platform linux/amd64 \
  --tag "vnpblc/orca-server:${ORCA_VERSION}" \
  --tag "vnpblc/orca-server:latest" \
  --file deploy/prod/Dockerfile \
  .

echo "✅ Docker image built"

# 2. Save and transfer image (if no registry)
docker save "vnpblc/orca-server:${ORCA_VERSION}" \
  | gzip \
  | ssh "$REMOTE_HOST" "mkdir -p ${DEPLOY_DIR} && gunzip | docker load"

echo "✅ Image transferred to ${REMOTE_HOST}"

# 3. Deploy on remote
ssh "$REMOTE_HOST" bash <<EOF
cd ${DEPLOY_DIR}

# Write env file
cat > .env <<ENVEOF
ORCA_VERSION=${ORCA_VERSION}
ORCA_IMAGE=vnpblc/orca-server
ENVEOF

# Pull compose file
EOF

# Transfer compose file
scp deploy/prod/docker-compose.yml "$REMOTE_HOST:${DEPLOY_DIR}/docker-compose.yml"

# Restart container
ssh "$REMOTE_HOST" \
  "cd ${DEPLOY_DIR} && ORCA_VERSION=${ORCA_VERSION} docker compose up -d --no-build"

echo "✅ Deployed! Container restarted."

# 4. Health check
sleep 10
if ssh "$REMOTE_HOST" "curl -sf http://localhost:6769/ > /dev/null"; then
  echo "✅ Health check PASSED"
else
  echo "❌ Health check FAILED — check container logs:"
  ssh "$REMOTE_HOST" "docker logs orca-server --tail 50"
  exit 1
fi

echo ""
echo "=== Deploy Complete: v${ORCA_VERSION} ==="
```

---

## Verification (TASK-017, 018, 019 combined)

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TASK-017: Script check
node -e "const p = require('./package.json'); console.log(p.scripts['build:backend'])"

# TASK-018: Docker build test (takes ~5 min)
docker build -f deploy/prod/Dockerfile -t orca-server:test .
echo "Docker build: $?"

# Quick smoke test
CTNR=$(docker run -d -p 16769:6769 -p 16768:6768 orca-server:test)
sleep 10
curl -sf http://localhost:16769/ && echo "HTTP OK" || echo "HTTP FAILED"
docker rm -f $CTNR

# TASK-019: Compose validate
docker compose -f deploy/prod/docker-compose.yml config --quiet && echo "Compose: valid"
```

---

## Done criteria

**TASK-017:**
- [x] `package.json` có scripts: `build:backend`, `build:frontend:web`, `build:server`, `dev:web`
- [x] Existing scripts không bị xóa

**TASK-018:**
- [x] `deploy/prod/Dockerfile` tạo thành công (multi-stage)
- [x] `deploy/prod/entrypoint.sh` tạo thành công (executable)
- [x] `docker build` hoàn thành không có error
- [x] Container start và respond HTTP

**TASK-019:**
- [x] `deploy/prod/docker-compose.yml` tạo thành công
- [x] `deploy/prod/.env.example` tạo thành công
- [x] `deploy/prod/scripts/deploy.sh` tạo thành công
- [x] `docker compose config` validate thành công
