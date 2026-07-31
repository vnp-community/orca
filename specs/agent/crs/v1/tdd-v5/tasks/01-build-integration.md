# TASK-01: Extend build-relay.mjs — Add Agent Build Entry

**Phase:** 1  
**SOL Ref:** SOL-01  
**Estimated time:** 1h  
**Precondition:** `src/relay/agent-entry.ts` file KHÔNG cần tồn tại trước — esbuild sẽ fail nếu file chưa có, nhưng script extend là an toàn để làm trước  

---

## Files cần thay đổi

### 1. `config/scripts/build-relay.mjs` — EXTEND (không rewrite)

Thêm đoạn sau vào **cuối file**, sau block `console.log('Relay build complete.')`:

```javascript
// === AGENT BUILD ===
// src/relay/agent-entry.ts → out/relay/agent.js
// Single platform-independent bundle (not per-platform like relay.js)
// Why: admin deploys out/relay/agent.js directly to dev server

import { existsSync } from 'node:fs'

const AGENT_ENTRY = join(ROOT, 'src', 'relay', 'agent-entry.ts')

if (existsSync(AGENT_ENTRY)) {
  await build({
    entryPoints: [AGENT_ENTRY],
    outfile: join(ROOT, 'out', 'relay', 'agent.js'),
    bundle: true,
    platform: 'node',
    target: 'node22',
    format: 'cjs',
    external: [
      'node-pty',
      'better-sqlite3',
      'keytar',
      '@parcel/watcher',
      'electron',
    ],
    sourcemap: false,
    minify: false,
    define: {
      'process.env.NODE_ENV': '"production"',
    },
  })

  const agentContent = readFileSync(join(ROOT, 'out', 'relay', 'agent.js'))
  const agentHash = createHash('sha256').update(agentContent).digest('hex').slice(0, 12)
  writeFileSync(join(ROOT, 'out', 'relay', '.agent-version'), `2.1.0+${agentHash}`)
  console.log(`Built agent → out/relay/agent.js`)
} else {
  console.log('Skipping agent build: src/relay/agent-entry.ts not found yet')
}
```

### 2. `package.json` — EXTEND scripts (không xóa existing scripts)

Tìm dòng `"build:relay"` trong `package.json`, **thêm dòng mới ngay sau**:

```json
"build:agent": "node config/scripts/build-relay.mjs",
```

---

## Verification

```bash
# Kiểm tra script parse được (không có syntax error)
node --check config/scripts/build-relay.mjs

# Chạy build (sẽ skip agent vì agent-entry.ts chưa có)
pnpm run build:relay
# Expected output cuối: "Skipping agent build: src/relay/agent-entry.ts not found yet"

# Kiểm tra package.json
grep "build:agent" package.json
```

## Definition of Done

- [x] `config/scripts/build-relay.mjs` đã có agent build block ở cuối
- [x] `node --check config/scripts/build-relay.mjs` không lỗi
- [x] `package.json` có `"build:agent"` script
- [x] Existing relay build không bị ảnh hưởng: `pnpm run build:relay` vẫn build relay.js bình thường
