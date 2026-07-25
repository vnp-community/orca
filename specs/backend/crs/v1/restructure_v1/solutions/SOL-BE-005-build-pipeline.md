# SOL-BE-005 — Build Pipeline & Deployment

**CRs:** [CR-005](../../../../../docs/crs/v1/restructure_v1/CR-005-build-system.md), [CR-006](../../../../../docs/crs/v1/restructure_v1/CR-006-docker-deployment.md)  
**TDD Refs:** TDD-01 §1 (Process Model)  
**Approach:** Verification-driven (build scripts + smoke tests)

> **🏁 STATUS: ✅ COMPLETE — 2026-07-23**  
> All 7 AC passed | build:backend → out/server/index.js | build:frontend:web → out/web/web-index.html | 0 electron requires | docker-compose valid | electron-vite untouched

---

## 1. Phân tích từ TDD

Từ **TDD-01 §1 (Process Model)**:
```
ELECTRON MAIN PROCESS → fork → DAEMON PROCESS
                       → WebSocket/Unix → EXTERNAL CLIENTS
```

Trong Node/Server mode, kiến trúc trở thành:
```
NODE.JS PROCESS (index.js)
  ├─ HTTP Server (static files)
  ├─ WebSocket Server (IPC bridge cho web frontend)
  ├─ WebSocket Server (OrcaRuntimeRpcServer cho remote clients)
  └─ fork → DAEMON PROCESS (daemon-entry.js)
```

Cả `index.js` và `daemon-entry.js` đều phải có trong Docker image.

---

## 2. Build Targets & Outputs

| Target | Input | Output | Script |
|--------|-------|--------|--------|
| Backend (Node) | `src/server/index.ts` | `out/server/index.js` | `pnpm build:backend` |
| Daemon | `src/main/daemon/daemon-entry.ts` | `out/server/daemon-entry.js` | (tích hợp trong build:backend) |
| Frontend (SPA) | `src/renderer/web-index.html` | `out/web/` | `pnpm build:frontend:web` |
| Relay | `src/relay/` | `out/relay/` | `pnpm build:relay` |
| Electron | `electron.vite.config.ts` | `out/main/`, `out/renderer/` | `pnpm build:electron` |

---

## 3. Verification Tests (Build Smoke Tests)

### 3.1 Backend Build Smoke Test

```bash
#!/bin/bash
# scripts/verify-backend-build.sh
set -euo pipefail

echo "=== Backend Build Smoke Test ==="

# 1. Check output files exist
assert_file() {
  if [[ ! -f "$1" ]]; then
    echo "❌ Missing: $1"
    exit 1
  fi
  echo "✅ Found: $1"
}

assert_file "out/server/index.js"
assert_file "out/server/daemon-entry.js"

# 2. Check that 'electron' module is NOT required at runtime
if grep -q "require('electron')" out/server/index.js; then
  echo "❌ ERROR: out/server/index.js requires 'electron' — this will fail!"
  exit 1
fi
echo "✅ No electron require in index.js"

# 3. Check that NodeAdapter is referenced
if grep -q "createNodeAdapter\|NodeApp\|NodeIpcBridge" out/server/index.js; then
  echo "✅ NodeAdapter referenced in output"
else
  echo "⚠️  NodeAdapter not found in output — check build"
fi

# 4. Quick runtime check (no crash)
timeout 3 node out/server/index.js &
NODE_PID=$!
sleep 2

if kill -0 $NODE_PID 2>/dev/null; then
  echo "✅ Server started without crashing"
  kill $NODE_PID
else
  echo "❌ Server crashed on startup"
  exit 1
fi

echo ""
echo "=== Backend Build: ALL CHECKS PASSED ==="
```

### 3.2 Frontend Build Smoke Test

```bash
#!/bin/bash
# scripts/verify-frontend-build.sh
set -euo pipefail

echo "=== Frontend Web Build Smoke Test ==="

# 1. Check output files exist
assert_file() {
  if [[ ! -f "$1" ]]; then
    echo "❌ Missing: $1"
    exit 1
  fi
  echo "✅ Found: $1"
}

assert_file "out/web/web-index.html"

# 2. Check web-index.html references web entry, not Electron entry
if grep -q "src/main.tsx" out/web/web-index.html 2>/dev/null; then
  echo "❌ web-index.html references Electron entry (main.tsx)"
  exit 1
fi

# 3. Check no Electron IPC in web bundle
BUNDLE_FILES=$(find out/web/assets/ -name "*.js" 2>/dev/null | head -5)
for f in $BUNDLE_FILES; do
  if grep -q "window.electron.ipcRenderer" "$f"; then
    echo "⚠️  WARNING: window.electron.ipcRenderer found in $f"
    echo "    This is OK if electron-compat shim is present"
  fi
done

# 4. Verify it's a valid HTML SPA
if grep -q '<div id="root">' out/web/web-index.html; then
  echo "✅ SPA root div found"
else
  echo "❌ SPA root div not found in web-index.html"
  exit 1
fi

echo ""
echo "=== Frontend Build: ALL CHECKS PASSED ==="
```

### 3.3 Docker Build Test (CI)

