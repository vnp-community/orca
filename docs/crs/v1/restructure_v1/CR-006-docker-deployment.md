# CR-006 — Docker & Deployment Restructure

**Status:** Proposed  
**Priority:** 🟡 Medium  
**Depends on:** CR-005  
**Blocks:** —

---

## Mục tiêu

Tổ chức lại cấu trúc Docker/deployment để:
1. Dockerfile sạch hơn, đúng multi-stage build
2. Tách biệt rõ ràng môi trường `dev` và `prod`
3. Hỗ trợ CI/CD build image tách biệt với deploy
4. Chuẩn bị cho deployment tự động (GitOps-ready)

---

## Bối cảnh & Vấn đề Hiện tại

`deploy/dev/docker/orca/Dockerfile` hiện tại:
- Build toàn bộ source trong Docker (chậm, cần internet trong build)
- `src/server/index.ts` dùng mock thô, chưa dùng NodeAdapter
- Không có image tag strategy rõ ràng
- `sync-to-server.sh` rsync toàn bộ source + build trong 1 bước

---

## Kiến trúc Deployment Đề xuất

```
┌─────────────────────────────────────────────────────┐
│                  CI/CD Pipeline                     │
│  git push → build → test → push image → deploy     │
└─────────────────────────────────────────────────────┘

┌──────────────┐    ┌──────────────┐    ┌────────────┐
│   Builder    │    │  Docker Hub  │    │  Server    │
│  (local/CI)  │ →  │  Registry    │ →  │  (pull)    │
│              │    │              │    │            │
│ pnpm build   │    │ orca-server: │    │ docker     │
│ docker build │    │   :latest    │    │ compose    │
│ docker push  │    │   :1.4.138   │    │ up -d      │
└──────────────┘    └──────────────┘    └────────────┘
```

### 1. Dockerfile Production (`deploy/prod/Dockerfile`)

```dockerfile
# ============================================================
# Orca Server — Production Dockerfile
# ============================================================
# Assumes artifacts đã được build trước (pnpm build:server)
# Docker image chỉ package artifacts — không compile source
# ============================================================

FROM node:22-bookworm-slim

LABEL org.opencontainers.image.title="Orca Server"
LABEL org.opencontainers.image.description="Orca AI Orchestrator — Node.js Web Server"
LABEL org.opencontainers.image.vendor="VNP-BLC"
LABEL org.opencontainers.image.version="${ORCA_VERSION:-dev}"

ENV DEBIAN_FRONTEND=noninteractive
ENV HOME=/home/orca
ENV NODE_ENV=production

# Runtime dependencies only
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    openssh-client \
    wget \
    ca-certificates \
    python3 \
    make \
    g++ \
  && rm -rf /var/lib/apt/lists/* \
  && apt-get clean

# Create non-root user
RUN groupadd -r orca \
 && useradd -r -g orca -d /home/orca -s /bin/bash orca \
 && mkdir -p \
    /home/orca/.config/orca \
    /home/orca/.ssh \
    /home/orca/.local/share/orca \
    /srv/projects \
 && chown -R orca:orca /home/orca /srv/projects

WORKDIR /opt/orca/app
RUN chown -R orca:orca /opt/orca

# Install production Node dependencies
COPY package.json pnpm-lock.yaml .npmrc ./
COPY config/patches ./config/patches
RUN npm pkg delete scripts.postinstall scripts.prepare \
 && corepack enable \
 && pnpm install --prod --frozen-lockfile

# Copy pre-built artifacts (built by CI or locally)
COPY out/server ./out/server
COPY out/web    ./out/web
COPY out/relay  ./out/relay

# Entrypoint
COPY deploy/prod/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 6768
USER orca
WORKDIR /home/orca

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=5 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:${ORCA_PORT:-6768}/ || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

### 2. Dockerfile Dev (`deploy/dev/docker/orca/Dockerfile`)

```dockerfile
# ============================================================
# Orca Server — Development Dockerfile (Build-in-container)
# ============================================================
# Dùng cho local development: build source ngay trong Docker
# Phù hợp khi local machine không có đủ dependencies (node, pnpm)
# ============================================================

FROM node:22-bookworm-slim AS builder
# ... (giữ nguyên hoặc cải thiện nhẹ)
```

### 3. `deploy/prod/entrypoint.sh`

```bash
#!/bin/bash
set -euo pipefail

