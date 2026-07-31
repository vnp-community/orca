# TDD-AG-08: Deployment (v2.1 — esbuild + src/)

**Document:** TDD-AG-08
**Version:** 2.1
**Date:** 2026-07-28
**Domain:** Production deployment — esbuild bundle, src/relay/, systemd
**Source files:**
- `config/scripts/build-relay.mjs` ← EXTEND
- `src/relay/agent-entry.ts` ← entry point
- `out/relay/agent.js` ← build output
**HLD Ref:** C3.8
**ADR:** ADR-004

---

## 1. Build Process (v2.1)

```bash
# Build agent (+ relay daemon) từ workspace root:
pnpm run build:relay

# Agent-only build (new script):
pnpm run build:agent

# Output:
# out/relay/agent.js      ← agent binary (esbuild CJS bundle)
# out/relay/relay.js      ← relay daemon (existing)
```

---

## 2. build-relay.mjs — Agent Build Config

```javascript
// config/scripts/build-relay.mjs (EXTEND thêm agent entry)

const AGENT_ENTRY = join(ROOT, 'src', 'relay', 'agent-entry.ts')

// Add to existing build function:
await build({
  entryPoints: [AGENT_ENTRY],
  outfile: join(ROOT, 'out', 'relay', 'agent.js'),
  bundle: true,
  platform: 'node',
  target: 'node22',
  format: 'cjs',
  sourcemap: false,   // Production: no sourcemap (readable code preferred)
  minify: false,      // Keep readable for debugging on dev server
  external: [
    'node-pty',       // Agent không dùng node-pty (không có PTY)
    'better-sqlite3', // Agent không cần SQLite
    'keytar',         // Agent không cần keytar
  ],
  // Note: 'ws' IS bundled (needed for WebSocket)
})
```

---

## 3. package.json Scripts

```json
{
  "scripts": {
    "build:relay":  "node config/scripts/build-relay.mjs",
    "build:agent":  "node config/scripts/build-relay.mjs --agent-only"
  }
}
```

---

## 4. Deploy to Dev Server

```bash
# From workspace root:
pnpm run build:agent

# Copy to dev server:
scp out/relay/agent.js ubuntu@devserver:/home/ubuntu/orca-agent/agent.js

# On dev server — same as before:
node /home/ubuntu/orca-agent/agent.js
```

---

## 5. systemd Service (Unchanged)

`deploy/dev/agent/orca-agent.service` không thay đổi — vẫn chạy `node agent.js`, chỉ khác là `agent.js` bây giờ là esbuild bundle thay vì raw JS.

---

## 6. Development Workflow

```bash
# Type check agent code (dùng chung tsconfig.node.json):
pnpm run typecheck:node

# Run agent tests (dùng chung vitest):
pnpm test -- src/relay/agent-*.test.ts

# Or watch mode:
pnpm exec vitest --watch src/relay/

# Local test run (no build needed):
ts-node src/relay/agent-entry.ts
# Or via tsx (faster):
npx tsx src/relay/agent-entry.ts
```

---

## 7. Migration Summary

| Cũ (v1) | Mới (v2.1) |
|---------|-----------|
| `deploy/dev/agent/package.json` | Không còn — dùng root `package.json` |
| `deploy/dev/agent/node_modules/` | Không còn — dùng root `node_modules/` |
| `node agent.js` (raw file) | `node out/relay/agent.js` (bundled) |
| Deploy: copy `agent.js` (raw) | Deploy: copy `out/relay/agent.js` (built) |
| `.env` file | `.env` hoặc env vars từ `start.sh` (unchanged) |
| `deploy/dev/agent/*.ts` | `src/relay/agent-*.ts` |