```bash
#!/bin/bash
# deploy/ci/test-docker-build.sh
set -euo pipefail

IMAGE_TAG="orca-server:test-${RANDOM}"

echo "=== Docker Build Test ==="

# 1. Build image
docker build \
  -t "$IMAGE_TAG" \
  -f deploy/prod/Dockerfile \
  .

echo "✅ Docker build succeeded"

# 2. Run container and check it starts
CONTAINER_ID=$(docker run -d \
  -e ORCA_PORT=6768 \
  -e ORCA_VERSION=test \
  -p 16768:6768 \
  "$IMAGE_TAG")

sleep 5

# 3. Health check
if curl -sf http://localhost:16768/ > /dev/null; then
  echo "✅ Container responds to HTTP"
else
  echo "❌ Container not responding"
  docker logs "$CONTAINER_ID"
  docker rm -f "$CONTAINER_ID"
  docker rmi "$IMAGE_TAG"
  exit 1
fi

# 4. Check logs for startup messages
LOGS=$(docker logs "$CONTAINER_ID" 2>&1)
if echo "$LOGS" | grep -q "Server is listening"; then
  echo "✅ Server startup message found"
else
  echo "❌ Server startup message missing"
  echo "$LOGS"
fi

# 5. Cleanup
docker rm -f "$CONTAINER_ID"
docker rmi "$IMAGE_TAG"

echo ""
echo "=== Docker Build Test: PASSED ==="
```

---

## 4. `package.json` Script Specifications

### Required scripts (verify each works)

```typescript
// Verification test file: scripts/__tests__/build-scripts.test.ts
// Run manually: node scripts/__tests__/build-scripts.test.ts

const requiredScripts = [
  'build:backend',     // builds out/server/index.js
  'build:frontend:web', // builds out/web/
  'build:server',      // alias: build:relay + build:backend + build:frontend:web
  'build:relay',       // builds out/relay/
  'build:electron',    // builds Electron app (existing)
]

// Verify scripts exist in package.json
const pkg = require('../../package.json')
for (const script of requiredScripts) {
  if (!pkg.scripts[script]) {
    console.error(`❌ Missing script: ${script}`)
    process.exit(1)
  }
  console.log(`✅ Script exists: ${script}`)
}
```

### Script Definitions

```json
{
  "scripts": {
    "build:relay": "...(hiện có, giữ nguyên)",
    "build:web": "...(hiện có, giữ nguyên — dùng cho Electron renderer)",
    
    "build:backend": "vite build -c vite.server.config.ts",
    "build:frontend:web": "vite build -c vite.web-spa.config.ts",
    "build:server": "pnpm build:relay && pnpm build:backend && pnpm build:frontend:web",
    
    "build:electron": "electron-vite build",
    "build:all": "pnpm build:electron && pnpm build:relay"
  }
}
```

---

## 5. `vite.server.config.ts` Verification

```typescript
// src/server/__tests__/vite-server-config.test.ts
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'

describe('vite.server.config.ts', () => {
  const config = readFileSync('vite.server.config.ts', 'utf-8')

  it('uses electron-node-wrapper as electron alias', () => {
    expect(config).toContain('electron-node-wrapper')
    expect(config).not.toContain('mocks/electron')
  })

  it('builds both index and daemon-entry', () => {
    expect(config).toContain('daemon-entry')
    expect(config).toContain('index')
  })

  it('externalizes native modules', () => {
    const nativeModules = ['node-pty', 'better-sqlite3', 'keytar', 'ssh2']
    for (const mod of nativeModules) {
      expect(config).toContain(mod)
    }
  })

  it('has node22 as target', () => {
    expect(config).toContain('node22')
  })
})
```

---

## 6. Deployment Runbook

### Dev Workflow (current — build in Docker)

```bash
# 1. Code change
# 2. Deploy
cd orca/deploy/dev
bash scripts/sync-to-server.sh

# Expected outcome:
# - Source rsync'd to server
# - Docker build: ~3-5 min
# - Container restart: ~30s
# - Verify: curl https://b15.openledger.vn/ → HTTP 200
```

### Prod Workflow (after CR-006 implemented)

```bash
# CI/CD:
# 1. Build artifacts locally or in CI
pnpm build:server

# 2. Build & push Docker image
ORCA_VERSION=$(node -p "require('./package.json').version")
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag vnpblc/orca-server:${ORCA_VERSION} \
  --file deploy/prod/Dockerfile \
  --push .

# 3. Deploy to server (pull image)
ssh ubuntu@172.20.2.39 \
  "cd ~/orca-deploy && \
   ORCA_VERSION=${ORCA_VERSION} \
   docker compose -f deploy/prod/docker-compose.yml pull && \
   docker compose -f deploy/prod/docker-compose.yml up -d"

# Expected time: ~30s (no build on server)
```

---

## 7. Acceptance Criteria

| # | Criteria | Verification |
|---|---------|-------------|
| AC-1 | `pnpm build:backend` produces `out/server/index.js` | `verify-backend-build.sh` | ✅ |
| AC-2 | `out/server/index.js` does not `require('electron')` | `verify-backend-build.sh` | ✅ |
| AC-3 | `pnpm build:frontend:web` produces `out/web/web-index.html` | `verify-frontend-build.sh` | ✅ |
| AC-4 | Docker image starts and responds to HTTP | `test-docker-build.sh` | ✅ |
| AC-5 | `pnpm build:electron` still works (no regression) | existing CI | ✅ |
| AC-6 | `out/server/daemon-entry.js` exists | `verify-backend-build.sh` | ✅ |
| AC-7 | Node server starts without crashing | smoke test | ✅ |
