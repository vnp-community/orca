# ============================================================
# Orca Backend + Frontend Runtime Container
# ============================================================
# Build: docker build -f deploy/dev/docker/backend/Dockerfile -t vnpblc/orca-server:dev .
# Ports: 6768 (WebSocket/RPC) + 6769 (HTTP static + /health/*)
#
# Why one image for two packages: backend/src/server/index.ts serves the
# frontend's static bundle itself (resolve(__dirname, '..', 'web') — see
# backend/src/server/index.ts) whenever out/web/ sits next to out/server/.
# So backend/ and frontend/ are built as two INDEPENDENT stages (from their
# own package.json, own deps) — matching the split — but combined into one
# runtime image at the end, preserving the exact out/server + out/web sibling
# layout the code already expects. Splitting into two containers would also
# work (nginx serving out/web directly) but isn't done here to avoid
# reworking deploy/dev/docker/nginx/conf.d/orca.conf's static-file routing.
# ============================================================

# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: backend builder — builds ONLY backend/ (its own package.json/deps)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-bookworm-slim AS backend-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 make g++ git \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Root workspace manifest — pnpm needs this to resolve backend/'s deps
# against the shared workspace lockfile (pnpm-workspace.yaml).
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY desktop/config/patches ./config/patches
COPY backend/package.json ./backend/package.json

RUN npm pkg delete scripts.postinstall scripts.prepare && corepack enable
RUN pnpm install --filter orca-backend...

COPY backend/ ./backend/
WORKDIR /app/backend
RUN pnpm run build
# → /app/backend/out/server/{index,daemon-entry,user-process-entry}.js

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: frontend builder — builds ONLY frontend/ (its own package.json/deps)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-bookworm-slim AS frontend-builder

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY desktop/config/patches ./config/patches
COPY frontend/package.json ./frontend/package.json

RUN npm pkg delete scripts.postinstall scripts.prepare && corepack enable
# --ignore-scripts: frontend/package.json carries the full monorepo dependency
# list (native modules like node-pty/cpu-features included) even though the
# browser bundle never imports them — skip their native-compile install
# scripts entirely rather than installing a build toolchain to satisfy code
# this image never runs.
RUN pnpm install --filter orca-frontend... --ignore-scripts

COPY frontend/ ./frontend/
WORKDIR /app/frontend
RUN pnpm run build
# → /app/frontend/out/web/{web-index,admin-index}.html + assets/

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: runtime — minimal image, combines both build outputs
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-bookworm-slim

LABEL maintainer="VNP-BLC DevOps <devops@vnpblc.com>"
LABEL description="Orca Backend (RPC/WS + admin API) + Frontend (web SPA) runtime"

ENV DEBIAN_FRONTEND=noninteractive
ENV HOME=/home/orca

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 make g++ git openssh-client wget curl ca-certificates \
  && mkdir -p /usr/share/keyrings \
  && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
       > /etc/apt/sources.list.d/github-cli.list \
  && apt-get update && apt-get install -y --no-install-recommends gh \
  && rm -rf /var/lib/apt/lists/* \
  && apt-get clean

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

# backend/'s own package.json + prod deps (express, ws, better-sqlite3, ssh2, ...)
COPY --from=backend-builder /app/backend/package.json ./package.json
COPY --from=backend-builder /app/pnpm-lock.yaml /app/pnpm-workspace.yaml /app/.npmrc ./
COPY --from=backend-builder /app/config/patches ./config/patches
RUN npm pkg delete scripts.postinstall scripts.prepare && corepack enable && pnpm install --prod --ignore-workspace

# Build artifacts — out/server + out/web as siblings (see header comment)
COPY --from=backend-builder /app/backend/out/server ./out/server
COPY --from=frontend-builder /app/frontend/out/web ./out/web

COPY deploy/dev/docker/backend/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV ORCA_PORT=6768
ENV ORCA_HTTP_PORT=6769
ENV ORCA_USER_DATA_PATH=/data/orca
ENV NODE_ENV=production

EXPOSE 6768
EXPOSE 6769

VOLUME ["/data/orca"]

USER orca
WORKDIR /home/orca

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
