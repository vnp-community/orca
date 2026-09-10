#!/usr/bin/env node
/**
 * build.mjs — Build the Orca Mobile Emulator Agent as a standalone package.
 * Adapted from agent/build.mjs — same esbuild options, paths rebased to this
 * package's own root. emulator/ is intentionally self-contained: no git/fs/pty
 * code, so no node-pty/better-sqlite3 externals are needed (see
 * specs/emulator/tdd/v1/01-architecture.md).
 *
 * Output:  out/emulator.js  (standalone Node.js CJS bundle)
 * Runtime: Node.js 22+ on any host with Android SDK and/or Xcode installed
 */

import { build } from 'esbuild'
import { existsSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'

const __dirname = import.meta.dirname
const ROOT = __dirname
const EMULATOR_ENTRY = join(ROOT, 'src', 'relay', 'emulator-entry.ts')
const EMULATOR_OUT = join(ROOT, 'out', 'emulator.js')
const EMULATOR_VERSION = '0.1.0'

const t0 = Date.now()

if (!existsSync(EMULATOR_ENTRY)) {
  console.error(`ERROR: Entry point not found: ${EMULATOR_ENTRY}`)
  process.exit(1)
}

const watchMode = process.argv.includes('--watch')

console.log('Building Orca Mobile Emulator Agent (standalone package)...')
console.log(`  Entry:  ${EMULATOR_ENTRY}`)
console.log(`  Output: ${EMULATOR_OUT}`)

mkdirSync(join(ROOT, 'out'), { recursive: true })

const buildOptions = {
  entryPoints: [EMULATOR_ENTRY],
  outfile: EMULATOR_OUT,
  bundle: true,
  platform: 'node',
  target: 'node22',
  format: 'cjs',
  external: [],
  sourcemap: false,
  minify: false,
  define: {
    'process.env.NODE_ENV': '"production"',
    'process.env.ORCA_EMULATOR_AGENT_VERSION': JSON.stringify(EMULATOR_VERSION)
  },
  logLevel: 'info'
}

if (watchMode) {
  const { context } = await import('esbuild')
  const ctx = await context(buildOptions)
  await ctx.watch()
  console.log('Watching for changes...')
} else {
  await build(buildOptions)
  console.log(`Build complete in ${Date.now() - t0}ms`)
}
