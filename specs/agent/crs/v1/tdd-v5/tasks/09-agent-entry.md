# TASK-09: Create src/relay/agent-entry.ts

**Phase:** 4  
**SOL Ref:** SOL-08  
**Estimated time:** 30m  
**Precondition:** TASK-08 (connections) hoàn thành  

---

## Tạo file mới: `src/relay/agent-entry.ts`

```typescript
// src/relay/agent-entry.ts
// Orca Dev Agent v2.1 — entry point.
// Replaces deploy/dev/agent/agent.js.
// Built by: pnpm run build:agent → out/relay/agent.js
// Deploy: scp out/relay/agent.js user@devserver:/home/ubuntu/orca-agent/agent.js

import { loadAgentConfig } from './agent-config'
import { createAgentLogger } from './agent-logger'
import { discoverTools } from './agent-tool-registry'
import { connectDirect } from './agent-connection-direct'
import { listenRelay } from './agent-connection-relay'

async function main(): Promise<void> {
  const config = loadAgentConfig()
  const log = createAgentLogger(config.logLevel)

  log.info('Orca Dev Agent v2.1.0')
  log.info(`Mode: ${config.mode}  DevServerId: ${config.devServerId}  WorkDir: ${config.workDir}`)
  log.info('Discovering tools...')

  const tools = await discoverTools(config)
  log.info(`Tools ready: ${tools.length} (${tools.map(t => t.name).join(', ')})`)

  process.on('SIGINT',  () => { log.info('Shutting down (SIGINT)');  process.exit(0) })
  process.on('SIGTERM', () => { log.info('Shutting down (SIGTERM)'); process.exit(0) })

  if (config.mode === 'relay-websocket') {
    await listenRelay(config, tools, log)
  } else {
    await connectDirect(config, tools, log)
  }
}

main().catch((err: unknown) => {
  const msg = err instanceof Error ? err.message : String(err)
  console.error(`[agent:FATAL] ${msg}`)
  process.exit(1)
})
```

---

## Build + Smoke test

```bash
# Build agent
pnpm run build:agent
# Expected: "Built agent → out/relay/agent.js"

# Kiểm tra file tồn tại
ls -lh out/relay/agent.js
cat out/relay/.agent-version

# Smoke test relay mode (không cần kết nối thực)
MODE=relay-websocket AGENT_PORT=16799 AGENT_TOKEN=test-tok DEV_SERVER_ID=smoke-test \
  timeout 3 node out/relay/agent.js 2>&1 || true
# Expected output (trong 3s):
# [agent] ... INFO  Orca Dev Agent v2.1.0
# [agent] ... INFO  Mode: relay-websocket  DevServerId: smoke-test ...
# [agent] ... INFO  Tools ready: N (...)
# [agent] ... INFO  ✅ Relay server ready: ws://0.0.0.0:16799/orca-relay
```

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-entry" || echo "No errors"
pnpm run build:agent
```

## Definition of Done

- [x] `src/relay/agent-entry.ts` created
- [x] `pnpm run typecheck:node` passes
- [x] `pnpm run build:agent` succeeds → `out/relay/agent.js` + `out/relay/.agent-version`
- [x] Smoke test outputs expected startup messages (relay-websocket mode)