ORCA_PORT="${ORCA_PORT:-6768}"
ORCA_DOMAIN="${ORCA_DOMAIN:-localhost}"

cat <<EOF
======================================================
 Orca Node.js Server
======================================================
  Version: ${ORCA_VERSION:-dev}
  Port:    ${ORCA_PORT}
  Domain:  ${ORCA_DOMAIN}
  Mode:    production
======================================================
EOF

# Ensure user data directories exist
mkdir -p "${HOME}/.config/orca/daemon"
mkdir -p "${HOME}/.config/orca/logs"
chmod 700 "${HOME}/.config/orca"

exec node /opt/orca/app/out/server/index.js
```

### 4. `deploy/prod/docker-compose.yml`

```yaml
# deploy/prod/docker-compose.yml
# ============================================================
# Production compose — pull image từ registry, không build local
# ============================================================

services:
  orca:
    image: ${ORCA_IMAGE:-vnpblc/orca-server}:${ORCA_VERSION:-latest}
    container_name: orca-server
    restart: unless-stopped

    environment:
      ORCA_DOMAIN: ${ORCA_DOMAIN}
      ORCA_PORT: ${ORCA_PORT:-6768}
      ORCA_VERSION: ${ORCA_VERSION:-dev}
      HOME: /home/orca
      # API Keys (từ secrets, không hardcode)
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      GEMINI_API_KEY: ${GEMINI_API_KEY:-}

    volumes:
      - orca-data:/home/orca/.config/orca
      - ./ssh:/home/orca/.ssh:ro
      - orca-projects:/srv/projects

    ports:
      - "${ORCA_BIND_ADDR:-127.0.0.1}:${ORCA_PORT:-6768}:${ORCA_PORT:-6768}"

    security_opt:
      - no-new-privileges:true

    tmpfs:
      - /tmp

    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "5"

    networks:
      - orca-net

networks:
  orca-net:
    driver: bridge

volumes:
  orca-data:
  orca-projects:
```

### 5. Tái cấu trúc `deploy/` Directory

```
deploy/
├── dev/                          # Development — build từ source
│   ├── docker/
│   │   └── orca/
│   │       ├── Dockerfile        # Multi-stage build (giữ nguyên)
│   │       └── entrypoint.sh
│   ├── docker-compose.orca.yml   # Giữ nguyên (build từ source)
│   ├── docker-compose.gateway.yml
│   ├── .env.example
│   └── scripts/
│       ├── sync-to-server.sh     # Cải thiện (xem mục 6)
│       └── deploy-server.sh      # Script mới: chỉ deploy image
│
├── prod/                         # Production — pull image từ registry
│   ├── Dockerfile                # [MỚI] Artifact-only Dockerfile
│   ├── docker-compose.yml        # [MỚI] Pull image, không build
│   ├── entrypoint.sh             # [MỚI]
│   └── .env.example              # [MỚI]
│
└── ci/                           # CI/CD helpers
    ├── build-and-push.sh         # [MỚI] Build image và push lên registry
    └── deploy.sh                 # [MỚI] Deploy image lên target server
```

### 6. Cải thiện `sync-to-server.sh`

```bash
#!/bin/bash
# deploy/dev/scripts/sync-to-server.sh
set -euo pipefail

# === Config ===
source "$(dirname "$0")/../.env" 2>/dev/null || true

SERVER_HOST="${SERVER_HOST:-172.20.2.39}"
SERVER_USER="${SERVER_USER:-ubuntu}"
SERVER_DEPLOY="${SERVER_DEPLOY:-/home/ubuntu/orca-deploy}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_rsa}"
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

# Mode: "build" (build in Docker, default) or "image" (pull pre-built image)
MODE="${DEPLOY_MODE:-build}"

echo "🚀 Deploying Orca to ${SERVER_USER}@${SERVER_HOST}..."
echo "   Mode: ${MODE}"
echo ""

