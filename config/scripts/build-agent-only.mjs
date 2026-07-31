#!/usr/bin/env node
/**
 * build-agent-only.mjs — Build ONLY the Orca Dev Agent (no Electron, no relay)
 *
 * Output:  out/relay/agent.js  (~200KB, standalone Node.js CJS bundle)
 * Runtime: Node.js 22+ on any Linux/macOS/Windows dev server
 * Deploy:  scp out/relay/agent.js user@devserver:~/orca-agent/agent.js
 *
 * Why this exists:
 *   build-relay.mjs builds everything (relay per 6 platforms + WSL + agent) and
 *   is slow. This script builds ONLY agent.js in ~3 seconds — no Electron, no
 *   cross-platform relay bundles needed for agent-only deploys.
 *
 * Usage:
 *   node config/scripts/build-agent-only.mjs
 *   node config/scripts/build-agent-only.mjs --watch   # watch mode for dev
 */

import { build } from 'esbuild'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const __dirname = import.meta.dirname
const ROOT       = join(__dirname, '..', '..')
const AGENT_ENTRY = join(ROOT, 'src', 'relay', 'agent-entry.ts')
const AGENT_OUT   = join(ROOT, 'out', 'relay', 'agent.js')
const VERSION_OUT = join(ROOT, 'out', 'relay', '.agent-version')
const AGENT_VERSION = '2.1.0'

const t0 = Date.now()

if (!existsSync(AGENT_ENTRY)) {
  console.error(`ERROR: Entry point not found: ${AGENT_ENTRY}`)
  process.exit(1)
}

const watchMode = process.argv.includes('--watch')

console.log('Building Orca Dev Agent (Node.js only, no Electron)...')
console.log(`  Entry:  ${AGENT_ENTRY}`)
console.log(`  Output: ${AGENT_OUT}`)
console.log('')

mkdirSync(join(ROOT, 'out', 'relay'), { recursive: true })

const buildOptions = {
  entryPoints: [AGENT_ENTRY],
  outfile: AGENT_OUT,
  bundle: true,
  platform: 'node',
  target: 'node22',   // matches deploy servers (Node 22 LTS)
  format: 'cjs',

  // ── External — NOT bundled (not available/needed on dev servers) ─────────
  external: [
    'node-pty',           // agent has no PTY
    'better-sqlite3',     // agent has no SQLite
    'keytar',             // agent has no keychain  
    '@parcel/watcher',    // agent has no fs watcher
    'electron',           // ← agent is NOT an Electron process
    // Note: 'ws' is intentionally NOT external — bundled for WebSocket support
  ],

  sourcemap: false,
  minify: false,        // Keep readable: devs debug agent on remote server via journald

  define: {
    'process.env.NODE_ENV': '"production"',
  },

  // Log level
  logLevel: 'info',
}

if (watchMode) {
  // Watch mode: rebuild on file change
  const ctx = await build({ ...buildOptions, logLevel: 'debug' })
  await ctx.watch?.()
  console.log('Watching for changes (Ctrl+C to stop)...')
} else {
  // One-shot build
  await build(buildOptions)

  // Write version file with content hash for deploy change detection
  const content = readFileSync(AGENT_OUT)
  const hash    = createHash('sha256').update(content).digest('hex').slice(0, 12)
  const version = `${AGENT_VERSION}+${hash}`
  writeFileSync(VERSION_OUT, version)

  const elapsed = ((Date.now() - t0) / 1000).toFixed(1)
  const sizeKB  = (content.length / 1024).toFixed(0)

  console.log('')
  console.log('══════════════════════════════════════════')
  console.log(` ✅ Agent built in ${elapsed}s`)
  console.log(`    Size:    ${sizeKB} KB`)
  console.log(`    Version: ${version}`)
  console.log(`    Output:  out/relay/agent.js`)
  console.log('══════════════════════════════════════════')
  console.log('')
  console.log('Deploy to dev server:')
  console.log('  bash deploy/dev/scripts/deploy-agents.sh')
  console.log('')
}
