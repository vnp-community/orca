# ============================================================
# Orca Runtime Container — Pure Node.js Backend
# ============================================================
# Build: docker build -f deploy/dev/docker/orca/Dockerfile -t vnpblc/orca-server:dev .
# Ports: 6768 (WebSocket/RPC) + 6769 (HTTP static + /health/*)
# Stage 1: Build
FROM node:22-bookworm-slim AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    make \
    g++ \
    git \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy package files first for better layer caching
COPY package.json pnpm-lock.yaml .npmrc ./
COPY config/patches ./config/patches

# Enable corepack for pnpm and prevent postinstall/prepare
RUN npm pkg delete scripts.postinstall scripts.prepare && corepack enable
RUN pnpm install

# Now copy the rest of the source code
COPY . .
RUN pnpm build:relay
RUN pnpm build:web
RUN pnpm exec vite build -c vite.server.config.ts

# ============================================================
# Stage 2: Runtime
FROM node:22-bookworm-slim

LABEL maintainer="VNP-BLC DevOps <devops@vnpblc.com>"
LABEL description="Orca AI Orchestrator — pure Node.js runtime"

ENV DEBIAN_FRONTEND=noninteractive
ENV HOME=/home/orca

# Install runtime dependencies (git, ssh, wget)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    make \
    g++ \
    git \
    openssh-client \
    wget \
    ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && apt-get clean

# Create non-root user
RUN groupadd -r orca \
 && useradd -r -g orca -d /home/orca -s /bin/bash -c "Orca runtime user" orca \
 && mkdir -p /home/orca/.config/orca \
              /home/orca/.ssh \
              /home/orca/.local/share/orca \
              /home/orca/.local/bin \
              /data/orca \
              /srv/projects \
 && chown -R orca:orca /home/orca /data/orca /srv/projects

WORKDIR /opt/orca/app
RUN chown -R orca:orca /opt/orca

# Copy package config, lockfile, and patches
COPY package.json pnpm-lock.yaml .npmrc ./
COPY config/patches ./config/patches

# Enable corepack and install production dependencies (remove postinstall and prepare to prevent Electron rebuild & husky)
RUN npm pkg delete scripts.postinstall scripts.prepare && corepack enable && pnpm install --prod

# Copy build artifacts from builder
COPY --from=builder /app/out/server ./out/server
COPY --from=builder /app/out/web ./out/web
COPY --from=builder /app/out/relay ./out/relay

COPY deploy/dev/docker/orca/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Environment defaults
ENV ORCA_PORT=6768
ENV ORCA_HTTP_PORT=6769
ENV ORCA_USER_DATA_PATH=/data/orca
ENV NODE_ENV=production

# Expose ports
# 6768: WebSocket/RPC endpoint
# 6769: HTTP endpoint (web UI + /health/*)
EXPOSE 6768
EXPOSE 6769

# Volume for persistent data
VOLUME ["/data/orca"]

USER orca
WORKDIR /home/orca

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
