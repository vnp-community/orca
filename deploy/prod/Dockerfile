# Orca Backend + Frontend — Production Image (Alpine, minimal)
#
# Build:   docker build -f deploy/prod/Dockerfile -t vnpblc/orca-server:latest .
# Run:     docker run -d -p 6768:6768 -p 6769:6769 vnpblc/orca-server:latest
#
# Why one image for two packages: backend/src/server/index.ts serves the
# frontend's static bundle itself (resolve(__dirname, '..', 'web') — see
# backend/src/server/index.ts) whenever out/web/ sits next to out/server/.
# So backend/ and frontend/ build as two INDEPENDENT stages (own
# package.json, own deps — mirrors deploy/dev/docker/backend/Dockerfile)
# then combine into one runtime image at the end.
#
# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: backend builder — builds ONLY backend/ (its own package.json/deps)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-alpine AS backend-builder

WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
RUN apk add --no-cache python3 make g++ git

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY desktop/config/patches ./config/patches
COPY backend/package.json ./backend/package.json

RUN npm pkg delete scripts.postinstall scripts.prepare
# --ignore-scripts: vite.config.ts marks node-pty/better-sqlite3/cpu-features/etc
# as build.rollupOptions.external (never bundled) — the build stage only needs
# these packages resolvable for imports/types, not their native binary. Actual
# compiled binaries are only required by the runtime stage's `pnpm install --prod`.
RUN pnpm install --frozen-lockfile --filter orca-backend... --ignore-scripts

COPY backend/ ./backend/
WORKDIR /app/backend
RUN pnpm run build
# → /app/backend/out/server/{index,daemon-entry,user-process-entry}.js

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: frontend builder — builds ONLY frontend/ (its own package.json/deps)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY desktop/config/patches ./config/patches
COPY frontend/package.json ./frontend/package.json

RUN npm pkg delete scripts.postinstall scripts.prepare
# --ignore-scripts: frontend/package.json carries the full monorepo dependency
# list (native modules like node-pty/cpu-features included) even though the
# browser bundle never imports them — skip their native-compile install
# scripts entirely rather than installing a build toolchain to satisfy code
# this image never runs.
RUN pnpm install --frozen-lockfile --filter orca-frontend... --ignore-scripts

COPY frontend/ ./frontend/
WORKDIR /app/frontend
RUN pnpm run build
# → /app/frontend/out/web/{web-index,admin-index}.html + assets/

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: runtime — minimal production image
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate

# Runtime system dependencies:
# - python3/make/g++: for native module rebuild (node-pty, better-sqlite3)
# - git: for git operations in workspace manager
# - openssh-client: for SSH tunnel features
# - bash: for shell scripts
RUN apk add --no-cache \
    python3 \
    make \
    g++ \
    git \
    openssh-client \
    bash \
    wget

# backend/'s own package.json + prod deps (express, ws, better-sqlite3, ssh2, ...)
COPY --from=backend-builder /app/backend/package.json ./package.json
COPY --from=backend-builder /app/pnpm-lock.yaml /app/pnpm-workspace.yaml /app/.npmrc ./
COPY --from=backend-builder /app/config/patches ./config/patches
RUN npm pkg delete scripts.postinstall scripts.prepare
RUN pnpm install --frozen-lockfile --prod --ignore-workspace

# Rebuild native modules for the target platform (belt-and-suspenders —
# --prod install above should already have compiled them correctly).
RUN pnpm rebuild node-pty better-sqlite3 2>/dev/null || \
    echo "[Runtime] Native rebuild failed — may cause runtime errors"

# Build artifacts — out/server + out/web as siblings (backend/src/server/index.ts
# resolves the web bundle via resolve(__dirname, '..', 'web')).
COPY --from=backend-builder /app/backend/out/server ./out/server
COPY --from=frontend-builder /app/frontend/out/web ./out/web

COPY deploy/prod/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# ─────────────────────────────────────────────────────────────────────────────
# Environment defaults
# ─────────────────────────────────────────────────────────────────────────────
ENV ORCA_PORT=6768
ENV ORCA_HTTP_PORT=6769
ENV NODE_ENV=production

EXPOSE 6768
EXPOSE 6769

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
    CMD wget -qO- "http://localhost:${ORCA_HTTP_PORT:-6769}/" > /dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "out/server/index.js"]
