# Multi-stage build for Orca Server production image
#
# Build:   docker build -f deploy/prod/Dockerfile -t vnpblc/orca-server:latest .
# Run:     docker run -d -p 6768:6768 -p 6769:6769 vnpblc/orca-server:latest
#
# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: builder — compile TypeScript + build web bundle
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Install native build tools needed for pnpm install (node-pty, better-sqlite3)
RUN apk add --no-cache python3 make g++ git

# Copy package manifests first (layer cache)
COPY package.json pnpm-lock.yaml ./

# Install ALL dependencies (including devDeps needed for build)
RUN pnpm install --frozen-lockfile

# Copy all source
COPY . .

# Build: relay + backend + web SPA
# (build:relay may be skipped in minimal builds — || true is intentional)
RUN pnpm run build:relay 2>/dev/null || echo "[Build] Relay build skipped"
RUN pnpm run build:backend
RUN pnpm run build:frontend:web

# Verify outputs
RUN ls -la out/server/ && ls -la out/web/ || true

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: runtime — minimal production image
# ─────────────────────────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

WORKDIR /app

# Install pnpm for production dependency installation
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

# Copy package manifests
COPY package.json pnpm-lock.yaml ./

# Install PRODUCTION dependencies only (no devDeps)
RUN pnpm install --frozen-lockfile --prod

# Rebuild native modules for Linux/Alpine
# node-pty and better-sqlite3 must be compiled for the target platform
RUN pnpm rebuild node-pty better-sqlite3 2>/dev/null || \
    echo "[Runtime] Native rebuild failed — may cause runtime errors"

# Copy built outputs from builder stage
COPY --from=builder /app/out/server ./out/server
COPY --from=builder /app/out/web ./out/web

# Copy relay binaries if they were built
COPY --from=builder /app/out/relay ./out/relay 2>/dev/null || true

# Copy entrypoint
COPY deploy/prod/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# ─────────────────────────────────────────────────────────────────────────────
# Environment defaults
# ─────────────────────────────────────────────────────────────────────────────
ENV ORCA_PORT=6768
ENV ORCA_HTTP_PORT=6769
ENV NODE_ENV=production

# Expose ports
# 6768: WebSocket/RPC endpoint (for Orca frontend clients)
# 6769: HTTP endpoint (web UI static files)
EXPOSE 6768
EXPOSE 6769

# Health check — verify HTTP server is responding
HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
    CMD wget -qO- "http://localhost:${ORCA_HTTP_PORT:-6769}/" > /dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "out/server/index.js"]