if [[ "$MODE" == "image" ]]; then
  # === Image mode: pull từ registry ===
  ORCA_IMAGE="${ORCA_IMAGE:-vnpblc/orca-server}"
  ORCA_VERSION="${ORCA_VERSION:-latest}"
  
  echo "[1/2] Syncing deploy configs to server..."
  rsync -avz -e "ssh -i ${SSH_KEY}" \
    "${REPO_ROOT}/deploy/prod/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/"
  
  echo "[2/2] Pulling image and restarting..."
  ssh -i "${SSH_KEY}" "${SERVER_USER}@${SERVER_HOST}" \
    "cd ${SERVER_DEPLOY} && \
     ORCA_IMAGE=${ORCA_IMAGE} ORCA_VERSION=${ORCA_VERSION} \
     docker compose pull && \
     docker compose up -d --force-recreate"

else
  # === Build mode: build từ source trong Docker ===
  echo "[1/3] Syncing source code..."
  rsync -avz --progress --delete \
    -e "ssh -i ${SSH_KEY}" \
    --exclude 'node_modules' \
    --exclude 'out' \
    --exclude 'dist' \
    --exclude '.git' \
    --exclude '.codegraph' \
    --exclude '.gitnexus' \
    "${REPO_ROOT}/" \
    "${SERVER_USER}@${SERVER_HOST}:${SERVER_DEPLOY}/"
  
  echo "[2/3] Building and deploying..."
  ssh -i "${SSH_KEY}" "${SERVER_USER}@${SERVER_HOST}" \
    "cd ${SERVER_DEPLOY}/deploy/dev && \
     docker compose -f docker-compose.orca.yml up -d --build --force-recreate"
fi

echo ""
echo "======================================================"
echo " ✅ Deploy complete!"
echo "======================================================"

sleep 5
ssh -i "${SSH_KEY}" "${SERVER_USER}@${SERVER_HOST}" \
  "docker logs orca-server --tail 30 2>&1 || true"
```

### 7. CI Build Script

```bash
#!/bin/bash
# deploy/ci/build-and-push.sh
set -euo pipefail

REGISTRY="${REGISTRY:-docker.io}"
IMAGE="${IMAGE:-vnpblc/orca-server}"
VERSION="${VERSION:-$(node -p "require('./package.json').version")}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

echo "🔨 Building Orca Server image..."
echo "   Image:     ${REGISTRY}/${IMAGE}:${VERSION}"
echo "   Platforms: ${PLATFORMS}"

# 1. Build artifacts
pnpm build:server

# 2. Build Docker image
docker buildx build \
  --platform "${PLATFORMS}" \
  --tag "${REGISTRY}/${IMAGE}:${VERSION}" \
  --tag "${REGISTRY}/${IMAGE}:latest" \
  --file deploy/prod/Dockerfile \
  --push \
  .

echo "✅ Image pushed: ${REGISTRY}/${IMAGE}:${VERSION}"
```

---

## Phạm vi thay đổi

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] deploy/prod/Dockerfile` | Production Dockerfile |
| `[NEW] deploy/prod/docker-compose.yml` | Production compose (image pull) |
| `[NEW] deploy/prod/entrypoint.sh` | Production entrypoint |
| `[NEW] deploy/prod/.env.example` | Env template |
| `[NEW] deploy/ci/build-and-push.sh` | CI build script |
| `[NEW] deploy/ci/deploy.sh` | CI deploy script |

### Files sửa đổi
| File | Thay đổi |
|------|---------|
| `[MODIFY] deploy/dev/scripts/sync-to-server.sh` | Thêm mode support, cải thiện UX |
| `[MODIFY] deploy/dev/docker/orca/Dockerfile` | Minor cleanup |

### Files KHÔNG thay đổi
- `deploy/dev/docker-compose.orca.yml` — Giữ nguyên cho dev workflow
- `src/main/` — **KHÔNG sửa**

---

## Deployment Workflow Summary

| Workflow | Command | Use case |
|---------|---------|---------|
| Dev (build-in-docker) | `./sync-to-server.sh` | Developer testing trên server |
| Dev (pre-built image) | `DEPLOY_MODE=image ./sync-to-server.sh` | Faster deploy |
| CI/CD | `./deploy/ci/build-and-push.sh` | Automated release |
| Prod | `docker compose -f deploy/prod/docker-compose.yml up -d` | Production deploy |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

| File | Status |
|------|--------|
| `deploy/prod/Dockerfile` | ✅ Done |
| `deploy/prod/docker-compose.yml` | ✅ Done |
| `deploy/prod/entrypoint.sh` | ✅ Done |
| `deploy/prod/scripts/` | ✅ Done — sync, deploy scripts |
