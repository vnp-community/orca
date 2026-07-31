# SOL-08: agent-entry.ts — Entry Point

**TDD Ref:** TDD-AG-01  
**File:** `src/relay/agent-entry.ts` [NEW]  
**Mức độ:** 🟢 Đơn giản  
**Thời gian ước tính:** 30m

---

## Full Implementation

```typescript
// src/relay/agent-entry.ts
// Agent process entry point — replaces deploy/dev/agent/agent.js main()

import { loadAgentConfig } from './agent-config'
import { createAgentLogger } from './agent-logger'
import { discoverTools } from './agent-tool-registry'
import { connectDirect } from './agent-connection-direct'
import { listenRelay } from './agent-connection-relay'

async function main(): Promise<void> {
  const config = loadAgentConfig()
  const log = createAgentLogger(config.logLevel)

  log.info('Orca Dev Agent v2.1.0')
  log.info(`Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`)
  log.info('Discovering tools...')

  const tools = await discoverTools(config)

  // Log available tools
  const toolNames = tools.map(t => t.name)
  log.info(`Tool discovery complete: ${tools.length} tools available`)
  if (config.logLevel === 'debug') {
    toolNames.forEach(name => log.debug(`  ✓ ${name}`))
  }

  // Register clean shutdown handlers
  process.on('SIGINT',  () => { log.info('Shutting down (SIGINT)');  process.exit(0) })
  process.on('SIGTERM', () => { log.info('Shutting down (SIGTERM)'); process.exit(0) })

  // Start connection (never returns unless process.exit() called)
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

## Verification

```bash
# Build
pnpm run build:agent

# Smoke test (requires ORCA_URL + AGENT_TOKEN or relay env)
MODE=relay-websocket AGENT_PORT=16799 AGENT_TOKEN=test node out/relay/agent.js &
# Expected output:
# [agent] ... Orca Dev Agent v2.1.0
# [agent] ... Mode: relay-websocket | DevServerId: dev-local | ...
# [agent] ... Discovering tools...
# [agent] ... Tool discovery complete: N tools available
# [agent] ... ✅ Relay server ready: ws://0.0.0.0:16799/orca-relay
```

---

## Definition of Done

- [x] `src/relay/agent-entry.ts` created
- [x] `tsc` passes (no type errors)
- [x] `pnpm run build:agent` succeeds → `out/relay/agent.js` created
- [x] Smoke test: `node out/relay/agent.js` starts and logs correctly
- [x] `out/relay/.agent-version` file written
