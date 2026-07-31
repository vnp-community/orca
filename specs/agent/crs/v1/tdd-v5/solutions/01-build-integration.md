# SOL-01: Build Integration

**TDD Ref:** TDD-AG-01, TDD-AG-08  
**Files:** `config/scripts/build-relay.mjs` (EXTEND), `package.json` (EXTEND)  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 1h

---

## Vấn đề

Agent code cần được build từ `src/relay/agent-entry.ts` → `out/relay/agent.js`.  
`build-relay.mjs` hiện tại chỉ build:
- `relay.ts` → `out/relay/{platform}/relay.js`
- `parcel-watcher-process-entry.ts` → `out/relay/{platform}/relay-watcher.js`
- `wsl-agent-hook-relay.ts` → `out/relay/wsl/wsl-agent-hook-relay.js`

---

## Giải pháp

### 1. Extend build-relay.mjs

Thêm agent build vào cuối `config/scripts/build-relay.mjs`:

```javascript
// === AGENT BUILD (thêm vào cuối build-relay.mjs) ===

const AGENT_ENTRY = join(ROOT, 'src', 'relay', 'agent-entry.ts')
const AGENT_OUT   = join(ROOT, 'out', 'relay', 'agent.js')

// Parse --agent-only flag for CI incremental builds
const agentOnly = process.argv.includes('--agent-only')

if (!agentOnly) {
  // ... existing relay + wsl builds unchanged ...
}

// Agent: single platform-independent bundle (always)
// Why: agent runs on remote dev server (Linux/Mac/Win), not bundled per-platform.
// Admin deploys out/relay/agent.js → /home/ubuntu/orca-agent/agent.js
mkdirSync(join(ROOT, 'out', 'relay'), { recursive: true })

await build({
  entryPoints: [AGENT_ENTRY],
  outfile: AGENT_OUT,
  bundle: true,
  platform: 'node',
  target: 'node22',   // matches vite.server.config.ts target
  format: 'cjs',
  external: [
    'node-pty',         // agent không dùng PTY
    'better-sqlite3',   // agent không dùng SQLite
    'keytar',           // agent không dùng keytar
    '@parcel/watcher',  // agent không dùng parcel watcher
    'electron',         // agent không chạy trong Electron
  ],
  // ws được bundle (needed, not external)
  sourcemap: false,
  minify: false,        // Keep readable: admin debug on dev server via logs
  define: {
    'process.env.NODE_ENV': '"production"',
  },
})

// Version file for deploy check
const agentContent = readFileSync(AGENT_OUT)
const agentHash = createHash('sha256').update(agentContent).digest('hex').slice(0, 12)
writeFileSync(join(ROOT, 'out', 'relay', '.agent-version'), `2.1.0+${agentHash}`)

console.log(`Built agent → ${AGENT_OUT}`)
```

### 2. package.json — Thêm build:agent script

```json
// package.json (EXTEND scripts section):
{
  "scripts": {
    "build:relay": "node config/scripts/build-relay.mjs",
    "build:agent": "node config/scripts/build-relay.mjs --agent-only"
  }
}
```

---

## Verification

```bash
# Build
pnpm run build:agent

# Check output
ls -lh out/relay/agent.js     # should exist, ~200KB
cat out/relay/.agent-version  # should print 2.1.0+<hash>

# Smoke test (xem log output)
node out/relay/agent.js --help 2>&1 | head -5
# Expected: [agent] ... Orca Dev Agent v2.1.0
```

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| `electron` import leaks from `src/main/` transitive deps | Đảm bảo `agent-entry.ts` chỉ import từ `src/relay/` và `src/shared/` |
| `better-sqlite3` import leaks | Mark as external; agent không cần SQLite |
| `ws` not bundled | `ws` không có trong `external` list → sẽ được bundle |
| Build fails on Windows (path sep) | Dùng `node:path join()` nhất quán — không hardcode `/` |

---

## Definition of Done

- [x] `config/scripts/build-relay.mjs` extended with agent build block
- [x] `package.json` has `build:agent` script
- [x] `pnpm run build:agent` succeeds → `out/relay/agent.js` created
- [x] `out/relay/.agent-version` file written with `2.1.0+<hash>` format
- [x] Agent bundle is platform-independent (no electron/sqlite/keytar/parcel deps)
- [x] `--agent-only` flag supported for CI incremental builds
